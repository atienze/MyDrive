package client

import (
    "bytes"
    "encoding/gob"
    "fmt"
    "io"
    "os"
    "path/filepath"

    "github.com/atienze/HomelabSecureSync/common/protocol"
)

// SendFile streams the file in chunks
func SendFile(encoder *gob.Encoder, rootDir string, path string, hash string, size int64) error {
    
    // 1. Open the file
    fullPath := filepath.Join(rootDir, path)
    file, err := os.Open(fullPath)
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

    // Direct Encode (No Protocol Wrapper needed if using raw Gob)
    headerPacket := protocol.Packet{
        Cmd:     protocol.CmdSendFile,
        Payload: headerBuf.Bytes(),
    }
    
    if err := encoder.Encode(headerPacket); err != nil {
        return err
    }

    // 3. Stream the Data
    buffer := make([]byte, 4*1024*1024) // 4MB chunks
    
    for {
        n, err := file.Read(buffer)
        if err != nil {
            if err == io.EOF {
                break 
            }
            return err
        }

        chunkPacket := protocol.Packet{
            Cmd:     protocol.CmdFileChunk,
            Payload: buffer[:n],
        }

        if err := encoder.Encode(chunkPacket); err != nil {
            return err
        }
    }

    return nil
}

// VerifyFile checks if the server needs the file
func VerifyFile(encoder *gob.Encoder, decoder *protocol.Decoder, path string, hash string) (bool, error) {
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