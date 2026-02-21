package receiver

import (
    "bytes"
    "encoding/gob"
    "fmt"
    "log"
    "net"
    "os"
    "path/filepath"

    "github.com/atienze/HomelabSecureSync/common/protocol"
    "github.com/atienze/HomelabSecureSync/server/internal/db"
    // Note: we no longer import crypto here because we replaced the
    // filesystem hash check with a database lookup
)

// VaultDataPath is the root folder where uploaded files are stored on disk.
// We define it as a constant so it's easy to change in one place.
/*

"<homelab-path>/VaultData"

*/
const VaultDataPath = "./uploads"

// HandleConnection now takes a *db.DB as a second argument.
// The * means it's a pointer — both main() and this function point at the SAME database.
func HandleConnection(conn net.Conn, database *db.DB) {
    defer conn.Close()
    log.Printf("New connection from: %s", conn.RemoteAddr().String())

    rawDecoder := gob.NewDecoder(conn)
    networkEncoder := protocol.NewEncoder(conn)

    // --- Handshake (same as before) ---
    var shake protocol.Handshake
    if err := rawDecoder.Decode(&shake); err != nil {
        log.Printf("Handshake failed: %v", err)
        return
    }
    log.Printf("Client connected: %s", shake.ClientID)

    // Register this device in the database if we haven't seen it before.
    // In Phase 2 we'll make this a proper auth check — for now, just record it.
    if err := database.RegisterDevice(shake.ClientID, shake.ClientID); err != nil {
        log.Printf("Warning: could not register device %s: %v", shake.ClientID, err)
        // We don't return here — a registration warning shouldn't kill the connection
    }

    // --- State variables (same as before) ---
    var currentFile *os.File
    var currentFileSize int64
    var currentFileReceived int64
    var currentPath string
    var currentHash string // NEW: we need to remember the hash to write to DB later

    for {
        var p protocol.Packet
        err := rawDecoder.Decode(&p)
        if err != nil {
            if currentFile != nil {
                currentFile.Close()
            }
            log.Printf("Connection closed from %s: %v", shake.ClientID, err)
            return
        }

        switch p.Cmd {

        case protocol.CmdPing:
            fmt.Println("Received PING")

        // -------------------------------------------------------
        // CmdCheckFile: "Do you need this file?"
        // CHANGED: now checks the DATABASE instead of the filesystem
        // -------------------------------------------------------
        case protocol.CmdCheckFile:
            var req protocol.CheckFileRequest
            gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&req)

            // Ask the database: do we already have this exact file with this exact hash?
            exists, err := database.FileExists(req.RelPath, req.Hash)
            if err != nil {
                // If the DB check fails, log it but tell client we need the file.
                // It's better to receive a duplicate than to silently lose a file.
                log.Printf("DB check error for %s: %v", req.RelPath, err)
                exists = false
            }

            status := protocol.StatusNeed
            if exists {
                status = protocol.StatusSkip
                fmt.Printf("⏭  Skipping %s (in database)\n", req.RelPath)
            }

            // Send the response back to the client (same as before)
            resp := protocol.FileStatusResponse{Status: uint8(status)}
            var buf bytes.Buffer
            gob.NewEncoder(&buf).Encode(resp)
            networkEncoder.Encode(protocol.Packet{
                Cmd:     protocol.CmdFileStatus,
                Payload: buf.Bytes(),
            })

        // -------------------------------------------------------
        // CmdSendFile: "Here comes a new file, this is its metadata"
        // CHANGED: we now store the hash for later use in CmdFileChunk
        // -------------------------------------------------------
        case protocol.CmdSendFile:
            if currentFile != nil {
                currentFile.Close()
            }

            var ft protocol.FileTransfer
            gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&ft)

            // Build the destination path on disk (same as before)
            safePath := filepath.Join(VaultDataPath, ft.RelPath)
            os.MkdirAll(filepath.Dir(safePath), 0755)

            f, err := os.Create(safePath)
            if err != nil {
                log.Printf("Failed to create file %s: %v", ft.RelPath, err)
                continue
            }

            currentFile = f
            currentFileSize = ft.Size
            currentFileReceived = 0
            currentPath = ft.RelPath
            currentHash = ft.Hash // NEW: save the hash so we can write it to the DB when done

            fmt.Printf("📥 Receiving: %s (%d bytes)\n", ft.RelPath, ft.Size)

        // -------------------------------------------------------
        // CmdFileChunk: "Here is a piece of the file"
        // CHANGED: when the file finishes, write a record to the database
        // -------------------------------------------------------
        case protocol.CmdFileChunk:
            if currentFile == nil {
                continue
            }

            n, err := currentFile.Write(p.Payload)
            if err != nil {
                log.Printf("Write error for %s: %v", currentPath, err)
                currentFile.Close()
                currentFile = nil
                continue
            }

            currentFileReceived += int64(n)

            // Progress display (same as before)
            if currentFileSize > 0 {
                percent := int(float64(currentFileReceived) / float64(currentFileSize) * 100)
                if percent%10 == 0 && currentFileReceived%32768 == 0 {
                    fmt.Printf("\r   ... %d%%", percent)
                }
            }

            // --- Is the file complete? ---
            if currentFileReceived >= currentFileSize {
                fmt.Printf("\n✔  Saved to disk: %s\n", currentPath)
                currentFile.Close()
                currentFile = nil

                // NEW: Write the record to the database now that the file is fully received.
                // We do this AFTER the file is on disk, not before — if the server crashes
                // mid-transfer, the DB record won't exist and the client will re-send it next time.
                err := database.UpsertFile(currentPath, currentHash, shake.ClientID, currentFileReceived)
                if err != nil {
                    // Log the error but don't crash — the file is on disk, the DB just missed it.
                    log.Printf("⚠️  Warning: failed to record %s in database: %v", currentPath, err)
                } else {
                    fmt.Printf("🗄  Recorded in database: %s\n", currentPath)
                }

                // Reset state variables
                currentPath = ""
                currentHash = ""
                currentFileSize = 0
                currentFileReceived = 0
            }
        }
    }
}