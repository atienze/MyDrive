# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HomelabSecureSync (internally called "VaultSync") is a TCP-based file synchronization tool that streams files from a client machine to a homelab server. It uses SHA-256 hashing for deduplication and SQLite for file metadata tracking. The project is in active development — Phases 1–3 complete (database + hash storage), Phases 4–6 in progress (watcher, bidirectional sync, web UI).

## Build & Run

This is a Go workspace project (go 1.25.6). Always run builds from the repo root so the workspace resolves shared modules correctly.

```bash
# Build
go build -o vault-sync-server ./server/cmd
go build -o vault-sync ./client/cmd

# Run server (listens on :9000, creates vaultsync.db, stores files in ./uploads)
./vault-sync-server

# Run client (scans hardcoded TestVault dir, connects to 127.0.0.1:9000)
./vault-sync

# Run tests
go test ./...

# Run a single package's tests
go test ./server/internal/db/...
```

## Architecture

The system has three Go modules in a workspace:

- **`common/`** — shared protocol and crypto utilities used by both client and server
- **`client/`** — scans a directory, hashes files, and streams them to the server over TCP
- **`server/`** — listens for connections, deduplicates via SQLite, writes files to disk

### Data Flow

```
Client:
  scanner/scan.go     → walks directory, computes SHA-256 per file → []FileMeta
  sender/client.go    → for each file: CmdCheckFile → if needed, CmdSendFile + CmdFileChunk (4MB chunks)

Server:
  receiver/handler.go → validates handshake, dispatches commands
  db/db.go            → FileExists(path, hash) for dedup; UpsertFile() after write
  ./uploads/          → files stored here preserving relative paths
```

### Protocol (`common/protocol/`)

Binary encoding via Go's `gob`. Every connection starts with a handshake:
- Magic number: `0xCAFEBABE`
- Version: 1
- ClientID: device name string

Packet commands: `CmdPing(1)`, `CmdSendFile(2)`, `CmdCheckFile(3)`, `CmdFileStatus(4)`, `CmdFileChunk(5)`.

### Database (`server/internal/db/`)

Pure-Go SQLite via `modernc.org/sqlite`. Two tables:
- `files` — tracks uploaded files: `rel_path`, `hash`, `size`, `device_id`, `uploaded_at`, `deleted`
- `devices` — registered clients by name

`SetMaxOpenConns(1)` is intentional — prevents SQLite write-lock contention across goroutines (each connection gets its own goroutine).

## Configuration

After Phase 2, all client configuration moves to `~/.vaultsync/config.toml` (see Phase 2 above). The server retains hardcoded defaults until a server-side config file is added.

**Current hardcoded values (pre-Phase 2):**

| Location | Value |
|---|---|
| `client/cmd/main.go` | Target dir: `/Users/<user>/Desktop/TestVault` |
| `client/cmd/main.go` | Server: `127.0.0.1:9000` |
| `server/cmd/main.go` | Port: `:9000` |
| `server/cmd/main.go` | DB path: `./vaultsync.db` |
| `server/cmd/main.go` | Upload dir: `./uploads` |

**Post-Phase 2 client config** (`~/.vaultsync/config.toml`):

| Key | Example Value |
|---|---|
| `server_addr` | `<server-ip>:9000` |
| `token` | 64-char hex string from `vault-sync-server register` |
| `watch_dir` | `/Users/<user>/VaultDrive` |
| `sync_interval_seconds` | `60` (default) |

## Planned Phases

- **Phase 2**: Device Token Auth — replace hardcoded ClientID with a cryptographically random token; unregistered clients rejected at handshake
- **Phase 3**: Hash-Based Flat Storage — transition to content-addressable storage (`objects/{hash[:2]}/{hash[2:]}`); DB becomes the sole path→content authority; identical files stored once
- **Phase 4**: Automatic File Watching — `fsnotify`-based daemon with 500ms debouncing; deletion sync via `CmdDeleteFile`; `sync` / `daemon` subcommands; reconnecting `ConnManager`
- **Phase 5**: Bidirectional Sync — server-to-client download; poll every `sync_interval_seconds`; echo suppression; last-write-wins conflict resolution; local `state.json`
- **Phase 6**: Simple Web UI — `localhost:9876` dashboard; status, activity log, force-sync button; embedded HTML/CSS/JS via `//go:embed`; auto-opens browser on daemon start

## Phase 2 — Device Token Auth

### New Files

| File | Purpose |
|------|---------|
| `server/internal/auth/register.go` | `GenerateToken()` — 32-byte `crypto/rand`, hex-encoded |
| `client/internal/config/config.go` | TOML loader from `~/.vaultsync/config.toml` |

### Config File (`~/.vaultsync/config.toml`)

