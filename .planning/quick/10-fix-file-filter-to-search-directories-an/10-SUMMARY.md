---
phase: quick-10
plan: 01
subsystem: web-ui
tags: [search, fab, import, dashboard]
dependency_graph:
  requires: []
  provides: [recursive-search-filter, fab-import]
  affects: [client/internal/ui/templates/dashboard.html]
tech_stack:
  added: []
  patterns: [multipart-import, recursive-client-filter]
key_files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html
decisions:
  - Hide breadcrumbs during search by short-circuiting breadcrumbHtml() call; avoids stale path context while showing global results
metrics:
  duration: ~5min
  completed: 2026-03-26
---

# Quick Task 10: Fix file filter to search directories and FAB import Summary

**One-liner:** Recursive search across all directory depths with breadcrumb suppression, plus FAB multipart import to sync dir via `/api/files/import`.

## What Was Done

### Task 1: Recursive search filter and FAB import endpoint

Reviewed the current state of `dashboard.html` and `server.go`. Most of the plan was already implemented in prior work:

- `rerenderLocal()` already filters `cachedClientFiles` (global) instead of just `entries.files` (current path)
- `renderServerGroups()` already filters `groups[device]` (global per device) when searchQuery is active
- Both panels already show full `f.rel_path` as displayName during search
- `handleImport` in `server.go` was already fully implemented with multipart parsing, sync dir write, and `UploadSingleFile` call
- FAB handler already posts to `/api/files/import` with FormData

**Changes made in this task:**

1. `rerenderLocal()`: Changed breadcrumb assignment to suppress output during active search:
   - `var bcHtml = searchQuery ? '' : breadcrumbHtml(localCurrentPath, 'local');`

2. `renderServerGroups()`: Skip breadcrumb line when searchQuery is active:
   - `if (!searchQuery) html += breadcrumbHtml(devicePath, 'server', device);`

**Build verification:** `go build ./client/...` passed with no errors.

## Deviations from Plan

None — all plan items were either already implemented or completed in this task. The two breadcrumb-hiding changes were the only delta needed.

## Self-Check

- [x] `client/internal/ui/templates/dashboard.html` modified (2 lines changed)
- [x] Commit `00e0dc2` exists: "feat(quick-10): recursive search filter and FAB import endpoint"
- [x] `go build ./client/...` passes

## Self-Check: PASSED
