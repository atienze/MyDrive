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

## Phase 5 — File Browser Web UI (OneDrive-Style)

Phase 5 upgrades the basic status dashboard into a two-panel file browser where users can selectively push, pull, and delete files from either client or server — similar to OneDrive or Dropbox.

### Existing Infrastructure (Already Built)

| Capability | Location | Status |
|---|---|---|
| All protocol commands (1–11) | `common/protocol/packet.go` | Complete |
| Upload file (CmdSendFile + CmdFileChunk) | `client/internal/sender/client.go` | Complete |
| Download file (CmdRequestFile → CmdFileDataHeader + CmdFileDataChunk) | `client/internal/sync/bidirectional.go` | Complete |
| Delete from server (CmdDeleteFile) | `client/internal/sync/bidirectional.go` | Complete |
| List server files (CmdListServerFiles → CmdServerFileList) | `client/internal/sync/bidirectional.go` | Complete |
| Scan client files | `client/internal/scanner/scan.go` | Complete |
| Status dashboard + activity log | `client/internal/ui/server.go` + `templates/dashboard.html` | Complete |
| Token-based auth handshake | `common/protocol/handshake.go` | Complete |
| Thread-safe status tracking | `client/internal/status/status.go` | Complete |
| Local state persistence | `client/internal/state/state.go` | Complete |

**Zero server-side changes needed.** All new work is client-side HTTP + UI.

### Architecture: How Browser Talks to VaultSync Server

The browser cannot speak the TCP protocol directly. The client's HTTP server (`:9876`) acts as a **proxy**: each browser action opens a fresh TCP connection to the VaultSync server (`:9000`), runs the operation, and returns JSON.

```
Browser → HTTP POST /api/files/upload?path=docs/notes.txt → UI server (localhost:9876)
                                                                  ↓
                                                            TCP connect to :9000
                                                            Handshake (token auth)
                                                            CmdSendFile + CmdFileChunk
                                                            Close connection
                                                                  ↓
                                                            HTTP 200 JSON response
```

This mirrors how `runSyncCycle()` in `client/cmd/main.go` already works — connect, do work, close. The difference is per-file instead of full-sync.

### New/Modified Files

| File | Change Type | Purpose |
|------|-------------|---------|
| `client/internal/sync/operations.go` | **NEW** | Extracted single-file operations: `DialAndHandshake`, `UploadSingleFile`, `DownloadSingleFile`, `DeleteServerFile`, `ListServerFiles` |
| `client/internal/ui/server.go` | **MODIFY** | Add 6 new HTTP endpoints; accept `*config.Config`, `*state.LocalState`, `*sync.Mutex` in constructor |
| `client/internal/ui/templates/dashboard.html` | **REWRITE** | Two-panel file browser with push/pull/delete per file |
| `client/cmd/main.go` | **MODIFY** | Add `sync.Mutex`; pass config + state + mutex to UIServer |

### New File: `client/internal/sync/operations.go`

Consolidates the TCP handshake (currently duplicated between `sender/client.go` and `sync/bidirectional.go`) and exposes single-file operations callable from the UI server.

```go
package sync

// DialAndHandshake opens a TCP connection and performs the auth handshake.
// Returns (conn, encoder, decoder, error). Caller must close conn.
func DialAndHandshake(cfg *config.Config) (net.Conn, *protocol.Encoder, *protocol.Decoder, error)

// UploadSingleFile connects to the server and uploads one file.
// Updates state on success. Uses DialAndHandshake internally.
func UploadSingleFile(cfg *config.Config, st *state.LocalState, statePath, relPath string) error

// DownloadSingleFile connects to the server and downloads one file.
// Updates state on success. Uses DialAndHandshake internally.
func DownloadSingleFile(cfg *config.Config, st *state.LocalState, statePath, relPath string) error

// DeleteServerFile connects to the server and sends CmdDeleteFile.
// Updates state on success. Uses DialAndHandshake internally.
func DeleteServerFile(cfg *config.Config, st *state.LocalState, statePath, relPath string) error

// FetchServerFileList connects and returns the server file manifest.
func FetchServerFileList(cfg *config.Config) ([]protocol.ServerFileEntry, error)
```

Each function opens its own connection, does one operation, and closes. No connection pooling needed — VaultSync operations are infrequent (user-initiated).

