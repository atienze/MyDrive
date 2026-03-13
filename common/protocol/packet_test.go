package protocol

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// TestServerFileEntryDeviceID verifies that a ServerFileEntry with a DeviceID field
// survives a gob encode/decode round-trip intact.
func TestServerFileEntryDeviceID(t *testing.T) {
	original := ServerFileEntry{
		RelPath:  "docs/notes.txt",
		Hash:     "abc123",
		Size:     1024,
		DeviceID: "device-A",
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(original); err != nil {
		t.Fatalf("gob encode failed: %v", err)
	}

	var decoded ServerFileEntry
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("gob decode failed: %v", err)
	}

	if decoded.DeviceID != "device-A" {
		t.Errorf("DeviceID: got %q, want %q", decoded.DeviceID, "device-A")
	}
	if decoded.RelPath != original.RelPath {
		t.Errorf("RelPath: got %q, want %q", decoded.RelPath, original.RelPath)
	}
	if decoded.Hash != original.Hash {
		t.Errorf("Hash: got %q, want %q", decoded.Hash, original.Hash)
	}
	if decoded.Size != original.Size {
		t.Errorf("Size: got %d, want %d", decoded.Size, original.Size)
	}
}

// TestProtocolVersion asserts that the protocol version constant is 3.
func TestProtocolVersion(t *testing.T) {
	if Version != 3 {
		t.Errorf("Version: got %d, want 3", Version)
	}
}

// TestServerFileEntryGobRoundtrip verifies that a ServerFileListResponse containing
// multiple entries with distinct DeviceIDs round-trips via gob without data loss.
func TestServerFileEntryGobRoundtrip(t *testing.T) {
	original := ServerFileListResponse{
		Files: []ServerFileEntry{
			{RelPath: "photos/img1.jpg", Hash: "hash1", Size: 2048, DeviceID: "device-A"},
			{RelPath: "photos/img2.jpg", Hash: "hash2", Size: 4096, DeviceID: "device-B"},
		},
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(original); err != nil {
		t.Fatalf("gob encode failed: %v", err)
	}

	var decoded ServerFileListResponse
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("gob decode failed: %v", err)
	}

	if len(decoded.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(decoded.Files))
	}

	tests := []struct {
		idx      int
		wantPath string
		wantID   string
	}{
		{0, "photos/img1.jpg", "device-A"},
		{1, "photos/img2.jpg", "device-B"},
	}

	for _, tc := range tests {
		f := decoded.Files[tc.idx]
		if f.RelPath != tc.wantPath {
			t.Errorf("Files[%d].RelPath: got %q, want %q", tc.idx, f.RelPath, tc.wantPath)
		}
		if f.DeviceID != tc.wantID {
			t.Errorf("Files[%d].DeviceID: got %q, want %q", tc.idx, f.DeviceID, tc.wantID)
		}
	}
}
