package sync

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/atienze/HomelabSecureSync/client/internal/scanner"
	sender "github.com/atienze/HomelabSecureSync/client/internal/sender"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	"github.com/atienze/HomelabSecureSync/common/protocol"
)

// Syncer orchestrates a full bidirectional sync cycle.
type Syncer struct {
	encoder   *gob.Encoder
	decoder   *protocol.Decoder
	syncDir   string
	state     *state.LocalState
	statePath string
}

// NewSyncer creates a Syncer with the given connection and config.
func NewSyncer(encoder *gob.Encoder, decoder *protocol.Decoder, syncDir, statePath string, st *state.LocalState) *Syncer {
	return &Syncer{
		encoder:   encoder,
		decoder:   decoder,
		syncDir:   syncDir,
		state:     st,
		statePath: statePath,
	}
}

// RunFullSync executes one complete bidirectional sync cycle:
// 1. Upload phase: detect local deletions, upload new/changed files
// 2. Download phase: fetch server files, remove server-deleted files
// Returns counts of uploaded, downloaded, and deleted files.
func (s *Syncer) RunFullSync() (uploaded, downloaded, deleted int, err error) {
	uploaded, err = s.uploadPhase()
	if err != nil {
		return uploaded, 0, 0, fmt.Errorf("upload phase: %w", err)
	}

	downloaded, deleted, err = s.downloadPhase()
	if err != nil {
		// Save state even on error — partial progress is better than none.
		s.state.Save(s.statePath)
		return uploaded, downloaded, deleted, fmt.Errorf("download phase: %w", err)
	}

	if err := s.state.Save(s.statePath); err != nil {
		return uploaded, downloaded, deleted, fmt.Errorf("save state: %w", err)
	}
	return uploaded, downloaded, deleted, nil
}

// uploadPhase scans local files, detects deletions, and uploads new/changed files.
func (s *Syncer) uploadPhase() (int, error) {
	// 1. Scan current files on disk.
	files, err := scanner.ScanDirectory(s.syncDir)
	if err != nil {
		return 0, fmt.Errorf("scan directory: %w", err)
	}

	// 2. Detect local deletions: files tracked in state but no longer on disk.
	currentFiles := make(map[string]bool, len(files))
	for _, f := range files {
		currentFiles[f.Path] = true
	}
	for relPath := range s.state.Files {
		if !currentFiles[relPath] {
			fmt.Printf("Local deletion detected: %s\n", relPath)
			if err := s.sendDeleteFile(relPath); err != nil {
				log.Printf("Warning: failed to send delete for %s: %v", relPath, err)
			}
			s.state.RemoveFile(relPath)
		}
	}

	// 3. For each file on disk: check with server, upload if needed.
	uploaded := 0
	for _, file := range files {
		needed, err := sender.VerifyFile(s.encoder, s.decoder, file.Path, file.Hash)
		if err != nil {
			log.Printf("Verification error for %s: %v", file.Path, err)
			continue
		}

		if needed {
			fmt.Printf("Uploading %s... ", file.Path)
			err = sender.SendFile(s.encoder, s.syncDir, file.Path, file.Hash, file.Size)
			if err != nil {
				fmt.Printf("FAILED: %v\n", err)
				continue
			}
			fmt.Println("Done.")
			uploaded++
		} else {
			fmt.Printf("Skipping %s (already on server)\n", file.Path)
		}

		// Update state regardless — file is now in sync.
		s.state.SetFile(file.Path, file.Hash)
	}

	return uploaded, nil
}

// downloadPhase fetches the server's file manifest, downloads missing/changed
// files, and removes files that were deleted on the server.
func (s *Syncer) downloadPhase() (downloaded, deleted int, err error) {
	// 1. Request the full file list from the server.
	serverFiles, err := s.listServerFiles()
	if err != nil {
		return 0, 0, fmt.Errorf("list server files: %w", err)
	}

	// 2. Build a lookup map of server files.
	serverMap := make(map[string]protocol.ServerFileEntry, len(serverFiles))
	for _, f := range serverFiles {
		serverMap[f.RelPath] = f
	}

	// 3. Download files that are missing locally or have a different hash.
	for _, sf := range serverFiles {
		localHash := s.state.GetHash(sf.RelPath)
		if localHash == sf.Hash {
			continue // Already in sync.
		}

		fmt.Printf("Downloading %s... ", sf.RelPath)
		if err := s.downloadFile(sf); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			continue
		}
		fmt.Println("Done.")
		s.state.SetFile(sf.RelPath, sf.Hash)
		downloaded++
	}

	// 4. Delete local files that no longer exist on the server (server-side deletions).
	for relPath := range s.state.Files {
		if _, exists := serverMap[relPath]; !exists {
			fullPath := filepath.Join(s.syncDir, relPath)
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				log.Printf("Warning: failed to remove %s: %v", fullPath, err)
				continue
			}
			// Clean up empty parent directories.
			cleanEmptyDirs(filepath.Dir(fullPath), s.syncDir)
			fmt.Printf("Removed (server-deleted): %s\n", relPath)
			s.state.RemoveFile(relPath)
			deleted++
		}
	}

	return downloaded, deleted, nil
}