```toml
server_addr              = "<server-ip>:9000"
token                    = "a3f9b2c1..."
watch_dir                = "/Users/<user>/VaultDrive"
sync_interval_seconds    = 60
```

### Protocol Change

`Handshake.ClientID` → `Handshake.Token`; `Version` bumped to `2`.

### Server Subcommand

```bash
./vault-sync-server register "MacBook-Pro"
# Inserts token into devices table, prints token once to stdout
```

### Auth Flow

After decoding the handshake, the server does:
```sql
SELECT name FROM devices WHERE id = ?
```
Invalid token → immediate `conn.Close()`. Valid token → log device name, continue.

### Testing Checklist

- `vault-sync-server register "MacBook-Pro"` prints a 64-char hex token
- Token appears in `SELECT * FROM devices`
- Client with correct token connects and syncs normally
- Client with wrong/missing token is immediately disconnected
- Client with no `~/.vaultsync/config.toml` exits with a clear error message
- Registering the same device name twice produces two different tokens (both valid)

## Phase 3 — Hash-Based Flat Storage

### New Files

| File | Purpose |
|------|---------|
| `server/internal/store/store.go` | `ObjectStore` — `ObjectPath`, `HasObject`, `WriteObject`, `ReadObject`, `DeleteObject` |
| `server/cmd/migrate/main.go` | One-time idempotent migration from path-based to hash-based layout |

### Storage Layout

```
VaultData/objects/{hash[:2]}/{hash[2:]}
```

`WriteObject` is a no-op if the blob already exists (dedup). `DeleteObject` checks `refCount` before removing — never deletes a blob still referenced by another `files` row.

### Handler Changes

- `CmdSendFile`: reassemble chunks → verify hash → `store.WriteObject` → `db.UpsertFile`
- `CmdCheckFile`: `SELECT hash FROM files WHERE rel_path = ? AND deleted = FALSE`; match → skip, mismatch/missing → send

### Testing Checklist

- Uploading a file creates `objects/{hash[:2]}/{hash[2:]}`, not a path-based file
- Uploading the same content at two different paths creates only one object blob
- `CmdCheckFile` correctly reports "already have it" based on DB lookup
- Migration script moves all existing files and creates correct DB rows
- Migration script is idempotent

## Phase 4 — Automatic File Watching

### New Files

| File | Purpose |
|------|---------|
| `client/internal/watcher/watcher.go` | `fsnotify` wrapper with 500ms debounce, recursive dir watching, ignore set |
| `client/internal/sender/connmanager.go` | Persistent TCP connection with reconnect + exponential backoff |

### New Protocol Command

`CmdDeleteFile = 6` with `DeleteFileRequest{RelPath, Token}` and `DeleteFileResponse{Success, Message}`.

### Watcher Behavior

- Walks the tree on startup; adds every subdirectory to `fsnotify`
- `CREATE` on a directory → `fsw.Add()` (recursive watching)
- `WRITE`/`CREATE` on a file → debounce 500ms, then emit `EventSync`
- `REMOVE` → cancel pending timer, immediately emit `EventDelete`
- Ignore set: `.DS_Store`, `.swp`, `.tmp`, `~` suffix, `.vaultsync/`, `.git/`

### Client Subcommands

```bash
vault-sync sync     # one-shot full scan and sync
vault-sync daemon   # initial full sync, then watch for changes
```

### Edge Cases

| Scenario | Handling |
|----------|----------|
| Large file being written | Debounce resets on every WRITE; syncs only when writes stop for 500ms |
| File created then immediately deleted | DELETE cancels the pending CREATE timer |
| Temp file renamed (`.tmp` → final name) | `.tmp` ignored; RENAME to final name triggers CREATE |
| New directory with files | Directory CREATE adds it to fsnotify; files fire their own events |
| Server offline | `ConnManager.Get()` returns error; log, retry with exponential backoff |

### Server-Side Deletion

Soft-delete in DB (`deleted = TRUE`), then call `store.DeleteObject(hash, refCount)` — only removes blob if `refCount == 0`.

### Testing Checklist

- Creating a file in `~/VaultDrive/` triggers automatic upload within ~1 second
- Rapidly writing to a file triggers only one sync after completion
- Deleting a file sends `CmdDeleteFile` and the server soft-deletes the DB row
- Deleting a file that's the last reference to a hash removes the object blob
- Deleting a file that shares a hash with another file does NOT remove the blob
- Creating a new subdirectory and adding files inside it works
- `.DS_Store`, `.swp`, `.tmp` files are ignored
- `vault-sync daemon` runs initial full sync on startup
- Daemon reconnects after server restart

## Phase 5 — Bidirectional Sync

### New Files

