# Architecture Research: VaultSync UI Overhaul

**Researched:** 2026-03-15
**Domain:** Single-file embedded responsive UI architecture

## Component Architecture

### 1. HTML Shell (Static Structure)

The single `dashboard.html` file contains all markup, styles, and scripts. Structure:

```
<html>
  <head>
    <!-- Tailwind CDN script -->
    <!-- Lucide Icons CDN -->
    <!-- Custom theme overrides -->
  </head>
  <body class="flex h-screen bg-gray-950 text-gray-100">
    <!-- Sidebar (desktop) -->
    <!-- Main content area -->
      <!-- Header (breadcrumbs + search + actions) -->
      <!-- Tab bar (Local | Server) -->
      <!-- File grid -->
      <!-- Activity log -->
    <!-- Bottom nav (mobile) -->
    <!-- FAB (mobile) -->
    <!-- Script block -->
  </body>
</html>
```

### 2. Layout Zones

| Zone | Desktop | Mobile | Purpose |
|------|---------|--------|---------|
| Sidebar | `w-64 hidden lg:flex flex-col` | Hidden | Navigation (Home, Recent, Starred) |
| Header | Full width of main area | Full width | Breadcrumbs, search, sync button, status |
| Tab Bar | Below header | Below header | Toggle Local/Server file views |
| Content Grid | `flex-1 overflow-y-auto` | Full width | File cards in responsive grid |
| Activity Log | Below grid or collapsible | Below grid | Sync activity feed |
| Bottom Nav | Hidden | `fixed bottom-0 lg:hidden` | Home, Files, Sync, Settings |
| FAB | Hidden | `fixed bottom-20 right-4 lg:hidden` | Upload action |

### 3. JavaScript Architecture

**State Model (minimal):**
```javascript
const state = {
  currentView: 'client',    // 'client' | 'server'
  clientFiles: [],
  serverFiles: [],
  status: {},
  searchQuery: '',
  operationInFlight: false
};
```

**Data Flow:**
```
User Action → fetch() to /api/* → Update state → Re-render affected zone
Timer (5s)  → fetch /api/status → Update status bar
Tab click   → Toggle currentView → Re-render file grid
Search input → Filter state.*Files → Re-render file grid
```

**Key Functions (preserve from existing):**
- `loadClientFiles()` / `loadServerFiles()` — API fetch + render
- `showFeedback()` — per-file operation result display
- `escapeHtml()` — XSS prevention for file names
- `formatSize()` — human-readable file sizes
- `syncNow()` — force-sync trigger

**New Functions Needed:**
- `renderFileGrid(files)` — card-based grid renderer (replaces table rows)
- `switchView(view)` — tab toggle handler
- `filterFiles(query)` — client-side search filter
- `initMobileNav()` — bottom nav + FAB setup

### 4. CSS Organization

Within the single file, CSS is organized as:
1. **Tailwind CDN** — utility classes handle 90% of styling
2. **`<style>` block** — custom animations, scrollbar styling, any overrides
3. **Responsive breakpoints** — handled entirely by Tailwind's `lg:` prefix (sidebar visible at lg+)

## Data Flow Diagram

```
Browser                          Go Server (:9876)           VaultSync TCP (:9000)
  │                                    │                            │
  ├─ GET /api/files/client ──────────► │ (reads sync_dir)          │
  │◄──── JSON [{name, size, hash}] ───┤                            │
  │                                    │                            │
  ├─ GET /api/files/server ──────────► ├─── TCP ListServerFiles ──►│
  │◄──── JSON [{name, size, hash}] ───┤◄── ServerFileList ────────┤
  │                                    │                            │
  ├─ POST /api/files/upload ─────────► ├─── TCP SendFile+Chunks ──►│
  │◄──── JSON {status} ──────────────┤◄── FileStatus ────────────┤
  │                                    │                            │
  ├─ POST /api/force-sync ──────────► ├─── TCP full sync cycle ──►│
  │◄──── JSON {status} ──────────────┤◄── completion ────────────┤
```

## Suggested Build Order

1. **HTML shell + Tailwind setup** — get the three-pane skeleton rendering
2. **Sidebar + header** — static navigation structure
3. **File grid renderer** — card-based display replacing table rows
4. **Tab toggle** — switch between client/server views
5. **Port existing operations** — upload, download, delete, sync buttons on cards
6. **Mobile responsive** — bottom nav, FAB, layout collapse
7. **Polish** — search, loading states, empty states, error handling
