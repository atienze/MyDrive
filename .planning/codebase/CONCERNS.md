# Concerns

## Security

### Plaintext Token Storage (Critical)

**Locations:**
- `devices.id` column in SQLite — token stored as primary key in plain hex
- `~/.vaultsync/config.toml` — token field is plaintext

**Risk:** Anyone with DB or filesystem access can extract tokens and impersonate devices.

**Options:**
1. Hash tokens in DB (bcrypt/scrypt) — server compares hash, breaks direct lookup but can iterate
2. Encrypt at rest (AES-256-GCM) — requires key management
3. Restrict file permissions: `config.toml` should be `0600`, DB should be `0600`

### No TLS (Critical)

**Location:** All TCP communication (`server/cmd/main.go`, `client/internal/sync/operations.go`)

**Risk:** Token, file contents, and hashes transmitted in plaintext. Any device on the LAN can sniff traffic.

**Current assumption:** Trusted homelab network.

**Mitigation:** Wrap TCP in TLS 1.3 (`tls.Dial`, `tls.Listen`). Consider mutual TLS for device auth.

### Config File Permissions (Medium)

**Location:** `client/internal/config/config.go` — `Load()` doesn't check file permissions

**Risk:** Default umask may create `config.toml` as `0644` (world-readable).

**Fix:** Check permissions on load, warn if not `0600`. Set `0600` on creation.

### No Rate Limiting (Medium)

**Location:** `server/cmd/main.go` TCP listener

**Risk:** Malicious client can flood server with connections or commands.

**Mitigation:** Per-connection command rate limit, max concurrent connections.

### No Audit Logging (Medium)

**Location:** All server operations

**Risk:** No record of who uploaded/deleted what, when. Can't trace unauthorized changes.

**Fix:** Add `audit` table or structured log file.

## Technical Debt

### Hardcoded Values

| Value | Location | Should Be |
|-------|----------|-----------|
| `:9000` (server port) | `server/cmd/main.go` | Env var or config file |
| `:9876` (UI port) | `client/cmd/main.go` | Config field in `config.toml` |
| `4 * 1024 * 1024` (chunk size) | Multiple files | Named constant in `common/protocol/` |
| `5s` (UI poll interval) | `dashboard.html` | Configurable or smarter polling |
| `10s` (dial timeout) | `sync/operations.go` | Config field |
| `5min` (operation deadline) | `sync/operations.go` | Config field for slow networks |
| `50` (max activities) | `status/status.go` | Configurable |

### Missing Server Config File

Server uses env vars (`VAULTSYNC_DB_PATH`, `VAULTSYNC_DATA_DIR`) and hardcoded port. No equivalent of client's `config.toml`.

### Duplicated Handshake Logic

TCP handshake + gob encoder/decoder setup appears in:
- `client/internal/sender/client.go`
- `client/internal/sync/operations.go` (extracted `DialAndHandshake`)
- `client/internal/sync/bidirectional.go`

`operations.go` consolidates this for UI operations, but `sender/client.go` and `bidirectional.go` still have their own connection setup.

### No File Exclusion Patterns

`scanner/scan.go` hardcodes skip rules (`.git`, `.DS_Store`). No `.gitignore`-style or configurable exclusion patterns. Risk of syncing `node_modules`, `.venv`, build artifacts.

## Performance

### Full Scan Every Sync

`scanner.ScanDirectory()` walks the entire sync directory and computes SHA-256 for every file on each sync cycle. For large directories this is O(n * file_size).

**Mitigation:** Incremental scan using file modification times (mtime). Only re-hash files whose mtime changed since last scan.

### Server File List Transfer

`CmdListServerFiles` returns the complete file manifest every time. No delta/incremental protocol.

**Mitigation:** Pagination, or timestamp-based "changed since" query.

### No Connection Reuse

Each UI operation opens a fresh TCP connection, handshakes, does one operation, and closes. Fine for infrequent user-initiated actions. Would be wasteful if automated.

## Fragile Areas

### Empty File Handling

**Status:** Fixed (previously 0-byte files never finalized because no `CmdFileChunk` was sent).

**Location:** `server/internal/receiver/handler.go` — immediate finalization when `ft.Size == 0`.

### Symlink Behavior

Valid symlinks are followed as regular files. On download, the symlink itself may be overwritten with a regular file. This is by-design but undocumented behavior that could surprise users.

Broken symlinks are gracefully skipped with a log warning.

### Path Traversal

`ValidateRelPath()` in `server/internal/store/store.go` rejects `..` and absolute paths. Edge case: null bytes in paths not explicitly tested.

### SQLite Locking

`SetMaxOpenConns(1)` serializes all DB access. A slow query (e.g., `GetAllFiles()` on a large DB) blocks all other operations including auth checks for new connections.

### Daemon Shutdown

SIGINT triggers `return` but doesn't explicitly shut down the UI server goroutine. The process exits, but no graceful cleanup of in-flight operations.

### Signal Buffer

`sigCh` channel has buffer size 1. If multiple SIGINT signals arrive rapidly, the second is dropped. Not a real problem in practice.

## Missing Features

| Feature | Impact | Effort |
|---------|--------|--------|
| Server config file | Can't customize port without env vars | Low |
| File exclusion patterns | Syncs unwanted directories | Medium |
| Bandwidth limiting | Large syncs saturate LAN | Medium |
| Selective folder sync | Must sync entire directory | Medium |
| Scheduled syncs | Only manual trigger | Low |
| Configurable conflict resolution | Always client-wins | Low |
| Device name display | Can't distinguish devices in logs | Low |

## Test Gaps

| Area | Risk |
|------|------|
| Server handler unit tests | Handler logic untested in isolation |
| Database query tests | Schema and query correctness assumed |
| Race detector (`-race`) | Not routinely run |
| Large file transfers | Chunking edge cases untested |
| Concurrent connections | Multi-client behavior untested |
| Network failure recovery | Partial transfer handling untested |
