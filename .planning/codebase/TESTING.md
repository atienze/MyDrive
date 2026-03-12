# Testing

## Framework

Standard Go `testing` package. No external test frameworks (testify, gomega, etc.).

## Test Files

| File | Package | Tests | Approach |
|------|---------|-------|----------|
| `client/cmd/main_test.go` | `main` | 2 | Integration: shared state behavior |
| `client/internal/sync/operations_test.go` | `sync` | 7+ | Mock TCP server, protocol exchange |
| `client/internal/ui/server_test.go` | `ui` | 10+ | httptest, handler behavior |
| `server/internal/store/store_test.go` | `store` | 10+ | File I/O, dedup, ref counting |

## Test Patterns

### Mock TCP Server (`operations_test.go`)

Tests create a real TCP listener on a random port, accept one connection, and run a handler function:

```go
func mockServer(t *testing.T, handler func(conn net.Conn)) (addr string, cleanup func()) {
    ln, _ := net.Listen("tcp", "127.0.0.1:0")
    done := make(chan struct{})
    go func() {
        defer close(done)
        conn, _ := ln.Accept()
        handler(conn)
    }()
    return ln.Addr().String(), func() {
        ln.Close()
        <-done
    }
}
```

Protocol exchange helper:
```go
type serverConn struct { enc *gob.Encoder; dec *gob.Decoder }
func (sc *serverConn) readPacket(t *testing.T) protocol.Packet
func (sc *serverConn) sendPacket(t *testing.T, p protocol.Packet)
```

### HTTP Handler Tests (`server_test.go`)

Uses `httptest.NewRequest` + `httptest.NewRecorder`:

```go
req := httptest.NewRequest(http.MethodPost, "/api/files/upload?path=test.txt", nil)
w := httptest.NewRecorder()
u.handleUpload(w, req)
// Assert w.Code, parse w.Body
```

### Temp Directory Tests (`store_test.go`)

Uses `t.TempDir()` for isolated file system tests — auto-cleaned after test:

```go
func setupStore(t *testing.T) *ObjectStore {
    dir := t.TempDir()
    s, err := New(dir)
    // ...
    return s
}
```

## Test Helpers

| Helper | Location | Purpose |
|--------|----------|---------|
| `testConfig(t, addr)` | `operations_test.go` | Config with temp SyncDir |
| `testServer(t, syncDir)` | `server_test.go` | UIServer with temp dir and mock deps |
| `writeFile(t, dir, name, content)` | `server_test.go` | Create file in test dir |
| `mockServer(t, handler)` | `operations_test.go` | TCP listener for one connection |
| `encodePayload(t, v)` | `operations_test.go` | Gob encode value to bytes |
| `sha256Hex(data)` | `operations_test.go` | Quick hash computation |
| `testHash(data)` | `store_test.go` | Quick hash computation |
| `setupStore(t)` | `store_test.go` | ObjectStore in temp dir |

## What's Tested

### Well-Covered

- **Protocol handshake** — magic number, version, token exchange
- **Single-file operations** — upload, download, delete via mock TCP
- **HTTP endpoints** — path validation, status codes, mutex acquisition
- **Object store** — write, read, dedup, ref counting, safe deletion, temp cleanup
- **Path validation** — traversal attacks (`..`), absolute paths, empty strings
- **Static analysis** — `TestNoBareNetDial` verifies all TCP connections use `net.DialTimeout` (not bare `net.Dial`)

### Partially Covered

- **Full sync cycle** — tested at integration level (cmd/main_test.go) but not unit-level
- **Server handler** — no direct unit tests (tested indirectly via operations_test.go mock servers)
- **Database** — no unit tests (schema creation, queries)

### Not Tested

- **Server command handler** end-to-end integration
- **Race conditions** (`-race` flag not routinely used)
- **Dashboard JavaScript** (not testable in Go)
- **Large file chunking** (4MB+ transfers)
- **Concurrent client connections**
- **OS-specific path handling** (Windows paths)
- **Network failure recovery** (partial transfers)

## Running Tests

```bash
go test ./...                              # All tests
go test ./client/internal/sync/...         # Single package
go test -v ./client/internal/ui/...        # Verbose output
go test -run TestNoBareNetDial ./...       # Single test
go test -race ./...                        # Race detector
go test -timeout 30s ./...                 # Custom timeout
go test -count=1 ./...                     # Disable test cache
```

## Test Execution Notes

- Tests use `127.0.0.1:0` for random port allocation (no port conflicts)
- Mock servers handle exactly one connection then shut down
- All temp files/dirs cleaned up via `t.TempDir()` and `t.Cleanup()`
- No external services needed — fully self-contained
