# Architecture

**Analysis Date:** 2026-03-11

## Pattern Overview

**Overall:** Client-Server File Synchronization with Content-Addressable Storage

**Key Characteristics:**
- Bidirectional sync: client-initiated upload and download phases within single TCP connection
- Token-based authentication at handshake layer for device registration
- Content-addressable blob storage (hash-based deduplication) on server
- Streaming protocol using gob encoding for binary messages
- Local state tracking via JSON for deletion detection across sync cycles
- Last-write-wins conflict resolution with client preference

## Layers

**Common Protocol Layer:**
- Purpose: Shared handshake, packet structures, and encoding/decoding between client and server
- Location: `common/protocol/`, `common/crypto/`
- Contains: Binary protocol definitions (11 command types), handshake validation, encoder/decoder wrappers
- Depends on: Go standard library, modernc.org/sqlite (via server only)
- Used by: Both client and server for all TCP communication

**Client Layer:**
- Purpose: Scan local files, manage sync state, upload changes, download updates, serve web dashboard
- Location: `client/internal/`, `client/cmd/`
- Contains: Scanner, sender, state management, bidirectional syncer, web UI server, config loader
- Depends on: Common protocol layer, Go standard library
- Used by: End users running `vault-sync sync` or `vault-sync daemon`

**Server Layer:**
- Purpose: Accept client connections, validate tokens, deduplicate content, store blobs, track file metadata
- Location: `server/internal/`, `server/cmd/`
- Contains: Connection handler, SQLite database operations, object store, device registry, migration tools
- Depends on: Common protocol layer, modernc.org/sqlite, Go standard library
- Used by: Homelab infrastructure running 24/7 as central repository

## Data Flow

**Upload Phase (Client → Server):**

1. `scanner.ScanDirectory(syncDir)` walks the sync directory recursively, computing SHA-256 hash per file → `[]FileMeta`
2. `state.Load(statePath)` reads the last-synced file manifest from `~/.vaultsync/state.json`
3. **Deletion detection**: Files in state but missing on disk → send `CmdDeleteFile` → server soft-deletes DB record and removes blob if unreferenced
4. **For each file on disk**:
   - `sender.VerifyFile()` sends `CmdCheckFile` with `{RelPath, Hash}` → server responds `CmdFileStatus` (Need or Skip)
   - If `StatusNeed`: `sender.SendFile()` streams file in 4MB chunks:
     - `CmdSendFile` packet with metadata `{RelPath, Hash, Size}`
     - Multiple `CmdFileChunk` packets (4MB each) until complete
   - Server computes running SHA-256, moves temp file to content-addressed storage, upserts DB record
5. `state.SetFile(relPath, hash)` updates in-memory state

**Download Phase (Server → Client):**

1. Send `CmdListServerFiles` → receive `CmdServerFileList` with all non-deleted files from server
2. Compare against state: compute per-file sync status (synced, conflict, download-only, local-only)
3. **For each server-only or changed file**:
   - `CmdRequestFile` with `{RelPath, Hash}` → server streams back:
     - `CmdFileDataHeader` with metadata `{RelPath, Hash, Size}`
     - Multiple `CmdFileDataChunk` packets (4MB each)
   - Client writes to temp file, verifies SHA-256, moves to final path, updates state
4. **For each local-only file** (exists on client, absent from server):
   - Delete local file (server-side deletion detected)
5. `state.Save(statePath)` persists updated manifest to disk

**State Management:**

- `LocalState` in-memory structure: `map[relPath]string{hash}` loaded from JSON at sync start
- Updated during upload as files are successfully sent
- Updated during download as files are successfully received
- Persisted atomically to `~/.vaultsync/state.json` after each complete sync cycle using temp-file-then-rename pattern

## Key Abstractions

**FileMeta (Scanner Output):**
- Purpose: Represent one file during directory scan
- Examples: `client/internal/scanner/scan.go`
- Pattern: Simple struct holding `{Path, Hash, Size}` — immutable once created

