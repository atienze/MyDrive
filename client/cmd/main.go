package main

import (
    "encoding/gob"
    "fmt"
    "log"
    "net"
    "time"

    "github.com/atienze/HomelabSecureSync/client/internal/scanner"
    "github.com/atienze/HomelabSecureSync/client/internal/sender"
    "github.com/atienze/HomelabSecureSync/common/protocol"
)

func main() {
    fmt.Println("--- Vault-Sync Manual Backup ---")

    // Configuration
    targetDir := "/Users/<user>/VaultDrive" 
    serverAddr := "<server-ip>:9000"

    fmt.Printf("Target: %s\n", targetDir)
    fmt.Printf("Server: %s\n", serverAddr)

    // Scan
    fmt.Print("Scanning files... ")
    files, err := scanner.ScanDirectory(targetDir)
    if err != nil {
        log.Fatalf("Scan failed: %v", err)
    }
    fmt.Printf("Found %d files.\n", len(files))

    // Connect
    conn, err := net.Dial("tcp", serverAddr)
    if err != nil {
        log.Fatalf("Connection failed: %v", err)
    }
    defer conn.Close()

    // Unified Encoder (Matches Server's rawDecoder)
    unifiedEncoder := gob.NewEncoder(conn)
    networkDecoder := protocol.NewDecoder(conn)

    // Handshake
    shake := protocol.Handshake{
        MagicNumber: protocol.MagicNumber,
        Version:     protocol.Version,
        ClientID:    "MacBook-Pro",
    }
    if err := unifiedEncoder.Encode(shake); err != nil {
        log.Fatalf("Handshake send failed: %v", err)
    }

    // Loop
    uploadCount := 0
    start := time.Now()

    for _, file := range files {
        needed, err := client.VerifyFile(unifiedEncoder, networkDecoder, file.Path, file.Hash)
        if err != nil {
            log.Printf("Verification error for %s: %v", file.Path, err)
            continue
        }

        if !needed {
            fmt.Printf("Skipping %s (Already exists)\n", file.Path)
            continue
        }

        fmt.Printf("Uploading %s... ", file.Path)
        err = client.SendFile(unifiedEncoder, targetDir, file.Path, file.Hash, file.Size)
        
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