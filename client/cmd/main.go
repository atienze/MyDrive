package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/atienze/HomelabSecureSync/client/internal/config"
	"github.com/atienze/HomelabSecureSync/client/internal/scanner"
	client "github.com/atienze/HomelabSecureSync/client/internal/sender"
	"github.com/atienze/HomelabSecureSync/common/protocol"
)

func main() {
	fmt.Println("--- Vault-Sync Manual Backup ---")

	// Load configuration from config.toml at the project root.
	// Exits with a clear, actionable message if the file is missing or malformed.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	fmt.Printf("Target:  %s\n", cfg.WatchDir)
	fmt.Printf("Server:  %s\n", cfg.ServerAddr)

	// Scan
	fmt.Print("Scanning files... ")
	files, err := scanner.ScanDirectory(cfg.WatchDir)
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}
	fmt.Printf("Found %d files.\n", len(files))

	// Connect
	conn, err := net.Dial("tcp", cfg.ServerAddr)
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close()

	unifiedEncoder := gob.NewEncoder(conn)
	networkDecoder := protocol.NewDecoder(conn)

	// Handshake — Version 2 sends Token (from config) instead of a plaintext ClientID.
	shake := protocol.Handshake{
		MagicNumber: protocol.MagicNumber,
		Version:     protocol.Version, // = 2
		Token:       cfg.Token,
	}
	if err := unifiedEncoder.Encode(shake); err != nil {
		log.Fatalf("Handshake failed: %v", err)
	}

	// Sync loop
	uploadCount := 0
	start := time.Now()

	for _, file := range files {
		needed, err := client.VerifyFile(unifiedEncoder, networkDecoder, file.Path, file.Hash)
		if err != nil {
			log.Printf("Verification error for %s: %v", file.Path, err)
			continue
		}

		if !needed {
			fmt.Printf("Skipping %s (already on server)\n", file.Path)
			continue
		}

		fmt.Printf("Uploading %s... ", file.Path)
		err = client.SendFile(unifiedEncoder, cfg.WatchDir, file.Path, file.Hash, file.Size)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			fmt.Println("Done.")
			uploadCount++
		}
	}

	fmt.Println("\n--- Sync Complete ---")
	fmt.Printf("Uploaded: %d\n", uploadCount)
	fmt.Printf("Time:     %s\n", time.Since(start))
}
