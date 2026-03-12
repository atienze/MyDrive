# Code Conventions

## Error Handling

### Client — Sentinel Errors + Wrapping

`client/internal/sync/operations.go` defines sentinel errors for HTTP status mapping:

```go
var (
    ErrServerUnreachable = errors.New("server unreachable")
    ErrTimeout           = errors.New("operation timed out")
    ErrAuthFailed        = errors.New("authentication failed")
    ErrHashMismatch      = errors.New("hash mismatch after transfer")
)
```

Errors are wrapped with context using `fmt.Errorf`:
```go
return fmt.Errorf("%w: %w", ErrServerUnreachable, err)
```

The UI server maps these to HTTP status codes via `httpStatusFromErr()`:
- `ErrServerUnreachable`, `ErrAuthFailed` → 502
- `ErrTimeout` → 504
- Everything else → 500

### Server — Log and Continue

Server handler (`receiver/handler.go`) logs errors and either continues or closes:
- Command decode error → log, continue to next command
- Auth failure → close connection immediately
- File write error → clean up temp file, continue
- Hash mismatch → log warning, remove temp, continue

No errors are returned to the client (except `DeleteFileResponse` and `FileStatusResponse`).

### General Patterns

- No `panic()` — all errors returned
- Atomic file writes: write to temp → rename (both client and server)
- Cleanup on failure: `os.Remove(tmpFile)` before returning
- Close before rename to avoid file-in-use issues

## Logging

### Client
- `fmt.Printf()` for user-facing output (sync progress, file names)
- `log.Printf()` for warnings and errors
- `status.AddActivity()` for daemon mode activity log (shown in UI)

### Server
- `log.Printf()` throughout (no structured logging)
- Connection context: remote address, device name
- Hash truncation in logs: `hash[:12]` for readability

## Naming

### Variables

| Pattern | Example | Usage |
|---------|---------|-------|
| `relPath` | `"docs/notes.txt"` | Path relative to sync dir |
| `fullPath` | `"/home/user/VaultDrive/docs/notes.txt"` | Absolute path |
| `tmpPath` | temp file location | Intermediate file |
| `hash` | 64-char SHA-256 hex | Content hash |
| `conn` | TCP connection | Network |
| `enc`, `dec` | gob encoder/decoder | Protocol |
| `st` | `*state.LocalState` | Sync state |
| `cfg` | `*config.Config` | Configuration |
| `syncMu` | `*sync.Mutex` | Sync operation serialization |
| `f` | `*os.File` | File handle |

### Functions

| Style | Examples |
|-------|---------|
| Verb-noun | `SendFile()`, `VerifyFile()`, `DownloadFile()`, `DeleteFile()` |
| Query | `FileExists()`, `HasObject()`, `DeviceExists()`, `HasFile()` |
| Setter | `SetFile()`, `SetSyncing()`, `SetConnected()` |
| Getter | `Snapshot()`, `GetHash()`, `GetDeviceName()`, `GetAllFiles()` |
| Constructor | `New()`, `Open()`, `Load()` |

### Types
- PascalCase structs: `LocalState`, `Syncer`, `ObjectStore`, `UIServer`
- Request/Response suffix: `CheckFileRequest`, `DeleteFileResponse`, `FileStatusResponse`

## Import Grouping

Three groups separated by blank lines:

```go
import (
    // Standard library
    "crypto/sha256"
    "encoding/json"
    "net"

    // Third-party
    "github.com/BurntSushi/toml"

    // Local workspace modules
    "github.com/atienze/HomelabSecureSync/client/internal/config"
    "github.com/atienze/HomelabSecureSync/common/protocol"
)
```

## File Organization

Consistent structure within each `.go` file:

1. Package declaration + doc comment
2. Imports (grouped as above)
3. Constants
4. Type definitions (structs)
5. Constructor / `New()` / `Open()` / `Load()`
6. Public methods
7. Private helper methods

## Comments

- **Package-level:** Brief purpose description
- **Exported types:** Describe struct responsibility
- **Methods:** Explain behavior, especially non-obvious logic
- **Inline:** Sparingly, for "why" not "what"
- **No TODO comments** — planning tracked in `.planning/` directory

Example from `state/state.go`:
```go
// LocalState tracks the last-known hash of each synced file.
// Persisted to state.json so deletions can be detected across runs.
type LocalState struct {
    Files map[string]string `json:"files"` // relPath → SHA-256 hash
}
```

## JSON Response Format

HTTP API uses consistent envelope:

```go
// File list endpoints
{"files": [{"rel_path": "...", "hash": "...", "size": 1024, "size_human": "1.0 KB"}]}

// Mutation endpoints
{"ok": true, "message": "..."}   // success
{"ok": false, "message": "..."}  // failure
```

## Concurrency Patterns

- `sync.Mutex` for serializing sync operations (UI handlers + daemon loop)
- `sync.RWMutex` for thread-safe status reads/writes
- Channel-based signaling: `forceSyncCh chan struct{}` (buffer 1)
- `SetMaxOpenConns(1)` for SQLite single-writer
- Atomic file operations: temp + rename
