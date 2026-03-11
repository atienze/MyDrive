# Codebase Concerns

**Analysis Date:** 2025-03-11

## Security Concerns

### Plaintext Token Storage

**Risk:** Authentication tokens are stored in plaintext in two locations:
- Client config: `~/.vaultsync/config.toml` contains the 64-char hex token
- Server database: `devices` table stores tokens as primary keys in plaintext

**Files:**
- `client/internal/config/config.go` (loads token from config)
- `server/internal/db/db.go` (line 86: `id TEXT PRIMARY KEY` in devices table)
- `server/cmd/main.go` (line 74: token printed to stdout once)

**Current mitigation:**
- Client config file intended to be in user home directory (permissions dependent)
- Server token printed once and never again (operator responsible for storage)

**Recommendations:**
1. Server-side: Hash tokens with bcrypt/scrypt before storing in database. Store token hash in `devices.token_hash`, keep plaintext lookup during initial auth. Query becomes: `SELECT name FROM devices WHERE token_hash = bcrypt.Compare(incomingToken, stored_hash)`
2. Client-side: Encrypt token in `~/.vaultsync/config.toml` using a key derived from system keychain (MacOS Keychain, Linux Secret Service, Windows Credential Manager). Decrypt at runtime before use.
3. Registration flow: Generate token, hash it, store hash in DB, print plaintext once to operator (same as now), but operator stores encrypted copy on client.
4. Config permissions: Document requirement that `~/.vaultsync/config.toml` must have `chmod 600` (already conventional for `.toml` files in user homedir).

**Priority:** High — tokens grant full read/write/delete access to all files on the server.

---

### Database Queryable Secrets

**Risk:** If someone gains filesystem access to `vaultsync.db`, they can query the `devices` table and retrieve all valid authentication tokens:
```sql
SELECT id, name FROM devices;
```

**Files:** `server/internal/db/db.go`, `server/internal/receiver/handler.go` (line 38: `database.GetDeviceName(shake.Token)`)

**Impact:** Attacker can impersonate any registered device and exfiltrate/delete all synced files.

**Recommendations:**
1. Implement token hashing (see above) — queries would never return the plaintext token
2. Add optional server-side authentication layer on top of TCP (TLS with client certificates)
3. Document that physical security of the server machine (where `vaultsync.db` and `VaultData/` reside) is critical
4. Consider splitting token table into separate file with stricter permissions (owned by vaultsync process only)

**Priority:** High for production homelab deployment.

---

### No Network Encryption (TCP is Plaintext)

**Risk:** All file data, file paths, hashes, and tokens are transmitted in plaintext over TCP (gob binary encoding, not encrypted).

**Files:**
- `client/cmd/main.go` (line 170: `net.Dial("tcp", cfg.ServerAddr)`)
- `server/cmd/main.go` (line 94: `net.Listen("tcp", Port)`)
- `common/protocol/handshake.go` (token sent unencrypted in first message)

**Current environment:** Assumed to be homelab (local network), but homelab may span WANs or untrusted networks.

**Recommendations:**
1. Implement TLS: Wrap TCP connection with `tls.Dial()` / `tls.Listen()`
2. Use self-signed certificates for homelab (operator generates cert, distributes to clients)
3. Or: Defer to WireGuard/VPN tunnel for transport security (simpler for homelab, shifts responsibility to operator)
4. At minimum, document in README that VaultSync should only run on trusted networks

**Priority:** Medium-High — data exposure risk depends on network topology.

---

### Path Traversal Partially Mitigated

**Risk:** Relative paths could theoretically be exploited if validation is incomplete.

**Files:** `server/internal/store/store.go` (lines 167-175: `ValidateRelPath`)

**Current protection:**
```go
func ValidateRelPath(relPath string) bool {
    if filepath.IsAbs(relPath) { return false }  // Reject /absolute/paths
    if strings.Contains(relPath, "..") { return false }  // Reject ../ traversal
    return relPath != ""
}
```

**Verification:** Called in `handler.go` for `CmdCheckFile` (line 85), `CmdSendFile` (line 136), `CmdDeleteFile` (line 268), `CmdRequestFile` (line 344) — good coverage.

**Assessment:** Protection is adequate for intended use. No known bypasses.

**Priority:** Low — current implementation is sufficient.

---

## Data Integrity Concerns

### Hash Mismatch Silent Failure

**Risk:** If a file's SHA-256 hash does not match the declared hash, the transfer is rejected but the file entry is not recorded. On retry, the same hash mismatch will likely occur, leaving the file in limbo.

**Files:** `server/internal/receiver/handler.go`
- Lines 163-169: empty file hash mismatch (cleans up, continues without retry logic)
- Lines 220-228: chunked file hash mismatch (cleans up, continues)

