package receiver

import (
    "bytes"
    "encoding/gob"
    "fmt"
    "log"
    "net"
    "os"
    "path/filepath"

    "github.com/atienze/HomelabSecureSync/common/crypto"
    "github.com/atienze/HomelabSecureSync/common/protocol"
)

func HandleConnection(conn net.Conn) {
    defer conn.Close()
    log.Printf("New connection from: %s", conn.RemoteAddr().String())

    // 1. Single Unified Decoder (Matches Client's Unified Encoder)
    rawDecoder := gob.NewDecoder(conn)
    networkEncoder := protocol.NewEncoder(conn)

    // 2. Handshake
    var shake protocol.Handshake
    if err := rawDecoder.Decode(&shake); err != nil {
        log.Printf("Handshake failed: %v", err)
        return
    }
    log.Printf("Client Authenticated: %s", shake.ClientID)

    // State Variables
    var currentFile *os.File
    var currentFileSize int64
    var currentFileReceived int64
    var currentPath string

    for {
        var p protocol.Packet
        // Read directly from the Gob stream
        err := rawDecoder.Decode(&p)
        if err != nil {
            if currentFile != nil { currentFile.Close() }
            log.Printf("Connection closed: %v", err)
            return
        }

        switch p.Cmd {
        case protocol.CmdPing:
            fmt.Println("Received PING")

        // Negotiation
        case protocol.CmdCheckFile:
            var req protocol.CheckFileRequest
            gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&req)

            status := protocol.StatusNeed
            fullPath := filepath.Join("<homelab-path>/VaultData", req.RelPath)
            
            if _, err := os.Stat(fullPath); err == nil {
                localHash, _ := crypto.CalculateFileHash(fullPath)
                if localHash == req.Hash {
                    status = protocol.StatusSkip
                    fmt.Printf("Skipping %s (Exists)\n", req.RelPath)
                }
            }

            resp := protocol.FileStatusResponse{Status: uint8(status)}
            var buf bytes.Buffer
            gob.NewEncoder(&buf).Encode(resp)
            networkEncoder.Encode(protocol.Packet{Cmd: protocol.CmdFileStatus, Payload: buf.Bytes()})

        // Start File
        case protocol.CmdSendFile:
            if currentFile != nil { currentFile.Close() }

            var ft protocol.FileTransfer
            gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&ft)

            safePath := filepath.Join("<homelab-path>/VaultData", ft.RelPath)
            os.MkdirAll(filepath.Dir(safePath), 0755)

            f, err := os.Create(safePath)
            if err != nil {
                log.Printf("Failed to create file: %v", err)
                continue
            }

            currentFile = f
            currentFileSize = ft.Size
            currentFileReceived = 0
            currentPath = ft.RelPath

            fmt.Printf("📥 Receiving: %s (%d bytes)...\n", ft.RelPath, ft.Size)

        // Chunk (Progress Bar Logic)
        case protocol.CmdFileChunk:
            if currentFile == nil { continue }

            n, err := currentFile.Write(p.Payload)
            if err != nil {
                currentFile.Close()
                currentFile = nil
                continue
            }

            currentFileReceived += int64(n)

            // PRINT PROGRESS (Only every ~5%)
            if currentFileSize > 0 {
                percent := int(float64(currentFileReceived) / float64(currentFileSize) * 100)
                if percent % 10 == 0 && currentFileReceived % 32768 == 0 { // avoid spam
                     fmt.Printf("\r   ... %d%%", percent)
                }
            }

            if currentFileReceived >= currentFileSize {
                fmt.Printf("\n✔ Saved: %s\n", currentPath)
                currentFile.Close()
                currentFile = nil
            }
        }
    }
}