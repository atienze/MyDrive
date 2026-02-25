package receiver

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"os"

	"github.com/atienze/HomelabSecureSync/common/protocol"
	"github.com/atienze/HomelabSecureSync/server/internal/db"
	"github.com/atienze/HomelabSecureSync/server/internal/store"
)

// HandleConnection validates the device token, then processes file sync commands.
// conn is closed on return via defer — including on auth failure.
func HandleConnection(conn net.Conn, database *db.DB, objectStore *store.ObjectStore) {
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

	// --- State variables for in-progress file transfers ---
	var currentFile *os.File       // temp file receiving chunks
	var currentHasher hash.Hash    // streaming SHA-256 computed as chunks arrive
	var currentFileSize int64      // declared total size
	var currentFileReceived int64  // bytes received so far
	var currentPath string         // client-side relative path (DB key only)
	var currentHash string         // declared hash from the client

	for {
		var p protocol.Packet
		err := rawDecoder.Decode(&p)
		if err != nil {
			if currentFile != nil {
				tmpPath := currentFile.Name()
				currentFile.Close()
				os.Remove(tmpPath) // clean up incomplete transfer
			}
			log.Printf("Connection closed from %s (%s): %v", deviceName, conn.RemoteAddr(), err)
			return
		}

		switch p.Cmd {

		case protocol.CmdPing:
			fmt.Println("Received PING")

		// -------------------------------------------------------
		// CmdCheckFile: "Do you need this file?"
		// Checks the database for deduplication. Also verifies
		// that the blob actually exists on disk as a safety net.
		// -------------------------------------------------------
		case protocol.CmdCheckFile:
			var req protocol.CheckFileRequest
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&req)

			// Validate the relative path — reject absolute paths or traversal attempts.
			if !store.ValidateRelPath(req.RelPath) {
				log.Printf("Rejected invalid path in CmdCheckFile from %s: %q", deviceName, req.RelPath)
				continue
			}

			exists, err := database.FileExists(req.RelPath, req.Hash)
			if err != nil {
				// If the DB check fails, tell client we need the file.
				// Better to receive a duplicate than to silently lose data.
				log.Printf("DB check error for %s: %v", req.RelPath, err)
				exists = false
			}

			// Safety net: if DB says the file exists but the blob is missing
			// from disk (corruption or manual deletion), request re-send.
			if exists && !objectStore.HasObject(req.Hash) {
				log.Printf("Warning: DB has %s but blob missing for hash %s, requesting re-send",
					req.RelPath, req.Hash[:12])
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
		// Opens a temp file for chunk assembly and initializes the
		// streaming hash.
		// -------------------------------------------------------
		case protocol.CmdSendFile:
			if currentFile != nil {
				tmpPath := currentFile.Name()
				currentFile.Close()
				os.Remove(tmpPath) // clean up previous incomplete transfer
			}

			var ft protocol.FileTransfer
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&ft)

			// Validate the relative path.
			if !store.ValidateRelPath(ft.RelPath) {
				log.Printf("Rejected invalid path in CmdSendFile from %s: %q", deviceName, ft.RelPath)
				continue
			}

			tmpFile, err := objectStore.CreateTempFile()
			if err != nil {
				log.Printf("Failed to create temp file for %s: %v", ft.RelPath, err)
				continue
			}

			currentFile = tmpFile
			currentHasher = sha256.New()
			currentFileSize = ft.Size
			currentFileReceived = 0
			currentPath = ft.RelPath
			currentHash = ft.Hash

			fmt.Printf("Receiving: %s (%d bytes)\n", ft.RelPath, ft.Size)

		// -------------------------------------------------------
		// CmdFileChunk: "Here is a piece of the file"
		// Writes chunks to the temp file and the streaming hasher.
		// On completion: verifies hash, stores object, upserts DB.
		// -------------------------------------------------------
		case protocol.CmdFileChunk:
			if currentFile == nil {
				continue
			}

			// Write to both the temp file and the running hash simultaneously.
			writer := io.MultiWriter(currentFile, currentHasher)
			n, err := writer.Write(p.Payload)
			if err != nil {
				log.Printf("Write error for %s: %v", currentPath, err)
				tmpPath := currentFile.Name()
				currentFile.Close()
				os.Remove(tmpPath)
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
				tmpPath := currentFile.Name()
				currentFile.Close()
				currentFile = nil

				// Verify the hash matches what the client declared.
				computedHash := hex.EncodeToString(currentHasher.Sum(nil))
				if computedHash != currentHash {
					log.Printf("Hash mismatch for %s: expected %s, got %s",
						currentPath, currentHash[:12], computedHash[:12])
					os.Remove(tmpPath)
					currentPath = ""
					currentHash = ""
					currentFileSize = 0
					currentFileReceived = 0
					continue
				}

				// Move the temp file to content-addressed storage.
				// StoreFromTemp handles dedup — if the blob already exists, the temp file is removed.
				err := objectStore.StoreFromTemp(currentHash, tmpPath)
				if err != nil {
					log.Printf("Failed to store object for %s: %v", currentPath, err)
					currentPath = ""
					currentHash = ""
					currentFileSize = 0
					currentFileReceived = 0
					continue
				}

				fmt.Printf("\nStored object: %s -> %s\n", currentPath, currentHash[:12])

				// Record the path->hash mapping in the database.
				err = database.UpsertFile(currentPath, currentHash, deviceName, currentFileReceived)
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
