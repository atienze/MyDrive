package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/atienze/HomelabSecureSync/client/internal/scanner"
	"github.com/atienze/HomelabSecureSync/client/internal/sender"
	"github.com/atienze/HomelabSecureSync/common/protocol"
)

func main() {
	// 1. Configuration
	serverAddr := "localhost:9000"
	targetDir := "." 

	startTime := time.Now()
	fmt.Println("--- Vault-Sync Manual Backup ---")
	fmt.Printf("Target: %s\n", targetDir)
	fmt.Printf("Server: %s\n", serverAddr)

	// 2. Connect
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("❌ Could not connect to server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 3. Handshake
	shake := protocol.Handshake{MagicNumber: protocol.MagicNumber, Version: protocol.Version, ClientID: "Laptop-01"}
	gob.NewEncoder(conn).Encode(shake)
	
	networkEncoder := protocol.NewEncoder(conn)
	networkDecoder := protocol.NewDecoder(conn)

	// 4. Scan
	fmt.Print("Scanning files... ")
	files, err := scanner.ScanDirectory(targetDir)
	if err != nil {
		fmt.Printf("❌ Scan failed: %v\n", err)
		return
	}
	fmt.Printf("Found %d files.\n", len(files))

	// 5. Sync Loop
	uploaded := 0
	skipped := 0
	errors := 0

	for _, f := range files {
		// A. Negotiate
		shouldSend, err := sender.VerifyFile(networkEncoder, networkDecoder, f.Path, f.Hash)
		if err != nil {
			fmt.Printf("❌ Error checking %s: %v\n", f.Path, err)
			errors++
			continue
		}

		if !shouldSend {
			skipped++
			continue
		}

		// B. Upload (STREAMING MODE)
		// We do NOT read the file here anymore. We just tell sender to go get it.
		fmt.Printf("Uploading %s... ", f.Path)
		
		// FIX: Pass f.Size instead of 'content'
		err = sender.SendFile(networkEncoder, f.Path, f.Hash, f.Size)
		
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			errors++
		} else {
			fmt.Println("Done.")
			uploaded++
		}
	}

	// 6. Summary Report
	duration := time.Since(startTime)
	fmt.Println("\n--- Sync Complete ---")
	fmt.Printf("Uploaded: %d\n", uploaded)
	fmt.Printf("Skipped:  %d\n", skipped)
	fmt.Printf("Errors:   %d\n", errors)
	fmt.Printf("Time:     %s\n", duration)
}