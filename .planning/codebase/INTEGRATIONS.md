# External Integrations

**Analysis Date:** 2026-03-11

## APIs & External Services

**None Detected:**
- This is an internal, self-contained file synchronization system
- No third-party APIs (Stripe, AWS, etc.) are integrated
- No webhooks to external services
- Entirely on-premises: client-server TCP communication only

## Data Storage

**Databases:**
- SQLite 3 (pure-Go via `modernc.org/sqlite`)
  - Location: `./vaultsync.db` (configurable: `VAULTSYNC_DB_PATH`)
  - Client: Direct Go `database/sql` with modernc.org/sqlite driver
  - Connection: Single persistent connection (intentional, no pooling)
  - Tables:
    - `files` (rel_path, hash, size, device_id, uploaded_at, deleted)
    - `devices` (id, name, token, created_at)

**File Storage:**
- Local filesystem only (on server)
  - Type: Content-addressable object store
  - Path: `./VaultData/objects/{hash[:2]}/{hash[2:]}` (configurable: `VAULTSYNC_DATA_DIR`)
  - Format: Raw binary files, deduplicated by SHA-256
  - Temporary storage: `./VaultData/tmp/` (auto-cleaned on server startup)

**Caching:**
- None - no external caching layer (Redis, Memcached)
- In-memory: Connection handles, file buffers during transfers (4 MB chunks)

## Authentication & Identity

**Auth Provider:**
- Custom token-based system (no OAuth, no LDAP)
  - Implementation: `server/internal/auth/register.go` generates 256-bit random tokens
  - Token format: 64-character hexadecimal string
  - Issued by: Server subcommand `vault-sync-server register "DeviceName"`
  - Validation: Token queried against `devices` table on handshake
  - Plaintext storage in SQLite (see security concerns in CLAUDE.md)

**Protocol:**
- Binary TCP handshake (`common/protocol/handshake.go`)
  - Magic number: `0xCAFEBABE` (validation)
  - Version: 2 (token-based, replaces v1 ClientID)
  - Token field: 64-char hex string from config
  - Authentication failure → immediate connection drop (no retry)

**Client Configuration:**
- TOML file: `~/.vaultsync/config.toml`
  - Fields: `server_addr`, `token`, `sync_dir`
  - Loaded at startup via `client/internal/config/config.go`
  - Missing config → fatal error with actionable instructions

## Monitoring & Observability

**Error Tracking:**
- None - no Sentry, CloudWatch, or external error logging service

**Logs:**
- Approach: Go standard library `log` package
  - Server logs: `log.Printf()` to stderr
    - Connection events, device registration, sync activity
    - File I/O errors, database errors
  - Client logs: `log.Fatalf()` for fatal errors, `log.Printf()` for info
  - UI activity log: In-memory status tracker (`client/internal/status/status.go`)
    - JSON endpoint: `/api/status` (recent 20 activities)
    - Dashboard refreshes every 5 seconds

**No external collectors:** Logs stay local to running process

## CI/CD & Deployment

**Hosting:**
- Self-hosted homelab server (no cloud provider required)
- No container orchestration: raw binary execution
- Docker support: Not configured (docker-compose.yml is empty)

**CI Pipeline:**
- None detected - no GitHub Actions, GitLab CI, or Jenkins
- Manual local testing with `go test ./...`

**Deployment Model:**
- Direct binary execution on homelab server
- Configuration via environment variables:
  - `VAULTSYNC_DB_PATH` - Database location
  - `VAULTSYNC_DATA_DIR` - Object store location
- No systemd service templates or supervisor configs included

## Environment Configuration

**Required env vars (for server deployment):**
- `VAULTSYNC_DB_PATH` (optional, default: `./vaultsync.db`)
- `VAULTSYNC_DATA_DIR` (optional, default: `./VaultData`)

**Client env vars:**
- None required - all config via `~/.vaultsync/config.toml`

**Secrets location:**
- Server: Device tokens stored plaintext in SQLite `devices` table
  - Security risk noted in CLAUDE.md ("token as plaintext in db")
  - Recommendation: Consider token hashing or secure enclave before homelab deploy
