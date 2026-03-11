# Codebase Structure

**Analysis Date:** 2026-03-11

## Directory Layout

```
HomelabSecureSync/
├── common/                          # Shared protocol and crypto utilities
│   ├── crypto/
│   │   └── hash.go                  # SHA-256 file hashing
│   ├── protocol/
│   │   ├── handshake.go             # Handshake with magic number, version, token
│   │   └── packet.go                # Binary protocol: 11 command types, encoder/decoder
│   ├── go.mod                       # Common module (imported by client and server)
│   └── go.sum
├── client/                          # Client: file scanner, uploader, downloader, web UI
│   ├── cmd/
│   │   └── main.go                  # Entry point: sync | daemon subcommands
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go            # TOML config loader (~/.vaultsync/config.toml)
│   │   ├── scanner/
│   │   │   └── scan.go              # Directory walk, SHA-256 per file
│   │   ├── sender/
│   │   │   └── client.go            # Upload: VerifyFile, SendFile, CmdCheckFile/CmdSendFile/CmdFileChunk
│   │   ├── state/
│   │   │   └── state.go             # LocalState: JSON persistence of synced file manifest
│   │   ├── status/
│   │   │   └── status.go            # Thread-safe activity log and sync metadata for UI
│   │   ├── sync/
│   │   │   └── bidirectional.go     # Syncer: orchestrate upload + download phases
│   │   └── ui/
│   │       ├── server.go            # HTTP server: dashboard, /api/status, /api/force-sync
│   │       └── templates/
│   │           └── dashboard.html   # Embedded web UI (two-panel file browser)
│   ├── go.mod                       # Client module
│   └── go.sum
├── server/                          # Server: listener, handler, database, object store
│   ├── cmd/
│   │   ├── main.go                  # Entry point: serve | register subcommands
│   │   └── migrate/
│   │       └── main.go              # One-time path-based → hash-based storage migration
│   ├── internal/
│   │   ├── auth/
│   │   │   └── register.go          # GenerateToken: 256-bit crypto/rand, hex-encoded
│   │   ├── db/
│   │   │   └── db.go                # SQLite: files table (rel_path, hash, size, device_id, deleted), devices table (id, name)
│   │   ├── receiver/
│   │   │   └── handler.go           # HandleConnection: auth, dispatch 11 commands, stream chunks
│   │   └── store/
│   │       ├── store.go             # ObjectStore: content-addressed blob storage, temp file management
│   │       └── store_test.go        # ObjectStore unit tests
│   ├── go.mod                       # Server module
│   └── go.sum
├── .planning/
│   └── codebase/                    # GSD analysis documents
│       ├── ARCHITECTURE.md
│       └── STRUCTURE.md
├── VaultData/                       # Runtime: object store + temp directory (created by server)
│   ├── objects/
│   │   ├── {hash[:2]}/
│   │   │   └── {hash[2:]}           # Content-addressed blobs
│   │   └── ...
│   └── tmp/                         # Temp files during transfer (cleaned on startup)
├── go.work                          # Go workspace definition
├── go.work.sum
├── CLAUDE.md                        # Project instructions and phase documentation
├── Makefile                         # (empty placeholder)
├── docker-compose.yml               # (empty placeholder)
└── .gitignore                       # Excludes: config.toml, state.json, *.db, VaultData/
```

## Directory Purposes

**`common/`:**
- Purpose: Shared code usable by both client and server
- Contains: Protocol definitions, gob encoder/decoder, handshake logic, SHA-256 hashing utilities
- Key files: `common/protocol/handshake.go` (version 2, token-based auth), `common/protocol/packet.go` (11 commands)

**`client/cmd/`:**
- Purpose: Executable entry point
- Contains: Command dispatch (sync vs daemon), connection setup, sync orchestration, daemon loop
- Key files: `client/cmd/main.go` (runSync, runDaemon, runSyncCycle, doSyncCycle)

**`client/internal/config/`:**
- Purpose: Configuration management
- Contains: TOML loader, validation, path resolution
- Key files: `client/internal/config/config.go` (Config struct, Load, ConfigPath, StatePath)

**`client/internal/scanner/`:**
- Purpose: Local filesystem scanning
- Contains: Directory tree walk, SHA-256 computation per file, FileMeta construction
- Key files: `client/internal/scanner/scan.go` (ScanDirectory function)

