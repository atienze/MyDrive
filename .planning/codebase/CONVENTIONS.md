# Coding Conventions

**Analysis Date:** 2025-03-11

## Naming Patterns

**Files:**
- Package names: lowercase single word (`scanner`, `store`, `sender`, `config`, `status`, `crypto`)
- Test files: `{module}_test.go` suffix (e.g., `store_test.go`)
- Entry point: `cmd/main.go`
- Internal modules: `internal/{package}` pattern

**Functions:**
- PascalCase for exported functions: `ScanDirectory`, `SendFile`, `VerifyFile`, `NewSyncer`, `RunFullSync`
- camelCase for unexported functions: `uploadPhase`, `downloadPhase`, `sendDeleteFile`, `createTables`
- Handler functions: `handleDashboard`, `handleStatus`, `handleForceSync`, `handleConnection`

**Variables:**
- camelCase for all variables: `files`, `currentFile`, `rootPath`, `relPath`, `deviceName`
- Acronyms in variables: lowercase except at start (e.g., `encoder`, `decoder`, `hash` not `Hash`)
- Short names acceptable in loops: `f`, `n`, `i` used in standard contexts
- Descriptive multi-word: `currentFileSize`, `currentHasher`, `lastSyncTime`, `forceSyncCh`

**Types (Structs):**
- PascalCase: `ObjectStore`, `FileMeta`, `FileTransfer`, `CheckFileRequest`, `LocalState`, `FileRecord`, `Config`, `Status`, `ActivityEntry`, `StatusSnapshot`, `Syncer`, `UIServer`
- Field names: PascalCase for exported (JSON/TOML), matching tags for serialization: `RelPath`, `Hash`, `Size`, `Token`, `ServerAddr`, `SyncDir`

**Constants:**
- UPPERCASE_SNAKE_CASE for private constants: `maxActivities = 50`
- Constants for protocol commands: `CmdPing`, `CmdSendFile`, `CmdCheckFile`, etc. (mixed case for command constants)
- Magic numbers: `4*1024*1024` for 4MB chunks (kept inline rather than named constant)
- Size unit constants use const block: `KB`, `MB`, `GB` in `status.go` FormatSize function

**Interface Names:**
- Use `Error` suffix: `error` interface is Go's built-in; no custom interfaces defined in codebase

## Code Style

**Formatting:**
- Standard Go formatting via implicit gofmt (go build uses gofmt style automatically)
- No `.editorconfig` or `.prettierrc` found; relies on Go defaults
- Indentation: tabs (Go standard)
- Line length: no strict limit observed, but files stay readable

**Linting:**
- No linter configuration found (no `.golangci.yml`, `.eslintrc`)
- Code follows Go idioms: error handling via `if err != nil`, resource cleanup via `defer`

## Import Organization

**Order:**
1. Standard library imports (stdlib, e.g., `fmt`, `os`, `log`)
2. External third-party (e.g., `github.com/google/uuid`, `github.com/BurntSushi/toml`, `modernc.org/sqlite`)
3. Internal module imports (e.g., `github.com/atienze/HomelabSecureSync/...`)

**Example from `client/internal/sync/bidirectional.go`:**
```go
import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/atienze/HomelabSecureSync/client/internal/scanner"
	sender "github.com/atienze/HomelabSecureSync/client/internal/sender"
	"github.com/atienze/HomelabSecureSync/client/internal/state"
	"github.com/atienze/HomelabSecureSync/common/protocol"
)
```

**Path Aliases:**
- Used for disambiguation: `sender "github.com/atienze/HomelabSecureSync/client/internal/sender"`
- Used as `bisync` in `client/cmd/main.go` for clarity when importing `sync` package (conflicts with stdlib)

## Error Handling

**Patterns:**
- Explicit check: `if err != nil { return ... }`
- Error wrapping with context: `fmt.Errorf("operation name: %w", err)` throughout codebase
- Meaningful context in wrapped errors: `fmt.Errorf("upload phase: %w", err)`
- Early returns on error (no nested try-catch equivalent)
- Partial progress persisted on error: `s.state.Save(s.statePath)` even if download phase fails (see `client/internal/sync/bidirectional.go` line 52-54)
- Non-fatal errors logged with `log.Printf` and execution continues (e.g., hash computation failures in scanner)
- Fatal errors use `log.Fatalf` to exit: `log.Fatalf("Configuration error: %v", err)`

**Example from `server/internal/db/db.go`:**
```go
if err := conn.Ping(); err != nil {
    return nil, fmt.Errorf("failed to connect to database: %w", err)
}
```

## Logging

**Framework:** Go's standard `log` package (no external logger)

**Patterns:**
- `log.Printf` for informational/warning messages
- `log.Fatalf` for fatal errors (exits immediately)
- Prefix-style messages: `log.Printf("Device %s: operation", deviceName)`
- Contextual info: `log.Printf("Scanning directory: %s\n", rootPath)`
- Status messages via `fmt.Printf` for user-facing output (not log package)
- UI updates via `appStatus.AddActivity` for daemon-mode events