### Modified: `client/internal/ui/server.go`

**Constructor change:**

```go
// Before:
func NewUIServer(status *status.Status, forceSyncCh chan<- struct{}) *UIServer

// After:
func NewUIServer(
    status    *status.Status,
    forceSyncCh chan<- struct{},
    cfg       *config.Config,
    st        *state.LocalState,
    statePath string,
    syncMu    *sync.Mutex,
) *UIServer
```

**New HTTP endpoints:**

| Endpoint | Method | Purpose | Implementation |
|----------|--------|---------|----------------|
| `/api/files/client` | GET | List local files in `sync_dir` | `scanner.ScanDirectory(cfg.SyncDir)` |
| `/api/files/server` | GET | List server files | `operations.FetchServerFileList(cfg)` via TCP |
| `/api/files/upload` | POST | Push one file to server | `operations.UploadSingleFile(cfg, st, statePath, relPath)` via TCP |
| `/api/files/download` | POST | Pull one file from server | `operations.DownloadSingleFile(cfg, st, statePath, relPath)` via TCP |
| `/api/files/server` | DELETE | Delete file from server | `operations.DeleteServerFile(cfg, st, statePath, relPath)` via TCP |
| `/api/files/client` | DELETE | Delete local file | `os.Remove` + `state.RemoveFile` + `state.Save` |

Existing endpoints remain unchanged: `/` (dashboard), `/api/status`, `/api/force-sync`.

**JSON response format for file lists:**

```json
{
  "files": [
    {
      "rel_path": "documents/notes.txt",
      "hash": "a3f9b2c1...",
      "size": 2048,
      "size_human": "2.0 KB"
    }
  ]
}
```

The browser joins both lists client-side by `rel_path` to determine sync status per file.

**All per-file endpoints acquire `syncMu.Lock()` before operating.** This prevents races with a background full sync.

### Modified: `client/cmd/main.go`

```go
// In runDaemon():
var syncMu sync.Mutex

appStatus   := status.New()
forceSyncCh := make(chan struct{}, 1)

// Load state for UI server
statePath, _ := cfg.StatePath()  // already exists
st, _ := state.Load(statePath)

uiServer := ui.NewUIServer(appStatus, forceSyncCh, cfg, st, statePath, &syncMu)
go uiServer.Start("127.0.0.1:9876")

// In main select loop, wrap sync with mutex:
syncMu.Lock()
doSyncCycle(cfg, appStatus)
syncMu.Unlock()
```

### Dashboard UI: Two-Panel File Browser

**Layout:**

```
┌──────────────────────────────────────────────────────────────┐
│  VaultSync                    [Full Sync]     ● Connected    │
├─────────────────────────────┬────────────────────────────────┤
│  LOCAL (~/VaultDrive)       │  SERVER                        │
│  ─────────────────────────  │  ────────────────────────────  │
│  📄 docs/notes.txt    2 KB  │  📄 docs/notes.txt    2 KB     │
│     ✓ synced                │     ✓ synced                   │
│                             │                                │
│  📄 photos/cat.jpg   4 MB   │  (not on server)               │
│     [Push →]  [Delete]      │                                │
│                             │                                │
│  (not local)                │  📄 archive/old.zip  150 MB    │
│                             │     [← Pull]  [Delete]         │
│                             │                                │
│  📄 work/report.pdf  1 MB   │  📄 work/report.pdf  1 MB      │
│     ⚠ hash mismatch        │     ⚠ hash mismatch            │
│     [Push →]  [Delete]      │     [← Pull]  [Delete]        │
├─────────────────────────────┴────────────────────────────────┤
│  Activity Log                                                │
│  10:32:05  Uploaded docs/notes.txt (2 KB)                    │
│  10:32:03  Downloaded archive/old.zip (150 MB)               │
│  10:31:58  Connected to server                               │
└──────────────────────────────────────────────────────────────┘
```

**File states (computed client-side by joining both lists on `rel_path`):**

| Client | Server | State | Actions Available |
|--------|--------|-------|-------------------|
| ✓ | ✓ same hash | Synced | Delete (either side) |
| ✓ | ✓ diff hash | Conflict | Push, Pull, Delete |
| ✓ | ✗ | Local only | Push, Delete local |
| ✗ | ✓ | Server only | Pull, Delete server |