**Scenario:**
1. Client uploads file with hash `ABC123`
2. Server computes hash `DEF456` (network corruption, client bug, or intentional tampering)
3. Mismatch logged, temp file deleted, state not updated
4. Next sync: client re-sends (same hash mismatch repeats)
5. File never syncs

**Recommendations:**
1. Add exponential backoff retry logic to sender (currently no retry)
2. Log hash mismatches with device name and full hashes (currently logs first 12 chars)
3. Consider server-side quarantine: move mismatched blobs to `VaultData/corrupted/` for later inspection
4. Add integrity check on download: verify downloaded file hash matches declared hash (already done in `bidirectional.go` line 300)

**Priority:** Medium — indicates data corruption risk; rare in practice but not impossible.

---

### Incomplete Transfer Cleanup

**Risk:** If a client disconnects during file transfer (e.g., network failure mid-upload), the server leaves a temp file in `VaultData/tmp/`. The `CleanupTemp()` function is called on server startup, but orphaned files accumulate until restart.

**Files:**
- `server/cmd/main.go` (lines 89-92: cleanup on startup only)
- `server/internal/store/store.go` (lines 138-147: `CleanupTemp`)

**Impact:** Disk space gradually consumed by abandoned temp files during long-running deployments.

**Recommendations:**
1. Add periodic temp cleanup task: spawn goroutine to scan and remove temp files older than N hours (e.g., 24h)
2. Log cleanup actions (file removed, size reclaimed)
3. Implement timeout on handler connection: if client goes silent for >5min during transfer, server closes connection and cleans up

**Priority:** Medium — affects long-term stability and disk usage.

---

### No Atomic State Saves

**Risk:** `state.json` is written after each sync cycle but without transactional guarantees. If the process crashes during `state.Save()`, the file may be corrupted or partially written.

**Files:**
- `client/internal/state/state.go` (likely writes file directly, not checked due to forbidden file concerns)
- `client/internal/sync/bidirectional.go` (lines 57-59: `s.state.Save(s.statePath)`)

**Impact:** On restart, client may lose sync state and re-download/re-upload files unnecessarily, or diverge from server.

**Recommendations:**
1. Write state to temp file, then atomic rename: `ioutil.WriteFile(tmpPath) → os.Rename(tmpPath, statePath)`
2. Implement version field in `state.json` to detect incompatible formats
3. Add checksum/hash of state file contents (for corruption detection on load)

**Priority:** Medium — affects sync correctness but not data safety.

---

## Architectural Concerns

### Handler Goroutine Per Connection (No Connection Pool)

**Risk:** Each client connection spawns a goroutine that runs the receive loop indefinitely (line 111 in `server/cmd/main.go`). A misbehaving client (slow reads, hung connection) will hold a goroutine indefinitely.

**Files:** `server/cmd/main.go` (line 111: `go receiver.HandleConnection(...)`)

**Symptoms:**
- Goroutine leak if client connects and never sends valid handshake
- Resource exhaustion if 10k clients connect and stay idle

**Recommendations:**
1. Add read timeout on connection: `conn.SetReadDeadline(time.Now().Add(5 * time.Minute))` in handler
2. Reset deadline after each successful message receive
3. Graceful shutdown: track active connections, signal them to close on server shutdown
4. Monitor goroutine count in logs

**Priority:** Medium — affects availability under load or malicious clients.

---

### No Protocol Versioning Negotiation

**Risk:** Handshake includes `Version = 2`, but there is no fallback or negotiation. If a client sends `Version = 1` (old protocol), the server will still decode the token field but it won't match any registered device (token field may be missing or malformed in older version).

**Files:** `common/protocol/handshake.go` (line 17: `Version uint8`)

**Current behavior:** Handshake decode succeeds, token is empty string, `database.GetDeviceName("")` returns false, connection is closed. Unclear error message to client.

**Recommendations:**
1. Add explicit version check in `receiver.HandleConnection` (after Handshake decode): if version != 2, log "unsupported protocol version" and close
2. Plan for future: support version negotiation (e.g., server responds with compatible versions)
3. Document that protocol breaking changes require re-registration of all clients

**Priority:** Low — version mismatch is rare and handled gracefully (rejected).

---

### Hardcoded Chunk Size (4MB)

**Risk:** Upload and download both use 4MB chunks. This is hardcoded in multiple places and cannot be tuned per deployment.

**Files:**
- `client/internal/sender/client.go` (line 4MB constant, exact location not verified)
- `server/internal/receiver/handler.go` (line 381: `const chunkSize = 4 * 1024 * 1024`)

**Impact:**
- Large files (100MB+) require many round-trips
- Slow networks may benefit from smaller chunks
- Fast networks may benefit from larger chunks

