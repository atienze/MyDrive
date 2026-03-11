# VaultSync — File Browser Web UI

## What This Is

A two-panel OneDrive-style file browser for VaultSync that lets users selectively push, pull, and delete files between their local machine and homelab server. Runs as an embedded web UI at `localhost:9876` inside the existing VaultSync daemon, with clean & minimal styling.

## Core Value

Users can see every file on both sides (local and server) and take per-file actions (push, pull, delete) from the browser — replacing the current status-only dashboard with a fully interactive file management interface.

## Requirements

### Validated

<!-- Shipped and confirmed valuable. Inferred from existing codebase. -->

- ✓ TCP protocol with 11 command types (CmdPing through CmdFileDataChunk) — existing
- ✓ Token-based device authentication at handshake — existing
- ✓ Full bidirectional sync (upload + download + deletion detection) — existing
- ✓ Content-addressable blob storage with deduplication — existing
- ✓ Local state tracking via state.json for change detection — existing
- ✓ Status dashboard with activity log at localhost:9876 — existing
- ✓ Force-sync button triggering full sync cycle — existing
- ✓ Scanner producing file list with SHA-256 hashes — existing
- ✓ Server file listing via CmdListServerFiles — existing
- ✓ TOML-based client configuration — existing

### Active

<!-- Current scope. Building toward these. -->

- [ ] Single-file TCP operations extracted into reusable module (DialAndHandshake, Upload/Download/Delete/List)
- [ ] HTTP API endpoint to list local files from sync_dir
- [ ] HTTP API endpoint to list server files (proxied via TCP)
- [ ] HTTP API endpoint to push a single file to server
- [ ] HTTP API endpoint to pull a single file from server
- [ ] HTTP API endpoint to delete a file from server
- [ ] HTTP API endpoint to delete a local file
- [ ] Two-panel file browser UI showing local and server files side-by-side
- [ ] Per-file sync status display (synced, conflict, local-only, server-only)
- [ ] Per-file action buttons (Push, Pull, Delete) based on sync state
- [ ] Full Sync button retained from existing dashboard
- [ ] Activity log retained from existing dashboard
- [ ] Sync mutex preventing races between UI ops and background sync
- [ ] Clean & minimal CSS styling (no framework, functional and readable)

### Out of Scope

- Mobile-responsive layout — desktop homelab use only
- File preview/editing in browser — this is sync management, not a file viewer
- Drag-and-drop upload — use Push button for explicit control
- Real-time WebSocket updates — polling at 5s interval is sufficient
- Server-side changes — all work is client-side HTTP + UI

## Context

VaultSync is a TCP-based file sync tool for homelab use. Phases 1–4 are complete: database, hash storage, token auth, and full bidirectional sync all work. The current Phase 5 dashboard shows status and activity but has no file-level visibility or per-file actions.

The browser cannot speak TCP directly. The client's HTTP server (`:9876`) acts as a proxy: each browser action opens a fresh TCP connection to the VaultSync server (`:9000`), runs the operation, and returns JSON. This mirrors how the existing `runSyncCycle()` already works — connect, do work, close.

Zero server-side changes needed. All new work is client-side.

## Constraints

- **Tech stack**: Go standard library only for HTTP/UI (html/template, net/http, go:embed). No JS frameworks, no build step.
- **Architecture**: Browser → HTTP → TCP proxy pattern. Each per-file op opens its own TCP connection.
- **Concurrency**: sync.Mutex shared between daemon sync loop and UI handlers to prevent races on state.json
- **Styling**: Clean & minimal CSS, no framework. Single embedded HTML file with inline CSS/JS.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Per-request TCP connections (no pool) | VaultSync operations are infrequent and user-initiated; simplicity over performance | — Pending |
| Client-side file list joining | Browser fetches both lists and computes sync status in JS; keeps Go handlers simple | — Pending |
| Mutex over channel for sync serialization | sync.Mutex is simpler than channel-based coordination for this use case | — Pending |
| Clean & minimal styling | Homelab tool — functional > pretty; no external dependencies | — Pending |

---
*Last updated: 2025-03-11 after initialization*
