---
phase: quick
plan: 9
subsystem: ui
tags: [search, fab, file-import, dashboard]
key-files:
  modified:
    - client/internal/ui/templates/dashboard.html
    - client/internal/ui/server.go
decisions:
  - "Search filter now operates on full flat cachedClientFiles / device-group arrays instead of the current-path slice from getEntriesAtPath"
  - "Folder cards are suppressed during search (no nested nav during filter); clearing search restores normal directory navigation"
  - "handleImport uses filepath.Base(header.Filename) so only the bare filename lands in destPath, preventing any path traversal via the upload filename"
  - "subdir param defaults to empty string (root of sync_dir) matching localCurrentPath default value"
metrics:
  duration: ~8min
  completed: "2026-03-26T08:50:09Z"
  tasks_completed: 2
  files_modified: 2
---

# Quick Task 9: Fix Search to Filter Nested Files and Import via FAB Summary

**One-liner:** Deep recursive search across all subdirectories and FAB copy-then-upload via new `/api/files/import` endpoint.

## What Was Built

### Task 1: Deep recursive search filter

**Before:** `rerenderLocal()` called `getEntriesAtPath()` first, then filtered `entries.files` — so search only matched files visible at the current navigation level. Same bug in `renderServerGroups()` which filtered `entries.files` (already scoped to `devicePath`).

**After:**
- When `searchQuery` is non-empty, `rerenderLocal()` filters `cachedClientFiles` directly (the full flat list). Folder cards are skipped. `displayName` is set to `f.rel_path` so the user sees which directory each result lives in.
- `renderServerGroups()` does the same: filters the full `groups[device]` array when `searchQuery` is set, skipping folder cards and showing full `rel_path`.
- When `searchQuery` is empty, both functions fall back to existing directory navigation behavior unchanged.
- Placeholder text updated from "Filter files..." to "Search all files...".

### Task 2: FAB import endpoint and copy-then-upload flow

**Backend — `POST /api/files/import` (server.go):**
- Parses a 32MB multipart form.
- Extracts `file` field and optional `subdir` query param.
- Validates `subdir` via existing `validateRelPath` (path traversal protection).
- Uses `filepath.Base(header.Filename)` to strip any path component from the uploaded filename.
- Writes file to `filepath.Join(cfg.SyncDir, subdir, filename)` with `os.MkdirAll` for parent dirs.
- Computes `relPath` via `filepath.Rel` then calls `syncclient.UploadSingleFile` under `syncMu`.
- Returns `{"ok": true, "rel_path": "<relPath>"}` on success; error JSON on failure.
- Logs activity via `u.status.AddActivity`.

**Frontend — `handleFabFileSelect` rewrite (dashboard.html):**
- Replaces old logic that checked `cachedClientFiles` by filename and rejected files not found ("Not in sync dir" error).
- Builds `FormData` with the picked file, POSTs to `/api/files/import?subdir=<localCurrentPath>`.
- Shows "Imported" feedback on success, refreshes file lists and status.
- Follows same `operationInFlight` / `setButtonsDisabled` pattern as other async handlers.
- FAB `title` updated to "Import and push a file to server".

## Commits

| Hash | Message |
|------|---------|
| 8929a19 | feat(quick-9): deep recursive search filter across all files |
| 4e728c2 | feat(quick-9): FAB import endpoint and copy-then-upload flow |

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check

Files exist:
- `client/internal/ui/templates/dashboard.html` — modified
- `client/internal/ui/server.go` — modified

Build verified: `go build -C client ./... && go vet -C client ./...` — both pass with no output.

## Self-Check: PASSED
