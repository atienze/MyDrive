package sync

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/atienze/myDrive/client/internal/config"
	sender "github.com/atienze/myDrive/client/internal/sender"
	"github.com/atienze/myDrive/client/internal/state"
	"github.com/atienze/myDrive/common/protocol"
)

// Sentinel errors for categorized failure handling by callers (e.g. HTTP handlers).
var (
	ErrServerUnreachable = errors.New("server unreachable")
	ErrTimeout           = errors.New("operation timed out")
	ErrAuthFailed        = errors.New("authentication failed")
	ErrHashMismatch      = errors.New("hash mismatch after transfer")
)

const (
	// dialTimeout is used for TCP connection establishment.
	// 10s catches server-unreachable / DNS failures promptly.
	dialTimeout = 10 * time.Second

	// OpDeadline is the per-operation wall-clock deadline set after handshake.
	// 5 minutes is generous for large file transfers on a homelab LAN.
	OpDeadline = 5 * time.Minute
)

// DialAndHandshake opens a TCP connection to cfg.ServerAddr and performs the
// myDrive token auth handshake. Returns (conn, encoder, decoder, nil) on
// success. The caller is responsible for closing conn.
//
// DialAndHandshake does not set a deadline on the connection. Callers must call
// conn.SetDeadline(time.Now().Add(opDeadline)) immediately after receiving the
// connection, because the appropriate deadline depends on the operation being
// performed.
func DialAndHandshake(cfg *config.Config) (net.Conn, *gob.Encoder, *protocol.Decoder, error) {
	conn, err := net.DialTimeout("tcp", cfg.ServerAddr, dialTimeout)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrServerUnreachable, err)
	}

	encoder := gob.NewEncoder(conn)
	decoder := protocol.NewDecoder(conn)

	shake := protocol.Handshake{
		MagicNumber: protocol.MagicNumber,
		Version:     protocol.Version,
		Token:       cfg.Token,
	}
	if err := encoder.Encode(shake); err != nil {
		conn.Close()
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrAuthFailed, err)
	}

	return conn, encoder, decoder, nil
}

// FetchServerFileList connects to the server, requests the full file manifest,
// and returns the list of files currently tracked on the server.
// This is a read-only operation — state is not modified.
func FetchServerFileList(cfg *config.Config) ([]protocol.ServerFileEntry, error) {
	conn, encoder, decoder, err := DialAndHandshake(cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(OpDeadline)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(protocol.ListServerFilesRequest{}); err != nil {
		return nil, fmt.Errorf("encode list request: %w", err)
	}

	if err := encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdListServerFiles,
		Payload: buf.Bytes(),
	}); err != nil {
		return nil, fmt.Errorf("send list request: %w", err)
	}

	var respPacket protocol.Packet
	if err := decoder.Decode(&respPacket); err != nil {
		return nil, fmt.Errorf("read server file list: %w", err)
	}

	if respPacket.Cmd != protocol.CmdServerFileList {
		return nil, fmt.Errorf("unexpected command: %d (expected %d)", respPacket.Cmd, protocol.CmdServerFileList)
	}

	var resp protocol.ServerFileListResponse
	if err := gob.NewDecoder(bytes.NewBuffer(respPacket.Payload)).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode server file list: %w", err)
	}

	return resp.Files, nil
}

