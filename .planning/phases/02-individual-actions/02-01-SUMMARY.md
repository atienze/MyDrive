---
phase: 02-individual-actions
plan: "01"
subsystem: ui
tags: [css, accessibility, hover, touch, a11y]

# Dependency graph
requires:
  - phase: 01-views
    provides: .files-table structure, CSS variable palette, td padding conventions

provides:
  - .row-actions container class with flex layout and right-alignment
  - .action-btn base style and .action-btn.danger variant
  - Desktop hover-reveal pattern via @media (pointer: fine)
  - Mobile always-visible pattern via @media (pointer: coarse)

affects: [02-individual-actions-02, 02-individual-actions-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "pointer: fine media query for desktop hover-reveal of row actions"
    - "pointer: coarse media query for mobile always-visible touch targets"
    - "visibility: hidden/visible (not display:none) to preserve layout space during hover"

key-files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html

key-decisions:
  - "Used visibility:hidden/visible rather than display:none so row layout is stable on hover reveal"
  - "min-height: 28px on .action-btn — row td padding (12px top+bottom) provides the remaining height for 44px total touch target"

patterns-established:
  - "Row action buttons use .row-actions wrapper + .action-btn children — Plans 02 and 03 emit this markup without touching CSS"
  - "Danger variant: .action-btn.danger:hover — border and color change to --danger on hover only"

requirements-completed: [A11Y-03]

# Metrics
duration: 5min
completed: 2026-04-14
---

# Phase 2 Plan 01: Individual Actions CSS Infrastructure Summary

**CSS hover-reveal and always-visible action button infrastructure via `.row-actions`/`.action-btn` using `pointer: fine/coarse` media queries — no hex literals, all CSS variables**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-04-14T20:49:00Z
- **Completed:** 2026-04-14T20:50:15Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Added `.row-actions` container (flex, right-aligned) for action buttons in file table rows
- Desktop hover-reveal: `@media (pointer: fine)` hides buttons by default, `tr:hover .row-actions` makes them visible
- Mobile always-visible: `@media (pointer: coarse)` ensures buttons never require hover on touchscreens
- `.action-btn` base style and `.action-btn.danger` hover variant fully defined — Plans 02 and 03 can emit markup immediately

## Task Commits

1. **Task 1: Add .row-actions and .action-btn CSS rules** - `863461d` (feat)

**Plan metadata:** (committed with state updates)

## Files Created/Modified
- `client/internal/ui/templates/dashboard.html` - Added 48 lines of CSS: .row-actions, .action-btn, .action-btn.danger, pointer media queries

## Decisions Made
- Used `visibility: hidden/visible` rather than `display: none` so the Actions column preserves its layout width when buttons are hidden (no column-width jitter on hover)
- `min-height: 28px` on `.action-btn` is intentional — the existing `td { padding: 12px 14px }` provides 24px vertical padding, giving the row a ~52px touch target total, meeting A11Y-03's 44px minimum

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- CSS layer is complete and stable; Plans 02 and 03 can wire `.row-actions` markup into JS renderers without any further CSS changes
- Action column `<th>` header intentionally deferred to Plan 02/03 per plan instructions

---
*Phase: 02-individual-actions*
*Completed: 2026-04-14*
