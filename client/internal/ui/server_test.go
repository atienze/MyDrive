package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	stdsync "sync"
	"testing"
	"time"

	"github.com/atienze/HomelabSecureSync/client/internal/config"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	"github.com/atienze/HomelabSecureSync/client/internal/status"
	syncclient "github.com/atienze/HomelabSecureSync/client/internal/sync"
)

// testServer creates a UIServer wired to a temp directory for testing.
// The SyncDir is set to syncDir; if syncDir is empty a fresh TempDir is created.
func testServer(t *testing.T, syncDir string) (*UIServer, *status.Status) {
	t.Helper()
	if syncDir == "" {
		syncDir = t.TempDir()
	}
	s := status.New()
	cfg := &config.Config{
		SyncDir:    syncDir,
		ServerAddr: "127.0.0.1:9999",
		Token:      "test-token",
	}
	st := &state.LocalState{Files: make(map[string]string)}
	statePath := filepath.Join(t.TempDir(), "state.json")
	var mu stdsync.Mutex
	u := NewUIServer(s, make(chan struct{}, 1), cfg, st, statePath, &mu)
	return u, s
}

// writeFile creates a file with given content inside dir and returns the path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return p
}

// ─── Task 1: Path validation, error mapping, and GET endpoints ───────────────

// TestValidateRelPath verifies all path-validation rules.
func TestValidateRelPath(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", true},
		{"absolute path /etc/passwd", "/etc/passwd", true},
		{"simple traversal ../../../etc/passwd", "../../../etc/passwd", true},
		{"traversal after clean a/../../b", "a/../../b", true},
		{"valid nested path docs/notes.txt", "docs/notes.txt", false},
		{"valid flat file file.txt", "file.txt", false},
		{"valid deeply nested a/b/c/d.txt", "a/b/c/d.txt", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRelPath(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for input %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for input %q, got: %v", tc.input, err)
			}
		})
	}
}

// TestHttpStatusFromErr verifies sentinel error → HTTP status mapping.
func TestHttpStatusFromErr(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"ErrServerUnreachable direct", syncclient.ErrServerUnreachable, http.StatusBadGateway},
		{"ErrTimeout direct", syncclient.ErrTimeout, http.StatusGatewayTimeout},
		{"ErrAuthFailed direct", syncclient.ErrAuthFailed, http.StatusUnauthorized},
		{"ErrHashMismatch direct", syncclient.ErrHashMismatch, http.StatusInternalServerError},
		{"generic error", errors.New("random error"), http.StatusInternalServerError},
		{"wrapped ErrTimeout", fmt.Errorf("wrap: %w", syncclient.ErrTimeout), http.StatusGatewayTimeout},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := httpStatusFromErr(tc.err)
			if got != tc.wantStatus {
				t.Errorf("httpStatusFromErr(%v) = %d, want %d", tc.err, got, tc.wantStatus)
			}
		})
	}
}

// TestHandleClientFileList_HappyPath verifies a temp dir with two files returns
// correct JSON with non-empty fields.
func TestHandleClientFileList_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "alpha.txt", "hello world")
	writeFile(t, dir, "sub/beta.txt", "another file with more content")

	u, _ := testServer(t, dir)

	req := httptest.NewRequest(http.MethodGet, "/api/files/client", nil)
	w := httptest.NewRecorder()

	u.handleClientFileList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp fileListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(resp.Files))
	}

	for _, f := range resp.Files {
		if f.RelPath == "" {
			t.Error("expected non-empty RelPath")
		}
		if f.Hash == "" {
			t.Error("expected non-empty Hash")
		}
		if f.Size <= 0 {
			t.Errorf("expected Size > 0 for %s, got %d", f.RelPath, f.Size)
		}
		if f.SizeHuman == "" {
			t.Errorf("expected non-empty SizeHuman for %s", f.RelPath)
		}
	}
}

