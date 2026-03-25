---
phase: quick-6
plan: 01
subsystem: server/db, client/sync
tags: [bug-fix, sql, migration, sync-behavior]
dependency_graph:
  requires: []
  provides: [db-schema-migration, local-only-deletion-tracking]
  affects: [server/internal/db/db.go, client/internal/sync/bidirectional.go]
tech_stack:
  added: []
  patterns: [sqlite-pragma-index-info, schema-migration-on-open]
key_files:
  created: []
  modified:
    - server/internal/db/db.go
    - client/internal/sync/bidirectional.go
decisions:
  - "Use pragma_index_list + pragma_index_info instead of sqlite_master sql LIKE to detect composite unique indexes — autoindexes have NULL sql column so LIKE patterns fail"
  - "Local sync deletions remove from state.json only; explicit Remove via Web UI still sends CmdDeleteFile"
metrics:
  duration: ~20min
  completed_date: "2026-03-25T08:34:48Z"
  tasks_completed: 2
  files_modified: 2
---

# Quick Task 6: Fix SQL ON CONFLICT Error on Push and Server Deletion Cascade

**One-liner:** Schema migration using pragma_index_info detects missing composite UNIQUE(rel_path, device_id) on pre-existing databases, and sync cycle no longer cascades local deletions to the server.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add migrateSchema() to db.go for UNIQUE constraint | aa526e6, 2a88c49 | server/internal/db/db.go |
| 2 | Remove CmdDeleteFile cascade from uploadPhase() | 5bc9eba | client/internal/sync/bidirectional.go |

## What Was Built

### Task 1: Database Schema Migration

Added `migrateSchema()` method to `DB`, called from `Open()` after `createTables()`. The migration:

1. Queries `pragma_index_list('files')` for all unique indexes on the files table
2. For each index, queries `pragma_index_info(indexName)` to get its columns
3. If any index covers both `rel_path` and `device_id` — schema is up to date, logs and returns
4. If no such index found — creates `idx_files_path_device` via `CREATE UNIQUE INDEX IF NOT EXISTS`

This handles both fresh databases (where `CREATE TABLE` generates a system autoindex with NULL `sql` in sqlite_master — undetectable via LIKE patterns) and old databases with only `UNIQUE(rel_path)`. The `ON CONFLICT(rel_path, device_id)` in `UpsertFile` now resolves correctly against the guaranteed-present composite index.

### Task 2: Local-Only Deletion Tracking

Changed the deletion detection loop in `uploadPhase()` (bidirectional.go) to remove files from `state.json` tracking only, without sending `CmdDeleteFile` to the server. This makes the server a persistent store: files accumulate server-side regardless of local deletions. Users who want to remove a server copy use the Web UI "Remove from server" action, which still sends `CmdDeleteFile` via the explicit `DeleteServerFile` path in operations.go.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] sqlite_master LIKE detection fails for system autoindexes**
- **Found during:** Task 1 — server db tests all failed after initial migration implementation
- **Issue:** The first implementation queried `sqlite_master WHERE sql LIKE '%rel_path%' AND sql LIKE '%device_id%'`. SQLite system autoindexes (generated from inline `UNIQUE(...)` in CREATE TABLE) have `sql = NULL` in sqlite_master, so the LIKE pattern never matched them. Fresh databases always showed "index not found" and tried to run the migration, hitting "index associated with UNIQUE or PRIMARY KEY constraint cannot be dropped" when attempting `DROP INDEX IF EXISTS sqlite_autoindex_files_1`.
- **Fix:** Replaced sqlite_master query with `pragma_index_list('files')` + `pragma_index_info(name)` loop. This iterates actual index columns regardless of how the index was created, correctly detecting both system autoindexes and named user indexes.
- **Files modified:** server/internal/db/db.go
- **Commits:** aa526e6 (initial, broken), 2a88c49 (corrected)

## Self-Check

### Files exist
- `server/internal/db/db.go` — modified, migrateSchema() present
- `client/internal/sync/bidirectional.go` — modified, sendDeleteFile() call removed from loop

### Commits exist
- aa526e6 — feat(quick-6): add migrateSchema
- 2a88c49 — fix(quick-6): use pragma_index_info to detect composite unique index safely
- 5bc9eba — feat(quick-6): remove CmdDeleteFile cascade from sync cycle

### Test results
All tests pass: common, server (including all 10 db tests), client.

## Self-Check: PASSED
