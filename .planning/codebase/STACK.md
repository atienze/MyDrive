# Technology Stack

## Language & Runtime

| Property | Value |
|----------|-------|
| Language | Go |
| Version | 1.25.6 |
| Workspace | Go workspace (`go.work`) managing 3 modules |
| Build system | `go build` from repo root |

## Modules

```
go.work
├── common/   → github.com/atienze/HomelabSecureSync/common
├── client/   → github.com/atienze/HomelabSecureSync/client
└── server/   → github.com/atienze/HomelabSecureSync/server
```

## Dependencies

### Direct

| Module | Package | Version | Purpose |
|--------|---------|---------|---------|
| server | `modernc.org/sqlite` | v1.46.1 | Pure-Go SQLite (no C/CGO) |
| client | `github.com/BurntSushi/toml` | v1.4.0 | Client config parsing |
| common | (none) | — | Standard library only |

### Notable Transitive (server)

- `dustin/go-humanize` — human-readable sizes
- `google/uuid` — temp file naming
- `modernc.org/libc`, `modernc.org/memory` — SQLite runtime
- `golang.org/x/sys`, `golang.org/x/exp` — OS interfaces

## Build Commands

```bash
# Binaries
go build -o vault-sync-server ./server/cmd
go build -o vault-sync ./client/cmd
go build -o vault-migrate ./server/cmd/migrate

# Tests
go test ./...
go test ./server/internal/db/...   # single package
go test -race ./...                # race detector
```

## Configuration

### Server

Environment variables (no config file):

| Variable | Default | Purpose |
|----------|---------|---------|
| `VAULTSYNC_DB_PATH` | `./vaultsync.db` | SQLite database location |
| `VAULTSYNC_DATA_DIR` | `./VaultData` | Object store root directory |

Hardcoded: TCP listen on `:9000`

### Client

TOML file at `~/.vaultsync/config.toml`:

```toml
server_addr = "192.168.1.100:9000"
token       = "64-char-hex-from-register-command"
sync_dir    = "~/VaultDrive"
```

Auto-generated state: `~/.vaultsync/state.json` (relPath → SHA-256 hash map)

### Ports

| Port | Service | Binding |
|------|---------|---------|
| 9000 | VaultSync TCP server | `0.0.0.0:9000` (all interfaces) |
| 9876 | Web UI HTTP server | `127.0.0.1:9876` (localhost only) |

## Standard Library Usage

| Package | Purpose |
|---------|---------|
| `crypto/sha256`, `crypto/rand` | File hashing, token generation |
| `encoding/gob` | Binary TCP protocol encoding |
| `encoding/json` | HTTP API responses, state.json |
| `net`, `net/http` | TCP server/client, HTTP UI server |
| `database/sql` | SQLite interface |
| `os`, `path/filepath` | File I/O, path manipulation |
| `sync` | Mutex for concurrent access control |
| `embed` | HTML template embedding (`//go:embed`) |
