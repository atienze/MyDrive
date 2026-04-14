---
phase: 03-bulk-select
plan: "02"
subsystem: ui
tags: [javascript, html, css, bulk-select, checkboxes, dashboard]

# Dependency graph
requires:
  - phase: 03-bulk-select
    plan: "01"
    provides: App.bulkMode state, enterBulkMode/clearBulkMode, .th-bulk/.td-bulk CSS, .bulk-bar HTML
provides:
  - handleRowCheck(view, cb): row checkbox change handler updating Set + UI
  - handleSelectAll(view, cb): select-all thead checkbox handler
  - updateSelectAllState(view): indeterminate/checked state for select-all
  - updateBulkBar(view): .visible toggle + count text update on bulk bar
  - Checkbox th in both static theads (#local-select-all, #server-select-all)
  - td.td-bulk with row-check input injected into file rows in both render functions
  - Empty td.td-bulk injected into folder rows in both render functions
  - .bulk-active class toggled on both tables in render functions
  - Set re-application after tbody.innerHTML rebuild in both render functions
affects: [03-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Row checkboxes use data-path attribute (escapeHtml applied) + dataset.path read in handler — no inline path embedding needed"
    - "Set re-application pattern: after tbody.innerHTML = html, querySelectorAll row-check forEach cb.checked = Set.has()"
    - "Select-all indeterminate: allChk.indeterminate = checked > 0 && checked < total"

key-files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html

key-decisions:
  - "data-path uses escapeHtml() only — no additional .replace() needed since dataset.path access bypasses HTML parsing (no function-call path embedding)"
  - "Folder rows get empty td.td-bulk for column alignment but no checkbox — folder rows are not selectable by design"

patterns-established:
  - "Re-apply pattern: renderFilesTable and renderServerTable both call updateSelectAllState + updateBulkBar after Set re-application so UI stays consistent on every re-render"

requirements-completed: [BULK-03, BULK-04]

# Metrics
duration: 9min
completed: 2026-04-14
---

# Phase 3 Plan 02: Bulk Checkbox Wiring Summary

**Functional row checkboxes in both file tables: handleRowCheck/handleSelectAll/updateSelectAllState/updateBulkBar wired to App.bulkMode Sets with indeterminate select-all and live bulk count updates**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-14T23:32:04Z
- **Completed:** 2026-04-14T23:41:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- Added `#local-select-all` and `#server-select-all` checkbox inputs inside `th.th-bulk` in both static theads
- Added four selection-management functions: `handleRowCheck`, `handleSelectAll`, `updateSelectAllState`, `updateBulkBar`
- Injected `td.td-bulk` with `.row-check` input into file rows in both `renderFilesTable()` and `renderServerTable()`
- Injected empty `td.td-bulk` into folder rows (preserves column alignment, folders not selectable)
- Both render functions now toggle `.bulk-active` on the table element and re-apply Set state after `tbody.innerHTML` rebuild

## Task Commits

Each task was committed atomically:

1. **Task 1: Add checkbox th to both static theads + four selection-management functions** - `976c84d` (feat)
2. **Task 2: Inject checkbox td into both render functions with Set re-application after render** - `5503d9e` (feat)

**Plan metadata:** (docs commit pending)

## Files Created/Modified

- `client/internal/ui/templates/dashboard.html` - Added select-all th inputs, four JS functions, td.td-bulk cells in both render functions, bulk-active toggling, Set re-application blocks

## Decisions Made

- `data-path` attribute uses `escapeHtml()` only — no `.replace(/'/g, '&#39;')` needed because `dataset.path` reads the decoded attribute value directly (no inline function-call path embedding)
- Folder rows get empty `td.td-bulk` for column alignment but intentionally no checkbox — folders cannot be bulk-selected

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 03 can now implement `bulkPush()`, `bulkPull()`, `bulkDeleteLocal()`, `bulkDeleteServer()` — the checkbox infrastructure is fully wired
- Checkboxes are hidden when not in bulk mode (CSS `.files-table:not(.bulk-active) .td-bulk { display: none }` from Plan 01)
- Select-all checkbox supports indeterminate state for partial selections

---
*Phase: 03-bulk-select*
*Completed: 2026-04-14*
