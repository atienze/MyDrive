---
phase: 04-mobile-search
plan: 01
subsystem: web-ui
tags: [mobile, navigation, ux, tailwind]
dependency_graph:
  requires: []
  provides: [bottom-nav, fab-upload, hover-reveal-cards]
  affects: [client/internal/ui/templates/dashboard.html]
tech_stack:
  added: []
  patterns: [tailwind-group-hover, media-pointer-coarse, env-safe-area-inset]
key_files:
  created: []
  modified:
    - client/internal/ui/templates/dashboard.html
decisions:
  - "showFeedback() extended to handle FAB (no data-card parent) with fixed-position floating label"
  - "Bottom nav uses same SVG icons as sidebar for visual consistency"
  - "Hamburger menu button retained for sidebar brand/device-info access on mobile"
metrics:
  duration: "~2 min"
  completed: "2026-03-26T07:26:00Z"
  tasks_completed: 2
  files_modified: 1
---

# Phase 4 Plan 1: Mobile Bottom Nav, FAB, and Hover-Reveal Cards Summary

**One-liner:** Bottom navigation bar and FAB for mobile upload with hover-reveal Tailwind group/group-hover card actions and always-visible touch fallback via @media(pointer:coarse).

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add bottom nav, FAB, and extend switchView | cae1b61 | dashboard.html |
| 2 | Add hover-reveal card action buttons | 9a5a1f3 | dashboard.html |

## What Was Built

**Task 1 — Bottom nav, FAB, handleFabFileSelect:**
- Fixed `<nav id="bottom-nav">` with `lg:hidden` — visible only on mobile, 3 tabs (All/Local/Server) each calling `switchView()`
- FAB `<button id="fab-btn">` at `fixed bottom-20 right-4 lg:hidden` triggers hidden file input
- `handleFabFileSelect()` matches picked filename against `cachedClientFiles` by basename, calls `doUpload()` if not synced
- `switchView()` extended to update `#bnav-all/local/server` text color (active: `text-[#4d9eff]`, inactive: `text-[#888]`)
- `<main>` padding updated to `pb-16 lg:pb-4` to prevent nav overlap on mobile
- `env(safe-area-inset-bottom)` on nav for iPhone notch/home bar clearance

**Task 2 — Hover-reveal card actions:**
- `group` class added to both `renderLocalCard` and `renderServerCard` root divs
- Action container updated to `opacity-0 group-hover:opacity-100 [@media(pointer:coarse)]:opacity-100 transition-opacity duration-150`
- `BTN_BASE` updated with `[@media(pointer:coarse)]:px-3 [@media(pointer:coarse)]:py-2` for 44px touch targets

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] showFeedback() silently dropped FAB feedback**
- **Found during:** Task 1
- **Issue:** `showFeedback()` does `btn.closest('[data-card]')` then `if (!card) return` — FAB is not inside a card, so all feedback calls from `handleFabFileSelect()` would silently no-op
- **Fix:** Added a FAB/standalone branch before the `if (!card) return` guard. Uses `document.body.appendChild` of a fixed-position floating label above the FAB with matching error/success colors
- **Files modified:** dashboard.html
- **Commit:** cae1b61

## Self-Check: PASSED

- dashboard.html: FOUND
- Commit cae1b61: FOUND
- Commit 9a5a1f3: FOUND