**JS behavior:**
- On page load and every 5s: fetch `/api/files/client` and `/api/files/server` in parallel
- Join results by `rel_path`, compute sync state, render two-column view
- Button clicks → `fetch()` to appropriate endpoint → refresh file lists on success
- Disable buttons while an operation is in progress (prevent double-clicks)
- Activity log continues polling `/api/status` for recent entries

### Concurrency & Safety

| Concern | Mitigation |
|---------|------------|
| Simultaneous UI op + full sync | `sync.Mutex` shared between daemon loop and UI handlers |
| `state.json` concurrent writes | Mutex ensures only one sync operation at a time |
| SQLite server-side contention | Already handled: `SetMaxOpenConns(1)` in `db/db.go` |
| File write during download | Mutex prevents; `downloadFile()` uses atomic temp+rename anyway |
| Browser double-click | UI disables buttons during in-flight requests |

### Implementation Order

1. **`client/internal/sync/operations.go`** — Extract `DialAndHandshake` from existing code; write `UploadSingleFile`, `DownloadSingleFile`, `DeleteServerFile`, `FetchServerFileList` reusing logic from `bidirectional.go` and `sender/client.go`
2. **`client/internal/ui/server.go`** — Update constructor, add 6 new HTTP endpoints using the operations from step 1
3. **`client/internal/ui/templates/dashboard.html`** — Rewrite to two-panel file browser; keep activity log from current dashboard
4. **`client/cmd/main.go`** — Add `sync.Mutex`, load state, pass new deps to `NewUIServer`

### Testing Checklist

- `vault-sync daemon` starts and UI loads at `http://localhost:9876`
- `/api/files/client` returns correct file list from `sync_dir`
- `/api/files/server` returns correct file list from VaultSync server
- Clicking "Push" on a local-only file uploads it; file appears on server panel after refresh
- Clicking "Pull" on a server-only file downloads it; file appears on local panel after refresh
- Clicking "Delete" on server removes it from server; local copy unaffected
- Clicking "Delete" on client removes local file; server copy unaffected
- Files with matching hashes show as "synced" with no push/pull buttons
- Files with mismatched hashes show conflict indicator with both push and pull options
- "Full Sync" button still triggers a complete bidirectional sync
- Activity log updates after each individual operation
- Concurrent operations are serialized (no race conditions on `state.json`)
- UI buttons are disabled during in-flight operations
- Dashboard status cards (connection, sync status, file count) continue to update

## Final Project Structure

```
HomelabSecureSync/
├── common/
│   ├── crypto/hash.go
│   └── protocol/
│       ├── handshake.go          (Token field, Version 2)
│       └── packet.go             (CmdPing–CmdFileDataChunk, commands 1–11)
├── client/
│   ├── cmd/main.go               (sync | daemon subcommands, UI startup, sync mutex)
│   └── internal/
│       ├── config/config.go      (TOML loader)
│       ├── scanner/scan.go
│       ├── sender/client.go
│       ├── state/state.go        (local file state → state.json)
│       ├── status/status.go      (shared daemon status for UI)
│       ├── sync/
│       │   ├── bidirectional.go  (full scan + upload + download)
│       │   └── operations.go     (single-file ops: DialAndHandshake, Upload/Download/Delete/List)
│       └── ui/
│           ├── server.go         (HTTP proxy: 9 endpoints, bridges browser ↔ TCP server)
│           └── templates/dashboard.html  (two-panel file browser, OneDrive-style)
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
Phase 4 (Bidirectional)  ✓ Complete — full sync with deletion detection
    ↓
Phase 5 (File Browser UI)  In Progress — 4 steps:
    Step 1: sync/operations.go     (extract reusable single-file ops)
    Step 2: ui/server.go           (add 6 HTTP endpoints)
    Step 3: dashboard.html         (two-panel file browser)
    Step 4: cmd/main.go            (wire sync mutex + new deps)
```

Each phase is independently deployable. Phase 5 requires zero server-side changes.


# additional notes:
- config.toml is exposed and should be cleaned up when im finished with implementing this program on my homelab machine
- sqlite3 can query for my auth token as well. if someone has access to the db then they can query my key and use it to steal data. find out if theres a way to lock that functionality or even hite the id's instead. find a secure way to store it.
