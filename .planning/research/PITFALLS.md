# Pitfalls Research: VaultSync UI Overhaul

**Researched:** 2026-03-15
**Domain:** Responsive file manager UI with Tailwind CDN in embedded Go template

## Critical Pitfalls

### 1. Tailwind CDN is now v4 — URL and config model changed

**Warning signs:** Using old `cdn.tailwindcss.com` URL; trying to pass `tailwind.config` object
**Prevention:** Use `https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4`. Configure via CSS `@theme {}` blocks, not JavaScript config objects.
**Phase:** Layout foundation phase

### 2. `100vh` breaks three-pane layout on iOS Safari

**Warning signs:** Content clipped behind Safari's address bar on iPhone; bottom nav partially hidden
**Prevention:** Use `h-dvh` (dynamic viewport height) instead of `h-screen` for the outermost container. Tailwind v4 supports `dvh` units natively.
**Phase:** Mobile responsive phase

### 3. Fixed bottom nav sits behind iPhone home indicator

**Warning signs:** Bottom nav buttons unreachable on iPhones with no home button
**Prevention:** Add `<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">` and use `pb-[env(safe-area-inset-bottom)]` on the bottom nav container.
**Phase:** Mobile responsive phase

### 4. Tailwind CDN cannot see dynamically composed class names

**Warning signs:** Dynamically built class strings (template literals like `` `text-${color}-500` ``) don't get compiled; elements appear unstyled
**Prevention:** All Tailwind classes used in JavaScript `innerHTML` must appear as complete static strings. Never compose class names dynamically. Use ternary with full class strings: `isActive ? 'bg-blue-500' : 'bg-gray-700'`
**Phase:** File grid renderer phase

### 5. `flex-1 overflow-y-auto` scroll containment fails silently

**Warning signs:** File grid doesn't scroll independently; entire page scrolls instead, pushing header off-screen
**Prevention:** The parent flex container needs explicit height (`h-screen` or `h-dvh`) AND the scrollable child needs `min-h-0` to allow flex shrinking below content height. Pattern: `<div class="flex-1 min-h-0 overflow-y-auto">`
**Phase:** Layout foundation phase

## Moderate Pitfalls

### 6. XSS via file names in innerHTML

**Warning signs:** File names containing `<script>` or event handlers render as HTML
**Prevention:** The existing `escapeHtml()` function MUST be applied to every file name, path, and user-derived string in the new card renderer. Audit every `innerHTML` assignment.
**Phase:** File grid renderer phase

### 7. Touch targets too small for mobile

**Warning signs:** Users can't tap buttons accurately on phone; accidental taps on wrong files
**Prevention:** All interactive elements must be at least 44x44px on mobile. Use `min-h-11 min-w-11` (44px) on buttons and card action areas. Current buttons use 4px padding — insufficient.
**Phase:** Mobile responsive phase

### 8. Go template `{{ }}` vs JavaScript template literals

**Warning signs:** Go template engine tries to interpret `${}` or `{{}}` in JavaScript code
**Prevention:** The current dashboard.html already handles this correctly — JavaScript is in a raw `<script>` block, not processed by Go templates. Maintain this pattern. Don't introduce Go template variables inside `<script>`.
**Phase:** All phases

### 9. CDN dependency breaks homelab when WAN is down

**Warning signs:** UI loads as unstyled HTML when internet is unavailable
**Prevention:** Consider inlining the Tailwind CDN script (~100KB). For Lucide icons, use inline SVGs for the ~15 icons needed instead of the full library. Or accept the WAN dependency for a simpler implementation.
**Phase:** Layout foundation phase (decision point)

### 10. DOM race with polling during operations

**Warning signs:** File grid re-renders from polling while an upload/delete is in progress, causing buttons to reset mid-operation
**Prevention:** The existing `operationInFlight` flag must block grid re-renders during active operations, not just disable buttons. Check the flag before any `innerHTML` update from polling.
**Phase:** File grid renderer phase

## Minor Pitfalls

### 11. Sticky headers require correct scroll ancestry

**Warning signs:** Device group headers in server view don't stick during scroll
**Prevention:** `sticky top-0` only works if the sticky element's scroll container is the one with `overflow-y-auto`. Verify the scroll container is the direct parent, not a wrapper div.
**Phase:** File grid renderer phase

### 12. Tailwind v4 color palette differences

**Warning signs:** Colors look different from the current dashboard's dark theme
**Prevention:** Tailwind v4 uses oklch color space. If matching current colors exactly, define custom colors in `@theme {}` using hex values. Don't mix v3 hex and v4 oklch defaults.
**Phase:** Layout foundation phase

### 13. File path truncation on grid cards

**Warning signs:** Long file paths overflow card boundaries or cause horizontal scroll
**Prevention:** Show filename only on cards (extract with `path.split('/').pop()`), full path in `title` attribute for hover tooltip. Use `truncate` class (Tailwind) for overflow ellipsis.
**Phase:** File grid renderer phase
