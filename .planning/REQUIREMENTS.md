# Requirements: VaultSync UI Overhaul

**Defined:** 2026-03-15
**Core Value:** Browse, upload, download, delete, and sync files with a responsive layout that works on desktop and mobile

## v1 Requirements

### Layout

- [x] **LAYOUT-01**: User sees a three-pane desktop layout with persistent sidebar, contextual header, and central content grid
- [ ] **LAYOUT-02**: User sees a stacked mobile layout with bottom navigation bar replacing the sidebar
- [ ] **LAYOUT-03**: User can tap a floating action button on mobile to upload a file
- [x] **LAYOUT-04**: User can switch between Local Files and Server Files via tab toggle

### File Display

- [x] **DISP-01**: User sees files as cards in a responsive grid with file name, size, and type icon
- [x] **DISP-02**: User sees sync status on each file card (synced, local-only, server-only)
- [x] **DISP-03**: User sees server files grouped by device name
- [ ] **DISP-04**: User sees action buttons (upload/download/delete) revealed on card hover
- [ ] **DISP-05**: User can filter files by name using a search input in the header
- [x] **DISP-06**: User sees an empty state message when no files exist
- [x] **DISP-07**: User sees a loading indicator while files are being fetched

### Operations

- [ ] **OPS-01**: User can upload a specific file to the server from its card
- [ ] **OPS-02**: User can download a specific file from the server from its card
- [ ] **OPS-03**: User can delete a file (client or server) from its card with confirmation
- [ ] **OPS-04**: User sees per-file success or error feedback after an operation

### Status

- [x] **STAT-01**: User sees connection status (connected/disconnected) in the header
- [x] **STAT-02**: User sees last sync timestamp in the header
- [x] **STAT-03**: User sees an error banner when operations fail
- [ ] **STAT-04**: User sees sync stats summary (file count, total size)

## v2 Requirements

### Operations

- **OPS-05**: User can trigger a full bidirectional force-sync from the UI
- **OPS-06**: User can pull files from other devices with device attribution

### Status

- **STAT-05**: User sees an activity log of sync operations

### Layout

- **LAYOUT-05**: User can toggle between grid and list view
- **LAYOUT-06**: Breadcrumb navigation showing current sync directory path

## Out of Scope

| Feature | Reason |
|---------|--------|
| Separate frontend framework (React/Vue) | Keeping single embedded HTML file architecture |
| New backend API endpoints | Backend is frozen; reuse existing REST API |
| Folder creation/navigation | VaultSync syncs flat file paths |
| Inline file preview | Would require new blob-serving endpoint |
| File rename | No backend endpoint |
| Drag-and-drop upload | Would require multipart upload endpoint |
| WebSocket real-time updates | 5s polling is sufficient |
| Multi-select bulk operations | Out of scope for v1 |
| Dark mode toggle | Current dark theme is intentional |
| Auth layer on web UI | Local network only |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| LAYOUT-01 | Phase 1 | Complete |
| LAYOUT-02 | Phase 1 | Partial |
| LAYOUT-03 | Phase 4 | Pending |
| LAYOUT-04 | Phase 3 | Complete |
| DISP-01 | Phase 2 | Complete |
| DISP-02 | Phase 2 | Complete |
| DISP-03 | Phase 2 | Complete |
| DISP-04 | Phase 4 | Pending |
| DISP-05 | Phase 4 | Pending |
| DISP-06 | Phase 2 | Complete |
| DISP-07 | Phase 2 | Complete |
| OPS-01 | Phase 3 | Pending |
| OPS-02 | Phase 3 | Pending |
| OPS-03 | Phase 3 | Pending |
| OPS-04 | Phase 3 | Pending |
| STAT-01 | Phase 1 | Complete |
| STAT-02 | Phase 1 | Complete |
| STAT-03 | Phase 3 | Complete |
| STAT-04 | Phase 1 | Complete |

**Coverage:**
- v1 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0

---
*Requirements defined: 2026-03-15*
*Last updated: 2026-03-24 — Phase 2 complete, traceability updated*
