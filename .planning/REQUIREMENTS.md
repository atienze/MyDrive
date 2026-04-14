# Requirements: myDrive Dashboard Overhaul

**Defined:** 2026-04-14
**Core Value:** Users can see everything stored on the server and on their local machine — and act on individual files or bulk selections — without triggering a full sync.

## v1 Requirements

### Navigation

- [x] **NAV-01**: User can switch between Overview, Local Files, and Server tabs from the header
- [x] **NAV-02**: Active tab is visually indicated and persists across data refreshes
- [x] **NAV-03**: Tab bar is touch-friendly on mobile (adequate tap target height)

### Local Files View

- [x] **LOCAL-01**: User can browse local machine files with folder navigation and breadcrumb
- [x] **LOCAL-02**: Each local file row shows sync status (synced / unsynced dot)
- [ ] **LOCAL-03**: User can push a single local file to the server via a per-row action button
- [ ] **LOCAL-04**: User can delete a single local file via a per-row action button (with confirmation)
- [x] **LOCAL-05**: Footer shows current directory item count and total size

### Server View

- [x] **SERV-01**: User can browse all files stored on the homelab server (the central file database) without triggering a full sync
- [x] **SERV-02**: Server view is populated from `/api/files/server` on demand (Refresh button, not auto-sync)
- [x] **SERV-03**: Server files not present on the local machine are visually distinguished (e.g. server-only indicator)
- [x] **SERV-04**: Server files that match a local file (same hash) are visually indicated as synced
- [ ] **SERV-05**: User can pull a single server file to local machine via a per-row action button
- [ ] **SERV-06**: User can delete a single server file via a per-row action button (with confirmation)
- [x] **SERV-07**: Server view supports folder navigation and breadcrumb (same pattern as local view)
- [x] **SERV-08**: Server footer shows total file count and total size of visible files

### Bulk Select

- [ ] **BULK-01**: User can enter bulk-select mode in Local Files view (checkbox column or select-all toggle)
- [ ] **BULK-02**: User can enter bulk-select mode in Server view
- [ ] **BULK-03**: User can select individual files via checkbox; select-all toggles all visible rows
- [ ] **BULK-04**: Bulk action bar appears when one or more items are selected, showing available actions and selection count
- [ ] **BULK-05**: User can bulk push selected local files to server
- [ ] **BULK-06**: User can bulk pull selected server files to local machine
- [ ] **BULK-07**: User can bulk delete selected server files (with confirmation showing count)
- [ ] **BULK-08**: User can bulk delete selected local files (with confirmation showing count)
- [ ] **BULK-09**: Bulk-select mode is cancelled when user switches tabs or navigates to a subfolder

### Layout & Accessibility

- [x] **A11Y-01**: All interactive elements have minimum 44×44px touch target on mobile
- [x] **A11Y-02**: File tables scroll horizontally on small screens rather than overflowing/clipping
- [ ] **A11Y-03**: Action buttons in file rows are visible on hover (desktop) and always visible on mobile
- [ ] **A11Y-04**: Bulk action bar is fixed/sticky so it remains visible when scrolling a long file list
- [x] **A11Y-05**: Minimal theme is maintained — no decorative color, consistent with existing CSS variable system
- [x] **A11Y-06**: Dark mode support is maintained for all new UI elements

## v2 Requirements

### Enhanced Server View

- **SERV-V2-01**: Cross-device pull — pull a file from a specific device (uses `/api/files/pull?from=<deviceID>`)
- **SERV-V2-02**: Per-device storage breakdown in Overview donut chart
- **SERV-V2-03**: Search/filter within server view

### Enhanced Local View

- **LOCAL-V2-01**: Drag-and-drop file import to push files into sync dir
- **LOCAL-V2-02**: Upload from device file picker (mobile)

### Sync & Status

- **SYNC-V2-01**: Modified timestamp column when API provides it

## Out of Scope

| Feature | Reason |
|---------|--------|
| New backend Go endpoints | All operations map to existing `/api/files/*` endpoints; avoids Go changes |
| Real-time filesystem watch | Sync is intentionally manual/on-demand by design |
| Auth / login UI | Handled at TCP layer, not the web UI |
| Mobile native app | Web-first; dashboard runs in mobile browser |
| Folder create / rename | Server is content-addressable; folder structure is implicit in rel_path |
| Conflict resolution UI | Client-wins is the design; no manual merge needed |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| NAV-01 | Phase 1 | Complete |
| NAV-02 | Phase 1 | Complete |
| NAV-03 | Phase 1 | Complete |
| LOCAL-01 | Phase 1 | Complete |
| LOCAL-02 | Phase 1 | Complete |
| LOCAL-03 | Phase 2 | Pending |
| LOCAL-04 | Phase 2 | Pending |
| LOCAL-05 | Phase 1 | Complete |
| SERV-01 | Phase 1 | Complete |
| SERV-02 | Phase 1 | Complete |
| SERV-03 | Phase 1 | Complete |
| SERV-04 | Phase 1 | Complete |
| SERV-05 | Phase 2 | Pending |
| SERV-06 | Phase 2 | Pending |
| SERV-07 | Phase 1 | Complete |
| SERV-08 | Phase 1 | Complete |
| BULK-01 | Phase 3 | Pending |
| BULK-02 | Phase 3 | Pending |
| BULK-03 | Phase 3 | Pending |
| BULK-04 | Phase 3 | Pending |
| BULK-05 | Phase 3 | Pending |
| BULK-06 | Phase 3 | Pending |
| BULK-07 | Phase 3 | Pending |
| BULK-08 | Phase 3 | Pending |
| BULK-09 | Phase 3 | Pending |
| A11Y-01 | Phase 1 | Complete |
| A11Y-02 | Phase 1 | Complete |
| A11Y-03 | Phase 2 | Pending |
| A11Y-04 | Phase 3 | Pending |
| A11Y-05 | Phase 1 | Complete |
| A11Y-06 | Phase 1 | Complete |

**Coverage:**
- v1 requirements: 31 total
- Mapped to phases: 31
- Unmapped: 0

---
*Requirements defined: 2026-04-14*
*Last updated: 2026-04-14 after roadmap creation*
