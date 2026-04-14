# Roadmap: myDrive Dashboard Overhaul

## Overview

Three phases deliver a complete dashboard overhaul on top of the existing vanilla JS/HTML/CSS UI. Phase 1 builds the browsable views — navigation, local files, and server files — so users can see everything without triggering a sync. Phase 2 wires individual file actions (push, pull, delete) to the existing API endpoints. Phase 3 adds bulk-select mode and bulk actions so users can act on many files at once.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Views** - Navigation tabs, local file browser, server file browser, base accessibility
- [ ] **Phase 2: Individual Actions** - Per-row push, pull, delete actions wired to existing API endpoints
- [ ] **Phase 3: Bulk Select** - Multi-select mode and bulk push, pull, delete across both views

## Phase Details

### Phase 1: Views
**Goal**: Users can browse both local machine contents and all server-stored files, with clear sync-status indicators, without triggering a full sync
**Depends on**: Nothing (first phase)
**Requirements**: NAV-01, NAV-02, NAV-03, LOCAL-01, LOCAL-02, LOCAL-05, SERV-01, SERV-02, SERV-03, SERV-04, SERV-07, SERV-08, A11Y-01, A11Y-02, A11Y-05, A11Y-06
**Success Criteria** (what must be TRUE):
  1. User can switch between Overview, Local Files, and Server tabs; active tab is visually indicated and survives a data refresh
  2. Local Files tab shows the current directory with breadcrumb navigation, a sync-status dot per file, and a footer with item count and total size
  3. Server tab shows all files from the homelab server — populated via Refresh button, not auto-sync — with folder navigation, breadcrumb, and a footer with total count and size
  4. Server files visually distinguish server-only files from files that match a local copy (same hash = synced indicator)
  5. All tabs are usable on mobile: touch targets meet 44px minimum, file tables scroll horizontally on small screens, minimal theme and dark mode are preserved throughout
**Plans**: 3 plans

Plans:
- [ ] 01-01-PLAN.md — Three-tab nav, rename view-files to view-local, fix refreshData (no auto server fetch)
- [ ] 01-02-PLAN.md — Server view HTML + JS: loadServerViewData, folder nav, sync indicators, footer
- [ ] 01-03-PLAN.md — A11Y CSS fixes (horizontal scroll, touch targets, dark mode) + human verification

### Phase 2: Individual Actions
**Goal**: Users can push, pull, and delete individual files from either view using per-row action buttons wired to the existing API
**Depends on**: Phase 1
**Requirements**: LOCAL-03, LOCAL-04, SERV-05, SERV-06, A11Y-03
**Success Criteria** (what must be TRUE):
  1. Each local file row has a Push action that uploads the file to the server via `/api/files/upload`
  2. Each local file row has a Delete action that removes the local file via DELETE `/api/files/client`, with a confirmation step
  3. Each server file row has a Pull action that downloads the file to local machine via `/api/files/download`
  4. Each server file row has a Delete action that removes the server file via DELETE `/api/files/server`, with a confirmation step
  5. Action buttons are hidden behind hover on desktop and always visible on mobile (no overflow or clipping)
**Plans**: TBD

### Phase 3: Bulk Select
**Goal**: Users can select multiple files in either view and perform bulk push, pull, or delete in a single action
**Depends on**: Phase 2
**Requirements**: BULK-01, BULK-02, BULK-03, BULK-04, BULK-05, BULK-06, BULK-07, BULK-08, BULK-09, A11Y-04
**Success Criteria** (what must be TRUE):
  1. User can enter bulk-select mode in either view; a checkbox appears per row and a select-all toggle is available
  2. A bulk action bar appears as soon as one item is selected, shows the selection count, and remains sticky while scrolling a long file list
  3. Selected local files can be bulk-pushed to server; selected server files can be bulk-pulled to local machine
  4. Selected server files can be bulk-deleted; selected local files can be bulk-deleted; both require a confirmation that shows the count of affected files
  5. Bulk-select mode is cleared automatically when the user switches tabs or navigates into a subfolder
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Views | 0/TBD | Not started | - |
| 2. Individual Actions | 0/TBD | Not started | - |
| 3. Bulk Select | 0/TBD | Not started | - |
