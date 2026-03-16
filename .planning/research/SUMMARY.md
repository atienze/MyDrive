# Project Research Summary

**Project:** VaultSync UI Overhaul
**Domain:** Responsive single-file embedded file manager UI in Go
**Researched:** 2026-03-15
**Confidence:** HIGH

## Executive Summary

VaultSync's UI overhaul is a frontend-only redesign: a single `dashboard.html` file embedded in a Go binary. The backend TCP protocol, API endpoints, and server logic are frozen. All existing functionality (upload, download, delete, force-sync, device-grouped server panel) must survive the redesign while gaining a responsive three-pane layout, card-based file grid, and mobile navigation. The recommended approach is Tailwind CSS v4 via CDN + Lucide icons + vanilla JavaScript — no build step, no framework, single file delivery.

The implementation is well-defined. The current `dashboard.html` already has the correct JS patterns (fetch, async/await, event delegation, escapeHtml, showFeedback). The overhaul replaces the table-row layout with a card grid and adds responsive breakpoints. Architecture research identified a 7-step build order that matches natural dependency order: HTML shell first, then static structure, then the card renderer, then operations, then mobile polish.

The primary risks are not feature complexity but implementation specifics: Tailwind v4 breaks from v3 on CDN URL and config model, iOS Safari requires `h-dvh` instead of `h-screen`, flex scroll containment requires `min-h-0` to work, and all dynamically injected class names must appear as complete static strings (no template literals). These are all fully-documented and avoidable given the research findings.

## Key Findings

### Recommended Stack

The constraint of a single embedded HTML file rules out any build step or framework. Tailwind CSS v4 via the jsdelivr CDN script is the correct choice — it compiles utilities at runtime in the browser, requires no config file, and accepts custom tokens via `@theme {}` CSS blocks. Lucide icons provide a CDN-based icon set with a simple `data-lucide` attribute API. Vanilla JS is sufficient and already the right pattern in the current codebase.

**Core technologies:**
- Tailwind CSS v4 (jsdelivr CDN): utility-first responsive styling — only viable option given single-file embed constraint, no build pipeline
- Lucide icons (jsdelivr CDN): icon set with `createIcons()` API — cleaner than alternatives (Heroicons no CDN API, Font Awesome requires account, inline SVGs are 300+ lines)
- Vanilla JavaScript: all logic — existing patterns are already correct, no new dependencies needed

### Expected Features

The research distinguishes between features that already exist in the codebase and must survive the redesign vs. net-new features the redesign introduces.

**Must have (table stakes):**
- File list with name, size, modified time, type icon — exists; must survive
- Sync status badges (synced / local-only / server-only) — exists; must survive
- Upload, download, delete with confirmation per file — exists; must survive
- Force-sync button and connection status indicator — exists; must survive in header
- Activity log, error banner, loading states, empty states — exist; must survive
- Responsive three-pane layout (desktop) + mobile stacked layout — new, core deliverable
- Tab toggle between Local and Server views — new, core deliverable

**Should have (differentiators):**
- Card grid with hover-reveal action buttons — visual centerpiece of redesign
- Device-grouped server panel with cross-device pull — JS logic exists; new card rendering needed
- Client-side search filter — no backend change needed
- Floating action button (FAB) for upload on mobile — CSS + 2 lines JS
- Bottom navigation bar on mobile — pure CSS/JS
- Breadcrumb showing sync directory path — static label from status payload
- Sync stats (file count, total size, operation counts) — already in status payload

**Defer (v2+):**
- List view toggle — explicitly deferred in PROJECT.md
- Inline file preview — requires new blob-serving endpoint; backend frozen
- Drag-and-drop upload — requires multipart upload endpoint; backend frozen
- WebSocket real-time updates — new backend infrastructure; 5s poll is sufficient
- Multi-select bulk operations — significant JS complexity, out of scope
- File rename / folder creation / folder navigation — no backend support

### Architecture Approach

