package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func testHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func setupStore(t *testing.T) *ObjectStore {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New(%q) failed: %v", dir, err)
	}
	return s
}

func TestObjectPath(t *testing.T) {
	s := setupStore(t)
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	got := s.ObjectPath(hash)
	want := filepath.Join(s.baseDir, "objects", "ab", "cdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890")
	if got != want {
		t.Errorf("ObjectPath = %q, want %q", got, want)
	}
}

func TestWriteAndHasObject(t *testing.T) {
	s := setupStore(t)
	data := []byte("hello world")
	hash := testHash(data)

	if s.HasObject(hash) {
		t.Fatal("HasObject should be false before write")
	}

	if err := s.WriteObject(hash, data); err != nil {
		t.Fatalf("WriteObject failed: %v", err)
	}

	if !s.HasObject(hash) {
		t.Fatal("HasObject should be true after write")
	}
}

func TestWriteObjectDedup(t *testing.T) {
	s := setupStore(t)
	data := []byte("duplicate content")
	hash := testHash(data)

	if err := s.WriteObject(hash, data); err != nil {
		t.Fatalf("first WriteObject failed: %v", err)
	}
	if err := s.WriteObject(hash, data); err != nil {
		t.Fatalf("second WriteObject (dedup) failed: %v", err)
	}

	got, err := s.ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("ReadObject = %q, want %q", got, data)
	}
}

func TestStoreFromTemp(t *testing.T) {
	s := setupStore(t)
	data := []byte("temp file content")
	hash := testHash(data)

	tmpFile, err := s.CreateTempFile()
	if err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write(data)
	tmpFile.Close()

	if err := s.StoreFromTemp(hash, tmpPath); err != nil {
		t.Fatalf("StoreFromTemp failed: %v", err)
	}

	if !s.HasObject(hash) {
		t.Fatal("HasObject should be true after StoreFromTemp")
	}

	// temp file should be gone (renamed)
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not exist after StoreFromTemp")
	}
}

func TestStoreFromTempDedup(t *testing.T) {
	s := setupStore(t)
	data := []byte("dedup via temp")
	hash := testHash(data)

	// Write the object first
	if err := s.WriteObject(hash, data); err != nil {
		t.Fatalf("WriteObject failed: %v", err)
	}

	// Now try StoreFromTemp with the same hash — should remove temp, not error
	tmpFile, err := s.CreateTempFile()
	if err != nil {
		t.Fatalf("CreateTempFile failed: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write(data)
	tmpFile.Close()

	if err := s.StoreFromTemp(hash, tmpPath); err != nil {
		t.Fatalf("StoreFromTemp (dedup) failed: %v", err)
	}

	// temp file should be cleaned up
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed on dedup")
	}
}

func TestReadObject(t *testing.T) {
	s := setupStore(t)
	data := []byte("read me back")
	hash := testHash(data)

	if err := s.WriteObject(hash, data); err != nil {
		t.Fatalf("WriteObject failed: %v", err)
	}

	got, err := s.ReadObject(hash)
	if err != nil {
		t.Fatalf("ReadObject failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("ReadObject = %q, want %q", got, data)
	}
}

func TestDeleteObjectZeroRefCount(t *testing.T) {
	s := setupStore(t)
	data := []byte("delete me")
	hash := testHash(data)

	s.WriteObject(hash, data)

	if err := s.DeleteObject(hash, 0); err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}
	if s.HasObject(hash) {
		t.Error("object should be gone after DeleteObject with refCount=0")
	}
}

func TestDeleteObjectNonZeroRefCount(t *testing.T) {
	s := setupStore(t)
	data := []byte("keep me")
	hash := testHash(data)

	s.WriteObject(hash, data)

	if err := s.DeleteObject(hash, 1); err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}
	if !s.HasObject(hash) {
		t.Error("object should still exist after DeleteObject with refCount=1")
	}
}

func TestCleanupTemp(t *testing.T) {
	s := setupStore(t)

	// Create a few temp files
	for i := 0; i < 3; i++ {
		f, err := s.CreateTempFile()
		if err != nil {
			t.Fatalf("CreateTempFile failed: %v", err)
		}
		f.Close()
	}

	tmpDir := filepath.Join(s.baseDir, "tmp")
	entries, _ := os.ReadDir(tmpDir)
	if len(entries) != 3 {
		t.Fatalf("expected 3 temp files, got %d", len(entries))
	}

	if err := s.CleanupTemp(); err != nil {
		t.Fatalf("CleanupTemp failed: %v", err)
	}

	entries, _ = os.ReadDir(tmpDir)
	if len(entries) != 0 {
		t.Errorf("expected 0 temp files after cleanup, got %d", len(entries))
	}
}

func TestValidateRelPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"Documents/resume.pdf", true},
		{"file.txt", true},
		{"a/b/c/d.txt", true},
		{"", false},
		{"/absolute/path", false},
		{"../escape", false},
		{"foo/../../etc/passwd", false},
	}
	for _, tt := range tests {
		if got := ValidateRelPath(tt.path); got != tt.want {
			t.Errorf("ValidateRelPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
