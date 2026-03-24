# Roadmap: VaultSync UI Overhaul

## Overview

Four phases replacing the two-panel dashboard with a modern three-pane layout. Phases are ordered by dependency: structure before content, content before interaction, interaction before polish. All phases operate on a single embedded `dashboard.html` file — no build step, no new backend endpoints.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Layout + Header** - Three-pane skeleton and status chrome render correctly on desktop
- [x] **Phase 2: File Grid** - Card-based file display with metadata, status badges, and device grouping (completed 2026-03-24)
- [ ] **Phase 3: Operations + Tab Toggle** - All file operations wired to card buttons; client/server view switching works
- [ ] **Phase 4: Mobile + Search** - Bottom nav, FAB upload, and search filter complete mobile and polish

## Phase Details

### Phase 1: Layout + Header
**Goal**: Users see the three-pane shell and live connection/sync status on desktop
**Depends on**: Nothing (first phase)
**Requirements**: LAYOUT-01, LAYOUT-02, STAT-01, STAT-02, STAT-04
**Success Criteria** (what must be TRUE):
  1. User sees a persistent sidebar, header bar, and central content area on a desktop-width browser
  2. User sees a stacked single-column layout (no sidebar) on a narrow-width browser window
  3. User sees a connected/disconnected indicator in the header that reflects the live server state
  4. User sees the last sync timestamp and sync stats summary (file count, total size) in the header
**Plans**: 2 plans

Plans:
- [x] 01-01: Three-pane skeleton with Tailwind v4 CDN, h-dvh, and min-h-0 scroll containment
- [x] 01-02: Sidebar navigation sections and header chrome (status indicator, breadcrumb, sync stats)

### Phase 2: File Grid
**Goal**: Users see their files as cards with metadata, sync status, and device grouping
**Depends on**: Phase 1
**Requirements**: DISP-01, DISP-02, DISP-03, DISP-06, DISP-07
**Success Criteria** (what must be TRUE):
  1. User sees files displayed as cards in a responsive grid with file name, size, and a type icon
  2. User sees a sync status badge on each card (synced, local-only, or server-only)
  3. User sees server files grouped under device name headings
  4. User sees a loading indicator while files are being fetched from the API
  5. User sees a descriptive empty state message when no files exist in the current view
**Plans**: 2 plans

Plans:
- [ ] 02-01-PLAN.md — Card grid renderer with file type icons, size, and sync status badges
- [ ] 02-02-PLAN.md — Device-grouped server view, loading state, and empty state

### Phase 3: Operations + Tab Toggle
**Goal**: Users can upload, download, and delete files from card buttons; can switch between Local and Server views
**Depends on**: Phase 2
**Requirements**: LAYOUT-04, OPS-01, OPS-02, OPS-03, OPS-04, STAT-03
**Success Criteria** (what must be TRUE):
  1. User can switch between Local Files and Server Files via a tab toggle and sees the correct file list for each
  2. User can upload a local file to the server by clicking its card action button
  3. User can download a server file to the local client by clicking its card action button
  4. User can delete a file (client or server) from its card after confirming the action
  5. User sees a per-file success or error message after each operation completes
  6. User sees a full-width error banner when a critical operation fails
**Plans**: TBD

Plans:
- [ ] 03-01: Tab toggle state and view switching wired to file grid renderer
- [ ] 03-02: Upload, download, delete operations wired to card action buttons with feedback

### Phase 4: Mobile + Search
**Goal**: Users on mobile can navigate, upload via FAB, and filter files by name
**Depends on**: Phase 3
**Requirements**: LAYOUT-03, DISP-04, DISP-05
**Success Criteria** (what must be TRUE):
  1. User on mobile sees a bottom navigation bar replacing the sidebar
  2. User on mobile can tap a floating action button to open the file picker and upload a file
  3. User sees upload/download/delete action buttons revealed on card hover (desktop) or always visible (mobile)
  4. User can type in the header search input and see the file list filtered to matching names in real time
**Plans**: TBD

Plans:
- [ ] 04-01: Bottom nav, FAB upload, and mobile-safe touch targets
- [ ] 04-02: Hover-reveal card actions and client-side search filter

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Layout + Header | 2/2 | Complete | 2026-03-24 |
| 2. File Grid | 2/2 | Complete   | 2026-03-24 |
| 3. Operations + Tab Toggle | 0/2 | Not started | - |
| 4. Mobile + Search | 0/2 | Not started | - |
