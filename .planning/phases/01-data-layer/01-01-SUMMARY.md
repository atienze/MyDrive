---
phase: 01-data-layer
plan: 01
status: complete
started: 2026-03-12
completed: 2026-03-12
---

# Plan 01-01: Schema Migration Binary — Summary

## Objective
Create the SQLite schema migration binary that transitions the `files` table from a global `UNIQUE(rel_path)` constraint to a composite `UNIQUE(rel_path, device_id)` constraint.

## What Shipped
- `server/cmd/migrate-v3/main.go` — standalone migration binary with `needsMigration` detection (counts autoindex columns via `PRAGMA index_info`), duplicate-row safety check, and transactional table-rename migration
- `server/cmd/migrate-v3/main_test.go` — 4 TDD tests: basic migration, device_id preservation, idempotency, and composite constraint validation

## Key Decisions
- **Detection via column count, not index presence**: Both v2 and v3 schemas produce `sqlite_autoindex_files_1`. Detection uses `PRAGMA index_info` to count columns (1 = v2, 2 = v3) instead of checking index existence.
- **Table-rename strategy**: Creates `files_new` with composite constraint, copies data, drops old, renames — standard SQLite migration pattern since ALTER TABLE can't modify constraints.

## Commits
- `7a29900` — test(01-01): add failing tests for migrate-v3 binary (RED)
- `95a0855` — feat(01-01): fix needsMigration to count index columns (GREEN)

## Self-Check: PASSED
- [x] All 4 tests pass
- [x] Migration correctly detects v2 vs v3 schema
- [x] Idempotent (safe to run twice)
- [x] Composite constraint enforced post-migration

## Key Files
### Created
- `server/cmd/migrate-v3/main.go`
- `server/cmd/migrate-v3/main_test.go`