// UploadSingleFile connects to the server and uploads a single file from the
// client's sync directory. Unlike the full sync cycle, this skips the
// CmdCheckFile verify step — the user explicitly requested this upload.
// Updates state and persists it to disk on success.
func UploadSingleFile(cfg *config.Config, st *state.LocalState, statePath, relPath string) error {
	conn, encoder, decoder, err := DialAndHandshake(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(OpDeadline)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	fullPath := filepath.Join(cfg.SyncDir, relPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	size := info.Size()

	hash, err := computeFileHash(fullPath)
	if err != nil {
		return fmt.Errorf("hash file: %w", err)
	}

	if err := sender.SendFile(encoder, cfg.SyncDir, relPath, hash, size); err != nil {
		return fmt.Errorf("send file: %w", err)
	}

	// After sending, issue a CmdCheckFile for the same path+hash. The server
	// processes commands sequentially, so by the time it responds the upload
	// has been committed to the DB. Without this round-trip the function
	// returns before the server finishes writing, and an immediate file-list
	// refresh may not see the new file.
	checkReq := protocol.CheckFileRequest{RelPath: relPath, Hash: hash}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(checkReq); err != nil {
		return fmt.Errorf("encode check request: %w", err)
	}
	if err := encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdCheckFile,
		Payload: buf.Bytes(),
	}); err != nil {
		return fmt.Errorf("send check request: %w", err)
	}
	var respPacket protocol.Packet
	if err := decoder.Decode(&respPacket); err != nil {
		return fmt.Errorf("read check response: %w", err)
	}

	st.SetFile(relPath, hash)
	if err := st.Save(statePath); err != nil {
		return fmt.Errorf("persist state after upload: %w", err)
	}

	return nil
}

// DownloadSingleFile connects to the server and downloads a single file into
// the client's sync directory. Uses a temp file + atomic rename for safety.
// Verifies the SHA-256 hash after receiving all chunks.
// Updates state and persists it to disk on success.
func DownloadSingleFile(cfg *config.Config, st *state.LocalState, statePath, relPath string) error {
	conn, encoder, decoder, err := DialAndHandshake(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(OpDeadline)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	// Send the download request. Hash is empty — download whatever the server has.
	req := protocol.RequestFileRequest{RelPath: relPath, Hash: ""}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(req); err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	if err := encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdRequestFile,
		Payload: buf.Bytes(),
	}); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	// Read the file data header.
	var hdrPacket protocol.Packet
	if err := decoder.Decode(&hdrPacket); err != nil {
		return fmt.Errorf("read file data header: %w", err)
	}
	if hdrPacket.Cmd != protocol.CmdFileDataHeader {
		return fmt.Errorf("unexpected command: %d (expected %d)", hdrPacket.Cmd, protocol.CmdFileDataHeader)
	}

	var header protocol.FileDataHeader
	if err := gob.NewDecoder(bytes.NewBuffer(hdrPacket.Payload)).Decode(&header); err != nil {
		return fmt.Errorf("decode file data header: %w", err)
	}

	// Empty hash signals "file not found" from the server.
	if header.Hash == "" {
		return fmt.Errorf("file not found on server: %s", relPath)
	}

	// Create parent directories and temp file.
	fullPath := filepath.Join(cfg.SyncDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(fullPath), ".mydrive-dl-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Receive chunks, write to temp file, and accumulate hash.
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	var received int64

	for received < header.Size {
		var chunkPacket protocol.Packet
		if err := decoder.Decode(&chunkPacket); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("read chunk: %w", err)
		}
		if chunkPacket.Cmd != protocol.CmdFileDataChunk {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("unexpected command during download: %d", chunkPacket.Cmd)
		}

		n, err := writer.Write(chunkPacket.Payload)
		if err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write chunk: %w", err)
		}
		received += int64(n)
	}
	tmpFile.Close()

	// Verify hash.
	computedHash := hex.EncodeToString(hasher.Sum(nil))
	if computedHash != header.Hash {
		os.Remove(tmpPath)
		return fmt.Errorf("%w: expected %s, got %s", ErrHashMismatch, header.Hash[:12], computedHash[:12])
	}

	// Atomic rename to final path.
	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to final path: %w", err)
	}

	st.SetFile(relPath, header.Hash)
	if err := st.Save(statePath); err != nil {
		return fmt.Errorf("persist state after download: %w", err)
	}

	return nil
}