**`client/internal/sender/`:**
- Purpose: File upload logic
- Contains: CmdCheckFile verification, file reading, chunk streaming, hash verification
- Key files: `client/internal/sender/client.go` (VerifyFile, SendFile functions)

**`client/internal/state/`:**
- Purpose: Persistent sync state
- Contains: JSON serialization/deserialization, file tracking, atomic save with temp-file-then-rename
- Key files: `client/internal/state/state.go` (LocalState struct, Load, Save, SetFile, RemoveFile)

**`client/internal/status/`:**
- Purpose: Shared daemon status for UI consumption
- Contains: Thread-safe activity log, sync metadata, connection state
- Key files: `client/internal/status/status.go` (Status struct with lock-protected fields)

**`client/internal/sync/`:**
- Purpose: Full bidirectional synchronization orchestration
- Contains: Upload phase (scan + detect deletions + upload), download phase (list + compare + download + delete)
- Key files: `client/internal/sync/bidirectional.go` (Syncer struct, RunFullSync, uploadPhase, downloadPhase)

**`client/internal/ui/`:**
- Purpose: Web dashboard and API server
- Contains: HTTP handler for dashboard HTML, /api/status endpoint, /api/force-sync trigger
- Key files: `client/internal/ui/server.go` (UIServer, Start, handleDashboard, handleStatus, handleForceSync)
- Key files: `client/internal/ui/templates/dashboard.html` (embedded HTML/CSS/JS for two-panel file browser)

**`server/cmd/`:**
- Purpose: Server executable entry point
- Contains: Subcommand dispatch (serve vs register), listener setup, connection acceptance
- Key files: `server/cmd/main.go` (runServer, runRegister)
- Key files: `server/cmd/migrate/main.go` (one-time migration from old path-based to new hash-based layout)

**`server/internal/auth/`:**
- Purpose: Device token generation
- Contains: 256-bit random token creation, hex encoding
- Key files: `server/internal/auth/register.go` (GenerateToken function)

**`server/internal/db/`:**
- Purpose: SQLite database operations
- Contains: Schema creation, file record UPSERT, dedup checks, device registry, blob reference counting
- Key files: `server/internal/db/db.go` (DB struct, Open, UpsertFile, FileExists, GetAllFiles, MarkDeleted, HashRefCount, GetDeviceName)

**`server/internal/receiver/`:**
- Purpose: Per-connection protocol handling
- Contains: Handshake validation, command dispatch, file transfer state machine, chunk reassembly, hash verification
- Key files: `server/internal/receiver/handler.go` (HandleConnection, streaming state variables, all 11 command handlers)

**`server/internal/store/`:**
- Purpose: Content-addressable blob storage
- Contains: Hash-based directory layout, temp file management, dedup checks, atomic write-then-rename, reference-counted deletion
- Key files: `server/internal/store/store.go` (ObjectStore, ObjectPath, HasObject, WriteObject, StoreFromTemp, ReadObject, OpenObject, DeleteObject, CreateTempFile, CleanupTemp)

**`VaultData/`:**
- Purpose: Runtime blob storage (created by server on startup)
- Contains: `objects/` directory tree storing deduplicated blobs, `tmp/` directory for in-flight transfers
- Generated: Yes (created by ObjectStore.New)
- Committed: No (ignored in .gitignore)

## Key File Locations

**Entry Points:**
- `client/cmd/main.go`: Client executable (sync | daemon)
- `server/cmd/main.go`: Server executable (listen | register)

**Configuration:**
- `~/.vaultsync/config.toml`: Client config (server_addr, token, sync_dir) — resolved by `config.ConfigPath()`
- `VAULTSYNC_DB_PATH` env var: Server DB path override (default `./vaultsync.db`)
- `VAULTSYNC_DATA_DIR` env var: Server data dir override (default `./VaultData`)

**Core Logic:**
- `client/internal/scanner/scan.go`: Directory walking and hashing
- `client/internal/sender/client.go`: Upload protocol logic
- `client/internal/sync/bidirectional.go`: Full sync orchestration
- `server/internal/receiver/handler.go`: Protocol command handling and file transfer state machine
- `server/internal/db/db.go`: All database operations
- `server/internal/store/store.go`: All blob storage operations

