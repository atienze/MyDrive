# Features Research: VaultSync UI Overhaul

**Researched:** 2026-03-15
**Domain:** Cloud storage file manager UI (Google Drive / OneDrive / Dropbox patterns)

## Table Stakes (must have)

| Feature | Complexity | Notes |
|---------|------------|-------|
| File list with name + size + modified time | Low | Name + size exist; verify modified_at in API response |
| File type icons | Low | Already exists via emoji map in dashboard.html |
| Sync status badges (synced / local-only / server-only) | Low | Already implemented; must survive redesign |
| Upload per file | Low | Existing push button; preserve |
| Download per file | Low | Existing pull button; preserve |
| Delete with confirmation dialog | Low | Existing confirm(); preserve |
| Force-sync button in header | Low | Existing; must stay visible |
| Connection status indicator | Low | Already in header |
| Last-sync timestamp | Low | Already in status cards |
| Sync-in-progress feedback (button disabled + label) | Low | Exists; extend |
| Empty state ("No files") | Low | Exists; adapt for card grid |
| Error display banner | Low | Exists; keep visible in new layout |
| Activity log | Low | Exists; keep as section |
| Loading state during fetch | Low | Exists; adapt for card grid |
| Responsive layout (desktop three-pane + mobile stacked) | Medium | Core of this milestone |
| Tab toggle (Local vs Server) | Low | Core of this milestone |
| Per-file inline feedback (ok/error text) | Low | Exists via showFeedback(); preserve |

## Differentiators (nice to have, low cost)

| Feature | Complexity | Notes |
|---------|------------|-------|
| Device-grouped server panel | Low | JS logic already exists; new card rendering needed |
| Cross-device pull with device attribution on cards | Low | handlePull endpoint exists; make it discoverable |
| Sync stats (file count, total size, upload/download/delete counts) | Low | Already in status payload |
| Card grid with hover-reveal actions | Medium | Visual centerpiece of redesign |
| FAB for upload on mobile | Low | CSS + 2 lines JS; high perceived quality gain |
| Bottom nav on mobile | Low | Pure CSS/JS tab switching |
| Breadcrumb showing sync directory path | Low | Static label from status payload |
| Persistent sidebar with labeled sections | Low | Structural HTML |
| Client-side search filter | Low | No backend change; input filters loaded file list |
| Client-wins conflict visual callout | Low | Sets correct user expectations |

## Anti-Features (do NOT build)

| Feature | Why Not |
|---------|---------|
| List view toggle | PROJECT.md explicitly defers; doubles layout complexity |
| Folder navigation / drill-down | VaultSync syncs flat paths; no real folder hierarchy |
| Folder creation | No backend support |
| Inline file preview | Requires new blob-serving endpoint; backend frozen |
| File rename | No backend endpoint |
| Drag-and-drop upload | Requires multipart upload endpoint; backend frozen |
| WebSocket real-time updates | New backend infrastructure; 5s poll is sufficient |
| Multi-select bulk operations | Significant JS complexity; out of scope |
| Auth layer on UI | Local network only; complexity with no security gain |
| Dark mode toggle | Current dark theme is intentional; toggle adds complexity |

## Key Gaps to Resolve

1. **Modified time in file list response:** Card grid should show modified time. Verify `/api/files/client` and `/api/files/server` response shapes include timestamps.
2. **sync_dir in /api/status:** Breadcrumb depends on this field being in the status payload.
3. **FAB action:** FAB triggers file upload — need to confirm the upload flow works from mobile (file picker).