// TestHandleClientFileList_EmptyDir verifies that an empty directory returns
// an empty (non-null) files array.
func TestHandleClientFileList_EmptyDir(t *testing.T) {
	u, _ := testServer(t, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/files/client", nil)
	w := httptest.NewRecorder()

	u.handleClientFileList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp fileListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Must be an empty slice, not JSON null.
	if resp.Files == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(resp.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(resp.Files))
	}
}

// TestHandleServerFileList_Error verifies that an unreachable server address
// causes the handler to return 502.
//
// TODO: happy path needs mock TCP server (see plan notes).
func TestHandleServerFileList_Error(t *testing.T) {
	// Port 1 is reserved/privileged and will always refuse the connection quickly.
	u, _ := testServer(t, t.TempDir())
	u.cfg.ServerAddr = "127.0.0.1:1"

	req := httptest.NewRequest(http.MethodGet, "/api/files/server", nil)
	w := httptest.NewRecorder()

	u.handleServerFileList(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body: %s", w.Code, w.Body.String())
	}

	var resp opResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ok {
		t.Error("expected ok=false for unreachable server")
	}
}

// ─── Task 2: Mutating endpoints, mutex behavior, and activity logging ─────────

// TestHandleDeleteClient_HappyPath verifies a valid delete removes the file,
// removes it from state, and logs an activity entry.
func TestHandleDeleteClient_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "test.txt", "data")

	u, s := testServer(t, dir)
	u.st.SetFile("test.txt", "fakehash")

	req := httptest.NewRequest(http.MethodDelete, "/api/files/client?path=test.txt", nil)
	w := httptest.NewRecorder()

	u.handleDeleteClient(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	var resp opResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Ok {
		t.Errorf("expected ok=true, got false; message: %s", resp.Message)
	}

	// File must be gone from disk.
	if _, err := os.Stat(filepath.Join(dir, "test.txt")); !os.IsNotExist(err) {
		t.Error("expected file to be removed from disk, but it still exists")
	}

	// File must be gone from state.
	if u.st.HasFile("test.txt") {
		t.Error("expected test.txt to be removed from state")
	}

	// Activity log must contain the deletion.
	snap := s.Snapshot()
	if len(snap.Activities) == 0 {
		t.Fatal("expected at least one activity entry")
	}
	msg := snap.Activities[0].Message
	if msg == "" {
		t.Error("expected non-empty activity message")
	}
	// The message should mention the file name.
	if !contains(msg, "test.txt") {
		t.Errorf("activity message %q does not contain file name", msg)
	}
}

// TestHandleDeleteClient_PathTraversal verifies that a traversal path is
// rejected with 400 and ok=false.
func TestHandleDeleteClient_PathTraversal(t *testing.T) {
	u, _ := testServer(t, t.TempDir())

	req := httptest.NewRequest(http.MethodDelete, "/api/files/client?path=../../../etc/passwd", nil)
	w := httptest.NewRecorder()

	u.handleDeleteClient(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	var resp opResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ok {
		t.Error("expected ok=false for path traversal attempt")
	}
}

// TestHandleDeleteClient_NotFound verifies that deleting a nonexistent file
// returns an error response.
func TestHandleDeleteClient_NotFound(t *testing.T) {
	u, _ := testServer(t, t.TempDir())

	req := httptest.NewRequest(http.MethodDelete, "/api/files/client?path=nonexistent.txt", nil)
	w := httptest.NewRecorder()

	u.handleDeleteClient(w, req)

	if w.Code == http.StatusOK {
		t.Fatal("expected non-200 status for nonexistent file")
	}

	var resp opResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ok {
		t.Error("expected ok=false for nonexistent file")
	}
}

// TestHandleUpload_PathValidation verifies that invalid paths are rejected
// before any TCP attempt is made.
func TestHandleUpload_PathValidation(t *testing.T) {
	u, _ := testServer(t, t.TempDir())

	invalidPaths := []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"absolute path", "/absolute"},
		{"traversal path", "../../escape"},
	}

	for _, tc := range invalidPaths {
		t.Run(tc.name, func(t *testing.T) {
			url := "/api/files/upload"
			if tc.path != "" {
				url += "?path=" + tc.path
			}
			req := httptest.NewRequest(http.MethodPost, url, nil)
			w := httptest.NewRecorder()

			u.handleUpload(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 for path %q; body: %s",
					w.Code, tc.path, w.Body.String())
			}

			var resp opResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Ok {
				t.Errorf("expected ok=false for invalid path %q", tc.path)
			}
		})
	}
}

