package main

import (
	"fmt"
	"net"
	"os"

	"github.com/atienze/HomelabSecureSync/client/internal/scanner"
	"github.com/atienze/HomelabSecureSync/client/internal/sender"
	"github.com/atienze/HomelabSecureSync/common/protocol"
    "encoding/gob"
)

func main() {
	serverAddr := "localhost:9000"
	targetDir := "." 

	// 1. Connect
	fmt.Println("Connecting to server...")
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// 2. Handshake
	shake := protocol.Handshake{MagicNumber: protocol.MagicNumber, Version: protocol.Version, ClientID: "Laptop-01"}
	gob.NewEncoder(conn).Encode(shake)

	// 3. CREATE THE ENCODER ONCE (The Fix)
	networkEncoder := protocol.NewEncoder(conn)

	// 4. Scan & Send
	fmt.Println("Scanning files...")
	files, _ := scanner.ScanDirectory(targetDir)

	fmt.Printf("Sending %d files...\n", len(files))
	for _, f := range files {
		content, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}

		fmt.Printf("Uploading %s...", f.Path)
		
		// Pass the EXISTING encoder, don't make a new one
		err = sender.SendFile(networkEncoder, f.Path, f.Hash, content)
		
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			break // Stop if connection dies
		} else {
			fmt.Println("Done.")
		}
	}
}