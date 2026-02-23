package receiver

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/atienze/HomelabSecureSync/common/protocol"
	"github.com/atienze/HomelabSecureSync/server/internal/db"
)

// VaultDataPath is the root folder where uploaded files are stored on disk.
// We define it as a constant so it's easy to change in one place.
/*

"<homelab-path>/VaultData"

*/
const VaultDataPath = "./uploads"

// HandleConnection validates the device token, then processes file sync commands.
// conn is closed on return via defer — including on auth failure.
func HandleConnection(conn net.Conn, database *db.DB) {
	defer conn.Close()
	log.Printf("New connection from: %s", conn.RemoteAddr().String())

	rawDecoder := gob.NewDecoder(conn)
	networkEncoder := protocol.NewEncoder(conn)

	// --- Handshake ---
	var shake protocol.Handshake
	if err := rawDecoder.Decode(&shake); err != nil {
		log.Printf("Handshake failed from %s: %v", conn.RemoteAddr(), err)
		return
	}

	// --- Phase 2 Auth: validate token against the devices table ---
	// If the token is not registered, close the connection immediately.
	// We do not reveal whether the token exists or why it was rejected.
	deviceName, ok, err := database.GetDeviceName(shake.Token)
	if err != nil {
		log.Printf("Auth DB error for connection %s: %v", conn.RemoteAddr(), err)
		return
	}
	if !ok {
		log.Printf("Rejected connection from %s: unregistered token", conn.RemoteAddr())
		return
	}
	log.Printf("Authenticated device: %s (from %s)", deviceName, conn.RemoteAddr())

	// --- State variables ---
	var currentFile *os.File
	var currentFileSize int64
	var currentFileReceived int64
	var currentPath string
	var currentHash string

	for {
		var p protocol.Packet
		err := rawDecoder.Decode(&p)
		if err != nil {
			if currentFile != nil {
				currentFile.Close()
			}
			log.Printf("Connection closed from %s (%s): %v", deviceName, conn.RemoteAddr(), err)
			return
		}

		switch p.Cmd {

		case protocol.CmdPing:
			fmt.Println("Received PING")

		// -------------------------------------------------------
		// CmdCheckFile: "Do you need this file?"
		// Checks the database for deduplication — not the filesystem.
		// -------------------------------------------------------
		case protocol.CmdCheckFile:
			var req protocol.CheckFileRequest
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&req)

			// Reject any path that tries to escape the vault root.
			absRoot, _ := filepath.Abs(VaultDataPath)
			cleanedCheck := filepath.Join(absRoot, filepath.Clean("/"+req.RelPath))
			if !strings.HasPrefix(cleanedCheck, absRoot+string(filepath.Separator)) {
				log.Printf("Rejected path traversal in CmdCheckFile from %s: %q", deviceName, req.RelPath)
				continue
			}

			exists, err := database.FileExists(req.RelPath, req.Hash)
			if err != nil {
				// If the DB check fails, tell client we need the file.
				// Better to receive a duplicate than to silently lose data.
				log.Printf("DB check error for %s: %v", req.RelPath, err)
				exists = false
			}

			status := protocol.StatusNeed
			if exists {
				status = protocol.StatusSkip
				fmt.Printf("Skipping %s (in database)\n", req.RelPath)
			}

			resp := protocol.FileStatusResponse{Status: uint8(status)}
			var buf bytes.Buffer
			gob.NewEncoder(&buf).Encode(resp)
			networkEncoder.Encode(protocol.Packet{
				Cmd:     protocol.CmdFileStatus,
				Payload: buf.Bytes(),
			})

		// -------------------------------------------------------
		// CmdSendFile: "Here comes a new file — here's its metadata"
		// -------------------------------------------------------
		case protocol.CmdSendFile:
			if currentFile != nil {
				currentFile.Close()
			}

			var ft protocol.FileTransfer
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&ft)

			// Sanitize rel_path to prevent directory traversal attacks.
			// filepath.Join + Clean collapses any ".." components, then we verify
			// the result is still rooted inside VaultDataPath before touching disk.
			absRoot, _ := filepath.Abs(VaultDataPath)
			safePath := filepath.Join(absRoot, filepath.Clean("/"+ft.RelPath))
			if !strings.HasPrefix(safePath, absRoot+string(filepath.Separator)) {
				log.Printf("Rejected path traversal attempt from %s: %q", deviceName, ft.RelPath)
				continue
			}

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
			currentHash = ft.Hash

			fmt.Printf("Receiving: %s (%d bytes)\n", ft.RelPath, ft.Size)

		// -------------------------------------------------------
		// CmdFileChunk: "Here is a piece of the file"
		// Writes a chunk to disk; upserts DB record when transfer is complete.
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

			if currentFileSize > 0 {
				percent := int(float64(currentFileReceived) / float64(currentFileSize) * 100)
				if percent%10 == 0 && currentFileReceived%32768 == 0 {
					fmt.Printf("\r   ... %d%%", percent)
				}
			}

			if currentFileReceived >= currentFileSize {
				fmt.Printf("\nSaved to disk: %s\n", currentPath)
				currentFile.Close()
				currentFile = nil

				// Write to DB after the file is fully on disk.
				// If the server crashes mid-transfer, the DB record won't exist
				// and the client will re-send the file on next sync.
				err := database.UpsertFile(currentPath, currentHash, deviceName, currentFileReceived)
				if err != nil {
					log.Printf("Warning: failed to record %s in database: %v", currentPath, err)
				} else {
					fmt.Printf("Recorded in database: %s\n", currentPath)
				}

				currentPath = ""
				currentHash = ""
				currentFileSize = 0
				currentFileReceived = 0
			}
		}
	}
}
