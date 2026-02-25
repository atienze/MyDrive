package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LocalState tracks the last-known hash of each synced file.
// Persisted to state.json so deletions can be detected across runs.
type LocalState struct {
	Files map[string]string `json:"files"` // relPath → SHA-256 hash
}

// Load reads state.json from the given path.
// Returns an empty state if the file does not exist (fresh client).
func Load(path string) (*LocalState, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &LocalState{Files: make(map[string]string)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var s LocalState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state file: %w", err)
	}
	if s.Files == nil {
		s.Files = make(map[string]string)
	}
	return &s, nil
}

// Save writes the state to disk atomically (temp file + rename).
func (s *LocalState) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename state file: %w", err)
	}
	return nil
}

// SetFile records or updates a file's hash in state.
func (s *LocalState) SetFile(relPath, hash string) {
	s.Files[relPath] = hash
}

// RemoveFile removes a file from state tracking.
func (s *LocalState) RemoveFile(relPath string) {
	delete(s.Files, relPath)
}

// HasFile checks if a file is tracked in state.
func (s *LocalState) HasFile(relPath string) bool {
	_, ok := s.Files[relPath]
	return ok
}

// GetHash returns the tracked hash for a file, or "" if not tracked.
func (s *LocalState) GetHash(relPath string) string {
	return s.Files[relPath]
}