**Testing:**
- `server/internal/store/store_test.go`: ObjectStore unit tests
- Run with `go test ./...` from workspace root

**Web UI:**
- `client/internal/ui/server.go`: HTTP server (endpoints: /, /api/status, /api/force-sync)
- `client/internal/ui/templates/dashboard.html`: Two-panel file browser HTML/CSS/JS (embedded via `//go:embed`)

## Naming Conventions

**Files:**
- Package directories use lowercase: `config/`, `scanner/`, `sender/`, `state/`, `status/`, `sync/`, `ui/`
- Implementation files use lowercase: `config.go`, `scan.go`, `client.go`, `state.go`, `status.go`, `bidirectional.go`, `server.go`
- Test files use `_test.go` suffix: `store_test.go`

**Directories:**
- Internal packages nested under `internal/`: `client/internal/`, `server/internal/`
- Command entry points in `cmd/` subdirectory: `client/cmd/`, `server/cmd/`
- Tools in `cmd/{tool}/main.go`: `server/cmd/migrate/main.go`

**Functions:**
- Exported (public) start with capital letter: `Load()`, `New()`, `Open()`, `Close()`, `Scan()`, `Save()`
- Unexported (private) start with lowercase: `createTables()`, `uploadPhase()`, `downloadPhase()`, `sendDeleteFile()`
- Handler functions use `handle{Resource}` pattern: `handleDashboard()`, `handleStatus()`, `handleForceSync()`
- Run functions use `run{Mode}` pattern: `runSync()`, `runDaemon()`, `runServer()`, `runRegister()`

**Types:**
- Structs use CamelCase: `Config`, `FileMeta`, `FileRecord`, `FileTransfer`, `LocalState`, `Syncer`, `ObjectStore`, `DB`, `UIServer`, `Status`
- Interfaces implied by receiver methods (no explicit interface declarations in codebase)

## Where to Add New Code

**New Feature (e.g., compression, filtering):**
- Primary code: `client/internal/sync/bidirectional.go` or new subpackage under `client/internal/sync/`
- Tests: `client/internal/sync/{feature}_test.go`
- If server-side: `server/internal/receiver/handler.go` for protocol dispatch, `server/internal/store/` or `server/internal/db/` for new operations

**New Component/Module (e.g., conflict resolver, cloud storage backend):**
- Implementation: New subdirectory under `client/internal/` or `server/internal/` (e.g., `client/internal/conflict/`)
- Entry point: `{component}.go` with constructor `New{Component}()`
- Tests: `{component}_test.go` in same directory

**Utilities (e.g., new hash algorithm, path normalization):**
- Shared utilities: `common/` (e.g., `common/util/paths.go`)
- Client-only utilities: `client/internal/util/` (new directory)
- Server-only utilities: `server/internal/util/` (new directory)

## Special Directories

**`VaultData/`:**
- Purpose: Runtime blob storage and temp files
- Generated: Yes (created by `store.New()` on server startup)
- Committed: No (ignored by .gitignore)
- Manual cleanup: `rm -rf VaultData/` followed by server restart

**`VaultData/objects/{hash[:2]}/{hash[2:]}`:**
- Purpose: Deduplicated file blobs using first 2 hex chars as prefix directory
- Format: Raw binary file (no wrapper)
- Deletion: Automatic when last reference in DB is removed (reference-counted)

**`VaultData/tmp/`:**
- Purpose: Incomplete file transfers in flight
- Cleanup: Automatic on server startup via `objectStore.CleanupTemp()` or when new `CmdSendFile` arrives
- Manual cleanup: `rm -rf VaultData/tmp/*` (safe anytime)

**`.planning/codebase/`:**
- Purpose: GSD analysis documents for code navigation
- Files: ARCHITECTURE.md, STRUCTURE.md, CONVENTIONS.md, TESTING.md, CONCERNS.md
- Committed: Yes (tracked in git for team reference)

**`~/.vaultsync/` (client-only):**
- Purpose: Client runtime configuration and state
- Location: User home directory (platform-agnostic via `os.UserHomeDir()`)
- Files: `config.toml` (user-configured), `state.json` (auto-generated by sync)
- Cleanup: Delete directory to reset client state (forces full re-sync on next run)

---

*Structure analysis: 2026-03-11*