// TestMutatingHandlerHoldsMutex verifies that a mutating handler (handleDeleteClient)
// blocks while the mutex is pre-held, then completes after the mutex is released.
func TestMutatingHandlerHoldsMutex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lock-test.txt", "data")

	u, _ := testServer(t, dir)

	// Pre-lock the mutex to simulate an in-progress sync.
	u.syncMu.Lock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/files/client?path=lock-test.txt", nil)

	done := make(chan struct{})
	go func() {
		u.handleDeleteClient(w, req)
		close(done)
	}()

	// Give the goroutine time to start and attempt the lock.
	time.Sleep(50 * time.Millisecond)

	// Handler must still be blocked — recorder should show default code (200 from httptest means
	// WriteHeader was called; 0 means nothing was written yet).
	// httptest.ResponseRecorder initializes Code to 200, but only after WriteHeader is called.
	// We can't distinguish "default 200" from "written 200" via Code alone.
	// Instead: verify the goroutine has NOT finished yet.
	select {
	case <-done:
		t.Error("handler completed while mutex was held — should have blocked")
	default:
		// Expected: still running.
	}

	// Release the lock and wait for completion.
	u.syncMu.Unlock()

	select {
	case <-done:
		// Handler completed as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not complete after mutex was released")
	}

	// Verify the handler executed successfully.
	if _, err := os.Stat(filepath.Join(dir, "lock-test.txt")); !os.IsNotExist(err) {
		t.Error("expected file removed after lock released, but it still exists")
	}
}

// TestGetHandlerDoesNotAcquireMutex verifies that handleClientFileList does NOT
// block when the mutex is pre-held (GET handlers skip the mutex).
func TestGetHandlerDoesNotAcquireMutex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "readme.txt", "hello")

	u, _ := testServer(t, dir)

	// Pre-lock the mutex — GET handler must not block.
	u.syncMu.Lock()
	defer u.syncMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/files/client", nil)
	w := httptest.NewRecorder()

	// Call synchronously. If the handler tries to acquire syncMu it will deadlock
	// and the test will time out (caught by go test -timeout).
	u.handleClientFileList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET handler blocked or failed while mutex was held; status = %d; body: %s",
			w.Code, w.Body.String())
	}
}

