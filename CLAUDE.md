# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HomelabSecureSync (internally called "VaultSync") is a TCP-based file synchronization tool that streams files from a client machine to a homelab server. It uses SHA-256 hashing for deduplication and SQLite for file metadata tracking. The project is in active development — Phases 1–3 complete (database + hash storage), Phases 4–5 in progress (bidirectional sync, web UI). All syncing is triggered manually from the Web UI or by a poll timer — there is no filesystem watcher.

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
| `client/cmd/main.go` | Target dir: `~/TestVault` (or configured via `~/.vaultsync/config.toml`) |
| `client/cmd/main.go` | Server: `127.0.0.1:9000` |
| `server/cmd/main.go` | Port: `:9000` |
| `server/cmd/main.go` | DB path: `./vaultsync.db` |
| `server/cmd/main.go` | Upload dir: `./uploads` |

**Post-Phase 2 client config** (`~/.vaultsync/config.toml`):

| Key | Example Value |
|---|---|
| `server_addr` | `<server-ip>:9000` |
| `token` | 64-char hex string from `vault-sync-server register` |
| `watch_dir` | `~/VaultDrive` (rename to `sync_dir` in Phase 4) |

## Planned Phases

- **Phase 2**: Device Token Auth — replace hardcoded ClientID with a cryptographically random token; unregistered clients rejected at handshake
- **Phase 3**: Hash-Based Flat Storage — transition to content-addressable storage (`objects/{hash[:2]}/{hash[2:]}`); DB becomes the sole path→content authority; identical files stored once
- **Phase 4**: Bidirectional Sync — server-to-client download; UI-triggered only (no poll timer); deletion detection via `state.json` comparison during full scan; last-write-wins conflict resolution; `sync` / `daemon` subcommands
- **Phase 5**: Simple Web UI — `localhost:9876` dashboard; status, activity log, sync-now button (primary sync trigger); embedded HTML/CSS/JS via `//go:embed`; auto-opens browser on daemon start

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
watch_dir                = "~/VaultDrive"
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

## Phase 4 — Bidirectional Sync

### New Files

| File | Purpose |
|------|---------|
| `client/internal/state/state.go` | `LocalState` — `map[relPath]hash`, persisted to `~/.vaultsync/state.json` |
| `client/internal/sync/bidirectional.go` | `PollAndDownload` — lists server files, downloads missing/changed, removes server-deleted |

### New Protocol Commands

| Command | Value | Direction |
|---------|-------|-----------|
| `CmdDeleteFile` | 6 | Client → Server |
| `CmdListServerFiles` | 7 | Client → Server |
| `CmdServerFileList` | 8 | Server → Client |
| `CmdRequestFile` | 9 | Client → Server |
| `CmdFileData` | 10 | Server → Client (chunked, same structure as upload) |

`CmdDeleteFile` uses `DeleteFileRequest{RelPath, Token}` and `DeleteFileResponse{Success, Message}`. Server soft-deletes in DB (`deleted = TRUE`), then calls `store.DeleteObject(hash, refCount)` — only removes blob if `refCount == 0`.

### Client Subcommands

```bash
vault-sync sync     # one-shot full scan and sync (upload + download)
vault-sync daemon   # initial full sync, then poll timer + UI server
```

### Sync Loop Integration

Sync is triggered exclusively by the "Sync Now" button in the Web UI (or the one-shot `vault-sync sync` command). There is no poll timer. Each sync cycle runs a full bidirectional sync:

**Upload phase** (`FullScanAndUpload`):
1. `scanner.ScanDirectory()` → current files on disk
2. Compare against `state.json` to detect local deletions (file in state but not on disk → `CmdDeleteFile`)
3. For each file on disk: `CmdCheckFile` → if needed, `CmdSendFile` + `CmdFileChunk`

**Download phase** (`PollAndDownload`):
1. Send `CmdListServerFiles`
2. Compare response to `state.Files`
3. Download missing/changed files via `CmdRequestFile` → `CmdFileData`
4. Delete local files absent from the server list (server-side deletions)
5. Save `state.json`

### Conflict Resolution

Last-write-wins, client preference: if hashes differ, client re-uploads its version. Server updates the `files` row.

### Testing Checklist

- `vault-sync sync` performs a complete upload + download cycle
- File added on server appears on client after sync
- File deleted locally is detected and `CmdDeleteFile` sent to server
- File deleted on server is deleted on client after sync
- Deleting a file that's the last reference to a hash removes the object blob
- Deleting a file that shares a hash with another file does NOT remove the blob
- Two clients registered to the same server share files bidirectionally
- Conflicting edits resolve with client-wins behavior
- `state.json` persists across daemon restarts
- Initial sync on a fresh client downloads everything from the server

## Phase 5 — Simple Web UI

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

The daemon `select` loop listens on `forceSyncCh` only. All syncing is user-initiated via the "Sync Now" button in the UI.

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
│       ├── sender/client.go
│       ├── state/state.go        (local file state → state.json)
│       ├── status/status.go      (shared daemon status for UI)
│       ├── sync/bidirectional.go (full scan + upload + download)
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
Phase 2 (Auth)           ✓ Complete
    ↓
Phase 3 (Hash Storage)   ✓ Complete
    ↓
Phase 4 (Bidirectional)  Medium complexity — new data flow + deletion detection
    ↓
Phase 5 (UI)             Low risk — independent of sync logic
```

Each phase is independently deployable.


# additional notes:
- config.toml is exposed and should be cleaned up when im finished with implementing this program on my homelab machine
- sqlite3 can query for my auth token as well. if someone has access to the db then they can query my key and use it to steal data. find out if theres a way to lock that functionality or even hite the id's instead. find a secure way to store it.
