# Roadmap: myDrive Dashboard UI Redesign

## Overview

Three phases that build the new dashboard.html from the ground up: first establish the CSS token system and deliver a working Overview view, then add the Files view with folder navigation, then wire everything together into a deployable single-file dashboard. Each phase produces something the user can open in a browser and verify.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: CSS Foundation + Overview** - CSS token system, global layout, and a working Overview view with donut chart, stat cards, and activity feed (completed 2026-04-12)
- [ ] **Phase 2: Files View** - File table, breadcrumb navigation, folder hierarchy reconstruction, sync status dots, and file-type badges
- [ ] **Phase 3: Integration + Polish** - Live API poll loop wired across both views, error banner, Full Sync button, nav tab switching, and go build verification

## Phase Details

### Phase 1: CSS Foundation + Overview
**Goal**: The dashboard opens in a browser showing a working Overview page — storage donut chart, stat cards, activity feed, global header with nav tabs — in both light and dark mode, with no external CDN dependencies
**Depends on**: Nothing (first phase)
**Requirements**: GLOB-01, GLOB-02, GLOB-03, GLOB-05, GLOB-07, OVR-01, OVR-02, OVR-03, OVR-04
**Success Criteria** (what must be TRUE):
  1. Opening dashboard.html in a browser shows a header with app logo, "myDrive" name, Overview/Files tabs, and a user avatar — no Tailwind CDN request in the network tab
  2. The Overview page displays a centered SVG donut ring with a used/free arc, the free space value inside the ring, and three stat cards (Used, Total, Sync status) populated from fixture or live data
  3. The activity feed shows rows with upload/download icon badges, filenames, relative timestamps, and file sizes
  4. Switching OS appearance to dark mode changes the entire UI color scheme without a page reload and without any JS color values visible in rendered elements
  5. All color and spacing values resolve from CSS custom properties; no hardcoded hex literals appear in JS-rendered HTML
**Plans**: 2 plans

Plans:
- [ ] 01-01-PLAN.md — CSS token system + dark mode block + global layout skeleton + header with nav tabs
- [ ] 01-02-PLAN.md — Overview section: SVG donut chart, stat cards, activity feed, browse button, fixture data bootstrap

### Phase 2: Files View
**Goal**: The Files view is a fully navigable file browser — breadcrumb navigation works, folders reconstruct correctly from flat rel_path data, each row shows a colored extension badge and sync status dot, and the footer summarizes item count and total size
**Depends on**: Phase 1
**Requirements**: FILE-01, FILE-02, FILE-03, FILE-04, FILE-05, FILE-06, GLOB-06
**Success Criteria** (what must be TRUE):
  1. The Files view shows a table with Name (colored extension badge), Modified, Size, and Sync columns; folders appear before files; folder rows show — for Modified and Size
  2. Clicking a folder row navigates into that folder and updates the breadcrumb path bar; clicking a breadcrumb segment navigates to that directory level without resetting the path on the next data refresh
  3. Files with spaces, ampersands, or non-ASCII characters in their names appear correctly and do not cause broken API calls when their paths are used
  4. The footer bar shows the correct item count and total size for the current directory; Upload and New Folder buttons are visible (non-functional stubs)
  5. Each file row's sync status dot is green when the file exists on both client and server with matching hash, and amber otherwise
**Plans**: 2 plans

Plans:
- [ ] 02-01-PLAN.md — Files view CSS + HTML structure + extension badge renderer + folder reconstruction + breadcrumb navigation JS
- [ ] 02-02-PLAN.md — Sync dot computation, footer bar, safe path encoding, server fixture data

### Phase 3: Integration + Polish
**Goal**: The complete dashboard.html is deployed — both views update from live API data on a 10-second poll, the Full Sync button triggers a sync, an error banner appears when the server is unreachable, and a go build confirms the single-file embed works correctly
**Depends on**: Phase 2
**Requirements**: GLOB-04
**Success Criteria** (what must be TRUE):
  1. Both the Overview and Files views update automatically every 10 seconds from live API responses without resetting the current Files directory path
  2. Stopping the Go server causes an error banner to appear in the UI; restarting the server causes the banner to dismiss on the next successful poll
  3. Clicking "Full Sync" triggers POST /api/force-sync and the sync status card shows "Syncing..." during the operation
  4. Running go build from the repo root produces a valid binary and opening localhost:9876 serves the complete dashboard from the embedded file
**Plans**: 1 plan

Plans:
- [ ] 03-01-PLAN.md — Poll loop (pollData + setInterval), error banner HTML/CSS/JS, Full Sync button, fixture removal

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. CSS Foundation + Overview | 2/2 | Complete   | 2026-04-12 |
| 2. Files View | 0/2 | Not started | - |
| 3. Integration + Polish | 0/1 | Not started | - |