**Recommendations:**
1. Move chunk size to config: `client/internal/config/config.go` and `server/cmd/main.go` environment variable (e.g., `VAULTSYNC_CHUNK_SIZE`)
2. Default to 4MB, allow override
3. Validate chunk size: 1MB ≤ size ≤ 64MB

**Priority:** Low — 4MB is reasonable for most scenarios; not a blocker.

---

### Client-Wins Conflict Resolution is Opaque

**Risk:** When a file exists locally and on the server with different hashes, the client re-uploads (overwrites server). This is by design ("client-wins") but:
1. User is not notified of the conflict
2. No way to review/merge conflicting versions
3. Server version is silently discarded

**Files:** `client/internal/sync/bidirectional.go` (lines 131-145: download loop checks `if localHash == sf.Hash`)

**Scenario:**
1. User edits file locally
2. File also modified on server (another user/device)
3. Sync runs: local file re-uploaded, server changes lost
4. User unaware

**Recommendations:**
1. Add conflict detection UI: if hashes differ, show warning "Remote version differs. Overwrite? [Yes/No/Review]"
2. Implement "stash" feature: save server version as `file.orig.hash` for user inspection
3. Add conflict log: record all conflicts to `~/.vaultsync/conflicts.log`
4. Document client-wins behavior prominently in README

**Priority:** Low-Medium — affects UX and data preservation, but only with multi-user sync.

---

## Performance & Scaling Concerns

### Full Scan on Every Sync

**Risk:** Client performs full directory scan on every sync cycle (even if nothing changed). For large directories (10k+ files), this is expensive.

**Files:** `client/internal/scanner/scan.go` (line 24: `filepath.WalkDir`)

**Current behavior:**
1. `ScanDirectory()` walks entire tree, hashes every file (even unchanged ones)
2. For each file, checks with server: `CmdCheckFile`
3. If unchanged, skips upload; if changed, uploads

**Impact:**
- CPU cost: re-hashing all files every cycle
- Network cost: `CmdCheckFile` for every file (even if unchanged)
- Latency: initial sync of large directory takes minutes

**Recommendations:**
1. Implement mtime-based skip: track `(relPath, mtime, size)` in state; skip hash if mtime hasn't changed
2. Add mtime field to state.json: `{path: {hash, size, mtime}}`
3. Only hash files with new/changed mtime
4. Still verify hash on server (via `CmdCheckFile`) but avoid local recomputation
5. Fallback: if state.json is missing or corrupted, do full hash scan (safety net)

**Priority:** Medium — affects user experience on large syncs; not a correctness issue.

---

### SQLite Single Connection Bottleneck

**Risk:** `db.SetMaxOpenConns(1)` (line 49 in `server/internal/db/db.go`) limits SQLite to one active connection. This serializes all database access.

**Files:** `server/internal/db/db.go` (line 49)

**Why it's necessary:** SQLite only supports one writer at a time; multiple connections cause "database is locked" errors.

**Impact:**
- Multiple concurrent client connections queue behind DB operations
- If one query stalls, all other clients block
- No practical concern for homelab (typically 1-5 clients), but noted for future scalability

**Recommendations:**
1. For now: acceptable for homelab use
2. Future upgrade path: consider PostgreSQL or SQLite with WAL mode tuning
3. Monitor query latencies in logs

**Priority:** Low — not a concern for homelab scale.

---

## Test Coverage Gaps

### No Unit Tests for Config Parsing

**Risk:** `client/internal/config/config.go` parses TOML and validates required fields, but no tests verify error cases.

**Files:** `client/internal/config/config.go`