// DeleteServerFile connects to the server and soft-deletes a file from the
// server's object store. Removes the file from local state on success.
func DeleteServerFile(cfg *config.Config, st *state.LocalState, statePath, relPath string) error {
	conn, encoder, decoder, err := DialAndHandshake(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(OpDeadline)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	req := protocol.DeleteFileRequest{RelPath: relPath}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(req); err != nil {
		return fmt.Errorf("encode delete request: %w", err)
	}

	if err := encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdDeleteFile,
		Payload: buf.Bytes(),
	}); err != nil {
		return fmt.Errorf("send delete request: %w", err)
	}

	var respPacket protocol.Packet
	if err := decoder.Decode(&respPacket); err != nil {
		return fmt.Errorf("read delete response: %w", err)
	}

	var resp protocol.DeleteFileResponse
	if err := gob.NewDecoder(bytes.NewBuffer(respPacket.Payload)).Decode(&resp); err != nil {
		return fmt.Errorf("decode delete response: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("server rejected delete: %s", resp.Message)
	}

	st.RemoveFile(relPath)
	if err := st.Save(statePath); err != nil {
		return fmt.Errorf("persist state after delete: %w", err)
	}

	return nil
}

// PullFile connects to the server, fetches the file manifest, finds the file
// owned by fromDevice at relPath, and downloads it by hash. Updates local state
// on success so the file will be tracked (and uploaded under this device on next sync).
func PullFile(cfg *config.Config, st *state.LocalState, statePath, fromDevice, relPath string) error {
	// Step 1: Fetch the full file list to find the hash for this device+path.
	entries, err := FetchServerFileList(cfg)
	if err != nil {
		return fmt.Errorf("fetch server file list: %w", err)
	}

	var targetHash string
	for _, e := range entries {
		if e.DeviceID == fromDevice && e.RelPath == relPath {
			targetHash = e.Hash
			break
		}
	}
	if targetHash == "" {
		return fmt.Errorf("file %q not found on device %q", relPath, fromDevice)
	}

	// Step 2: Download by hash using a dedicated connection.
	conn, encoder, decoder, err := DialAndHandshake(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(OpDeadline)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	// Send CmdRequestFile with the known hash.
	req := protocol.RequestFileRequest{RelPath: relPath, Hash: targetHash}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(req); err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	if err := encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdRequestFile,
		Payload: buf.Bytes(),
	}); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	// Read file data header.
	var hdrPacket protocol.Packet
	if err := decoder.Decode(&hdrPacket); err != nil {
		return fmt.Errorf("read file data header: %w", err)
	}
	if hdrPacket.Cmd != protocol.CmdFileDataHeader {
		return fmt.Errorf("unexpected command: %d (expected %d)", hdrPacket.Cmd, protocol.CmdFileDataHeader)
	}

	var header protocol.FileDataHeader
	if err := gob.NewDecoder(bytes.NewBuffer(hdrPacket.Payload)).Decode(&header); err != nil {
		return fmt.Errorf("decode file data header: %w", err)
	}

	// Empty hash signals "file not found" from the server.
	if header.Hash == "" {
		return fmt.Errorf("file not found on server: %s (device %s)", relPath, fromDevice)
	}

	// Create parent directories and temp file.
	fullPath := filepath.Join(cfg.SyncDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(fullPath), ".mydrive-dl-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Receive chunks, write to temp, compute hash.
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	var received int64

	for received < header.Size {
		var chunkPacket protocol.Packet
		if err := decoder.Decode(&chunkPacket); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("read chunk: %w", err)
		}
		if chunkPacket.Cmd != protocol.CmdFileDataChunk {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("unexpected command during download: %d", chunkPacket.Cmd)
		}

		n, err := writer.Write(chunkPacket.Payload)
		if err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write chunk: %w", err)
		}
		received += int64(n)
	}
	tmpFile.Close()

	// Verify hash.
	computedHash := hex.EncodeToString(hasher.Sum(nil))
	if computedHash != header.Hash {
		os.Remove(tmpPath)
		return fmt.Errorf("%w: expected %s, got %s", ErrHashMismatch, header.Hash[:12], computedHash[:12])
	}

	// Atomic rename.
	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to final path: %w", err)
	}

	// Track in local state so next sync uploads under this device's ID.
	st.SetFile(relPath, header.Hash)
	if err := st.Save(statePath); err != nil {
		return fmt.Errorf("persist state after pull: %w", err)
	}

	return nil
}

// computeFileHash opens a file and returns its SHA-256 hash as a hex string.
func computeFileHash(fullPath string) (string, error) {
	f, err := os.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
