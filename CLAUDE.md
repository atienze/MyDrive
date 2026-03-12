# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

HomelabSecureSync (internally called "VaultSync") is a TCP-based file synchronization tool that streams files between a client machine and a homelab server. It uses SHA-256 hashing for deduplication, content-addressable blob storage, and SQLite for file metadata tracking. All syncing is triggered manually from the Web UI or via the `vault-sync sync` CLI command — there is no filesystem watcher or poll timer.

## Build & Run

This is a Go workspace project (go 1.25.6). Always run builds from the repo root.

```bash
# Build
GOOS=linux GOARCH=arm64 go build -o vault-sync-server ./server/cmd
go build -o vault-sync ./client/cmd

# Run server (listens on :9000)
./vault-sync-server

# Register a device (prints token once to stdout)
./vault-sync-server register "DeviceName"

# Run client (one-shot sync or daemon mode)
./vault-sync sync
./vault-sync daemon

# Run tests
go test -C common ./... && go test -C client ./... && go test -C server ./...
```

## Architecture

Three Go modules in a workspace:

- **`common/`** — shared protocol and crypto utilities
- **`client/`** — scans a directory, syncs bidirectionally with the server, serves a web UI
- **`server/`** — listens for TCP connections, deduplicates via SQLite, stores blobs in content-addressable storage

### Data Flow

```
Client:
  scanner/scan.go         → walks sync_dir, computes SHA-256 per file → []FileMeta
  sender/client.go        → CmdCheckFile → if needed, CmdSendFile + CmdFileChunk (4MB chunks)
  sync/bidirectional.go   → full upload + download cycle with deletion detection
  sync/operations.go      → single-file ops (upload/download/delete/list) for the web UI
  ui/server.go            → HTTP proxy: browser actions → TCP operations → JSON responses

Server:
  receiver/handler.go     → validates handshake + token auth, dispatches all protocol commands
  db/db.go                → SQLite: file metadata, device registry, ref counting
  store/store.go          → content-addressable blob storage: objects/{hash[:2]}/{hash[2:]}
```

### Protocol (`common/protocol/`)

Binary encoding via Go's `gob`. Every connection starts with a handshake:
- Magic number: `0xCAFEBABE`
- Version: 2
- Token: 64-char hex auth token

Commands (1–11): Ping, SendFile, CheckFile, FileStatus, FileChunk, DeleteFile, ListServerFiles, ServerFileList, RequestFile, FileDataHeader, FileDataChunk.

### Database (`server/internal/db/`)

Pure-Go SQLite via `modernc.org/sqlite`. Two tables:
- `files` — tracks synced files: `rel_path`, `hash`, `size`, `device_id`, `uploaded_at`, `deleted` (soft-delete flag)
- `devices` — registered clients by token (primary key) and name

`SetMaxOpenConns(1)` is intentional — prevents SQLite write-lock contention.

### Content-Addressable Storage (`server/internal/store/`)

Blobs stored at `VaultData/objects/{hash[:2]}/{hash[2:]}`. Deduplication: identical content stored once regardless of path count. `DeleteObject` only removes a blob when its reference count reaches zero.

### Web UI (`client/internal/ui/`)

Two-panel file browser at `localhost:9876` (OneDrive-style). The HTTP server proxies browser actions to the VaultSync TCP server:

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/` | GET | Dashboard HTML (embedded via `//go:embed`) |
| `/api/status` | GET | Daemon status JSON |
| `/api/force-sync` | POST | Trigger full bidirectional sync |
| `/api/files/client` | GET | List local files |
| `/api/files/server` | GET | List server files (via TCP) |
| `/api/files/upload` | POST | Push one file to server |
| `/api/files/download` | POST | Pull one file from server |
| `/api/files/server` | DELETE | Soft-delete file from server |
| `/api/files/client` | DELETE | Delete local file |

Mutating endpoints acquire a shared `sync.Mutex` to prevent races with background full syncs.

### Sync Behavior

- **Client-wins conflict resolution**: if hashes differ, client re-uploads its version
- **Deletion detection**: comparing `state.json` (last-known state) against current disk contents
- **Valid symlinks**: followed as regular files; broken symlinks logged and skipped
- **Zero-byte files**: handled with immediate finalization (no chunk transfer)

## Configuration

**Client config** (`~/.vaultsync/config.toml`):

```toml
server_addr = "<server-ip>:9000"
token       = "<64-char-hex-token>"
sync_dir    = "~/VaultDrive"
```

**Client state**: `~/.vaultsync/state.json` — tracks `relPath → hash` for deletion detection.

**Server config** (environment variables with defaults):

| Variable | Default | Purpose |
|----------|---------|---------|
| `VAULTSYNC_DB_PATH` | `./vaultsync.db` | SQLite database path |
| `VAULTSYNC_DATA_DIR` | `./VaultData` | Object store root |

USEFUL DATABASE ACCESS COMMANDS:
"SELECT rel_path, deleted FROM files;"
"SELECT * FROM devices;"

Server always listens on `:9000`. UI always listens on `127.0.0.1:9876`.

## Project Structure

```
HomelabSecureSync/
├── common/
│   ├── crypto/hash.go
│   └── protocol/
│       ├── handshake.go
│       └── packet.go
├── client/
│   ├── cmd/main.go
│   └── internal/
│       ├── config/config.go
│       ├── scanner/scan.go
│       ├── sender/client.go
│       ├── state/state.go
│       ├── status/status.go
│       ├── sync/
│       │   ├── bidirectional.go
│       │   └── operations.go
│       └── ui/
│           ├── server.go
│           └── templates/dashboard.html
└── server/
    ├── cmd/
    │   ├── main.go
    │   └── migrate/main.go
    └── internal/
        ├── auth/register.go
        ├── db/db.go
        ├── receiver/handler.go
        └── store/store.go
```

## Security Notes

- `config.toml` contains the auth token in plaintext — restrict file permissions on the homelab machine
- The `devices` table stores tokens as plaintext primary keys — consider hashing tokens for production use
- All relative paths are validated against traversal attacks (`..`) on both client and server sides
