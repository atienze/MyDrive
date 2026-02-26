package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/atienze/HomelabSecureSync/server/internal/auth"
	"github.com/atienze/HomelabSecureSync/server/internal/db"
	"github.com/atienze/HomelabSecureSync/server/internal/receiver"
	"github.com/atienze/HomelabSecureSync/server/internal/store"
)

const Port = ":9000"

// DatabasePath and VaultDataDir can be overridden via environment variables
// for deployment (e.g., on a homelab server). Defaults to relative paths.
var DatabasePath = envOrDefault("VAULTSYNC_DB_PATH", "./vaultsync.db")
var VaultDataDir = envOrDefault("VAULTSYNC_DATA_DIR", "./VaultData")

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	// Subcommand dispatch
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "register":
			runRegister()
			return
		case "migrate":
			fmt.Fprintln(os.Stderr, "Migration is a separate binary.")
			fmt.Fprintln(os.Stderr, "Build and run it with: go build -o vault-migrate ./server/cmd/migrate && ./vault-migrate")
			os.Exit(1)
		}
	}
	runServer()
}

// runRegister handles: vault-sync-server register "DeviceName"
// Generates a cryptographic token, stores it in the DB, and prints it once to stdout.
func runRegister() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: vault-sync-server register <device-name>")
		os.Exit(1)
	}
	deviceName := os.Args[2]

	database, err := db.Open(DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	token, err := auth.GenerateToken()
	if err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}

	// token becomes the device's primary key in the devices table.
	// Tokens are 256-bit random values — collision probability is negligible.
	// Registering the same device name twice produces two different tokens, both valid.
	if err := database.RegisterDevice(token, deviceName); err != nil {
		log.Fatalf("Failed to register device: %v", err)
	}

	// Print the token ONCE — this is the only time it appears in plaintext.
	// The caller must save it to ~/.vaultsync/config.toml immediately.
	fmt.Println(token)
}

// runServer is the main server loop — listens for TCP connections and handles them.
func runServer() {
	database, err := db.Open(DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	objectStore, err := store.New(VaultDataDir)
	if err != nil {
		log.Fatalf("Failed to initialize object store: %v", err)
	}
	// Clean up any incomplete temp files from a previous crash.
	if err := objectStore.CleanupTemp(); err != nil {
		log.Printf("Warning: failed to clean up temp files: %v", err)
	}

	listener, err := net.Listen("tcp", Port)
	if err != nil {
		log.Fatalf("Failed to bind to port %s: %v", Port, err)
	}
	defer listener.Close()

	log.Printf("Vault-Sync Server listening on %s", Port)
	log.Printf("Database: %s", DatabasePath)
	log.Printf("Object store: %s", VaultDataDir)
	log.Println("Waiting for connections...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go receiver.HandleConnection(conn, database, objectStore)
	}
}
