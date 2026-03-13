package receiver

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
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

	if shake.MagicNumber != protocol.MagicNumber {
		log.Printf("Rejected connection from %s: invalid magic number", conn.RemoteAddr())
		return
	}
	if shake.Version != protocol.Version {
		log.Printf("Rejected connection from %s: protocol version mismatch (client=%d, server=%d)",
			conn.RemoteAddr(), shake.Version, protocol.Version)
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
	var currentFile *os.File      // temp file receiving chunks
	var currentHasher hash.Hash   // streaming SHA-256 computed as chunks arrive
	var currentFileSize int64     // declared total size
	var currentFileReceived int64 // bytes received so far
	var currentPath string        // client-side relative path (DB key only)
	var currentHash string        // declared hash from the client

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
			// no-op

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

			exists, err := database.FileExists(req.RelPath, req.Hash, deviceName)
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
				log.Printf("Skipping %s (already stored)", req.RelPath)
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

			log.Printf("Receiving: %s (%d bytes)", ft.RelPath, ft.Size)

			// Zero-byte files send no CmdFileChunk — finalize immediately.
			if ft.Size == 0 {
				tmpPath := currentFile.Name()
				currentFile.Close()
				currentFile = nil

				computedHash := hex.EncodeToString(currentHasher.Sum(nil))
				if computedHash != currentHash {
					log.Printf("Hash mismatch for empty file %s: expected %s, got %s",
						currentPath, currentHash[:12], computedHash[:12])
					os.Remove(tmpPath)
					currentPath = ""
					currentHash = ""
					continue
				}

				if err := objectStore.StoreFromTemp(currentHash, tmpPath); err != nil {
					log.Printf("Failed to store empty object for %s: %v", currentPath, err)
					currentPath = ""
					currentHash = ""
					continue
				}
				if err := database.UpsertFile(currentPath, currentHash, deviceName, 0); err != nil {
					log.Printf("Warning: failed to record %s in database: %v", currentPath, err)
				} else {
					log.Printf("Stored empty file: %s -> %s", currentPath, currentHash[:12])
				}
				currentPath = ""
				currentHash = ""
				currentFileSize = 0
				currentFileReceived = 0
			}

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

				log.Printf("Stored object: %s -> %s", currentPath, currentHash[:12])

				// Record the path->hash mapping in the database.
				err = database.UpsertFile(currentPath, currentHash, deviceName, currentFileReceived)
				if err != nil {
					log.Printf("Warning: failed to record %s in database: %v", currentPath, err)
				} else {
					log.Printf("Recorded in database: %s", currentPath)
				}

				currentPath = ""
				currentHash = ""
				currentFileSize = 0
				currentFileReceived = 0
			}

		// -------------------------------------------------------
		// CmdDeleteFile: Client reports a local file deletion.
		// Soft-deletes the DB record, then removes the blob if
		// no other files reference it.
		// -------------------------------------------------------
		case protocol.CmdDeleteFile:
			var req protocol.DeleteFileRequest
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&req)

			if !store.ValidateRelPath(req.RelPath) {
				log.Printf("Rejected invalid path in CmdDeleteFile from %s: %q", deviceName, req.RelPath)
				sendDeleteResponse(networkEncoder, false, "invalid path")
				continue
			}

			// Get the hash before marking deleted (needed for blob cleanup).
			fileHash, exists, err := database.GetFileHash(req.RelPath, deviceName)
			if err != nil {
				log.Printf("DB error looking up %s for deletion: %v", req.RelPath, err)
				sendDeleteResponse(networkEncoder, false, "server error")
				continue
			}
			if !exists {
				// File not found or already deleted — treat as success.
				sendDeleteResponse(networkEncoder, true, "already deleted")
				continue
			}

			if err := database.MarkDeleted(req.RelPath, deviceName); err != nil {
				log.Printf("Failed to mark %s as deleted: %v", req.RelPath, err)
				sendDeleteResponse(networkEncoder, false, "server error")
				continue
			}

			// Check if any other non-deleted files still reference this hash.
			refCount, err := database.HashRefCount(fileHash)
			if err != nil {
				log.Printf("Warning: ref count check failed for hash %s: %v", fileHash[:12], err)
			} else {
				if err := objectStore.DeleteObject(fileHash, refCount); err != nil {
					log.Printf("Warning: blob cleanup failed for hash %s: %v", fileHash[:12], err)
				}
			}

			log.Printf("Deleted: %s (hash %s, refs remaining: %d)", req.RelPath, fileHash[:12], refCount)
			sendDeleteResponse(networkEncoder, true, "deleted")

		// -------------------------------------------------------
		// CmdListServerFiles: Client requests the full file manifest.
		// Returns ALL non-deleted files (not filtered by device)
		// so multiple clients can share files bidirectionally.
		// -------------------------------------------------------
		case protocol.CmdListServerFiles:
			files, err := database.GetFilesForDevice(deviceName)
			if err != nil {
				log.Printf("Failed to list files for %s: %v", deviceName, err)
				continue
			}

			entries := make([]protocol.ServerFileEntry, len(files))
			for i, f := range files {
				entries[i] = protocol.ServerFileEntry{
					RelPath:  f.RelPath,
					Hash:     f.Hash,
					Size:     f.Size,
					DeviceID: f.DeviceID,
				}
			}

			var buf bytes.Buffer
			gob.NewEncoder(&buf).Encode(protocol.ServerFileListResponse{Files: entries})
			networkEncoder.Encode(protocol.Packet{
				Cmd:     protocol.CmdServerFileList,
				Payload: buf.Bytes(),
			})
			log.Printf("Sent file list to %s: %d files", deviceName, len(entries))

		// -------------------------------------------------------
		// CmdRequestFile: Client requests a file download.
		// Streams the blob in 4MB chunks, mirroring the upload
		// chunking pattern.
		// -------------------------------------------------------
		case protocol.CmdRequestFile:
			var req protocol.RequestFileRequest
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&req)

			if !store.ValidateRelPath(req.RelPath) {
				log.Printf("Rejected invalid path in CmdRequestFile from %s: %q", deviceName, req.RelPath)
				continue
			}

			// If client sent an empty hash, look up the current hash by path.
			if req.Hash == "" {
				fileHash, exists, dbErr := database.GetFileHash(req.RelPath, deviceName)
				if dbErr != nil || !exists {
					log.Printf("No hash found for download request %s from %s", req.RelPath, deviceName)
					continue
				}
				req.Hash = fileHash
			}

			if !objectStore.HasObject(req.Hash) {
				log.Printf("Requested blob missing for %s (hash %s)", req.RelPath, req.Hash[:12])
				continue
			}

			f, err := objectStore.OpenObject(req.Hash)
			if err != nil {
				log.Printf("Failed to open blob for %s: %v", req.RelPath, err)
				continue
			}

			info, err := f.Stat()
			if err != nil {
				f.Close()
				log.Printf("Failed to stat blob for %s: %v", req.RelPath, err)
				continue
			}

			// Send the file metadata header.
			header := protocol.FileDataHeader{
				RelPath: req.RelPath,
				Hash:    req.Hash,
				Size:    info.Size(),
			}
			var hdrBuf bytes.Buffer
			gob.NewEncoder(&hdrBuf).Encode(header)
			networkEncoder.Encode(protocol.Packet{
				Cmd:     protocol.CmdFileDataHeader,
				Payload: hdrBuf.Bytes(),
			})

			// Stream the file in 4MB chunks.
			const chunkSize = 4 * 1024 * 1024
			chunk := make([]byte, chunkSize)
			for {
				n, readErr := f.Read(chunk)
				if n > 0 {
					networkEncoder.Encode(protocol.Packet{
						Cmd:     protocol.CmdFileDataChunk,
						Payload: chunk[:n],
					})
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					log.Printf("Read error streaming %s: %v", req.RelPath, readErr)
					break
				}
			}
			f.Close()
			log.Printf("Sent file to %s: %s (%d bytes)", deviceName, req.RelPath, info.Size())
		}
	}
}

// sendDeleteResponse encodes and sends a DeleteFileResponse packet.
func sendDeleteResponse(encoder *protocol.Encoder, success bool, message string) {
	var buf bytes.Buffer
	gob.NewEncoder(&buf).Encode(protocol.DeleteFileResponse{
		Success: success,
		Message: message,
	})
	encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdDeleteFile,
		Payload: buf.Bytes(),
	})
}
