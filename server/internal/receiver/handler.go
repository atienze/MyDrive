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
)

// HandleConnection is the logic for ONE client.
func HandleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("New connection from: %s", conn.RemoteAddr().String())

	// --- STEP 1: THE HANDSHAKE ---
	decoder := gob.NewDecoder(conn)
	var shake protocol.Handshake

	err := decoder.Decode(&shake)
	if err != nil {
		log.Printf("Failed to read handshake: %v", err)
		return
	}

	if shake.MagicNumber != protocol.MagicNumber {
		log.Printf("Invalid protocol from %s. Hanging up.", conn.RemoteAddr())
		return
	}

	if shake.Version != protocol.Version {
		log.Printf("Client version mismatch. Server: %d, Client: %d", protocol.Version, shake.Version)
		return
	}

	log.Printf("Client Authenticated: %s", shake.ClientID)

	// --- STEP 2: LISTEN FOR COMMANDS ---
	packetDecoder := protocol.NewDecoder(conn)
	
	for {
		var p protocol.Packet
		err := packetDecoder.Decode(&p)
		if err != nil {
			log.Printf("Connection closed: %v", err)
			return
		}

		handlePacket(p)
	}
}

func handlePacket(p protocol.Packet) {
	switch p.Cmd {
	case protocol.CmdPing:
		fmt.Println("Received PING!")

	case protocol.CmdSendFile:
		var ft protocol.FileTransfer
		buf := bytes.NewBuffer(p.Payload)
		decoder := gob.NewDecoder(buf)
		
		if err := decoder.Decode(&ft); err != nil {
			fmt.Printf("Failed to decode file transfer: %v\n", err)
			return
		}

		fmt.Printf("Receiving: %s (%d bytes)\n", ft.RelPath, len(ft.Content))
		saveFile(ft)
	}
}

func saveFile(ft protocol.FileTransfer) {
	// Force everything into the "uploads" folder for safety
	safePath := filepath.Join("server/uploads", ft.RelPath)

	// Ensure the directory exists
	dir := filepath.Dir(safePath)
	os.MkdirAll(dir, 0755)

	// Write the file
	err := os.WriteFile(safePath, ft.Content, 0644)
	if err != nil {
		fmt.Printf("Error writing file %s: %v\n", safePath, err)
	} else {
		fmt.Printf("✔ Saved: %s\n", safePath)
	}
}