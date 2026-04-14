---
phase: 03-bulk-select
plan: "01"
subsystem: ui
tags: [javascript, html, css, bulk-select, dashboard]

# Dependency graph
requires:
  - phase: 02-individual-actions
    provides: renderFilesTable(), renderServerTable(), .action-btn CSS, .files-table structure
provides:
  - bulk-bar CSS with position:sticky; bottom:0 for both views
  - App.bulkMode state sub-object (localActive, serverActive, localSelected Set, serverSelected Set)
  - enterBulkMode(view) and clearBulkMode(view) functions
  - Select button in both Local and Server view headers
  - #local-bulk-bar and #server-bulk-bar HTML outside .files-table-wrap
  - clearBulkMode() hooked into switchTab(), filesNavTo(), serverNavTo()
affects: [03-02, 03-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Bulk mode toggled via App.bulkMode flags; tables add/remove .bulk-active class"
    - ".th-bulk/.td-bulk columns hidden via CSS :not(.bulk-active) selector — no JS toggle needed"
    - "Bulk bars placed as siblings of .files-table-wrap (not children) so position:sticky works against #app-main overflow container"

key-files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html

key-decisions:
  - "Bulk bars are siblings of .files-table-wrap, not children, so position:sticky works correctly (files-table-wrap has overflow-x:auto which breaks sticky)"
  - "enterBulkMode disables the Select button with opacity 0.5 to prevent double-entry; clearBulkMode re-enables it"
  - "bulkPush/bulkPull/bulkDeleteLocal/bulkDeleteServer stubs referenced in HTML but defined in Plan 03 — acceptable ReferenceError until then"

patterns-established:
  - "view parameter pattern: functions accept 'local' or 'server' string and use dynamic property access (App.bulkMode[view+'Active'])"

requirements-completed: [BULK-01, BULK-02, BULK-04, BULK-09, A11Y-04]

# Metrics
duration: 12min
completed: 2026-04-14
---

# Phase 3 Plan 01: Bulk Select Infrastructure Summary

**Sticky bulk-select bars, App.bulkMode state machine, and clearBulkMode() hooked into all navigation paths for Local and Server views**

## Performance

- **Duration:** 12 min
- **Started:** 2026-04-14T21:15:00Z
- **Completed:** 2026-04-14T21:27:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Added `.bulk-bar` CSS with `position:sticky; bottom:0` and `.visible` toggle for both views
- Added `.files-table:not(.bulk-active) .th-bulk/.td-bulk` hidden-by-default column rules
- Added `App.bulkMode` sub-object with localActive, serverActive, localSelected (Set), serverSelected (Set)
- Added `enterBulkMode(view)` and `clearBulkMode(view)` functions
- Hooked `clearBulkMode` into `switchTab()`, `filesNavTo()`, and `serverNavTo()` to clear selection on navigation

## Task Commits

Each task was committed atomically:

1. **Task 1 + 2: Bulk CSS, HTML, state, functions, and navigation hooks** - `310a6ed` (feat)

**Plan metadata:** (docs commit pending)

## Files Created/Modified

- `client/internal/ui/templates/dashboard.html` - Added bulk-bar CSS block, Select buttons in both view headers, #local-bulk-bar and #server-bulk-bar HTML, App.bulkMode state, enterBulkMode/clearBulkMode functions, navigation hooks

## Decisions Made

- Bulk bars placed as siblings of `.files-table-wrap`, not children — `overflow-x:auto` on `.files-table-wrap` would break `position:sticky` if bars were inside it
- `enterBulkMode` disables the Select button (opacity 0.5) to prevent double-activation; `clearBulkMode` re-enables it
- Action function stubs (`bulkPush`, `bulkPull`, `bulkDeleteLocal`, `bulkDeleteServer`) are referenced in HTML but will be defined in Plan 03 — the forward references produce ReferenceErrors only if clicked before Plan 03 runs

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02 can now add checkbox rendering to `renderFilesTable()` and `renderServerTable()` — the `.th-bulk`/`.td-bulk` CSS columns and `.bulk-active` table class are already wired
- Plan 03 can define `bulkPush()`, `bulkPull()`, `bulkDeleteLocal()`, `bulkDeleteServer()` and update the bulk count span

---
*Phase: 03-bulk-select*
*Completed: 2026-04-14*
