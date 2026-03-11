package sync_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atienze/HomelabSecureSync/client/internal/config"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	syncclient "github.com/atienze/HomelabSecureSync/client/internal/sync"
	"github.com/atienze/HomelabSecureSync/common/protocol"
)

// ----------------------------------------------------------------------------
// Test Helpers
// ----------------------------------------------------------------------------

// testConfig returns a minimal *config.Config pointing to the given server address
// and using a temporary sync directory.
func testConfig(t *testing.T, serverAddr string) (*config.Config, string) {
	t.Helper()
	syncDir := t.TempDir()
	return &config.Config{
		ServerAddr: serverAddr,
		Token:      "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
		SyncDir:    syncDir,
	}, syncDir
}

// mockServer starts a TCP listener on a random local port, accepts exactly one
// connection, reads (and optionally validates) the handshake, then calls the
// provided handler goroutine. It returns the listener address and a cleanup
// function that closes the listener and waits for the handler to finish.
func mockServer(t *testing.T, handler func(conn net.Conn)) (addr string, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mockServer: listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			// Listener closed before a connection arrived — this is expected in
			// tests that close early (e.g. timeout tests that cancel the server).
			return
		}
		handler(conn)
	}()

	return ln.Addr().String(), func() {
		ln.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Log("mockServer: handler did not finish within timeout")
		}
	}
}

// readHandshake reads and returns the gob-encoded Handshake sent by the client.
// It uses the provided serverConn's decoder so the same gob stream is maintained
// for all subsequent packet reads. The Handshake is the first message on the wire,
// gob-encoded directly (not wrapped in a Packet).
func readHandshake(t *testing.T, sc *serverConn) protocol.Handshake {
	t.Helper()
	var shake protocol.Handshake
	if err := sc.dec.Decode(&shake); err != nil {
		t.Errorf("readHandshake: decode: %v", err)
	}
	return shake
}

// encodePayload gob-encodes v into a []byte for use as a Packet.Payload.
func encodePayload(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatalf("encodePayload: %v", err)
	}
	return buf.Bytes()
}

// sendPacket gob-encodes a Packet onto conn using the same gob stream that
// protocol.Packet responses are sent on. Each new call should share the same
// *gob.Encoder instance to avoid re-sending gob type descriptors.
type serverConn struct {
	enc *gob.Encoder
	dec *gob.Decoder
}

func newServerConn(conn net.Conn) *serverConn {
	return &serverConn{
		enc: gob.NewEncoder(conn),
		dec: gob.NewDecoder(conn),
	}
}

func (sc *serverConn) readPacket(t *testing.T) protocol.Packet {
	t.Helper()
	var p protocol.Packet
	if err := sc.dec.Decode(&p); err != nil {
		t.Errorf("serverConn.readPacket: %v", err)
	}
	return p
}

func (sc *serverConn) sendPacket(t *testing.T, p protocol.Packet) {
	t.Helper()
	if err := sc.enc.Encode(p); err != nil {
		t.Errorf("serverConn.sendPacket: %v", err)
	}
}

// sha256Hex returns the SHA-256 hex digest of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ----------------------------------------------------------------------------
// Task 1: DialAndHandshake tests
// ----------------------------------------------------------------------------