**Example from `server/internal/receiver/handler.go`:**
```go
log.Printf("New connection from: %s", conn.RemoteAddr().String())
log.Printf("Authenticated device: %s (from %s)", deviceName, conn.RemoteAddr())
```

## Comments

**When to Comment:**
- Algorithm explanation or non-obvious logic: see `server/internal/db/db.go` lines 46-49 explaining SQLite quirk
- SQL comment blocks in schema: `-- Tracks every file the server has received`
- Phase references: `// Phase 2 Auth:`, `// Phase 4:` mark protocol version boundaries
- Caveats and safety nets: `// Safety net: if DB says the file exists but the blob is missing...`
- Caller responsibility notes: `// The caller is responsible for verifying the hash before calling this.`

**JSDoc/TSDoc:**
- Not used; this is Go, not TypeScript
- Doc comments (godoc style) used for exported types and functions:

**Example from `server/internal/store/store.go`:**
```go
// ObjectStore manages content-addressable blob storage.
// Files are stored at {baseDir}/objects/{hash[:2]}/{hash[2:]}.
type ObjectStore struct {
	baseDir string
}

// New creates an ObjectStore rooted at baseDir.
// It ensures the objects/ and tmp/ subdirectories exist.
func New(baseDir string) (*ObjectStore, error) {
```

## Function Design

**Size:** Functions are generally 10-60 lines; larger operations split across phases or helper functions

**Parameters:**
- Passed by value for primitives and small structs
- Passed by pointer for receiver methods: `func (s *ObjectStore) HasObject(hash string) bool`
- Pointers for large/mutable structs: `func (s *LocalState) Save(path string) error`
- Multiple parameters accepted inline (no parameter objects in most cases, except protocol types like `FileTransfer`)

**Return Values:**
- Go idiom: return value + error tuple: `func (s *LocalState) Load(path string) (*LocalState, error)`
- Single value for simple queries: `func (s *ObjectStore) HasObject(hash string) bool` returns bool only
- Multiple return values for counts: `func (s *Syncer) RunFullSync() (uploaded, downloaded, deleted int, err error)`
- Error as last return value always

**Example from `server/internal/db/db.go`:**
```go
// GetDeviceName looks up the human-readable name for a registered token.
// Returns ("", false, nil) if the token is not found (unregistered).
// Returns (name, true, nil) on success.
// Returns ("", false, err) on a database error.
func (db *DB) GetDeviceName(token string) (string, bool, error) {
```

## Module Design

**Exports:**
- Package-level functions and types are exported (PascalCase)
- Helper functions unexported (camelCase)
- Struct fields exported if needed for serialization (JSON/TOML tags)

**Barrel Files:**
- Not used; imports are explicit (`import "github.com/atienze/HomelabSecureSync/server/internal/db"`)
- Each package is imported directly, no aggregator files

**Example of clean module interface from `client/internal/scanner/scan.go`:**
```go
// FileMeta represents one file we found
type FileMeta struct {
	Path string
	Hash string
	Size int64
}

// ScanDirectory walks through a folder and fingerprints every file
func ScanDirectory(rootPath string) ([]FileMeta, error) {
```

## Defer Usage

**Resource Cleanup:**
- All file opens: `defer file.Close()`
- All DB queries: `defer rows.Close()`
- Network connections: `defer conn.Close()` in handlers
- Temp file cleanup on error: `defer os.Remove(tmpPath)` in function bodies before potential error

**Example from `client/internal/state/state.go`:**
```go
if err := tmp.Close(); err != nil {
    os.Remove(tmpPath)
    return fmt.Errorf("close temp state file: %w", err)
}
```

## Concurrency Patterns

**Synchronization:**
- `sync.RWMutex` used for protecting shared state: `Status` type in `client/internal/status/status.go`
- Lock/Unlock pattern: `s.mu.Lock(); defer s.mu.Unlock()`
- Channels for signaling: `forceSyncCh chan struct{}` for triggering syncs in daemon mode
- Non-blocking send on channel: `select { case ch <- val: default: }` to avoid blocking daemon loop

**Example from `client/internal/status/status.go`:**
```go
func (s *Status) AddActivity(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// ... mutation
}
```

**Example of snapshot pattern for thread-safe reading:**
```go
func (s *Status) Snapshot() StatusSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	acts := make([]ActivityEntry, len(s.activities))
	copy(acts, s.activities)  // Deep copy for safety
	return StatusSnapshot{...}
}
```

## Go Workspace Structure

**Modules:**
- Three modules in a workspace: `common/`, `client/`, `server/`
- Each has its own `go.mod`
- Workspace file binds them together for coordinated builds
- Import paths: `github.com/atienze/HomelabSecureSync/{common,client,server}/...`

**Build Requirement:**
- Build commands must be run from workspace root so `go build` resolves relative module imports correctly
- `go build ./server/cmd` from root works; from subdirectory may fail

---

*Convention analysis: 2025-03-11*
