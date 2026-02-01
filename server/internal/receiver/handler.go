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

	// Handshake
	decoder := gob.NewDecoder(conn)
	var shake protocol.Handshake
	if err := decoder.Decode(&shake); err != nil {
		return
	}
	log.Printf("Client Authenticated: %s", shake.ClientID)

	// Setup
	networkEncoder := protocol.NewEncoder(conn)
	networkDecoder := protocol.NewDecoder(conn)

	// --- STATE VARIABLES ---
	// These track the file currently being uploaded
	var currentFile *os.File
	var currentFileSize int64
	var currentFileReceived int64
	var currentPath string

	for {
		var p protocol.Packet
		err := networkDecoder.Decode(&p)
		if err != nil {
			if currentFile != nil { currentFile.Close() }
			log.Printf("Connection closed: %v", err)
			return
		}

		switch p.Cmd {
		case protocol.CmdPing:
			fmt.Println("Received PING!")

		// 1. Negotiation
		case protocol.CmdCheckFile:
			var req protocol.CheckFileRequest
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&req)

			status := protocol.StatusNeed
			fullPath := filepath.Join("server/uploads", req.RelPath)
			if _, err := os.Stat(fullPath); err == nil {
				// Optimization: Check if hash matches
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

		// 2. Start Receiving File (The Header)
		case protocol.CmdSendFile:
			// If we were open, close the old one
			if currentFile != nil { currentFile.Close() }

			var ft protocol.FileTransfer
			gob.NewDecoder(bytes.NewBuffer(p.Payload)).Decode(&ft)

			// Prepare the file on disk
			safePath := filepath.Join("server/uploads", ft.RelPath)
			os.MkdirAll(filepath.Dir(safePath), 0755)
			
			f, err := os.Create(safePath)
			if err != nil {
				log.Printf("Failed to create file: %v", err)
				continue
			}

			// Update State
			currentFile = f
			currentFileSize = ft.Size
			currentFileReceived = 0
			currentPath = ft.RelPath
			
			fmt.Printf("Receiving: %s (%d bytes)...\n", ft.RelPath, ft.Size)

		// 3. Receive Data Chunk
		case protocol.CmdFileChunk:
			if currentFile == nil {
				continue // We received data but don't know where to put it
			}

			// Write to disk
			n, err := currentFile.Write(p.Payload)
			if err != nil {
				log.Printf("Write error: %v", err)
				currentFile.Close()
				currentFile = nil
				continue
			}

			// Track progress
			currentFileReceived += int64(n)

			// Are we done?
			if currentFileReceived >= currentFileSize {
				fmt.Printf("✔ Saved: %s\n", currentPath)
				currentFile.Close()
				currentFile = nil // Reset
			}
		}
	}
}