package sender

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"os"

	"github.com/atienze/HomelabSecureSync/common/protocol"
)

func ConnectAndPing(serverAddr string) {
	fmt.Printf("Connecting to server at %s...\n", serverAddr)

	// 1. Dial the Server
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("Error connecting: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 2. Perform Handshake
	// We create the struct with our Secret Number and ID
	shake := protocol.Handshake{
		MagicNumber: protocol.MagicNumber,
		Version:     protocol.Version,
		ClientID:    "Laptop-01",
	}

	// Send it!
	encoder := gob.NewEncoder(conn)
	if err := encoder.Encode(shake); err != nil {
		fmt.Printf("Handshake failed: %v\n", err)
		return
	}
	fmt.Println("Handshake sent successfully.")

	// 3. Send a PING Packet
	// Now we switch to the packet encoder
	packetEncoder := protocol.NewEncoder(conn)
	
	ping := protocol.Packet{
		Cmd:     protocol.CmdPing,
		Payload: []byte("Hello Server!"),
	}

	if err := packetEncoder.Encode(ping); err != nil {
		fmt.Printf("Failed to send Ping: %v\n", err)
		return
	}
	fmt.Println("Ping sent! Check your server logs.")
}

// SendFile wraps the data and ships it using an EXISTING encoder
// CHANGE: We now ask for *protocol.Encoder, not net.Conn
func SendFile(encoder *protocol.Encoder, path string, hash string, content []byte) error {
	
	// 1. Pack the FileTransfer (Payload)
	ft := protocol.FileTransfer{
		RelPath: path,
		Hash:    hash,
		Content: content,
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(ft); err != nil {
		return err
	}

	// 2. Wrap it in a Packet
	packet := protocol.Packet{
		Cmd:     protocol.CmdSendFile,
		Payload: buf.Bytes(),
	}

	// 3. Send using the shared encoder
	// (No more NewEncoder() here!)
	return encoder.Encode(packet)
}