| File | Purpose |
|------|---------|
| `client/internal/state/state.go` | `LocalState` — `map[relPath]hash`, persisted to `~/.vaultsync/state.json` |
| `client/internal/sync/bidirectional.go` | `PollAndDownload` — lists server files, downloads missing/changed, removes server-deleted |

### New Protocol Commands

| Command | Value | Direction |
|---------|-------|-----------|
| `CmdListServerFiles` | 7 | Client → Server |
| `CmdServerFileList` | 8 | Server → Client |
| `CmdRequestFile` | 9 | Client → Server |
| `CmdFileData` | 10 | Server → Client (chunked, same structure as upload) |

### Sync Loop Integration

Poll ticker fires every `cfg.SyncIntervalSeconds`. `PollAndDownload`:
1. Send `CmdListServerFiles`
2. Compare response to `state.Files`
3. Download missing/changed files via `CmdRequestFile` → `CmdFileData`
4. Delete local files absent from the server list
5. Save `state.json`

### Echo Suppression

When the client writes a downloaded file, fsnotify fires. Add `suppressUntil map[string]time.Time` to `Watcher`; suppress events on a path for 5 seconds after a download.

### Conflict Resolution

Last-write-wins, client preference: if hashes differ, client re-uploads its version. Server updates the `files` row.

### Testing Checklist

- File added on server appears on client within one poll interval
- File deleted on server is deleted on client within one poll interval
- Downloaded files don't trigger re-upload (echo suppression works)
- Two clients registered to the same server share files bidirectionally
- Conflicting edits resolve with client-wins behavior
- `state.json` persists across daemon restarts
- Initial sync on a fresh client downloads everything from the server

## Phase 6 — Simple Web UI

### New Files

| File | Purpose |
|------|---------|
| `client/internal/status/status.go` | Thread-safe `Status` struct; last 50 activity entries; `Snapshot()` for safe reads |
| `client/internal/ui/server.go` | HTTP handlers; `//go:embed templates/*` |
| `client/internal/ui/templates/dashboard.html` | Single-page dashboard; embedded CSS; JS auto-refresh every 5s |

### Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/` | GET | Render dashboard HTML |
| `/api/status` | GET | JSON status snapshot (polled by JS) |
| `/api/force-sync` | POST | Signal daemon to run full sync immediately |

### Daemon Integration

```go
// In client/cmd/main.go daemon case:
appStatus    := status.New()
forceSyncCh  := make(chan struct{}, 1)
uiServer     := ui.NewUIServer(appStatus, forceSyncCh)
go uiServer.Start("127.0.0.1:9876")
exec.Command("open", "http://localhost:9876").Start() // macOS; use xdg-open on Linux
```

`forceSyncCh` is added to the daemon's `select` loop alongside the watcher events and poll ticker.

### Testing Checklist

- `vault-sync daemon` starts and opens `http://localhost:9876` in the browser
- Dashboard shows current connection status
- Dashboard updates every 5 seconds without full page reload
- "Force Sync Now" button triggers a sync and the activity log updates
- Recent activity shows the last 50 operations with correct relative timestamps

## Final Project Structure

```
HomelabSecureSync/
├── common/
│   ├── crypto/hash.go
│   └── protocol/
│       ├── handshake.go          (Token field, Version 2)
│       └── packet.go             (CmdPing–CmdFileData, commands 1–10)
├── client/
│   ├── cmd/main.go               (sync | daemon subcommands, UI startup)
│   └── internal/
│       ├── config/config.go      (TOML loader)
│       ├── scanner/scan.go
│       ├── sender/
│       │   ├── client.go
│       │   └── connmanager.go    (persistent/reconnecting connection)
│       ├── state/state.go        (local file state → state.json)
│       ├── status/status.go      (shared daemon status for UI)
│       ├── sync/bidirectional.go (poll + download)
│       ├── watcher/watcher.go    (fsnotify + debounce + suppression)
│       └── ui/
│           ├── server.go
│           └── templates/dashboard.html
└── server/
    ├── cmd/
    │   ├── main.go               (serve | register subcommands)
    │   └── migrate/main.go       (one-time storage migration)
    └── internal/
        ├── auth/register.go
        ├── db/db.go
        ├── receiver/handler.go
        └── store/store.go        (hash-based object storage)
```

## Build Order

```
Phase 2 (Auth)           Low risk — mostly plumbing
    ↓
Phase 3 (Hash Storage)   Medium risk — migration required
    ↓
Phase 4 (Watcher)        High complexity — many edge cases
    ↓
Phase 5 (Bidirectional)  High complexity — new data flow direction
    ↓
Phase 6 (UI)             Low risk — independent of sync logic
```

Each phase is independently deployable.
