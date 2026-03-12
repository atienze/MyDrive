# Integrations

## External Services

**None.** HomelabSecureSync is a self-contained homelab tool with no external API calls, cloud services, or third-party integrations.

## Database — SQLite

**Driver:** `modernc.org/sqlite` v1.46.1 (pure Go, no CGO)
**Location:** `server/internal/db/db.go`
**File:** Configured via `VAULTSYNC_DB_PATH` env var (default `./vaultsync.db`)

### Schema

```sql
CREATE TABLE devices (
    id         TEXT PRIMARY KEY,           -- 64-char hex token (crypto/rand)
    name       TEXT NOT NULL,              -- device name ("MacBook-Pro")
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    rel_path    TEXT NOT NULL UNIQUE,      -- "Documents/resume.pdf"
    hash        TEXT NOT NULL,             -- SHA-256 hex (64 chars)
    size        INTEGER NOT NULL,
    device_id   TEXT NOT NULL,             -- token of uploading device
    uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted     BOOLEAN DEFAULT FALSE      -- soft-delete flag
);
```

### Key Operations

| Method | SQL | Purpose |
|--------|-----|---------|
| `FileExists(path, hash)` | `SELECT id FROM files WHERE rel_path=? AND hash=? AND deleted=FALSE` | Dedup check |
| `UpsertFile(path, hash, device, size)` | `INSERT ... ON CONFLICT(rel_path) DO UPDATE` | Create/update file record |
| `GetAllFiles()` | `SELECT rel_path, hash, size FROM files WHERE deleted=FALSE` | Server manifest |
| `GetFileHash(path)` | `SELECT hash FROM files WHERE rel_path=? AND deleted=FALSE` | Lookup for download/delete |
| `MarkDeleted(path)` | `UPDATE files SET deleted=TRUE WHERE rel_path=?` | Soft-delete |
| `HashRefCount(hash)` | `SELECT COUNT(*) FROM files WHERE hash=? AND deleted=FALSE` | Safe blob deletion check |
| `RegisterDevice(token, name)` | `INSERT INTO devices(id, name)` | Token registration |
| `GetDeviceName(token)` | `SELECT name FROM devices WHERE id=?` | Auth lookup |

**Concurrency:** `SetMaxOpenConns(1)` — single writer prevents SQLite lock contention across goroutines.

## TCP Protocol (Custom Binary)

**Port:** 9000
**Encoding:** Go `encoding/gob`
**Location:** `common/protocol/`

### Handshake

```go
type Handshake struct {
    MagicNumber uint32 // 0xCAFEBABE
    Version     uint8  // 2
    Token       string // 64-char hex
}
```

### Commands (1–11)

| Cmd | Name | Direction | Payload | Purpose |
|-----|------|-----------|---------|---------|
| 1 | CmdPing | C→S | — | Keepalive |
| 2 | CmdSendFile | C→S | `FileTransfer{RelPath, Hash, Size}` | Upload header |
| 3 | CmdCheckFile | C→S | `CheckFileRequest{RelPath, Hash}` | Dedup check |
| 4 | CmdFileStatus | S→C | `FileStatusResponse{Status}` | Need(1) or Skip(2) |
| 5 | CmdFileChunk | C→S | `[]byte` | 4MB upload chunk |
| 6 | CmdDeleteFile | C→S | `DeleteFileRequest{RelPath}` | Server soft-delete |
| 7 | CmdListServerFiles | C→S | `ListServerFilesRequest{Token}` | Request manifest |
| 8 | CmdServerFileList | S→C | `ServerFileListResponse{Files}` | File manifest |
| 9 | CmdRequestFile | C→S | `RequestFileRequest{RelPath, Hash}` | Request download |
| 10 | CmdFileDataHeader | S→C | `FileDataHeader{RelPath, Hash, Size}` | Download header |
| 11 | CmdFileDataChunk | S→C | `[]byte` | 4MB download chunk |

**Chunk size:** 4 MB (`4 * 1024 * 1024`)

## HTTP API (Web UI)

**Port:** 9876 (localhost only)
**Location:** `client/internal/ui/server.go`
**Template:** `client/internal/ui/templates/dashboard.html` (embedded via `//go:embed`)

### Endpoints

| Method | Path | Purpose | Acquires Mutex |
|--------|------|---------|----------------|
| GET | `/` | Dashboard HTML | No |
| GET | `/api/status` | Daemon status JSON | No |
| POST | `/api/force-sync` | Trigger full sync cycle | No (signals channel) |
| GET | `/api/files/client` | Local file list | No |
| GET | `/api/files/server` | Server file list via TCP | No |
| POST | `/api/files/upload?path=` | Push file to server | Yes |
| POST | `/api/files/download?path=` | Pull file from server | Yes |
| DELETE | `/api/files/server?path=` | Delete from server | Yes |
| DELETE | `/api/files/client?path=` | Delete local file | Yes |

### Error Handling

Sentinel errors mapped to HTTP status codes via `httpStatusFromErr()`:

| Error | HTTP Status |
|-------|-------------|
| `ErrServerUnreachable` | 502 Bad Gateway |
| `ErrAuthFailed` | 502 Bad Gateway |
| `ErrTimeout` | 504 Gateway Timeout |
| `ErrHashMismatch` | 500 Internal Server Error |
| Other | 500 Internal Server Error |

## Authentication

**Mechanism:** Pre-shared token (64-char hex)
**Registration:** `vault-sync-server register "DeviceName"` → generates token via `crypto/rand`, stores in `devices` table, prints once
**Auth flow:** Client includes token in TCP handshake → server looks up `devices.id` → match continues, mismatch closes connection
**No TLS:** All traffic is plaintext (homelab trusted network assumption)

## File Storage — Content-Addressed

**Location:** `server/internal/store/store.go`
**Root:** Configured via `VAULTSYNC_DATA_DIR` (default `./VaultData`)

```
VaultData/
├── objects/{hash[:2]}/{hash[2:]}   # blob storage
└── tmp/{uuid}                       # in-progress transfers
```

- **Dedup:** Same hash = same blob, regardless of path or device
- **Safe delete:** `DeleteObject(hash, refCount)` — only removes blob if refCount == 0
- **Atomic writes:** temp file → rename to final path
