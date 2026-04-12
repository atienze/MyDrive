# Requirements: myDrive Dashboard UI Redesign

**Defined:** 2026-04-12
**Core Value:** A homelab user can glance at storage usage and recent sync activity, then browse and manage files — all from a single polished page that feels like a real cloud drive dashboard.

## v1 Requirements

### Overview View

- [x] **OVR-01**: SVG donut ring chart centered on page shows used vs free storage; free space value displayed inside the ring; used arc renders in blue; track fills from 12 o'clock position
- [x] **OVR-02**: Three inline stat cards below the donut — Used (formatted size), Total (250 GB), Sync status (green "Up to date" / amber "Syncing..." colored text)
- [x] **OVR-03**: Recent activity feed below stat cards; each row shows upload/download icon badge, filename, relative timestamp (e.g. "2 hours ago"), and file size
- [x] **OVR-04**: "Browse files ↗" button at bottom of overview switches to Files view

### Files View

- [ ] **FILE-01**: Breadcrumb path bar at top shows current directory path (e.g. `/ home / documents`); each segment is clickable and navigates to that path level
- [ ] **FILE-02**: File table with four columns — Name (colored extension badge + filename), Modified (formatted date/time), Size (formatted), Sync (status dot)
- [ ] **FILE-03**: Folders listed before files; folder rows show `—` for Modified and Size; synthetic folder entries reconstructed client-side from flat `rel_path` strings
- [ ] **FILE-04**: Table rows highlight on hover; 0.5px border-bottom dividers between rows; table container has 6px border-radius with 0.5px border
- [ ] **FILE-05**: Footer bar shows item count + total size on left; Upload and New Folder buttons on right (present but non-functional stubs in v1)
- [ ] **FILE-06**: Sync status dot — green if file exists on both client and server with matching hash; amber if pending/mismatched; derived by comparing `/api/files/server` and `/api/files/client` responses

### Global Layout

- [x] **GLOB-01**: Persistent header bar — app triangle logo + "myDrive" name on left; Overview and Files nav tabs in center-right area; user avatar circle (initials) on far right
- [x] **GLOB-02**: Active nav tab shows 2px underline in accent color; clicking a tab switches the active view; inactive tab has no underline and uses muted text color
- [x] **GLOB-03**: Light/dark mode via CSS custom properties at `:root`; one `@media (prefers-color-scheme: dark)` block overrides token values; no JS, no runtime toggle, no flash-of-wrong-theme
- [ ] **GLOB-04**: All data fetched from existing Go API endpoints (`/api/status`, `/api/files/server`, `/api/files/client`); 10-second poll loop with `Promise.all`; error banner displayed if server unreachable
- [x] **GLOB-05**: Storage calculation: sum of `size` fields from `/api/files/server` response = used bytes; `TOTAL_BYTES = 250 * 1024 ** 3` JS constant; free = total − used
- [ ] **GLOB-06**: File-type extension badges: small colored pill with abbreviated extension text (e.g. green `{}` for yml/yaml, blue `M` for md, grey `◇` for unknown); no CDN icon library; inline only
- [x] **GLOB-07**: Flat aesthetic — no box shadows, no gradients; 0.5px borders (`rgba(0,0,0,0.08)`); generous whitespace; system font stack

## v2 Requirements

### Interactivity

- **ACT-01**: Upload button wired to `POST /api/files/upload` with file picker
- **ACT-02**: New Folder button creates a local directory via a new API endpoint
- **ACT-03**: File row "Remove from server" action via `DELETE /api/files/server`
- **ACT-04**: File row "Delete local" action via `DELETE /api/files/client`
- **ACT-05**: Force sync button wired to `POST /api/force-sync`

### UX Enhancements

- **UX-01**: Table column sort (click column header to sort by Name/Modified/Size)
- **UX-02**: Search/filter bar in Files view
- **UX-03**: Mobile responsive layout (bottom nav bar, stacked views)
- **UX-04**: Runtime light/dark mode toggle button

## Out of Scope

| Feature | Reason |
|---------|--------|
| React / Tailwind / any build tooling | No npm in repo; Go embed requires single file; complexity not justified |
| Server-side changes (new Go endpoints) | Project constraint — no modifications to handler.go, db.go, or server |
| File preview (images, PDF, text) | Complexity; not core to homelab sync dashboard |
| Multi-file bulk operations | Out of scope for v1; single-file actions only |
| User authentication on the UI | Wrong layer — the Go server already handles token auth at TCP level |
| Filesystem watcher / push updates | Requires WebSocket or SSE; polling sufficient for homelab use |
| Upload progress bar | Requires server-side streaming changes |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| GLOB-01 | Phase 1 | Complete |
| GLOB-02 | Phase 1 | Complete |
| GLOB-03 | Phase 1 | Complete |
| GLOB-05 | Phase 1 | Complete |
| GLOB-07 | Phase 1 | Complete |
| OVR-01 | Phase 1 | Complete |
| OVR-02 | Phase 1 | Complete |
| OVR-03 | Phase 1 | Complete |
| OVR-04 | Phase 1 | Complete |
| FILE-01 | Phase 2 | Pending |
| FILE-02 | Phase 2 | Pending |
| FILE-03 | Phase 2 | Pending |
| FILE-04 | Phase 2 | Pending |
| FILE-05 | Phase 2 | Pending |
| FILE-06 | Phase 2 | Pending |
| GLOB-06 | Phase 2 | Pending |
| GLOB-04 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 17 total
- Mapped to phases: 17
- Unmapped: 0 ✓

---
*Requirements defined: 2026-04-12*
*Last updated: 2026-04-12 after roadmap creation*
