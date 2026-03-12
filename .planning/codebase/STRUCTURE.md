# Directory Structure

## Full Tree

```
HomelabSecureSync/
├── go.work                              # Workspace: common, client, server
├── CLAUDE.md                            # Project instructions
│
├── common/                              # Shared module (no external deps)
│   ├── go.mod
│   ├── crypto/
│   │   └── hash.go                      # CalculateFileHash() — SHA-256
│   └── protocol/
│       ├── handshake.go                 # Handshake{MagicNumber, Version, Token}
│       └── packet.go                    # Packet, Encoder/Decoder, all command types
│
├── client/                              # Client module
│   ├── go.mod                           # Deps: toml, common
│   ├── cmd/
│   │   ├── main.go                      # Entry: sync | daemon subcommands
│   │   └── main_test.go                 # Tests: shared state, storage stats
│   └── internal/
│       ├── config/
│       │   └── config.go                # Config struct, TOML loader, path helpers
│       ├── scanner/
│       │   └── scan.go                  # ScanDirectory() → []FileMeta
│       ├── sender/
│       │   └── client.go                # SendFile(), VerifyFile() over TCP
│       ├── state/
│       │   └── state.go                 # LocalState — relPath→hash persistence
│       ├── status/
│       │   └── status.go               # Thread-safe Status for UI
│       ├── sync/
│       │   ├── bidirectional.go         # Syncer — full sync orchestration
│       │   ├── operations.go            # Single-file ops, DialAndHandshake
│       │   └── operations_test.go       # Mock TCP server tests
│       └── ui/
│           ├── server.go                # HTTP server — 9 endpoints
│           ├── server_test.go           # httptest-based handler tests
│           └── templates/
│               └── dashboard.html       # Two-panel file browser (embedded)
│
├── server/                              # Server module
│   ├── go.mod                           # Deps: sqlite, common
│   ├── cmd/
│   │   ├── main.go                      # Entry: register | serve
│   │   └── migrate/
│   │       └── main.go                  # One-time Phase 3 migration tool
│   └── internal/
│       ├── auth/
│       │   └── register.go              # GenerateToken() — crypto/rand
│       ├── db/
│       │   └── db.go                    # SQLite: files + devices tables
│       ├── receiver/
│       │   └── handler.go               # TCP command dispatcher
│       └── store/
│           ├── store.go                 # ObjectStore — content-addressed blobs
│           └── store_test.go            # Dedup, ref counting, cleanup tests
│
└── .planning/                           # GSD planning artifacts
    └── codebase/                        # This mapping
```

## Package Roles

### `common/` — Shared Protocol & Crypto

| Package | Files | Role |
|---------|-------|------|
| `crypto` | `hash.go` | SHA-256 file hashing (streaming, returns hex string) |
| `protocol` | `handshake.go`, `packet.go` | Binary protocol: handshake, packet envelope, all 11 command types, encoder/decoder wrappers |

### `client/` — Device-Side Application

| Package | Files | Role |
|---------|-------|------|
| `cmd` | `main.go` | CLI entry point: `sync` and `daemon` subcommands, sync mutex wiring |
| `config` | `config.go` | TOML config loader (`~/.vaultsync/config.toml`), path helpers |
| `scanner` | `scan.go` | Recursive directory walk, per-file SHA-256, returns `[]FileMeta` |
| `sender` | `client.go` | Upload primitives: `SendFile()` (chunks), `VerifyFile()` (check+status) |
| `state` | `state.go` | Local file state: `map[relPath]hash`, atomic save to `state.json` |
| `status` | `status.go` | Thread-safe daemon status: connected, syncing, file counts, activity log |
| `sync` | `bidirectional.go`, `operations.go` | Full sync orchestration + single-file TCP operations |
| `ui` | `server.go`, `templates/dashboard.html` | HTTP server on :9876, two-panel file browser UI |

### `server/` — Homelab Receiver

| Package | Files | Role |
|---------|-------|------|
| `cmd` | `main.go` | TCP listener, `register` subcommand |
| `cmd/migrate` | `main.go` | One-time migration from path-based to hash-based storage |
| `auth` | `register.go` | Token generation (32 bytes → 64-char hex) |
| `db` | `db.go` | SQLite abstraction: files table, devices table, ref counting |
| `receiver` | `handler.go` | Per-connection TCP handler: auth + command dispatch |
| `store` | `store.go` | Content-addressed object storage with dedup and safe deletion |

## Naming Conventions

### Files
- One file per concern (e.g., `scan.go`, `client.go`, `state.go`)
- Test files: `*_test.go` in same package
- Entry points: `main.go` in `cmd/` directories
- Templates: `templates/` subdirectory with `//go:embed`

### Packages
- Lowercase, single-word: `db`, `store`, `auth`, `sync`, `state`
- `internal/` for private packages (Go convention)
- `cmd/` for binary entry points

### Variables & Functions
- CamelCase (Go standard): `relPath`, `syncDir`, `forceSyncCh`
- Verb-noun methods: `SendFile()`, `VerifyFile()`, `MarkDeleted()`
- Query methods: `FileExists()`, `HasObject()`, `DeviceExists()`
- Constructors: `New()`, `Open()`, `Load()`

## Runtime Artifacts

```
# Server-side (created at runtime)
./vaultsync.db                           # SQLite database
./VaultData/objects/{hash[:2]}/{hash[2:]} # Content-addressed blobs
./VaultData/tmp/{uuid}                   # In-progress transfers

# Client-side (created at runtime)
~/.vaultsync/config.toml                 # Device configuration
~/.vaultsync/state.json                  # Sync state persistence
~/VaultDrive/                            # Default sync directory
```
