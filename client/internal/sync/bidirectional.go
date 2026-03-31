package sync

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/atienze/myDrive/client/internal/config"
	"github.com/atienze/myDrive/client/internal/scanner"
	sender "github.com/atienze/myDrive/client/internal/sender"
	"github.com/atienze/myDrive/client/internal/state"
	"github.com/atienze/myDrive/common/protocol"
)

// Syncer orchestrates a bidirectional sync cycle.
type Syncer struct {
	encoder   *gob.Encoder
	decoder   *protocol.Decoder
	syncDir   string
	cfg       *config.Config
	state     *state.LocalState
	statePath string
}

// NewSyncer creates a Syncer with the given connection and config.
func NewSyncer(encoder *gob.Encoder, decoder *protocol.Decoder, syncDir, statePath string, st *state.LocalState, cfg *config.Config) *Syncer {
	return &Syncer{
		encoder:   encoder,
		decoder:   decoder,
		syncDir:   syncDir,
		cfg:       cfg,
		state:     st,
		statePath: statePath,
	}
}

// RunSync executes one bidirectional sync cycle:
// 1. Detect local deletions -> remove from tracking
// 2. Upload new/changed local files to server
// 3. Download all server files (from any device) missing or changed locally
// Returns counts of uploaded and downloaded files.
func (s *Syncer) RunSync() (uploaded, downloaded int, err error) {
	uploaded, err = s.uploadPhase()
	if err != nil {
		return uploaded, 0, fmt.Errorf("upload phase: %w", err)
	}
	downloaded, err = s.downloadPhase()
	if err != nil {
		return uploaded, downloaded, fmt.Errorf("download phase: %w", err)
	}
	if err := s.state.Save(s.statePath); err != nil {
		return uploaded, downloaded, fmt.Errorf("save state: %w", err)
	}
	return uploaded, downloaded, nil
}

// downloadPhase fetches the server file list and downloads any files missing
// locally or whose hash has changed. All devices' files are considered so that
// files uploaded by other devices are synced to this machine.
//
// Deduplication uses a two-pass strategy (Option 1 — own device takes priority):
//   - Pass 1: mark all paths that this device owns on the server.
//   - Pass 2: for each server file, skip paths already owned by this device
//     when processing another device's entry.
//
// This ensures a device's own files are never overwritten by another device's
// copy of the same path. Cross-device files are only downloaded when this
// device has no server entry for that path.
func (s *Syncer) downloadPhase() (int, error) {
	serverFiles, err := FetchServerFileList(s.cfg)
	if err != nil {
		return 0, fmt.Errorf("fetch server file list: %w", err)
	}

	// Pass 1: collect all paths this device owns on the server.
	ownedPaths := make(map[string]bool)
	for _, f := range serverFiles {
		if f.DeviceID == s.cfg.Token {
			ownedPaths[f.RelPath] = true
		}
	}

	seen := make(map[string]bool)
	downloaded := 0
	for _, f := range serverFiles {
		// Skip cross-device entries for paths this device already owns.
		if f.DeviceID != s.cfg.Token && ownedPaths[f.RelPath] {
			log.Printf("Skipping %s from device %s (own device entry takes priority)", f.RelPath, f.DeviceID[:8])
			continue
		}

		// Deduplicate: if multiple devices have the same path and none is ours,
		// download once (first encountered wins).
		if seen[f.RelPath] {
			continue
		}
		seen[f.RelPath] = true

		fullPath := filepath.Join(s.syncDir, f.RelPath)
		localHash, tracked := s.state.Files[f.RelPath]

		// Skip if local file exists with a matching hash.
		if tracked && localHash == f.Hash {
			if _, statErr := os.Stat(fullPath); statErr == nil {
				continue
			}
		}

		fmt.Printf("Downloading %s... ", f.RelPath)
		if err := DownloadSingleFile(s.cfg, s.state, s.statePath, f.RelPath); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			continue
		}
		fmt.Println("Done.")
		downloaded++
	}
	return downloaded, nil
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
			fmt.Printf("Local deletion detected (removed from tracking): %s\n", relPath)
			// Only remove from local state — do NOT send CmdDeleteFile to server.
			// The server is the persistent store; local deletions should not
			// cascade to the server. Users can explicitly delete from server
			// via the Web UI's "Remove from server" action.
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