The architecture is a single HTML file with a minimal state object (`currentView`, `clientFiles`, `serverFiles`, `status`, `searchQuery`, `operationInFlight`) driving DOM updates via direct `innerHTML` assignment. All API calls go to the Go HTTP server at `:9876` which proxies to the VaultSync TCP server at `:9000`. The layout splits into 7 named zones managed entirely by Tailwind's `lg:` breakpoint prefix.

**Major components:**
1. HTML Shell + Tailwind setup — outer `h-dvh flex` skeleton with zone slots
2. Sidebar + Header — static navigation (desktop), breadcrumbs, force-sync, status indicator
3. File Grid Renderer — `renderFileGrid(files)` replacing table rows with card grid; handles device grouping for server view
4. Tab Toggle + State — `switchView(view)` + `currentView` state; filters which file list renders
5. Operations Layer — upload, download, delete, sync wired to existing API endpoints on card action buttons
6. Mobile Layer — bottom nav, FAB, `h-dvh` fix, safe-area insets, 44px touch targets
7. Polish Layer — search filter, loading states, empty states, error handling, scroll containment

### Critical Pitfalls

1. **Tailwind CDN URL changed in v4** — use `https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4`; the old `cdn.tailwindcss.com` serves v3. Config is CSS-first `@theme {}` blocks, not a JavaScript object.
2. **`h-screen` breaks on iOS Safari** — use `h-dvh` (dynamic viewport height) on the outermost container; Tailwind v4 supports `dvh` natively.
3. **Flex scroll containment requires `min-h-0`** — without `min-h-0` on the scrollable flex child, the file grid won't scroll independently; the entire page scrolls instead.
4. **Tailwind CDN cannot see dynamically composed class strings** — all Tailwind classes in JavaScript `innerHTML` must be complete static strings. No template literal composition like `` `text-${color}-500` ``.
5. **XSS via file names in innerHTML** — the existing `escapeHtml()` function must be applied to every file name, path, and user-derived string in the new card renderer. Every `innerHTML` assignment is an audit point.

## Implications for Roadmap

Based on research, the architecture's suggested build order directly maps to a natural phase structure. Each phase has a clear deliverable and the phases are ordered by dependency: structure before content, content before interaction, interaction before polish.

### Phase 1: Layout Foundation
**Rationale:** Shell and zone structure must exist before any content or interaction can be built. Tailwind CDN setup and `h-dvh` fix must be resolved first to avoid rework.
**Delivers:** Three-pane skeleton renders correctly on desktop and mobile breakpoints; CDN dependencies confirmed loading; correct scroll containment in place.
**Addresses:** Responsive layout (table stakes), three-pane desktop structure
**Avoids:** Tailwind v4 CDN URL pitfall, `h-screen` iOS breakage, flex `min-h-0` scroll containment failure, Tailwind v4 color palette differences

### Phase 2: Static Structure (Sidebar + Header)
**Rationale:** Navigation and header are static HTML — no JS needed. Establishes the visual frame before dynamic content is introduced.
**Delivers:** Sidebar with navigation sections (desktop), header with breadcrumb, force-sync button, connection status indicator.
**Addresses:** Force-sync visibility (table stakes), breadcrumb path, connection status
**Uses:** Lucide icons setup and `createIcons()` call

### Phase 3: File Grid Renderer
**Rationale:** Core visual deliverable of the redesign. Card renderer is the most complex new component and must be stable before operations are wired to it.
**Delivers:** `renderFileGrid(files)` function producing card-based layout with file type icons, size, status badges, hover-reveal action areas; device-grouped server view.
**Addresses:** Card grid (differentiator), device attribution, file metadata display
**Avoids:** Dynamic class name composition pitfall, XSS via escapeHtml(), DOM polling race with `operationInFlight` flag, sticky header scroll ancestry, file path truncation

### Phase 4: Tab Toggle + Operations
**Rationale:** Once the card renderer exists, wire the tab toggle and port all existing operations (upload, download, delete, sync) to card action buttons.
**Delivers:** Fully functional file manager — all operations work from card UI; tab switching between Local and Server views; per-file feedback via `showFeedback()`.
**Addresses:** All table stakes operations (upload, download, delete), tab toggle
**Uses:** Existing API endpoints unchanged; `operationInFlight` mutex preserved

