---
phase: 01-views
plan: 02
subsystem: ui
tags: [html, javascript, dashboard, server-view, sync-dots, breadcrumb]

# Dependency graph
requires:
  - phase: 01-views plan 01
    provides: App state with serverPath/serverLoaded fields, getFilesViewEntries, extensionBadge, folderIconSvg, formatBytes, escapeHtml, showErrorBanner/hideErrorBanner, stub #view-server div
provides:
  - Full #view-server HTML section with breadcrumb, table (server-tbody), and footer (server-footer)
  - loadServerViewData() — on-demand fetch of /api/files/server triggered by Refresh button only
  - computeServerSyncStatus() — three-state sync indicator (synced/amber/server-only) per file
  - renderServerTable(), renderServerBreadcrumb(), serverNavTo(), renderServerFooter()
  - .sync-dot.server-only CSS class using var(--text-muted)
affects:
  - 02-files-view (relies on independent App.serverPath vs App.filesPath navigation state)
  - future phases reading App.cachedServerMap

# Tech tracking
tech-stack:
  added: []
  patterns:
    - On-demand data loading (server files fetched only on user-initiated Refresh, never auto-fetched)
    - Independent navigation state (App.serverPath vs App.filesPath — mutually isolated)
    - Reuse of getFilesViewEntries() as a data-source-agnostic helper for both views

key-files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html

key-decisions:
  - "Device column omitted from Server view in Phase 1 — device_id display is Phase 2 scope"
  - "/api/files/server never called from refreshData() — strictly user-initiated via Refresh button"
  - "Three sync-dot states in Server view: synced (green), amber (modified), server-only (muted grey)"

patterns-established:
  - "Pattern: Server view JS functions grouped as a block before refreshData() for readability"
  - "Pattern: computeServerSyncStatus reads App.cachedClientMap and App.cachedServerMap — both must be populated for synced state"

requirements-completed: [SERV-01, SERV-02, SERV-03, SERV-04, SERV-07, SERV-08]

# Metrics
duration: 2min
completed: 2026-04-14
---

# Phase 1 Plan 2: Server View Summary

**Full #view-server HTML and JS wired to /api/files/server on demand — breadcrumb, table, three-state sync dots (synced/amber/server-only), and folder navigation independent from local view**

## Performance

- **Duration:** 2 min
- **Started:** 2026-04-14T18:20:11Z
- **Completed:** 2026-04-14T18:22:14Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- Replaced stub #view-server div with full HTML (breadcrumb, table with server-tbody, footer with server-footer)
- Added .sync-dot.server-only CSS class using var(--text-muted) — three sync-dot variants now present
- Added six JS functions: loadServerViewData, computeServerSyncStatus, renderServerTable, renderServerBreadcrumb, serverNavTo, renderServerFooter
- Server view navigation is fully independent of local Files view (App.serverPath vs App.filesPath)
- fetch('/api/files/server') appears exactly once — inside loadServerViewData, never in refreshData()

## Task Commits

Each task was committed atomically:

1. **Task 1: Add #view-server HTML section and .sync-dot.server-only CSS** - `6758548` (feat)
2. **Task 2: Add loadServerViewData(), computeServerSyncStatus(), renderServerTable(), serverNavTo(), renderServerBreadcrumb(), renderServerFooter()** - `471568a` (feat)

**Plan metadata:** (see final commit below)

## Files Created/Modified
- `client/internal/ui/templates/dashboard.html` - Server view HTML skeleton + CSS + six JS functions

## Decisions Made
- Device column omitted from Server view table — displaying device_id is Phase 2 scope per RESEARCH.md
- /api/files/server is never auto-fetched; loadServerViewData() is only wired to the Refresh button onclick
- Status column uses three-state dot: synced (green, matching hash in both maps), amber (differing hash), server-only (muted, not in cachedClientMap)

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Server view is fully functional end-to-end; users can browse homelab server files without triggering a sync
- Phase 2 (files-view enhancements) can build on independent serverPath/filesPath navigation state
- Device ID display in Server view deferred to Phase 2

## Self-Check: PASSED

All created files confirmed present. All task commits confirmed in git log.

---
*Phase: 01-views*
*Completed: 2026-04-14*
