---
phase: quick-5
plan: 01
subsystem: ui
tags: [html, css, javascript, bulk-select, file-table]

# Dependency graph
requires:
  - phase: quick-3
    provides: bulk select mode state (App.bulkMode.localActive / serverActive)
provides:
  - Checkboxes right-aligned in bulk mode (td-bulk last column)
  - Per-row action buttons hidden when bulk-active CSS class present
  - Entire file row tappable in bulk mode via handleRowTap
  - Folder navigation suppressed in bulk mode via inline guard
affects: [bulk-select, file-table, renderFilesTable, renderServerTable]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "td-bulk last column: checkbox and action button share the same visual slot — bulk-active CSS class toggles which one is visible"
    - "handleRowTap guard: event.target.tagName === 'INPUT' prevents double-toggle when clicking directly on checkbox"
    - "Folder row onclick inline guard: App.bulkMode.localActive ? void(0) : navFn() suppresses navigation without removing the handler"

key-files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html

key-decisions:
  - "td-bulk occupies same column slot as row-actions; CSS visibility toggling means no column count change needed"
  - "handleRowTap placed adjacent to handleRowCheck for logical grouping — both deal with individual row selection"

patterns-established:
  - "Bulk mode column swap: td-bulk last, CSS display:none on .row-actions when .bulk-active"

requirements-completed: [QUICK-5]

# Metrics
duration: 12min
completed: 2026-04-15
---

# Quick Task 5: Move Select Checkboxes Closer to Select Summary

**Bulk checkboxes moved to right side of file rows, action buttons hidden in bulk mode, and entire rows tappable to toggle selection**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-04-15
- **Completed:** 2026-04-15
- **Tasks:** 2 of 3 complete (Task 3 is human verification checkpoint)
- **Files modified:** 1

## Accomplishments
- th-bulk and td-bulk columns moved from first to last position in both local and server tables
- CSS rule `.files-table.bulk-active .row-actions { display: none; }` ensures action buttons and checkbox column share the same visual slot, toggling based on mode
- `handleRowTap` function added — guards on bulkMode state and INPUT tag to prevent double-toggle
- File row `<tr>` elements in both renderFilesTable and renderServerTable call handleRowTap on click
- Folder row onclick guards prevent navigation when bulk mode is active
- CSS `.files-table.bulk-active tbody tr { cursor: pointer; }` provides tap affordance

## Task Commits

1. **Task 1: Move td-bulk column and hide action buttons** + **Task 2: handleRowTap row tap** - `c1dd18d` (feat)

## Files Created/Modified
- `client/internal/ui/templates/dashboard.html` - Reordered th-bulk/td-bulk to last column in all table structures, added CSS rules, added handleRowTap function, updated file row tr onclick handlers, updated folder row onclick guards

## Decisions Made
- Both Task 1 and Task 2 changes were committed together in c1dd18d since git staged both sets of edits before the first commit ran

## Deviations from Plan
None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Awaiting Task 3 human verification at http://localhost:9876
- Build and run: `go build -o /tmp/mydrive-test ./client/cmd && /tmp/mydrive-test daemon`

---
*Phase: quick-5*
*Completed: 2026-04-15*
