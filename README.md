# myDrive 
### v1.2.0

A self-hosted, TCP-based file synchronization tool that streams files between client machines and a homelab server over a Tailscale mesh network. Built entirely in Go with no third-party cloud dependency. Uses SHA-256 content addressing for deduplication, SQLite for file metadata tracking, and a responsive web UI for managing syncs across multiple devices.

## Features

- Content-addressable blob storage with SHA-256 deduplication
- Multi-device sync with server
- Responsive web UI with file browser, search, and drag-and-drop import
- Tailscale mesh networking — no open ports required
- Cross-platform: Linux, Windows, iOS (iSH)

## Setup

Prerequisites
- Go v1.25+ 
- Repo cloned on client machine
- Homelab server reachable on port 9000

Process: (ORDER MATTERS)

server
- Build server binary and put on server machine
- Set optional configs on server machine
  - config details further down in README
- Make file executable
- Register client devices. (token only prints once)
  - ./mydrive-server register "DeviceName" -> copy output, will need in client config
- Run server

client
- Build client binary and put on client machine
- Create ~/.mydrive/config.toml and setup client config with token in server steps above
  - config details further down in README
- run file daemon and sync files to and from server 

## Build & Run

This is a Go workspace project (go 1.25.6). Always run builds from the repo root.

```bash
# Build

## COMPILED SERVER BINARY
#ubuntu linux
GOOS=linux GOARCH=amd64 go build -o mydrive-server ./server/cmd


## COMPILED CLIENT BINARY
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

The server authenticates each connection by computing `HMAC-SHA256(key="mydrive-v1", data=token)` and looking up the resulting hash in `devices.token_hash`.

Commands (1–11): Ping, SendFile, CheckFile, FileStatus, FileChunk, DeleteFile, ListServerFiles, ServerFileList, RequestFile, FileDataHeader, FileDataChunk.

`RequestFile` with an empty `Hash` field tells the server to resolve the blob hash from its DB — used for cross-device pull where the requester doesn't know the hash.

### Database

Pure-Go SQLite via `modernc.org/sqlite`. Two tables:
- `files` — `id`, `rel_path`, `hash`, `size`, `device_id` (FK), `uploaded_at`, `deleted` (soft-delete). Composite unique index on `(rel_path, device_id)`.
- `devices` — `id` (UUID v4, PK), `token_hash` (HMAC-SHA256 of raw token, unique), `name`, `created_at`

`SetMaxOpenConns(1)` prevents SQLite write-lock contention. Auto-migration runs two upgrades on pre-existing databases: (1) creates the composite unique index on `files(rel_path, device_id)` if missing, and (2) upgrades the `devices` table from raw-token PK to UUID PK + `token_hash` column, rewriting `files.device_id` references accordingly.

### Web UI

File browser at `localhost:9876` with three views: All Files, Local, Server.

| Endpoint | Method | Purpose |
|---|---|---|
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
- **Connection timeout**: All sync dial attempts use a 10-second timeout (`syncDialTimeout`). If the server is unreachable, the sync fails immediately with a clear error instead of hanging silently.
- **Import size cap**: The `/api/files/import` endpoint enforces a 512 MiB per-request body limit via `http.MaxBytesReader`; requests exceeding this return HTTP 413.
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

For normal homelab use, `config.toml` is all you need. 

The `MYDRIVE_TOKEN` environment variable is
  - optional client side override
  - it replaces the `token` field at runtime without modifying the file. 
  - Useful when running `mydrive sync` in scripts or CI environments where writing the token to disk is undesirable.

**Server config** (environment variables):

| Variable | Default | Purpose |
| `MYDRIVE_DB_PATH` | `./mydrive.db` | SQLite database path |
| `MYDRIVE_DATA_DIR` | `./VaultData` | Object store root |

Set these before starting the server. Default is automatically applied if no env vars detected. For a one-off run:

```bash
MYDRIVE_DB_PATH=/data/mydrive.db MYDRIVE_DATA_DIR=/data/VaultData ./mydrive-server
```

For a persistent systemd service, add them under `[Service]` in your systems service file:

```.service
[Service]
Environment=MYDRIVE_DB_PATH=/data/mydrive.db
Environment=MYDRIVE_DATA_DIR=/data/VaultData
ExecStart=/usr/local/bin/mydrive-server
```

Server listens on `:9000`. Web UI listens on `127.0.0.1:9876`.

## Security Notes

- `config.toml` contains the auth token in plaintext — restrict file permissions on the homelab machine
- Raw tokens are never stored — the `devices` table stores only `HMAC-SHA256(key="mydrive-v1", data=token)` as `token_hash`; the raw token is discarded after registration.
- All relative paths are validated against path traversal attacks (`..`) on both client and server sides
