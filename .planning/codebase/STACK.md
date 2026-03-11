# Technology Stack

**Analysis Date:** 2026-03-11

## Languages

**Primary:**
- Go 1.25.6 - All server and client application code

## Runtime

**Environment:**
- Go 1.25.6 runtime

**Package Manager:**
- Go modules (workspace-based with go.work)
- Lockfile: `go.sum` present in each module

## Frameworks

**Core:**
- Go standard library `net` package - TCP server implementation for client-server communication
- Go standard library `encoding/gob` - Binary protocol serialization for all packet exchanges
- Go standard library `net/http` - Web UI server at localhost:9876
- Go standard library `log` - Application logging

**Testing:**
- Go standard library `testing` - Unit test framework
- No external test framework dependencies (pure stdlib testing)

**Build/Dev:**
- Go workspace model (go.work) - Multi-module coordination
- Makefile (empty, present but unused)

## Key Dependencies

**Critical:**
- `modernc.org/sqlite` v1.46.1 - Pure-Go SQLite3 driver for file metadata and device registration (`server/internal/db/`)
- `github.com/BurntSushi/toml` v1.4.0 - TOML configuration parser for client config loading (`client/internal/config/`)
- `github.com/google/uuid` v1.6.0 - UUID generation for temporary file handling (`server/internal/store/`)

**Infrastructure:**
- `golang.org/x/exp` v0.0.20251023183803-a4bb9ffd2546 - Extended standard library support (indirect)
- `golang.org/x/sys` v0.37.0 - System call abstractions (indirect)
- `github.com/dustin/go-humanize` v1.0.1 - Human-readable file size formatting (indirect)
- `github.com/mattn/go-isatty` v0.0.20 - Terminal detection (indirect)
- `modernc.org/libc` v1.67.6 - C library bindings for SQLite (indirect)
- `modernc.org/mathutil` v1.7.1 - Math utilities for SQLite (indirect)
- `modernc.org/memory` v1.11.0 - Memory allocator for SQLite (indirect)

## Configuration

**Environment:**
- Server configuration via environment variables (optional):
  - `VAULTSYNC_DB_PATH` - SQLite database file location (default: `./vaultsync.db`)
  - `VAULTSYNC_DATA_DIR` - Object store directory (default: `./VaultData`)
- No `.env` file required - all defaults functional for local development

**Client Configuration:**
- TOML-based: `~/.vaultsync/config.toml`
- Required fields: `server_addr`, `token`, `sync_dir`
- Example:
  ```toml
  server_addr = "127.0.0.1:9000"
  token       = "a3f9b2c1..."
  sync_dir    = "~/VaultDrive"
  ```

**Build:**
- `go.work` - Workspace configuration at repo root
- `server/go.mod` - Server module dependencies
- `client/go.mod` - Client module dependencies
- `common/go.mod` - Common/protocol module (no external dependencies)

## Platform Requirements

**Development:**
- Go 1.25.6+ toolchain
- POSIX-compatible OS (tested on macOS)
- For browser launch: `open` command (macOS) or equivalent

**Production:**
- Homelab server with persistent storage for:
  - SQLite database file (`vaultsync.db`)
  - Object store directory (`VaultData/objects/`)
  - TCP port 9000 (server listener)
- Client machines need:
  - Go runtime or compiled binary
  - Writable `~/.vaultsync/` directory for config and state
  - Network access to server on port 9000

## Network Communication

**Protocol:**
- Custom binary TCP protocol using Go's `encoding/gob`
- Magic number handshake: `0xCAFEBABE`
- Version: 2 (token-based authentication)
- Commands: 11 packet types (CmdPing through CmdFileDataChunk)
- Chunk size: 4 MB per file transfer packet

**Ports:**
- `:9000` - Server TCP listener (client connections)
- `127.0.0.1:9876` - Client UI web server (localhost only)

## Data Storage

**Primary Database:**
- SQLite 3 via `modernc.org/sqlite`
- Location: `./vaultsync.db` (configurable via `VAULTSYNC_DB_PATH`)
- Tables:
  - `files` - file metadata, hashes, upload timestamps
  - `devices` - registered clients with authentication tokens
- Connection pool: 1 connection (by design, to prevent SQLite write-lock contention)

**Object Storage:**
- Content-addressable filesystem storage
- Layout: `VaultData/objects/{hash[:2]}/{hash[2:]}`
- Deduplication via SHA-256 hashing
- Temporary files: `VaultData/tmp/` (cleaned on server start)

**Client State:**
- JSON file: `~/.vaultsync/state.json`
- Tracks local file hashes for change detection
- Persisted across daemon restarts

## Embedded Resources

**UI Assets:**
- `client/internal/ui/templates/dashboard.html` - Embedded via Go `//go:embed` directive
- Single-file HTML with inline CSS and JavaScript
- No external frontend dependencies (vanilla JS)
- No CDN resources required

## Cryptography

**Hash Function:**
- SHA-256 (Go standard library `crypto/sha256`)
- Used for: file deduplication, integrity verification

**Token Generation:**
- `crypto/rand` - Cryptographically secure 32-byte random tokens
- Token length: 64-char hexadecimal (256 bits)
- Stored plaintext in SQLite `devices` table

## Build & Run

**Build Commands (from repo root):**
```bash
# Build server
go build -o vault-sync-server ./server/cmd

# Build client
go build -o vault-sync ./client/cmd

# Build migration tool
go build -o vault-migrate ./server/cmd/migrate

# Run all tests
go test ./...
```

**Run Commands:**
```bash
# Server (listens on :9000, creates vaultsync.db, stores in ./VaultData)
./vault-sync-server

# Register device on server
./vault-sync-server register "MacBook-Pro"

# Client one-shot sync
./vault-sync sync

# Client daemon mode (UI at :9876)
./vault-sync daemon
```

---

*Stack analysis: 2026-03-11*
