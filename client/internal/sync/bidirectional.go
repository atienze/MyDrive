package sync

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/atienze/HomelabSecureSync/client/internal/scanner"
	sender "github.com/atienze/HomelabSecureSync/client/internal/sender"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	"github.com/atienze/HomelabSecureSync/common/protocol"
)

// Syncer orchestrates a push-only sync cycle.
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

// RunSync executes one push-only sync cycle:
// 1. Detect local deletions -> send CmdDeleteFile for each
// 2. Upload new/changed files
// Returns count of uploaded files.
func (s *Syncer) RunSync() (uploaded int, err error) {
	uploaded, err = s.uploadPhase()
	if err != nil {
		return uploaded, fmt.Errorf("upload phase: %w", err)
	}
	if err := s.state.Save(s.statePath); err != nil {
		return uploaded, fmt.Errorf("save state: %w", err)
	}
	return uploaded, nil
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