**FileRecord (Database Row):**
- Purpose: Track metadata about uploaded files in server DB
- Examples: `server/internal/db/db.go`
- Pattern: Maps relative path → hash + size + device + timestamp; soft-delete flag for async cleanup

**FileTransfer (Protocol Header):**
- Purpose: Announce incoming file with metadata before chunk streaming begins
- Examples: `common/protocol/packet.go`
- Pattern: Sent as `CmdSendFile` payload; declares total size so receiver knows when to finalize

**ObjectStore (Blob Storage):**
- Purpose: Content-addressable file storage with deduplication
- Examples: `server/internal/store/store.go`
- Pattern: Hash-based layout `{baseDir}/objects/{hash[:2]}/{hash[2:]}` prevents path traversal; temp directory for atomic writes

**LocalState (Sync State Tracking):**
- Purpose: Persist which files were synced and their hashes for deletion detection across restarts
- Examples: `client/internal/state/state.go`
- Pattern: JSON map serialization with atomic save via temp file + rename

**Syncer (Full Sync Orchestrator):**
- Purpose: Coordinate complete bidirectional sync in two phases
- Examples: `client/internal/sync/bidirectional.go`
- Pattern: Takes connection + config + state, returns (uploaded, downloaded, deleted counts)

## Entry Points

**Client: `vault-sync sync`**
- Location: `client/cmd/main.go`, function `runSync()`
- Triggers: User runs command from terminal
- Responsibilities: Load config, perform one full sync cycle, print results, exit

**Client: `vault-sync daemon`**
- Location: `client/cmd/main.go`, function `runDaemon()`
- Triggers: User runs command from terminal
- Responsibilities: Load config, start HTTP UI server on `:9876`, perform initial sync, poll for "Force Sync" button presses, manage graceful shutdown

**Server: `vault-sync-server`**
- Location: `server/cmd/main.go`, function `runServer()`
- Triggers: Systemd/init system or manual execution
- Responsibilities: Listen on port `:9000`, accept TCP connections, spawn goroutine per connection, manage database and object store lifecycle

**Server: `vault-sync-server register <device-name>`**
- Location: `server/cmd/main.go`, function `runRegister()`
- Triggers: Administrator runs command once per new device
- Responsibilities: Generate 256-bit random token, insert into devices table, print token to stdout for manual config entry

## Error Handling

**Strategy:** Defensive fail-fast with recovery context

**Patterns:**

- **Authentication failure** → `receiver.HandleConnection()` closes TCP immediately without explanation (prevents token enumeration)
- **File transfer hash mismatch** → Log warning, discard temp file, continue next file (prevents corruption)
- **Database errors during dedup check** → Assume file is needed, re-send (fail-safe: better to duplicate than lose data)
- **Blob missing but DB says it exists** → Request re-send, log warning (corruption recovery)
- **Sync phase errors** → `doSyncCycle()` catches, records in status, logs, continues daemon (UI remains responsive)
- **State save failure** → Return error up to caller; if in daemon, record in status but don't crash
- **Incomplete file transfer** → Temp file automatically cleaned on next `CmdSendFile` or connection close (cleanup-on-new-file pattern)

## Cross-Cutting Concerns

**Logging:** Go standard `log` package; prefixed messages include device name (server) or operation context (client); all I/O timing printed to stdout during sync

**Validation:** All relative paths checked with `store.ValidateRelPath()` on server to reject absolute paths and `..` traversal attempts; socket communication validates magic number and version in handshake

**Authentication:** Token stored in plaintext in SQLite devices table (Phase 2); validated on every connection before any file operations allowed; no per-operation auth checks (implicit once handshake succeeds)

**Concurrency:**
- **Server**: One goroutine per client connection; SQLite configured with `SetMaxOpenConns(1)` to serialize writes and prevent lock contention
- **Client daemon**: Main goroutine blocked in `select{}` waiting for sync triggers or shutdown signal; sync cycle runs synchronously (blocking until complete); UI server runs in separate goroutine

---

*Architecture analysis: 2026-03-11*