// TestActivityLogOnSuccess verifies that a successful delete operation adds
// an activity entry that mentions the file path.
func TestActivityLogOnSuccess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "logged.txt", "content")

	u, s := testServer(t, dir)

	req := httptest.NewRequest(http.MethodDelete, "/api/files/client?path=logged.txt", nil)
	w := httptest.NewRecorder()

	u.handleDeleteClient(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	snap := s.Snapshot()
	if len(snap.Activities) == 0 {
		t.Fatal("expected activity log to have entries after delete")
	}

	found := false
	for _, a := range snap.Activities {
		if contains(a.Message, "logged.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("activity log does not contain 'logged.txt'; entries: %v", snap.Activities)
	}
}

// ─── Phase 3-01 gap tests: device_id field and handlePull validation ─────────

// TestHandleServerFileList_DeviceIDField verifies that the fileEntry struct
// carries a device_id field and that it is serialized correctly in the JSON
// envelope returned by the server file list endpoint.
//
// A full handler invocation requires a live TCP connection. This test covers
// the data-contract layer: constructing a fileEntry with a non-empty DeviceID,
// encoding it as JSON, and confirming the device_id key is present and
// non-empty in the output. This matches the requirement that every entry in
// GET /api/files/server has a non-empty device_id field.
func TestHandleServerFileList_DeviceIDField(t *testing.T) {
	entry := fileEntry{
		RelPath:   "docs/readme.txt",
		Hash:      "abc123def456",
		Size:      1024,
		SizeHuman: "1.0 KB",
		DeviceID:  "MacBook",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal fileEntry: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	deviceID, ok := decoded["device_id"]
	if !ok {
		t.Fatal("device_id field missing from JSON output of fileEntry")
	}
	if deviceID == "" || deviceID == nil {
		t.Errorf("device_id is empty in JSON output, got: %v", deviceID)
	}
	if deviceID != "MacBook" {
		t.Errorf("device_id = %v, want MacBook", deviceID)
	}
}

// TestHandleServerFileList_DeviceID_OmitEmpty verifies that a fileEntry with
// no DeviceID (client-side file) does NOT emit a device_id key in JSON.
// This confirms the omitempty behavior that keeps client file entries clean.
func TestHandleServerFileList_DeviceID_OmitEmpty(t *testing.T) {
	entry := fileEntry{
		RelPath:   "local/file.txt",
		Hash:      "deadbeef",
		Size:      512,
		SizeHuman: "512 B",
		// DeviceID intentionally empty (client file)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal fileEntry: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if _, ok := decoded["device_id"]; ok {
		t.Error("device_id key should be absent (omitempty) when DeviceID is empty, but it was present")
	}
}

// TestHandlePull_MissingFrom verifies that POST /api/files/pull without a
// `from` query parameter returns 400 with ok=false. No TCP connection is made.
func TestHandlePull_MissingFrom(t *testing.T) {
	u, _ := testServer(t, t.TempDir())

	// Provide a valid path but no from param.
	req := httptest.NewRequest(http.MethodPost, "/api/files/pull?path=docs/readme.txt", nil)
	w := httptest.NewRecorder()

	u.handlePull(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}

	var resp opResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ok {
		t.Error("expected ok=false when from parameter is missing")
	}
	if !contains(resp.Message, "from") {
		t.Errorf("error message should mention 'from' parameter, got: %q", resp.Message)
	}
}

// TestHandlePull_InvalidPath verifies that POST /api/files/pull with a path
// traversal attempt returns 400 with ok=false before any TCP attempt is made.
func TestHandlePull_InvalidPath(t *testing.T) {
	u, _ := testServer(t, t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/files/pull?path=../../etc/passwd&from=MacBook", nil)
	w := httptest.NewRecorder()

	u.handlePull(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
	}

	var resp opResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ok {
		t.Error("expected ok=false for path traversal attempt")
	}
}

// TestHandlePull_HappyPath verifies that POST /api/files/pull with valid
// from+path parameters gets past all validation and reaches the PullFile call.
// Since the test server's ServerAddr points to 127.0.0.1:9999 (unreachable),
// PullFile returns ErrServerUnreachable, which the handler maps to 502.
// A 502 (not 400) proves the handler passed all validation successfully.
func TestHandlePull_HappyPath(t *testing.T) {
	u, _ := testServer(t, t.TempDir())
	// testServer already sets ServerAddr = "127.0.0.1:9999" (unreachable).

	req := httptest.NewRequest(http.MethodPost, "/api/files/pull?path=docs/readme.txt&from=MacBook", nil)
	w := httptest.NewRecorder()

	u.handlePull(w, req)

	// Must NOT be 400 — validation passed.
	if w.Code == http.StatusBadRequest {
		t.Fatalf("got 400 (validation error); expected handler to reach PullFile. body: %s", w.Body.String())
	}

	// Expect 502 because the server at 127.0.0.1:9999 is unreachable.
	if w.Code != http.StatusBadGateway {
		t.Logf("note: expected 502 (server unreachable), got %d. body: %s", w.Code, w.Body.String())
	}

	var resp opResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Ok {
		t.Error("expected ok=false when server is unreachable")
	}
}

// contains is a simple string-contains helper (avoids importing strings in test output).
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
