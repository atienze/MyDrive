package sender

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
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

// SendFile streams the file in chunks
func SendFile(encoder *protocol.Encoder, path string, hash string, size int64) error {
	// 1. Open the file (Do NOT read the whole thing)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 2. Send the Header (Metadata)
	ft := protocol.FileTransfer{
		RelPath: path,
		Hash:    hash,
		Size:    size,
	}
	
	var headerBuf bytes.Buffer
	if err := gob.NewEncoder(&headerBuf).Encode(ft); err != nil {
		return err
	}

	headerPacket := protocol.Packet{
		Cmd:     protocol.CmdSendFile,
		Payload: headerBuf.Bytes(),
	}
	
	if err := encoder.Encode(headerPacket); err != nil {
		return err
	}

	// 3. Stream the Data (Chunks)
	// We use a 32KB buffer. Small enough for network safety, big enough for speed.
	buffer := make([]byte, 32*1024) 
	
	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break // Done reading
			}
			return err
		}

		// Wrap the chunk in a packet
		chunkPacket := protocol.Packet{
			Cmd:     protocol.CmdFileChunk,
			Payload: buffer[:n], // Only send the bytes we actually read
		}

		if err := encoder.Encode(chunkPacket); err != nil {
			return err
		}
	}

	return nil
}

func VerifyFile(encoder *protocol.Encoder, decoder *protocol.Decoder, path string, hash string) (bool, error) {
	// 1. Send the Question
	req := protocol.CheckFileRequest{RelPath: path, Hash: hash}
	
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(req); err != nil {
		return false, err
	}

	packet := protocol.Packet{
		Cmd:     protocol.CmdCheckFile,
		Payload: buf.Bytes(),
	}

	if err := encoder.Encode(packet); err != nil {
		return false, err
	}

	// 2. Wait for the Answer
	// We need to listen for the NEXT packet from the server
	var respPacket protocol.Packet
	if err := decoder.Decode(&respPacket); err != nil {
		return false, err
	}

	if respPacket.Cmd != protocol.CmdFileStatus {
		return false, fmt.Errorf("unexpected command: %d", respPacket.Cmd)
	}

	var resp protocol.FileStatusResponse
	if err := gob.NewDecoder(bytes.NewBuffer(respPacket.Payload)).Decode(&resp); err != nil {
		return false, err
	}

	return resp.Status == protocol.StatusNeed, nil
}