# Architecture

## Pattern

Client-server with custom TCP protocol. Three Go modules in a workspace:

```
common/     ← shared protocol + crypto (no external deps)
client/     ← device-side daemon, HTTP UI, sync logic
server/     ← homelab receiver, SQLite DB, object storage
```

The client's HTTP server (`localhost:9876`) acts as a proxy between the browser and the VaultSync TCP server (`:9000`). Each browser action opens a fresh TCP connection, runs one operation, and closes.

## Layers

### Server

```
TCP Listener (cmd/main.go)
  ↓ spawns goroutine per connection
Handler (receiver/handler.go)
  ├── Auth: validate handshake token → db.GetDeviceName()
  ├── Commands: dispatch by Packet.Cmd
  │   ├── CmdCheckFile  → db.FileExists() + store.HasObject()
  │   ├── CmdSendFile   → temp file → verify hash → store.StoreFromTemp() → db.UpsertFile()
  │   ├── CmdDeleteFile → db.MarkDeleted() → store.DeleteObject(hash, refCount)
  │   ├── CmdListServerFiles → db.GetAllFiles()
  │   └── CmdRequestFile → store.OpenObject() → stream chunks
  └── Storage layer
      ├── db/db.go       — SQLite path→hash mapping
      └── store/store.go — content-addressed blob I/O
```

### Client

```
Entry (cmd/main.go)
  ├── "sync" subcommand  → one-shot full sync
  └── "daemon" subcommand → UI server + sync loop
      ├── UI Server (ui/server.go on :9876)
      │   ├── Dashboard HTML (//go:embed templates/)
      │   ├── Status API → status/status.go (thread-safe)
      │   ├── File list APIs → scanner + TCP proxy
      │   └── Mutation APIs → sync/operations.go (acquire syncMu)
      └── Sync Loop (select on forceSyncCh / sigCh)
          └── sync/bidirectional.go → uploadPhase() + downloadPhase()
              ├── scanner/scan.go    — walk dir, compute hashes
              ├── sender/client.go   — CmdCheckFile + CmdSendFile
              ├── state/state.go     — local file state persistence
              └── sync/operations.go — DialAndHandshake, single-file ops
```

## Data Flow

### Upload (Client → Server)

```
1. scanner.ScanDirectory(syncDir) → []FileMeta{RelPath, Hash, Size}
2. Compare against state.json → detect local deletions
3. For each file:
   a. CmdCheckFile{RelPath, Hash} → CmdFileStatus
   b. If StatusNeed: CmdSendFile{RelPath, Hash, Size} + CmdFileChunk (4MB chunks)
4. Server: temp file → verify SHA-256 → store.StoreFromTemp() → db.UpsertFile()
5. Client: state.SetFile(relPath, hash) → state.Save()
```

### Download (Server → Client)

```
1. CmdListServerFiles → CmdServerFileList (all non-deleted files)
2. Compare against state.Files
3. For each missing/changed file:
   a. CmdRequestFile{RelPath} → CmdFileDataHeader + CmdFileDataChunk
   b. Write to temp → verify hash → atomic rename
   c. state.SetFile(relPath, hash)
4. For each file in state but not on server:
   a. os.Remove(localPath) → cleanEmptyDirs()
   b. state.RemoveFile(relPath)
5. state.Save()
```

### Delete

```
Client-initiated:
  CmdDeleteFile{RelPath} → server: db.MarkDeleted() → if refCount==0: store.DeleteObject()

Server-side detection (during download phase):
  File in state but not in server list → os.Remove(local) + state.RemoveFile()
```

## Entry Points

| Binary | File | Subcommands |
|--------|------|-------------|
| `vault-sync-server` | `server/cmd/main.go` | `register "name"`, default: serve |
| `vault-sync` | `client/cmd/main.go` | `sync` (one-shot), `daemon` (UI + loop) |
| `vault-migrate` | `server/cmd/migrate/main.go` | (none — runs migration) |

## Key Abstractions

| Type | Package | Responsibility |
|------|---------|----------------|
| `Syncer` | `client/internal/sync` | Orchestrates full bidirectional sync cycle |
| `LocalState` | `client/internal/state` | Persists relPath→hash map to state.json |
| `Status` | `client/internal/status` | Thread-safe daemon status for UI consumption |
| `UIServer` | `client/internal/ui` | HTTP proxy between browser and TCP server |
| `Config` | `client/internal/config` | TOML config loader with path helpers |
| `ObjectStore` | `server/internal/store` | Content-addressed blob storage with dedup |
| `DB` | `server/internal/db` | SQLite wrapper for files + devices tables |

## Concurrency Model

| Concern | Mechanism |
|---------|-----------|
| Per-client TCP handling | Goroutine per connection |
| SQLite write contention | `SetMaxOpenConns(1)` — single writer |
| UI mutation vs sync loop | `sync.Mutex` (`syncMu`) shared between daemon and UI handlers |
| Status reads/writes | `sync.RWMutex` inside Status struct |
| State persistence | Protected by `syncMu` — only one writer at a time |
| File downloads | Atomic temp+rename (no partial files visible) |