// sendDeleteFile tells the server to soft-delete a file.
func (s *Syncer) sendDeleteFile(relPath string) error {
	req := protocol.DeleteFileRequest{RelPath: relPath}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(req); err != nil {
		return err
	}

	if err := s.encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdDeleteFile,
		Payload: buf.Bytes(),
	}); err != nil {
		return err
	}

	// Read the server's response.
	var respPacket protocol.Packet
	if err := s.decoder.Decode(&respPacket); err != nil {
		return fmt.Errorf("read delete response: %w", err)
	}

	var resp protocol.DeleteFileResponse
	if err := gob.NewDecoder(bytes.NewBuffer(respPacket.Payload)).Decode(&resp); err != nil {
		return fmt.Errorf("decode delete response: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("server rejected delete: %s", resp.Message)
	}
	return nil
}

// listServerFiles requests the full file manifest from the server.
func (s *Syncer) listServerFiles() ([]protocol.ServerFileEntry, error) {
	var buf bytes.Buffer
	gob.NewEncoder(&buf).Encode(protocol.ListServerFilesRequest{})

	if err := s.encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdListServerFiles,
		Payload: buf.Bytes(),
	}); err != nil {
		return nil, err
	}

	var respPacket protocol.Packet
	if err := s.decoder.Decode(&respPacket); err != nil {
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

// downloadFile requests a single file from the server and writes it to disk.
// Uses a temp file + rename for atomicity, and verifies the hash after download.
func (s *Syncer) downloadFile(entry protocol.ServerFileEntry) error {
	// 1. Send the download request.
	req := protocol.RequestFileRequest{RelPath: entry.RelPath, Hash: entry.Hash}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(req); err != nil {
		return err
	}

	if err := s.encoder.Encode(protocol.Packet{
		Cmd:     protocol.CmdRequestFile,
		Payload: buf.Bytes(),
	}); err != nil {
		return err
	}

	// 2. Read the file data header.
	var hdrPacket protocol.Packet
	if err := s.decoder.Decode(&hdrPacket); err != nil {
		return fmt.Errorf("read file data header: %w", err)
	}
	if hdrPacket.Cmd != protocol.CmdFileDataHeader {
		return fmt.Errorf("unexpected command: %d (expected %d)", hdrPacket.Cmd, protocol.CmdFileDataHeader)
	}

	var header protocol.FileDataHeader
	if err := gob.NewDecoder(bytes.NewBuffer(hdrPacket.Payload)).Decode(&header); err != nil {
		return fmt.Errorf("decode file data header: %w", err)
	}

	// 3. Create parent directories and a temp file.
	fullPath := filepath.Join(s.syncDir, entry.RelPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(fullPath), ".vaultsync-dl-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// 4. Receive chunks, write to temp file, and compute hash.
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	var received int64

	for received < header.Size {
		var chunkPacket protocol.Packet
		if err := s.decoder.Decode(&chunkPacket); err != nil {
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

	// 5. Verify the hash.
	computedHash := hex.EncodeToString(hasher.Sum(nil))
	if computedHash != header.Hash {
		os.Remove(tmpPath)
		return fmt.Errorf("hash mismatch: expected %s, got %s", header.Hash[:12], computedHash[:12])
	}

	// 6. Atomic rename to final path.
	if err := os.Rename(tmpPath, fullPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to final path: %w", err)
	}

	return nil
}

// cleanEmptyDirs removes empty directories walking up from dir toward stopAt.
// Stops when it reaches stopAt or encounters a non-empty directory.
func cleanEmptyDirs(dir, stopAt string) {
	for dir != stopAt {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
