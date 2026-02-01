package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/atienze/HomelabSecureSync/server/internal/receiver"
)

const Port = ":9000"

func main() {
	// 1. Create the Listener (Open the port)
	listener, err := net.Listen("tcp", Port)
	if err != nil {
		log.Fatalf("Failed to bind to port %s: %v", Port, err)
		os.Exit(1)
	}
	
	fmt.Printf("Vault-Sync Server Started on Port %s\n", Port)
	fmt.Println("Waiting for connections...")

	// 2. The Infinite Loop
	for {
		// Wait here until someone connects
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		// 3. Spawn a Goroutine
		// "go" keyword means: Run this function in the background
		// and immediately go back to waiting for the next person.
		go receiver.HandleConnection(conn)
	}
}