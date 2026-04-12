---
phase: 02-files-view
plan: "02"
subsystem: ui
tags: [html, javascript, css, files-view, sync-status, fixture-data]

# Dependency graph
requires:
  - phase: 02-files-view/02-01
    provides: renderFilesTable, FIXTURE_CLIENT_FILES, sync-dot CSS classes, App state object, loadFilesViewData

provides:
  - computeSyncStatus(relPath, cachedClientMap, cachedServerMap) — returns 'synced' or 'amber'
  - safeEncodePath(relPath) — per-segment encodeURIComponent encoding for fetch() URLs
  - renderFilesFooter() — item count, total size, Upload/New Folder stub buttons
  - FIXTURE_SERVER_FILES — 8-entry server fixture with matched/mismatched hashes for meaningful dots
  - loadFilesViewData updated to accept serverFiles and build cachedClientMap/cachedServerMap
  - Footer CSS rules (#files-footer, .footer-summary, .footer-actions, .footer-btn)

affects:
  - 03-live-data (Phase 3 replaces fixture bootstrap with live /api/files/client and /api/files/server calls; safeEncodePath used for all fetch URLs)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "O(1) sync status: build client/server maps keyed by rel_path, compare hashes inline"
    - "Stub button pattern: footer-btn with cursor:not-allowed and opacity:0.5, onclick=void(0)"
    - "safeEncodePath: split('/').map(encodeURIComponent).join('/') for non-ASCII/special-char paths"

key-files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html

key-decisions:
  - "computeSyncStatus uses Object.prototype.hasOwnProperty.call for safe property existence check"
  - "loadFilesViewData now accepts (clientFiles, serverFiles) — builds both lookup maps before rendering"
  - "renderFilesFooter called at end of renderFilesTable so footer always reflects current view state"
  - "FIXTURE_SERVER_FILES intentionally omits portrait.webp, logs.tar.gz, syncé.go to produce amber dots for client-only files"
  - "backup/db.sqlite uses hash 'clienthash' on client vs 'different' on server to demonstrate conflict amber state"

patterns-established:
  - "Sync dot pattern: computeSyncStatus(relPath, cachedClientMap, cachedServerMap) → CSS class → span.sync-dot.{class}"
  - "Footer pattern: renderFilesFooter() reads getFilesViewEntries() for current level count/size"

requirements-completed: [FILE-05, FILE-06]

# Metrics
duration: 2min
completed: 2026-04-12
---

# Phase 02 Plan 02: Files View — Sync Dots, Footer, and Safe Path Encoding Summary

**Sync status dot computation (green for hash-matched synced files, amber for conflicts/client-only), footer bar with item count and stub Upload/New Folder buttons, and per-segment encodeURIComponent path encoding — all wired to fixture data producing meaningful green/amber states without a running server.**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-04-12T20:32:00Z
- **Completed:** 2026-04-12T20:32:00Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments

- computeSyncStatus() derives green/amber dot from hash comparison across client and server maps — synced files show green, conflicts and client-only files show amber
- renderFilesFooter() populates the #files-footer placeholder with item count, total size for the current directory level, and non-functional Upload/New Folder stub buttons (FILE-05 complete)
- safeEncodePath() encodes each path segment individually so non-ASCII filenames (syncé.go), spaces, and ampersands do not corrupt fetch() URLs in Phase 3
- FIXTURE_SERVER_FILES added with deliberate hash mismatches and omissions to demonstrate all three sync states (synced green, conflict amber, client-only amber) in the fixture bootstrap

## Task Commits

1. **Task 1: Add sync dot computation, footer CSS, and safeEncodePath** - `f02718b` (feat)
2. **Task 2: Add server fixture data and wire fixture bootstrap** - `0f6c580` (feat)

## Files Created/Modified

- `client/internal/ui/templates/dashboard.html` — Added footer CSS rules, safeEncodePath(), computeSyncStatus(), renderFilesFooter(); updated renderFilesTable() to call computeSyncStatus and renderFilesFooter(); updated loadFilesViewData() to accept/cache server files and build lookup maps; added FIXTURE_SERVER_FILES; updated FIXTURE_CLIENT_FILES with hash fields; updated boot call to pass both fixture arrays

## Decisions Made

- computeSyncStatus uses Object.prototype.hasOwnProperty.call rather than `in` operator for safe map key checks that work with Object.create(null) maps
- loadFilesViewData signature extended to (clientFiles, serverFiles) — Phase 3 passes live API responses here, no other call sites to update
- renderFilesFooter is called inside renderFilesTable (not separately) so footer always stays in sync with whatever level is currently rendered

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Files view is feature-complete for v1 (FILE-01 through FILE-06 all done)
- Phase 3 (live data wiring) can now replace the fixture boot call with live /api/files/client and /api/files/server responses passed to loadFilesViewData()
- safeEncodePath() is in place and ready to be used in all Phase 3 fetch() URL construction

---
*Phase: 02-files-view*
*Completed: 2026-04-12*