// TestDialAndHandshake_Success verifies that DialAndHandshake opens a connection
// and performs the gob-encoded handshake against a real TCP mock listener.
func TestDialAndHandshake_Success(t *testing.T) {
	handshakeCh := make(chan protocol.Handshake, 1)

	addr, cleanup := mockServer(t, func(conn net.Conn) {
		defer conn.Close()
		sc := newServerConn(conn)
		shake := readHandshake(t, sc)
		handshakeCh <- shake
	})
	defer cleanup()

	cfg, _ := testConfig(t, addr)
	conn, enc, dec, err := syncclient.DialAndHandshake(cfg)
	if err != nil {
		t.Fatalf("DialAndHandshake returned unexpected error: %v", err)
	}
	defer conn.Close()

	if enc == nil {
		t.Error("DialAndHandshake returned nil encoder")
	}
	if dec == nil {
		t.Error("DialAndHandshake returned nil decoder")
	}

	// Verify the handshake fields the server received.
	select {
	case shake := <-handshakeCh:
		if shake.MagicNumber != protocol.MagicNumber {
			t.Errorf("MagicNumber = %#x; want %#x", shake.MagicNumber, protocol.MagicNumber)
		}
		if shake.Version != protocol.Version {
			t.Errorf("Version = %d; want %d", shake.Version, protocol.Version)
		}
		if shake.Token != cfg.Token {
			t.Errorf("Token = %q; want %q", shake.Token, cfg.Token)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for handshake to arrive at mock server")
	}
}

// TestDialAndHandshake_Unreachable verifies that dialing a closed port returns
// an error wrapping ErrServerUnreachable.
func TestDialAndHandshake_Unreachable(t *testing.T) {
	// Port 1 is privileged and effectively always refused on macOS/Linux.
	cfg := &config.Config{
		ServerAddr: "127.0.0.1:1",
		Token:      "doesnotmatter",
		SyncDir:    t.TempDir(),
	}

	_, _, _, err := syncclient.DialAndHandshake(cfg)
	if err == nil {
		t.Fatal("DialAndHandshake expected error for unreachable server, got nil")
	}
	if !errors.Is(err, syncclient.ErrServerUnreachable) {
		t.Errorf("errors.Is(err, ErrServerUnreachable) = false; got: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Task 1: FetchServerFileList tests
// ----------------------------------------------------------------------------

// TestFetchServerFileList_Success verifies that FetchServerFileList sends
// CmdListServerFiles and correctly parses the CmdServerFileList response.
func TestFetchServerFileList_Success(t *testing.T) {
	wantFiles := []protocol.ServerFileEntry{
		{RelPath: "docs/notes.txt", Hash: "aabbcc", Size: 1024},
		{RelPath: "photos/cat.jpg", Hash: "ddeeff", Size: 4096},
	}

	addr, cleanup := mockServer(t, func(conn net.Conn) {
		defer conn.Close()

		sc := newServerConn(conn)
		// Step 1: Read and discard the handshake.
		readHandshake(t, sc)

		// Step 2: Expect CmdListServerFiles.
		pkt := sc.readPacket(t)
		if pkt.Cmd != protocol.CmdListServerFiles {
			t.Errorf("expected CmdListServerFiles (%d), got %d", protocol.CmdListServerFiles, pkt.Cmd)
		}

		// Step 3: Respond with CmdServerFileList containing 2 entries.
		sc.sendPacket(t, protocol.Packet{
			Cmd:     protocol.CmdServerFileList,
			Payload: encodePayload(t, protocol.ServerFileListResponse{Files: wantFiles}),
		})
	})
	defer cleanup()

	cfg, _ := testConfig(t, addr)
	files, err := syncclient.FetchServerFileList(cfg)
	if err != nil {
		t.Fatalf("FetchServerFileList returned unexpected error: %v", err)
	}

	if len(files) != len(wantFiles) {
		t.Fatalf("got %d files; want %d", len(files), len(wantFiles))
	}
	for i, got := range files {
		want := wantFiles[i]
		if got.RelPath != want.RelPath || got.Hash != want.Hash || got.Size != want.Size {
			t.Errorf("files[%d] = %+v; want %+v", i, got, want)
		}
	}
}

// ----------------------------------------------------------------------------
// Task 2: UploadSingleFile tests
// ----------------------------------------------------------------------------

// TestUploadSingleFile_Success verifies that UploadSingleFile sends a CmdSendFile
// header and CmdFileChunk data, then updates and persists state.
func TestUploadSingleFile_Success(t *testing.T) {
	const relPath = "subdir/hello.txt"
	content := []byte("hello world")
	wantHash := sha256Hex(content)

	addr, cleanup := mockServer(t, func(conn net.Conn) {
		defer conn.Close()

		sc := newServerConn(conn)
		// Consume handshake.
		readHandshake(t, sc)

		// Expect CmdSendFile header.
		headerPkt := sc.readPacket(t)
		if headerPkt.Cmd != protocol.CmdSendFile {
			t.Errorf("expected CmdSendFile (%d), got %d", protocol.CmdSendFile, headerPkt.Cmd)
		}

		// Decode header and verify fields.
		var ft protocol.FileTransfer
		if err := gob.NewDecoder(bytes.NewBuffer(headerPkt.Payload)).Decode(&ft); err != nil {
			t.Errorf("decode FileTransfer: %v", err)
		}
		if ft.RelPath != relPath {
			t.Errorf("FileTransfer.RelPath = %q; want %q", ft.RelPath, relPath)
		}
		if ft.Hash != wantHash {
			t.Errorf("FileTransfer.Hash = %q; want %q", ft.Hash, wantHash)
		}

		// Expect at least one CmdFileChunk (read until EOF / connection close).
		for {
			chunkPkt := sc.readPacket(t)
			if chunkPkt.Cmd == 0 {
				// readPacket returned zero-value (decode error / conn closed) — done.
				break
			}
			if chunkPkt.Cmd != protocol.CmdFileChunk {
				t.Errorf("expected CmdFileChunk (%d), got %d", protocol.CmdFileChunk, chunkPkt.Cmd)
				break
			}
			// If we got all the data, the connection should close next iteration.
			if int64(len(chunkPkt.Payload)) == ft.Size {
				break
			}
		}
	})
	defer cleanup()

	cfg, syncDir := testConfig(t, addr)

	// Create the file in the sync directory.
	fullPath := filepath.Join(syncDir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	st := &state.LocalState{Files: make(map[string]string)}
	statePath := filepath.Join(t.TempDir(), "state.json")

	err := syncclient.UploadSingleFile(cfg, st, statePath, relPath)
	if err != nil {
		t.Fatalf("UploadSingleFile returned unexpected error: %v", err)
	}

	// Verify in-memory state updated.
	if got := st.Files[relPath]; got != wantHash {
		t.Errorf("st.Files[%q] = %q; want %q", relPath, got, wantHash)
	}

	// Verify state.json was written to disk and contains the relPath.
	loaded, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("state.Load after upload: %v", err)
	}
	if loaded.Files[relPath] != wantHash {
		t.Errorf("persisted state Files[%q] = %q; want %q", relPath, loaded.Files[relPath], wantHash)
	}
}

// ----------------------------------------------------------------------------
// Task 2: DownloadSingleFile tests
// ----------------------------------------------------------------------------

// TestDownloadSingleFile_Success verifies that DownloadSingleFile writes the
// file to disk, verifies the hash, and updates state.
func TestDownloadSingleFile_Success(t *testing.T) {
	const relPath = "downloads/report.pdf"
	content := []byte("pdf content here")
	wantHash := sha256Hex(content)

	addr, cleanup := mockServer(t, func(conn net.Conn) {
		defer conn.Close()

		sc := newServerConn(conn)
		// Consume handshake.
		readHandshake(t, sc)

		// Expect CmdRequestFile.
		pkt := sc.readPacket(t)
		if pkt.Cmd != protocol.CmdRequestFile {
			t.Errorf("expected CmdRequestFile (%d), got %d", protocol.CmdRequestFile, pkt.Cmd)
		}

		// Send CmdFileDataHeader.
		sc.sendPacket(t, protocol.Packet{
			Cmd: protocol.CmdFileDataHeader,
			Payload: encodePayload(t, protocol.FileDataHeader{
				RelPath: relPath,
				Hash:    wantHash,
				Size:    int64(len(content)),
			}),
		})

		// Send one CmdFileDataChunk with all the content.
		sc.sendPacket(t, protocol.Packet{
			Cmd:     protocol.CmdFileDataChunk,
			Payload: content,
		})
	})
	defer cleanup()

	cfg, syncDir := testConfig(t, addr)
	st := &state.LocalState{Files: make(map[string]string)}
	statePath := filepath.Join(t.TempDir(), "state.json")

	err := syncclient.DownloadSingleFile(cfg, st, statePath, relPath)
	if err != nil {
		t.Fatalf("DownloadSingleFile returned unexpected error: %v", err)
	}

	// Verify file was written to disk with correct content.
	got, err := os.ReadFile(filepath.Join(syncDir, relPath))
	if err != nil {
		t.Fatalf("ReadFile after download: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("downloaded content = %q; want %q", got, content)
	}

	// Verify in-memory state updated.
	if st.Files[relPath] != wantHash {
		t.Errorf("st.Files[%q] = %q; want %q", relPath, st.Files[relPath], wantHash)
	}

	// Verify state.json persisted.
	loaded, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("state.Load after download: %v", err)
	}
	if loaded.Files[relPath] != wantHash {
		t.Errorf("persisted state Files[%q] = %q; want %q", relPath, loaded.Files[relPath], wantHash)
	}
}

// TestDownloadSingleFile_HashMismatch verifies that DownloadSingleFile returns
// ErrHashMismatch when the server declares one hash but the content hashes to
// something different. It also verifies no file is left at the final path.
func TestDownloadSingleFile_HashMismatch(t *testing.T) {
	const relPath = "corrupted/file.bin"
	content := []byte("real content")
	// Claim a wrong hash so the verification fails.
	badHash := "0000000000000000000000000000000000000000000000000000000000000000"

	addr, cleanup := mockServer(t, func(conn net.Conn) {
		defer conn.Close()

		sc := newServerConn(conn)
		readHandshake(t, sc)

		// Expect CmdRequestFile.
		pkt := sc.readPacket(t)
		if pkt.Cmd != protocol.CmdRequestFile {
			t.Errorf("expected CmdRequestFile (%d), got %d", protocol.CmdRequestFile, pkt.Cmd)
		}

		// Send header with wrong hash.
		sc.sendPacket(t, protocol.Packet{
			Cmd: protocol.CmdFileDataHeader,
			Payload: encodePayload(t, protocol.FileDataHeader{
				RelPath: relPath,
				Hash:    badHash,
				Size:    int64(len(content)),
			}),
		})

		// Send the actual content (hash will not match the declared badHash).
		sc.sendPacket(t, protocol.Packet{
			Cmd:     protocol.CmdFileDataChunk,
			Payload: content,
		})
	})
	defer cleanup()

	cfg, syncDir := testConfig(t, addr)
	st := &state.LocalState{Files: make(map[string]string)}
	statePath := filepath.Join(t.TempDir(), "state.json")

	err := syncclient.DownloadSingleFile(cfg, st, statePath, relPath)
	if err == nil {
		t.Fatal("DownloadSingleFile expected error for hash mismatch, got nil")
	}
	if !errors.Is(err, syncclient.ErrHashMismatch) {
		t.Errorf("errors.Is(err, ErrHashMismatch) = false; got: %v", err)
	}

	// Verify the temp file was cleaned up — no file at final path.
	finalPath := filepath.Join(syncDir, relPath)
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Errorf("expected file at %q to not exist after hash mismatch, but it does", finalPath)
	}
}

// ----------------------------------------------------------------------------
// Task 2: DeleteServerFile tests
// ----------------------------------------------------------------------------

// TestDeleteServerFile_Success verifies that DeleteServerFile removes the file
// from local state and persists it after the server confirms deletion.
func TestDeleteServerFile_Success(t *testing.T) {
	const relPath = "old/archive.zip"
	const existingHash = "cafebabe0000000000000000000000000000000000000000000000000000dead"

	addr, cleanup := mockServer(t, func(conn net.Conn) {
		defer conn.Close()

		sc := newServerConn(conn)
		readHandshake(t, sc)

		// Expect CmdDeleteFile.
		pkt := sc.readPacket(t)
		if pkt.Cmd != protocol.CmdDeleteFile {
			t.Errorf("expected CmdDeleteFile (%d), got %d", protocol.CmdDeleteFile, pkt.Cmd)
		}

		// Decode and verify request.
		var req protocol.DeleteFileRequest
		if err := gob.NewDecoder(bytes.NewBuffer(pkt.Payload)).Decode(&req); err != nil {
			t.Errorf("decode DeleteFileRequest: %v", err)
		}
		if req.RelPath != relPath {
			t.Errorf("DeleteFileRequest.RelPath = %q; want %q", req.RelPath, relPath)
		}

		// Respond with success.
		sc.sendPacket(t, protocol.Packet{
			Payload: encodePayload(t, protocol.DeleteFileResponse{Success: true, Message: "deleted"}),
		})
	})
	defer cleanup()

	cfg, _ := testConfig(t, addr)
	st := &state.LocalState{Files: map[string]string{relPath: existingHash}}
	statePath := filepath.Join(t.TempDir(), "state.json")

	// Pre-persist state so we can verify it's updated after delete.
	if err := st.Save(statePath); err != nil {
		t.Fatalf("state.Save (pre-test): %v", err)
	}

	err := syncclient.DeleteServerFile(cfg, st, statePath, relPath)
	if err != nil {
		t.Fatalf("DeleteServerFile returned unexpected error: %v", err)
	}

	// Verify in-memory state no longer has the file.
	if _, ok := st.Files[relPath]; ok {
		t.Errorf("st.Files[%q] still present after delete; expected removal", relPath)
	}

	// Verify state.json persisted the removal.
	loaded, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("state.Load after delete: %v", err)
	}
	if _, ok := loaded.Files[relPath]; ok {
		t.Errorf("persisted state still contains %q after delete", relPath)
	}
}

// TestDeleteServerFile_Rejected verifies that an error is returned when the
// server responds with Success: false.
func TestDeleteServerFile_Rejected(t *testing.T) {
	const relPath = "ghost/file.txt"

	addr, cleanup := mockServer(t, func(conn net.Conn) {
		defer conn.Close()

		sc := newServerConn(conn)
		readHandshake(t, sc)

		// Consume the delete request.
		sc.readPacket(t)

		// Respond with rejection.
		sc.sendPacket(t, protocol.Packet{
			Payload: encodePayload(t, protocol.DeleteFileResponse{
				Success: false,
				Message: "file not found",
			}),
		})
	})
	defer cleanup()

	cfg, _ := testConfig(t, addr)
	st := &state.LocalState{Files: make(map[string]string)}
	statePath := filepath.Join(t.TempDir(), "state.json")

	err := syncclient.DeleteServerFile(cfg, st, statePath, relPath)
	if err == nil {
		t.Fatal("DeleteServerFile expected error for rejected delete, got nil")
	}

	// Verify error message contains the rejection context.
	const wantSubstring = "server rejected delete"
	if !containsString(err.Error(), wantSubstring) {
		t.Errorf("error = %q; want it to contain %q", err.Error(), wantSubstring)
	}
}

// ----------------------------------------------------------------------------
// Task 2: Static check for bare net.Dial usage (OPS-06)
// ----------------------------------------------------------------------------

// TestNoBareNetDial is a static check that operations.go does not use net.Dial
// without a timeout (i.e., no bare net.Dial( calls — only net.DialTimeout).
func TestNoBareNetDial(t *testing.T) {
	// Read operations.go relative to this test file.
	srcPath := filepath.Join(testSourceDir(), "operations.go")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", srcPath, err)
	}

	src := string(content)

	// Check that net.DialTimeout appears at least once.
	if !containsString(src, "net.DialTimeout(") {
		t.Error("operations.go: expected net.DialTimeout to be used, but found none")
	}

	// Check that bare net.Dial( (without "Timeout") does NOT appear.
	// We scan for net.Dial( but exclude net.DialTimeout(.
	bare := findBareDial(src)
	if bare {
		t.Error("operations.go: found bare net.Dial( call — must use net.DialTimeout for all connections")
	}
}

// testSourceDir returns the absolute path of the sync package source directory.
// Because tests run with the working directory set to the package directory,
// we can use os.Getwd() to locate the source files.
func testSourceDir() string {
	// During 'go test', cwd is set to the package directory.
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// findBareDial returns true if src contains "net.Dial(" that is NOT followed
// immediately by "Timeout" (i.e., bare net.Dial calls without timeout).
func findBareDial(src string) bool {
	const needle = "net.Dial("
	idx := 0
	for {
		pos := indexString(src[idx:], needle)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		// Check what follows "net.Dial(" — if it starts with "Timeout" it's fine.
		after := src[abs+len("net.Dial("):]
		if !startsWithString(after, "Timeout") {
			return true
		}
		idx = abs + len(needle)
	}
}

// containsString reports whether s contains substr (replaces strings.Contains
// to avoid importing strings in a test-only helper).
func containsString(s, substr string) bool {
	return indexString(s, substr) >= 0
}

// indexString returns the index of the first instance of substr in s, or -1.
func indexString(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}

// startsWithString reports whether s starts with prefix.
func startsWithString(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
