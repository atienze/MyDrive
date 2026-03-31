# myDrive 
### v1.2.0

A self-hosted, TCP-based file synchronization tool that streams files between client machines and a homelab server over a Tailscale mesh network. Built entirely in Go with no third-party cloud dependency. Uses SHA-256 content addressing for deduplication, SQLite for file metadata tracking, and a responsive web UI for managing syncs across multiple devices.

## Features

- **Content-addressable storage** — identical files stored once regardless of path; blobs removed only when all references are deleted
- **Multi-device sync** — per-device file namespacing with cross-device pull; any registered device can pull files from another; own-device entries always take priority over another device's copy of the same path during download
- **Custom binary protocol** — 11-command TCP protocol with magic-number handshake, token auth, and 4MB chunked file transfer
- **Responsive web UI** — full file browser at `localhost:9876` with desktop sidebar and mobile bottom nav; real-time recursive search, drag-and-drop import, FAB push sheet on mobile
- **Conflict resolution** — client-wins strategy; if hashes differ, client re-uploads its version
- **Deletion safety** — server acts as persistent store; local deletions do not cascade to server without explicit UI action
- **Tailscale mesh networking** — all TCP traffic routes through Tailscale's encrypted overlay network; `server_addr` in config points to a Tailscale IP, giving every device zero-config secure connectivity with no open ports or VPN setup
- **Cross-platform** — builds for Linux (amd64), Windows (amd64), and iOS via iSH (linux/386)

## Build & Run

This is a Go workspace project (go 1.25.6). Always run builds from the repo root.

```bash
# Build
#ubuntu linux
GOOS=linux GOARCH=amd64 go build -o mydrive-server ./server/cmd

#windows
GOOS=windows GOARCH=amd64 go build -o mydrive.exe ./client/cmd

#ios (using iSH Shell)
GOOS=linux GOARCH=386 go build -o mydrive-ish ./client/cmd

#native
go build -o mydrive ./client/cmd

# Run server (listens on :9000)
./mydrive-server

# Register a device (prints token once to stdout)
./mydrive-server register "DeviceName"

# Run client
./mydrive sync                              # one-shot push sync and exit
./mydrive daemon                            # start web UI + initial sync
./mydrive pull --from <deviceID> <path>     # download single file from specific device
```

## Architecture

Three Go modules in a workspace:

- **`common/`** — shared protocol and crypto utilities
- **`client/`** — scans a directory, syncs with the server, serves a web UI
- **`server/`** — listens for TCP connections, deduplicates via SQLite, stores blobs in content-addressable storage

### Data Flow

```
Client:
  scanner/scan.go         → walks sync_dir, computes SHA-256 per file → []FileMeta
  sender/client.go        → CmdCheckFile → if needed, CmdSendFile + CmdFileChunk (4MB chunks)
  sync/bidirectional.go   → push-only upload cycle with deletion detection
  sync/operations.go      → single-file ops (upload/download/delete/pull/list) for the web UI
  ui/server.go            → HTTP proxy: browser actions → TCP operations → JSON responses

Server:
  receiver/handler.go     → validates handshake + token auth, dispatches all protocol commands
  db/db.go                → SQLite: file metadata, device registry, ref counting
  store/store.go          → content-addressable blob storage: objects/{hash[:2]}/{hash[2:]}
```

### Protocol

Binary encoding via Go's `gob`. Every connection starts with a handshake:
- Magic number: `0xCAFEBABE`
- Version: 3
- Token: 64-char hex auth token

Commands (1–11): Ping, SendFile, CheckFile, FileStatus, FileChunk, DeleteFile, ListServerFiles, ServerFileList, RequestFile, FileDataHeader, FileDataChunk.

`RequestFile` with an empty `Hash` field tells the server to resolve the blob hash from its DB — used for cross-device pull where the requester doesn't know the hash.

### Database

Pure-Go SQLite via `modernc.org/sqlite`. Two tables:
- `files` — `id`, `rel_path`, `hash`, `size`, `device_id` (FK), `uploaded_at`, `deleted` (soft-delete). Composite unique index on `(rel_path, device_id)`.
- `devices` — `id` (token, PK), `name`, `created_at`

`SetMaxOpenConns(1)` prevents SQLite write-lock contention. Auto-migration creates the composite unique index if missing from pre-existing DBs.

### Web UI

File browser at `localhost:9876` with three views: All Files, Local, Server.

| Endpoint | Method | Purpose |

| `/` | GET | Dashboard HTML (embedded via `//go:embed`) |
| `/api/status` | GET | Daemon status JSON |
| `/api/force-sync` | POST | Trigger full sync |
| `/api/files/client` | GET | List local files |
| `/api/files/server` | GET | List server files (cross-device) |
| `/api/files/upload` | POST | Push one file to server |
| `/api/files/download` | POST | Pull one file from server |
| `/api/files/pull` | POST | Pull file from specific device (`?from=<deviceID>&path=<rel>`) |
| `/api/files/import` | POST | Import uploaded file(s) to sync dir (`?subdir=<rel>`) |
| `/api/files/server` | DELETE | Soft-delete file from server |
| `/api/files/client` | DELETE | Delete local file |

Mutating endpoints acquire a shared `sync.Mutex` to prevent races with background full syncs.

### Sync Behavior

- **Push-only full sync**: `mydrive sync` and "Full Sync" button scan local dir, detect deletions, and upload new/changed files to the server.
- **Download phase — own-device priority**: the download pass runs in two stages. First, all paths this device already owns on the server are marked. Then, when iterating cross-device files, any path already owned by this device is skipped entirely — another device's copy never overwrites your own version. Cross-device files with no local ownership are downloaded normally (first-encountered wins when multiple foreign devices share a path).
- **Client-wins conflict resolution**: if hashes differ, client re-uploads its version
- **Deletion detection**: comparing `state.json` against current disk contents; local deletions do not cascade to server
- **Zero-byte files**: handled with immediate finalization (no chunk transfer)
- **Scanner skips**: `.git`, `server` directories, `.DS_Store` files

## Configuration

**Client config** (`~/.mydrive/config.toml`):

```toml
server_addr = "<server-ip>:9000"
token       = "<64-char-hex-token>"
sync_dir    = "~/VaultDrive"
device_name = "MyLaptop"
```

**Client state**: `~/.mydrive/state.json` — tracks `relPath → hash` for deletion detection.

**Server config** (environment variables):

| Variable | Default | Purpose |

| `MYDRIVE_DB_PATH` | `./mydrive.db` | SQLite database path |
| `MYDRIVE_DATA_DIR` | `./VaultData` | Object store root |

Server listens on `:9000`. Web UI listens on `127.0.0.1:9876`.

## Security Notes

- `config.toml` contains the auth token in plaintext — restrict file permissions on the homelab machine
- The `devices` table stores tokens as plaintext primary keys — consider hashing tokens for production use
- All relative paths are validated against path traversal attacks (`..`) on both client and server sides
