---
phase: quick-6
plan: 01
subsystem: web-ui
tags: [bulk-select, navigation, ux-fix]
dependency_graph:
  requires: []
  provides: [fixed-bulk-select-mode]
  affects: [dashboard.html]
tech_stack:
  added: []
  patterns: [css-class-toggle, data-attribute-selection]
key_files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html
decisions:
  - Row selection uses CSS class toggle (.row-selected) on <tr> rather than checkbox inputs — eliminates visible checkboxes and aligns with row-highlight UX pattern
  - clearBulkMode no longer re-renders the table — callers (bulk ops) do their own full refreshes to prevent stale intermediate renders
  - data-path attribute moved from checkbox input to <tr> element itself — enables querySelectorAll-based selection without any form inputs
metrics:
  duration: "10 min"
  completed_date: "2026-04-15"
  tasks_completed: 1
  files_modified: 1
---

# Phase quick-6 Plan 01: Fix Select Mode Navigation, Replace Checkboxes, Add Select All Summary

Row-highlight bulk selection replacing checkbox inputs, folder navigation always active, Select All toggle button, and stale-refresh fix after bulk operations.

## What Was Built

Fixed four bugs in the Web UI bulk select mode in a single `dashboard.html` edit:

1. **Folder navigation in bulk mode** — Removed `App.bulkMode.localActive ? void(0) : filesNavTo(...)` guards from folder row `onclick` in both `renderFilesTable()` and `renderServerTable()`. Folders always navigate.

2. **Checkbox removal + row-highlight selection** — Removed all `<th class="th-bulk">` and `<td class="td-bulk">` elements from both local and server tables (HTML + JS renderers). Removed `handleRowCheck`, `handleSelectAll`, `updateSelectAllState` functions. Added `data-path` attribute directly to file `<tr>` elements. `handleRowTap` now toggles `.row-selected` CSS class and updates the Set directly.

3. **Select All button** — Added `selectAllFiles(view)` function that toggles all visible `tr[data-path]` rows (selects all if not all selected, deselects all if all selected). Added "Select All" button to both `#local-bulk-bar` and `#server-bulk-bar`.

4. **Stale panel state after bulk ops** — Removed `renderFilesTable()` / `renderServerTable()` calls from `clearBulkMode`. These caused a stale re-render before the awaited refresh completed. Bulk ops now call `clearBulkMode` (state reset only) then do their own `await refreshData()` / `await loadServerViewData()` / `renderFilesTable()`. Also added missing `renderFilesTable()` at end of `bulkPull()` so local sync dots update after pulling.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 7d69e79 | All four bug fixes in dashboard.html |

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check

- [x] `client/internal/ui/templates/dashboard.html` modified
- [x] No `input[type=checkbox]` elements inside `.files-table`
- [x] Folder `<tr>` onclick never calls `void(0)` for bulk mode
- [x] `handleRowCheck` and `updateSelectAllState` functions removed
- [x] `selectAllFiles(view)` function exists
- [x] Both bulk bars contain a "Select All" button
- [x] `clearBulkMode` does not call renderFilesTable or renderServerTable
- [x] `renderFilesTable` re-applies `.row-selected` class from Set after re-render
- [x] Commit 7d69e79 exists

## Self-Check: PASSED
