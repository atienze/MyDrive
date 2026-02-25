package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// ObjectStore manages content-addressable blob storage.
// Files are stored at {baseDir}/objects/{hash[:2]}/{hash[2:]}.
type ObjectStore struct {
	baseDir string
}

// New creates an ObjectStore rooted at baseDir.
// It ensures the objects/ and tmp/ subdirectories exist.
func New(baseDir string) (*ObjectStore, error) {
	s := &ObjectStore{baseDir: baseDir}
	if err := os.MkdirAll(filepath.Join(baseDir, "objects"), 0755); err != nil {
		return nil, fmt.Errorf("create objects dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "tmp"), 0755); err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}
	return s, nil
}

// ObjectPath returns the content-addressed path for a given hash.
// Layout: {baseDir}/objects/{hash[:2]}/{hash[2:]}
func (s *ObjectStore) ObjectPath(hash string) string {
	return filepath.Join(s.baseDir, "objects", hash[:2], hash[2:])
}

// HasObject checks if a blob with the given hash exists on disk.
func (s *ObjectStore) HasObject(hash string) bool {
	_, err := os.Stat(s.ObjectPath(hash))
	return err == nil
}

// WriteObject writes data to the content-addressed path.
// No-op if the object already exists (dedup).
func (s *ObjectStore) WriteObject(hash string, data []byte) error {
	if s.HasObject(hash) {
		return nil
	}

	objPath := s.ObjectPath(hash)
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		return fmt.Errorf("create prefix dir: %w", err)
	}

	tmpFile, err := s.CreateTempFile()
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	if err := os.Rename(tmpPath, objPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to object path: %w", err)
	}
	return nil
}

// StoreFromTemp moves a verified temp file to its content-addressed location.
// The caller is responsible for verifying the hash before calling this.
// If the object already exists (dedup), the temp file is removed.
func (s *ObjectStore) StoreFromTemp(hash string, tmpPath string) error {
	if s.HasObject(hash) {
		os.Remove(tmpPath)
		return nil
	}

	objPath := s.ObjectPath(hash)
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("create prefix dir: %w", err)
	}

	if err := os.Rename(tmpPath, objPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to object path: %w", err)
	}
	return nil
}

// ReadObject reads the blob for the given hash from disk.
func (s *ObjectStore) ReadObject(hash string) ([]byte, error) {
	return os.ReadFile(s.ObjectPath(hash))
}

// DeleteObject removes the blob from disk only if refCount is 0.
// The caller must query the DB for the ref count before calling this.
func (s *ObjectStore) DeleteObject(hash string, refCount int) error {
	if refCount > 0 {
		return nil
	}
	path := s.ObjectPath(hash)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete object %s: %w", hash[:12], err)
	}
	return nil
}

// CreateTempFile creates a new temp file in {baseDir}/tmp/ with a UUID name.
func (s *ObjectStore) CreateTempFile() (*os.File, error) {
	name := uuid.New().String()
	path := filepath.Join(s.baseDir, "tmp", name)
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	return f, nil
}

// CleanupTemp removes all files in the tmp/ directory.
// Called on server startup to clean up incomplete transfers from previous runs.
func (s *ObjectStore) CleanupTemp() error {
	tmpDir := filepath.Join(s.baseDir, "tmp")
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return fmt.Errorf("read tmp dir: %w", err)
	}
	for _, e := range entries {
		os.Remove(filepath.Join(tmpDir, e.Name()))
	}
	return nil
}

// VerifyHash computes the SHA-256 hash of a file and returns it as a hex string.
func VerifyHash(path string) (string, error) {
	f, err := os.Open(path)
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

// ValidateRelPath checks that a relative path is safe to store in the DB.
// Rejects absolute paths and paths containing "..".
func ValidateRelPath(relPath string) bool {
	if filepath.IsAbs(relPath) {
		return false
	}
	if strings.Contains(relPath, "..") {
		return false
	}
	return relPath != ""
}