- Client: Token stored plaintext in `~/.vaultsync/config.toml`
  - Must protect file permissions: `chmod 600 ~/.vaultsync/config.toml`

## Webhooks & Callbacks

**Incoming:**
- None - server does not accept webhooks

**Outgoing:**
- None - client does not post to external services
- Internal only: HTTP callbacks within UI server (`:9876` → `:9000` TCP for operations)

## Data Flow & Integration Points

**Client → Server:**
```
Browser (localhost:9876)
    ↓ HTTP (vanilla JS fetch)
UI Server (localhost:9876)
    ↓ TCP connection
Server (:9000)
    ↓ Query/Insert
SQLite database + Object store
```

**Server → Client (downloads):**
```
Browser (localhost:9876)
    ↓ HTTP /api/files/download?path=...
UI Server (localhost:9876)
    ↓ TCP CmdRequestFile + CmdFileData chunks
Server (:9000)
    ↓ Read from object store
SQLite + Object store
```

**File Operations:**
- Upload: Client scans local `sync_dir` → computes SHA-256 → streams chunks to server
- Download: Client requests files → server reads from object store → client writes to disk
- Deduplication: Before writing, server checks if hash exists in `files` table + object store
- Deletion: Client sends CmdDeleteFile → server soft-deletes in DB → removes object blob if no other references

## Client-Server Communication Protocol

**Binary Protocol (Go gob-encoded):**
- No REST API, no JSON at TCP level
- Packet structure:
  ```
  Cmd (uint8) | Payload (gob-encoded struct)
  ```
- Commands (1-11):
  1. CmdPing - Keep-alive
  2. CmdSendFile - Upload file metadata
  3. CmdCheckFile - Query if file exists
  4. CmdFileStatus - Response (Need/Skip)
  5. CmdFileChunk - Upload file data (4 MB chunks)
  6. CmdDeleteFile - Soft-delete on server
  7. CmdListServerFiles - Request manifest
  8. CmdServerFileList - File manifest response
  9. CmdRequestFile - Download request
  10. CmdFileDataHeader - Download metadata
  11. CmdFileDataChunk - Download data (4 MB chunks)

**HTTP API (Client UI only):**
- GET `/` - Dashboard HTML
- GET `/api/status` - Current sync status + activity log (JSON)
- POST `/api/force-sync` - Trigger immediate sync
- GET `/api/files/client` - List local files (Phase 5)
- GET `/api/files/server` - List server files (Phase 5)
- POST `/api/files/upload` - Push single file (Phase 5)
- POST `/api/files/download` - Pull single file (Phase 5)
- DELETE `/api/files/server` - Delete from server (Phase 5)
- DELETE `/api/files/client` - Delete locally (Phase 5)

## State Persistence

**Client State File:**
- Format: JSON
- Location: `~/.vaultsync/state.json` (derived from config path)
- Purpose: Track uploaded files' hashes locally for change detection
- Lifecycle:
  - Created on first sync
  - Updated after each successful sync
  - Used to detect local deletions (file in state but not on disk)

**Server State:**
- No persistent state files - all state in SQLite
- Database is the source of truth for:
  - Which files exist (rel_path → hash mapping)
  - Which devices are registered
  - Deduplication info (blob refcounts, if tracked)

## Physical Data Flow

```
Client Machine                      Server (Homelab)
─────────────────                   ───────────────
sync_dir (local files)
    ↓
state.json (local tracking)
    ↓
config.toml (server address + token)
    ↓ TCP port 9000 (encrypted by firewall, not by app)
                                    vaultsync.db (SQLite)
                                    VaultData/objects/ (blobs)
                                    VaultData/tmp/ (temp)
```

**No encryption:** TCP stream is cleartext. Security relies on:
- Network isolation (homelab only)
- Token-based authentication (prevents unauthorized access)
- Firewall rules (restrict port 9000 access)

---

*Integration audit: 2026-03-11*
