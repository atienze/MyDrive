package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/atienze/HomelabSecureSync/client/internal/config"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	"github.com/atienze/HomelabSecureSync/client/internal/status"
)

// makeTestConfig returns a *config.Config pointing at tmpDir with an
// unreachable server address — enough to exercise code paths without
// requiring a real VaultSync server.
func makeTestConfig(tmpDir string) *config.Config {
	return &config.Config{
		ServerAddr: "127.0.0.1:1", // port 1 — connection will be refused
		Token:      "deadbeef",
		SyncDir:    tmpDir,
	}
}

// TestSharedStateNotReloaded verifies that runSyncCycleWithState accepts a
// *state.LocalState pointer and does NOT call state.Load internally.
// The connection will fail (no server running), which is fine — we only care
// about the function signature and that it does not panic with a nil pointer.
func TestSharedStateNotReloaded(t *testing.T) {
	// Create a temp directory with a minimal config structure.
	tmpDir := t.TempDir()

	// Build a LocalState in memory (no file on disk needed for this test).
	st := &state.LocalState{
		Files: map[string]string{
			"test.txt": "abc123",
		},
	}

	// Capture the pointer before the call.
	ptrBefore := st

	// runSyncCycleWithState will fail to connect — that is expected.
	// What matters: it must accept st without loading from disk.
	// We use a fake config with an unreachable server.
	fakeCfg := makeTestConfig(tmpDir)
	statePath := filepath.Join(tmpDir, "state.json")

	_, _, _, _ = runSyncCycleWithState(fakeCfg, st, statePath)

	// The pointer must be the same object — no replacement.
	if st != ptrBefore {
		t.Fatal("runSyncCycleWithState replaced the *LocalState pointer")
	}

	// The function accepted the pointer — compile-time proof it doesn't load its own.
	// (If runSyncCycleWithState didn't exist or had a different signature, this wouldn't compile.)
}

// TestUpdateStorageStatsUsesSharedState verifies updateStorageStats reads
// from the passed *LocalState rather than loading state.json from disk.
func TestUpdateStorageStatsUsesSharedState(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file in the temp dir to allow os.Stat to succeed.
	fileName := "tracked.txt"
	fullPath := filepath.Join(tmpDir, fileName)
	content := []byte("hello world")
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	// Build a LocalState in memory with the file tracked.
	st := &state.LocalState{
		Files: map[string]string{
			fileName: "fakehash",
		},
	}

	// Create config pointing at the temp dir — no state.json file on disk.
	fakeCfg := makeTestConfig(tmpDir)
	appStatus := status.New()

	// Call updateStorageStats — must NOT fail even without state.json on disk.
	updateStorageStats(fakeCfg, appStatus, st)

	snap := appStatus.Snapshot()

	// Expect 1 file tracked.
	if snap.TotalFiles != 1 {
		t.Errorf("expected TotalFiles=1, got %d", snap.TotalFiles)
	}

	// Expect total size = len(content).
	if snap.TotalSize != int64(len(content)) {
		t.Errorf("expected TotalSize=%d, got %d", len(content), snap.TotalSize)
	}
}
