package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/atienze/HomelabSecureSync/server/internal/receiver"
    "github.com/atienze/HomelabSecureSync/server/internal/db"
)

const Port = ":9000"

/*

"<homelab-path>/vaultsync.db"

*/
const DatabasePath = "./vaultsync.db"

func main() {
    // --- Step 1: Open the database ---
    // We open the database ONCE here, then pass it to every connection handler.
    // This is important — you never want to open a new DB connection per request.
    database, err := db.Open(DatabasePath)
	if err != nil {
		log.Fatalf("Failed to bind to port %s: %v", Port, err)
		os.Exit(1)
	}

	// defer means "run this when main() exits" — ensures the DB is always closed	cleanly
    defer database.Close()

	// --- Step 2: Start the TCP listener
	listener, err := net.Listen("tcp", Port)
    if err != nil {
        log.Fatalf("Failed to bind to port %s: %v", Port, err)
        os.Exit(1)
    }
    defer listener.Close()
	
	fmt.Printf("Vault-Sync Server Started on Port %s\n", Port)
	fmt.Println("Database: " + DatabasePath)
	fmt.Println("Waiting for connections...")

// --- Step 3: Accept connections (same as before, but now pass the DB in) ---
    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Failed to accept connection: %v", err)
            continue
        }
        // We now pass 'database' into HandleConnection so the handler can use it.
        // Each goroutine gets the SAME database handle — that's fine and intentional.
        go receiver.HandleConnection(conn, database)
    }
}