### Phase 5: Mobile Responsive + Polish
**Rationale:** Mobile layer is last because it depends on all desktop zones being stable. Polish (search, loading states, empty states) is lowest priority and can be cut if time-constrained.
**Delivers:** Bottom nav, FAB, `pb-[env(safe-area-inset-bottom)]` safe-area fix, 44px touch targets, client-side search filter, loading/empty/error states.
**Addresses:** Mobile layout (table stakes), FAB upload, search filter (differentiator)
**Avoids:** Fixed bottom nav behind iPhone home indicator, touch targets too small pitfall

### Phase Ordering Rationale

- Phases 1-2 are pure HTML/CSS with no JS dependencies — safe to build and verify visually before any dynamic behavior
- Phase 3 (card renderer) is isolated from operations intentionally — render first with static data, then wire actions in Phase 4
- Mobile is last because `lg:` breakpoints mean desktop layout works without mobile; adding mobile last avoids debugging two layouts simultaneously
- The CDN offline resilience decision (inline vs. CDN) is a Phase 1 decision point that affects all subsequent phases — resolve it first

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (File Grid Renderer):** The server file list API response shape needs verification — specifically whether `modified_at` is included in `/api/files/server` response. May require a minor backend change.
- **Phase 5 (Mobile):** FAB upload flow on mobile depends on the browser's file picker working with the existing `<input type="file">` upload mechanism. Needs device testing.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Layout Foundation):** Tailwind three-pane layout is well-documented with exact class patterns identified in research.
- **Phase 2 (Static Structure):** Pure HTML structure, no novel patterns.
- **Phase 4 (Operations):** All API endpoints are frozen and working; this is a wiring exercise.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Tailwind v4 CDN URL and config model verified against official docs; Lucide CDN API verified |
| Features | HIGH | Clear separation of existing features (must preserve) vs. new features; anti-features explicitly scoped out |
| Architecture | HIGH | Build order is deterministic from dependency graph; state model is minimal and verified against existing patterns |
| Pitfalls | HIGH | All 5 critical pitfalls have specific prevention patterns; iOS Safari behavior is well-documented |

**Overall confidence:** HIGH

### Gaps to Address

- **`modified_at` in API responses:** Card grid should show file modification time. Verify `/api/files/client` and `/api/files/server` response shapes include this field. If missing, a minor backend change to `operations.go` or `handler.go` is needed before Phase 3.
- **`sync_dir` in `/api/status`:** Breadcrumb depends on this field. Verify it's present in the status payload before Phase 2.
- **CDN offline decision:** Inlining Tailwind (~100KB) vs. accepting WAN dependency is a Phase 1 decision point. For a homelab where WAN may be down, inline is safer. This affects the initial file size and setup approach.
- **Tailwind v4 color migration:** Current dark theme uses specific colors. If those colors are defined as v3 hex values, they'll need to be redefined in `@theme {}` with hex values to avoid oklch color shift.

## Sources

### Primary (HIGH confidence)
- tailwindcss.com/docs/installation/play-cdn — Tailwind v4 CDN URL and configuration model
- Tailwind v4 breakpoint documentation — sm/md/lg/xl/2xl breakpoint values
- Lucide icons CDN documentation — `data-lucide` API and `createIcons()` usage

### Secondary (MEDIUM confidence)
- MDN Web Docs — `env(safe-area-inset-bottom)`, `dvh` viewport units, iOS Safari viewport behavior
- Community consensus on `flex-1 min-h-0 overflow-y-auto` scroll containment pattern

### Tertiary (LOW confidence)
- iOS Safari testing behavior for `h-dvh` vs `h-screen` — needs device validation during Phase 5
- FAB file picker behavior on mobile — needs device testing during Phase 5

---
*Research completed: 2026-03-15*
*Ready for roadmap: yes*
