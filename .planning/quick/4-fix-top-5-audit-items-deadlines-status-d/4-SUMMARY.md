---
phase: quick-4
plan: 01
subsystem: client-sync, server-receiver, server-db
tags: [bug-fix, dead-code, dry, validation, deadline]
dependency_graph:
  requires: []
  provides: [op-deadline-on-full-sync, accurate-downloaded-count, device-name-validation, hmac-dedup, dead-code-removed]
  affects: [client/cmd/main.go, client/internal/status/status.go, client/internal/sync/operations.go, client/internal/config/config.go, server/internal/receiver/handler.go, client/internal/sync/bidirectional.go, server/internal/db/db.go]
tech_stack:
  added: []
  patterns: [exported-constant, 3-arg-status-setter, config-validation-chain]
key_files:
  modified:
    - client/internal/sync/operations.go
    - client/cmd/main.go
    - client/internal/status/status.go
    - client/internal/config/config.go
    - server/internal/receiver/handler.go
    - client/internal/sync/bidirectional.go
    - server/internal/db/db.go
decisions:
  - "OpDeadline exported (not DialTimeout) — only the op deadline was missing from full sync; dial timeout was already handled"
  - "SetLastSync takes 3 params (uploaded, downloaded, err) — matches bidirectional sync which already returned both counts"
  - "device_name validation added after sync_dir check — consistent ordering with other required fields"
  - "crypto/hmac removed from handler.go imports — sha256 and hex kept (used by fileTransfer chunk finalization)"
  - "bytes import removed from bidirectional.go — only used by deleted sendDeleteFile"
metrics:
  duration: "~10 minutes"
  completed_date: "2026-04-07"
  tasks_completed: 3
  files_modified: 7
---

# Quick Task 4: Fix Top 5 Audit Items Summary

**One-liner:** 5-minute op deadline on full sync, accurate downloaded count in status, device_name config validation, HMAC deduplication via db.ComputeTokenHash, and removal of 5 dead methods.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | BUG-02 + export OpDeadline | 6ac0543 | operations.go, main.go |
| 2 | BUG-01 + VAL-01 | 0f1f4b5 | status.go, main.go, config.go |
| 3 | DRY-01 + DEAD-01/02/03 | 239e0ae | handler.go, bidirectional.go, db.go |

## Changes by Audit Item

### BUG-02: Missing op deadline on full sync
`client/internal/sync/operations.go`: Renamed `opDeadline` constant to `OpDeadline` (exported). Updated all 5 internal `conn.SetDeadline` call sites.

`client/cmd/main.go`: Added `conn.SetDeadline(time.Now().Add(bisync.OpDeadline))` in both `runSyncCycleWithState` and `runSyncCycle` immediately after the handshake encode succeeds. Full sync cycles can no longer hang indefinitely.

### BUG-01: Downloaded count always 0 in dashboard
`client/internal/status/status.go`: Changed `SetLastSync(uploaded int, err error)` to `SetLastSync(uploaded, downloaded int, err error)`. Replaced hardcoded `s.lastSyncDown = 0` with `s.lastSyncDown = downloaded`.

`client/cmd/main.go`: Updated `doSyncCycle` call site from `appStatus.SetLastSync(uploaded, err)` to `appStatus.SetLastSync(uploaded, downloaded, err)`. The `downloaded` variable was already captured from `runSyncCycleWithState`.

### VAL-01: Missing device_name validation
`client/internal/config/config.go`: Added `if cfg.DeviceName == ""` check after the `sync_dir` check. Starting the client with an empty `device_name` now exits with `config: device_name is required`.

### DRY-01: Inline HMAC duplicated from db.ComputeTokenHash
`server/internal/receiver/handler.go`: Replaced the 3-line `hmac.New` / `mac.Write` / `hex.EncodeToString` block with a single `db.ComputeTokenHash(shake.Token)` call. Removed the now-unused `crypto/hmac` import. `crypto/sha256`, `encoding/hex` were retained — both are still used by the fileTransfer chunk finalization path.

### DEAD-01/02: sendDeleteFile and cleanEmptyDirs
`client/internal/sync/bidirectional.go`: Deleted the `sendDeleteFile` method (31 lines) and `cleanEmptyDirs` function (18 lines). Removed the `bytes` import which was only used by the deleted code. Remaining imports (`encoding/gob`, `fmt`, `log`, `os`, `path/filepath`) are all still referenced.

### DEAD-03: GetFilesForDevice and GetSharedFiles
`server/internal/db/db.go`: Deleted `GetFilesForDevice` (25 lines) and `GetSharedFiles` (25 lines). These queries were superseded by `GetAllFiles` in the Phase 1 multi-device refactor and had no remaining callers.

## Verification

```
go build ./client/...  # PASS
go build ./server/...  # PASS
go build ./common/...  # PASS
```

Spot-checks:
- `OpDeadline` exported and `SetDeadline` called in both sync functions in main.go
- `SetLastSync` call in main.go uses 3 arguments
- `device_name` validation block present in config.go
- `ComputeTokenHash` in handler.go; no `hmac.New`
- No `sendDeleteFile` or `cleanEmptyDirs` in bidirectional.go
- No `GetFilesForDevice` or `GetSharedFiles` in db.go

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- [FOUND] client/internal/sync/operations.go — OpDeadline exported
- [FOUND] client/cmd/main.go — SetDeadline in both sync functions, 3-arg SetLastSync
- [FOUND] client/internal/status/status.go — SetLastSync(uploaded, downloaded int, err error)
- [FOUND] client/internal/config/config.go — device_name validation
- [FOUND] server/internal/receiver/handler.go — ComputeTokenHash, no hmac.New
- [FOUND] client/internal/sync/bidirectional.go — sendDeleteFile and cleanEmptyDirs absent
- [FOUND] server/internal/db/db.go — GetFilesForDevice and GetSharedFiles absent
- [FOUND] commit 6ac0543
- [FOUND] commit 0f1f4b5
- [FOUND] commit 239e0ae
