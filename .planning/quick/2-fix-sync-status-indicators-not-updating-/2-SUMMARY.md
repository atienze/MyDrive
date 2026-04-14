---
phase: quick-2
plan: 01
subsystem: web-ui
tags: [bug-fix, sync-status, push-file, render]
dependency_graph:
  requires: []
  provides: [correct-sync-dot-after-push]
  affects: [client/internal/ui/templates/dashboard.html]
tech_stack:
  added: []
  patterns: [await-then-render]
key_files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html
decisions:
  - "renderFilesTable() called unconditionally after the loadServerViewData() conditional — harmless when serverLoaded is false (empty map, amber dots are correct by design)"
metrics:
  duration: 5m
  completed_date: "2026-04-14"
  tasks_completed: 1
  files_modified: 1
---

# Quick Task 2: Fix Sync Status Indicators Not Updating — Summary

**One-liner:** Added `renderFilesTable()` call after `loadServerViewData()` in `pushFile()` so sync dots turn green immediately on first push.

## What Was Built

A one-line fix in `pushFile()` inside `dashboard.html`. The local file table was re-rendering with stale `cachedServerMap` data: `refreshData()` triggered `renderFilesTable()` before `loadServerViewData()` had finished updating `App.cachedServerMap`, so pushed files computed their sync dot against the old map and stayed amber.

The fix adds `renderFilesTable()` after the `if (App.serverLoaded) await loadServerViewData()` block. Now both awaits complete before the table re-renders, so dot computation sees the freshly uploaded file in `cachedServerMap` and correctly turns green.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Fix pushFile() to re-render local table after server map refresh | fa456be | client/internal/ui/templates/dashboard.html |

## Deviations from Plan

None - plan executed exactly as written.

## Self-Check: PASSED

- File modified: `client/internal/ui/templates/dashboard.html` — confirmed `renderFilesTable()` present at line 1152 inside `pushFile()`
- Commit `fa456be` exists and contains the fix
