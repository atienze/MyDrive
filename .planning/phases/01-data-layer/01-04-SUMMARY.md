---
phase: 01-data-layer
plan: "04"
subsystem: server-db
tags: [gap-closure, hard-delete, metadata, sqlite, tdd]
dependency_graph:
  requires: ["01-03"]
  provides: ["PurgeDeletedRecord", "DBLR-08"]
  affects: ["server/internal/db", "server/internal/receiver"]
tech_stack:
  added: []
  patterns: ["physical row removal after soft-delete", "AND deleted=TRUE safety guard"]
key_files:
  created: []
  modified:
    - server/internal/db/db.go
    - server/internal/db/db_test.go
    - server/internal/receiver/handler.go
decisions:
  - "PurgeDeletedRecord placed immediately after MarkDeleted in db.go for logical grouping"
  - "Purge runs unconditionally in CmdDeleteFile (after MarkDeleted + blob cleanup), not only when refCount==0 — each device's row is removed regardless of other devices' references"
  - "Failure to purge is logged as a warning (non-fatal) — metadata leak is less severe than blocking a delete response"
metrics:
  duration: "~2 minutes"
  completed: "2026-03-14"
  tasks_completed: 2
  files_modified: 3
---

# Phase 01 Plan 04: Hard-delete orphaned file metadata rows (DBLR-08) Summary

**One-liner:** Physical row deletion from the files table via PurgeDeletedRecord, called in CmdDeleteFile after soft-delete and blob cleanup, eliminating indefinitely-accumulating orphaned metadata rows.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (RED) | Add failing tests for PurgeDeletedRecord | 8ad49bf | server/internal/db/db_test.go |
| 1 (GREEN) | Add PurgeDeletedRecord to db.go | 09a3310 | server/internal/db/db.go |
| 2 | Wire PurgeDeletedRecord into CmdDeleteFile | 9238e9a | server/internal/receiver/handler.go |

## What Was Built

**PurgeDeletedRecord (db.go):** A new DB method that executes `DELETE FROM files WHERE rel_path = ? AND device_id = ? AND deleted = TRUE`. The `AND deleted = TRUE` clause is a safety guard ensuring live (non-deleted) rows are never accidentally removed. The method is idempotent — calling it on an already-purged or non-existent row is a no-op.

**CmdDeleteFile integration (handler.go):** After `MarkDeleted` soft-deletes the row and `HashRefCount`/`DeleteObject` handles blob cleanup, a call to `PurgeDeletedRecord` physically removes the metadata row. The purge is unconditional — even if the blob is still referenced by other devices under different paths, this device's row is removed. Errors are logged as warnings (non-fatal) to avoid blocking the delete response.

**Test coverage (db_test.go):**
- `TestPurgeDeletedRecord` — verifies row is physically gone after purge (raw `COUNT(*)` without deleted filter), idempotency, and safety guard (live rows untouched)
- `TestBlobCleanupAfterAllDevicesDelete` — extended to call `PurgeDeletedRecord` for both devices after all deletes, then assert `SELECT COUNT(*) FROM files WHERE hash = 'deadhash'` returns 0

## Verification

- All DB tests pass: `go test -C server ./internal/db/... -v` — 9/9 PASS
- Server builds cleanly: `go build ./server/...`
- Full test suite: all packages green (common, client, server)

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

Files exist:
- server/internal/db/db.go — FOUND (contains PurgeDeletedRecord)
- server/internal/db/db_test.go — FOUND (contains TestPurgeDeletedRecord)
- server/internal/receiver/handler.go — FOUND (contains PurgeDeletedRecord call)

Commits exist:
- 8ad49bf — test(01-04): add failing tests for PurgeDeletedRecord
- 09a3310 — feat(01-04): add PurgeDeletedRecord DB method
- 9238e9a — feat(01-04): wire PurgeDeletedRecord into CmdDeleteFile handler