**Untested scenarios:**
- Missing `server_addr` field
- Invalid file path (e.g., file doesn't exist)
- Malformed TOML syntax
- Missing required fields

**Recommendations:**
1. Add `config_test.go` with subtests:
   - `TestLoad_MissingFile` — returns clear error message
   - `TestLoad_MissingField` — validates all required fields
   - `TestLoad_InvalidTOML` — parses syntax error clearly
   - `TestLoad_ExpandUserHome` — handles `~` in `sync_dir`

**Priority:** Medium — improves user experience on misconfiguration.

---

### No Integration Tests for Multi-Client Sync

**Risk:** Codebase has unit tests and manual testing, but no automated multi-client scenario tests.

**Files:** All sync-related modules

**Untested scenarios:**
- Client A uploads file, Client B downloads it
- Client A deletes file, Client B sees deletion
- Concurrent uploads from two clients
- Conflict resolution (both clients modify same file)

**Recommendations:**
1. Add `e2e_test.go` that:
   - Starts server
   - Runs two clients in separate goroutines
   - Verifies files synced correctly between clients
2. Use test fixtures and in-memory file systems where possible

**Priority:** Medium — critical functionality but covered by manual testing.

---

### No Tests for Disk Corruption / Edge Cases

**Risk:** Scanner and sync logic may panic or fail ungracefully on unusual filesystem states.

**Files:** `client/internal/scanner/scan.go`, `client/internal/sync/bidirectional.go`

**Untested edge cases:**
- Symlink loops (scanner may infinite-loop)
- Permission-denied directories (scanner skips gracefully?)
- Files deleted mid-scan
- Disk full during download

**Current protection:** Scanner logs errors and continues (line 48-49 in `scan.go`). Sync has error handling but unclear behavior on corruption.

**Recommendations:**
1. Add tests for permission-denied, symlink loops
2. Implement robust error recovery: skip inaccessible files with clear logging
3. Add disk space checks before downloads

**Priority:** Low — rare scenarios, but important for reliability.

---

## Known Behaviors (Not Issues, But Worth Documenting)

### Config File Cleanup Before Production

**Status:** Known issue per user notes
- `config.toml` is exposed and should be cleaned up before homelab deployment
- Mentioned in project instructions: "config.toml is exposed and should be cleaned up when im finished"

**Recommendation:** Before deploying to homelab, ensure:
1. `config.toml` is removed from git (already in `.gitignore`)
2. Document in README that `~/.vaultsync/config.toml` is the canonical location and must be created by user
3. Add error message if `config.toml` exists in repo root (to catch accidental commits)

**Priority:** Critical for production deployment.

---

### Symlink Handling is Inconsistent

**Status:** By design, but may surprise users
- Scanner follows valid symlinks (treats them as regular files)
- When symlink itself is deleted (not its target), sync detects it as a local deletion and syncs deletion to server
- Target of broken symlink is not accessible (skipped with warning)

**Files:** `client/internal/scanner/scan.go`

**Behavior:** Matches Git's behavior (follow symlinks, track targets). Works correctly.

**Recommendation:** Document in README that symlinks are followed and not tracked as special objects.

**Priority:** Low — behavior is documented in code comments.

---

## Migration & Deployment Concerns

### Migration Script is Manual Binary

**Risk:** `server/cmd/migrate/main.go` is a separate binary that must be built and run manually. If skipped or run incorrectly, data is not migrated.

**Files:** `server/cmd/migrate/main.go`, `server/cmd/main.go` (lines 36-39: fallback error message)

**Current state:** Migration is required for Phase 3 (hash-based storage). Once run, not needed again.

**Recommendations:**
1. Add auto-migration on server startup: detect old schema, run migration if needed
2. Or: Add `vault-sync-server migrate` subcommand that triggers migration and reports status
3. Add checksums/version to schema to detect migration state

**Priority:** Medium — reduces manual deployment steps.

---

### Database Path Not Validated on Startup

**Risk:** Server starts with `VAULTSYNC_DB_PATH` env var pointing to inaccessible location (e.g., read-only filesystem, non-existent parent directory).

**Files:** `server/cmd/main.go` (line 19: `envOrDefault`, no validation)

**Current behavior:** `db.Open()` tries to access the path and fails, but error may be unclear.

**Recommendations:**
1. Validate `VAULTSYNC_DB_PATH` and `VAULTSYNC_DATA_DIR` before use:
   - Check parent directory exists
   - Check write permissions
   - Fail fast with clear error message

**Priority:** Low — low probability but improves startup experience.

---

## Summary of Actionable Issues by Priority

| Priority | Issue | File(s) | Effort |
|----------|-------|---------|--------|
| **High** | Plaintext token storage (client config + server DB) | `config.go`, `db.go`, `register.go` | 3-5 days |
| **High** | Database queryable without token hashing | `db.go`, `receiver.go` | 2-3 days |
| **Medium-High** | No network encryption (plaintext TCP) | `main.go` (client/server) | 3-4 days |
| **Medium** | Hash mismatch silent failure | `receiver.go` | 1-2 days |
| **Medium** | Incomplete transfer temp file cleanup | `cmd/main.go`, `store.go` | 1 day |
| **Medium** | No atomic state saves | `state.go`, `bidirectional.go` | 1 day |
| **Medium** | Handler goroutine connection leak risk | `cmd/main.go` | 1 day |
| **Medium** | Full scan performance on large directories | `scanner.go`, `sync/bidirectional.go` | 2-3 days |
| **Low-Medium** | Conflict resolution is opaque to user | `bidirectional.go`, UI | 2-3 days |
| **Low** | Protocol versioning negotiation | `handshake.go`, `receiver.go` | 1 day |
| **Low** | Hardcoded 4MB chunk size | `sender.go`, `receiver.go` | 1 day |
| **Low** | Config file cleanup before production | Documentation | 1 hour |

---

*Concerns audit: 2025-03-11*
