---
phase: 02-individual-actions
plan: "03"
subsystem: ui
tags: [vanilla-js, dashboard, file-actions, server-files]

# Dependency graph
requires:
  - phase: 02-individual-actions
    provides: Action button CSS and local file actions (pushFile, deleteLocalFile, renderFilesTable Actions column) from plans 02-01 and 02-02
provides:
  - pullFile(relPath) — downloads server file to local machine via POST /api/files/download
  - deleteServerFile(relPath) — removes server file via DELETE /api/files/server with confirm()
  - Server file table Actions column with Pull and Delete buttons in renderServerTable()
  - Human-verified completion of all Phase 2 individual action requirements
affects: [03-bulk-select]

# Tech tracking
tech-stack:
  added: []
  patterns: [escapeHtml() + .replace(/\'/g, '&#39;') for onclick attribute safety, confirm() native browser dialog for destructive actions]

key-files:
  created: []
  modified: [client/internal/ui/templates/dashboard.html]

key-decisions:
  - "confirm() native browser dialog used for delete confirmation — no custom modal in Phase 2"
  - "pullFile() only calls refreshData() after download — server copy unchanged so no loadServerViewData() needed"
  - "deleteServerFile() calls loadServerViewData() then refreshData() — removes file from server table and updates sync dots"
  - "escapeHtml() + .replace(/'/g, '&#39;') pattern for onclick attribute safety in server table rows"
  - "Folder rows in renderServerTable() get an empty 4th td for Actions column alignment — folders not actionable"

patterns-established:
  - "onclick inline handlers use escapeHtml() for display and .replace(/'/g, '&#39;') for attribute quoting"
  - "Post-action refresh order: loadServerViewData first, then refreshData — ensures server table and sync dots both update"

requirements-completed: [SERV-05, SERV-06]

# Metrics
duration: ~45min
completed: 2026-04-14
---

# Phase 2 Plan 03: Server File Actions Summary

**pullFile() and deleteServerFile() wired to /api/files/download and DELETE /api/files/server, with Actions column added to renderServerTable() completing Phase 2**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-04-14
- **Completed:** 2026-04-14
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Added `pullFile(relPath)` async function that calls POST `/api/files/download`, shows a temporary "Downloading..." log entry, and calls `refreshData()` on completion
- Added `deleteServerFile(relPath)` async function with `confirm()` guard, calls DELETE `/api/files/server`, then `loadServerViewData()` followed by `refreshData()` to update the server table and sync dots
- Extended `renderServerTable()` with a 4-column header (Name, Size, Device, Actions) and an Actions cell per file row containing Pull and Delete buttons using the `.row-actions` / `.action-btn` pattern from plan 02-01
- Folder rows in the server table received an empty 4th `<td>` for column alignment; folders are not actionable by design
- Human verification approved: all 7 Phase 2 action checks passed by user

## Task Commits

Each task was committed atomically:

1. **Task 1: pullFile and deleteServerFile functions** - `cb24fd3` (feat)
2. **Task 2: Actions column in renderServerTable + human verification** - `cb24fd3` (feat)

## Files Created/Modified
- `client/internal/ui/templates/dashboard.html` - Added pullFile(), deleteServerFile(), extended renderServerTable() with Actions column

## Decisions Made
- Used `confirm()` native browser dialog for delete confirmation — no custom modal added in Phase 2 scope
- `pullFile()` only refreshes via `refreshData()` after download (server copy unchanged; no need to reload server view data)
- `deleteServerFile()` calls `loadServerViewData()` then `refreshData()` — ensures the file disappears from the server table immediately and sync dots are recalculated
- `escapeHtml()` + `.replace(/'/g, '&#39;')` pattern ensures onclick attribute safety for paths containing apostrophes or HTML special chars
- Folder rows receive an empty `<td>` (no buttons) so the 4-column layout remains consistent

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All individual file actions (push, pull, delete) complete for both local and server views
- Phase 3 (Bulk Select) can now build on the `.row-actions` / `.action-btn` CSS and all four single-file API wrappers
- No blockers

---
*Phase: 02-individual-actions*
*Completed: 2026-04-14*
