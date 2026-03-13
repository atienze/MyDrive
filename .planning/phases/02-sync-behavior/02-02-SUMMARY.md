---
phase: 02-sync-behavior
plan: 02
subsystem: sync
tags: [go, tcp, cli, push-only, pull-command, bidirectional-removal]

# Dependency graph
requires:
  - phase: 01-data-layer
    provides: ServerFileEntry.DeviceID field used by PullFile to filter server manifest by device

provides:
  - Push-only RunSync replacing bidirectional RunFullSync in bidirectional.go
  - PullFile function in operations.go for explicit cross-device file download
  - vault-sync pull --from <device> <path> CLI subcommand in main.go
  - Updated SetLastSync signature accepting (uploaded int, err error) in status.go

affects: [03-web-ui, integration-testing, deployment]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Push-only default sync — upload phase only, no automatic download from server"
    - "Explicit pull model — cross-device downloads require vault-sync pull --from <device> <path>"
    - "PullFile uses FetchServerFileList + filtered lookup by DeviceID before dedicated download connection"

key-files:
  created: []
  modified:
    - client/internal/sync/bidirectional.go
    - client/internal/sync/operations.go
    - client/cmd/main.go
    - client/internal/status/status.go
    - client/cmd/main_test.go

key-decisions:
  - "SetLastSync simplified to (uploaded int, err error) — lastSyncDown/lastSyncDeleted fields kept in struct and JSON for Phase 3 UI compatibility"
  - "cleanEmptyDirs retained in bidirectional.go for potential future use even though downloadPhase removed"
  - "PullFile does two connections: one FetchServerFileList to resolve hash, one dedicated for download — clean separation, reuses existing helpers"

patterns-established:
  - "Sync is push-only: RunSync() returns (uploaded int, err error) — no download return values"
  - "CLI pull subcommand uses flag.NewFlagSet for --from flag parsing"

requirements-completed: [SYNC-01, SYNC-02, SYNC-03, SYNC-04]

# Metrics
duration: 2min
completed: 2026-03-13
---

# Phase 02 Plan 02: Push-Only Sync and Explicit Pull Command Summary

**Converted bidirectional sync to push-only RunSync and added vault-sync pull --from <device> <path> for explicit cross-device file downloads**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-13T06:01:35Z
- **Completed:** 2026-03-13T06:03:49Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Removed bidirectional download phase from sync — vault-sync sync now only uploads
- Added PullFile to operations.go that filters server manifest by DeviceID then downloads the file by hash
- Added vault-sync pull --from <device> <path> CLI subcommand wired end-to-end
- Updated SetLastSync, doSyncCycle, runSyncCycle, runSyncCycleWithState to match new push-only signatures

## Task Commits

Each task was committed atomically:

1. **Task 1: Convert bidirectional.go to push-only and add PullFile to operations.go** - `7840968` (feat)
2. **Task 2: Update CLI callers, status reporting, and add pull subcommand** - `0e4d069` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `client/internal/sync/bidirectional.go` - Replaced RunFullSync with RunSync (push-only); removed downloadPhase, downloadFile, listServerFiles
- `client/internal/sync/operations.go` - Added PullFile function with device-filtered cross-device download
- `client/cmd/main.go` - Added pull subcommand, updated runSync/runSyncCycle/runSyncCycleWithState/doSyncCycle signatures
- `client/internal/status/status.go` - Simplified SetLastSync to (uploaded int, err error)
- `client/cmd/main_test.go` - Updated test assertion to match new 2-return signature of runSyncCycleWithState

## Decisions Made

- SetLastSync simplified to `(uploaded int, err error)` — the `lastSyncDown` and `lastSyncDeleted` fields are kept in the Status struct and StatusSnapshot JSON for Phase 3 UI compatibility (they will always be 0 after push-only sync)
- `cleanEmptyDirs` retained in bidirectional.go for potential future use
- PullFile uses two separate connections: first FetchServerFileList to resolve the hash for the given device+path, then a dedicated download connection — clean, composable, reuses existing helpers

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated main_test.go to match new 2-return signature**
- **Found during:** Task 2 (Update CLI callers)
- **Issue:** `client/cmd/main_test.go` line 48 called `runSyncCycleWithState` expecting 4 return values `(_, _, _, _)` but the function now returns only 2
- **Fix:** Changed `_, _, _, _ = runSyncCycleWithState(...)` to `_, _ = runSyncCycleWithState(...)`
- **Files modified:** `client/cmd/main_test.go`
- **Verification:** `go test -C client ./...` passes
- **Committed in:** `0e4d069` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug fix in test)
**Impact on plan:** Test was checking the old return signature; fix was mechanical and necessary for compilation.

## Issues Encountered

None beyond the test signature mismatch above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Push-only sync and explicit pull command are complete
- Phase 3 (web UI) can update the dashboard to reflect push-only sync (remove downloaded/deleted counters) — lastSyncDown/lastSyncDeleted fields are still in the JSON contract to avoid breaking Phase 3 before it is ready
- No blockers

---
*Phase: 02-sync-behavior*
*Completed: 2026-03-13*

## Self-Check: PASSED

- All modified files exist on disk
- Commit 7840968 (Task 1) verified
- Commit 0e4d069 (Task 2) verified
