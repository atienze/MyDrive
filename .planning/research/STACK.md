# Stack Research: VaultSync UI Overhaul

**Researched:** 2026-03-15
**Domain:** Responsive file manager UI — single embedded HTML file in Go

## Recommended Stack

### Tailwind CSS v4 via CDN — HIGH confidence

- **CDN script:** `<script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>`
- Tailwind v4 is the current major version (verified against tailwindcss.com/docs/installation/play-cdn)
- Breakpoints: sm/640px, md/768px, lg/1024px, xl/1280px, 2xl/1536px
- Tailwind officially labels the Play CDN "not for production" but for a homelab internal tool this is acceptable — the tradeoff is ~100-200ms first-paint compilation vs. adding a build pipeline
- Custom theme tokens work via `<style type="text/tailwindcss">@theme { ... }</style>`

### Lucide Icons — MEDIUM confidence

- **CDN:** `<script src="https://cdn.jsdelivr.net/npm/lucide@latest"></script>`
- **Usage:** `<i data-lucide="upload-cloud" class="size-5"></i>` + `lucide.createIcons()` after DOM ready
- Better choice than Heroicons (no CDN createIcons API), Font Awesome (requires account), or inline SVGs (300+ line boilerplate)
- Pin version in production to avoid breaking changes

### Vanilla JavaScript — HIGH confidence

- No additional libraries needed
- The existing `fetch()`, `async/await`, `closest()`, event delegation, and `innerHTML` patterns in the current `dashboard.html` are exactly right for this scale
- Extend with one new `currentView` state variable for the tab toggle between Local/Server views

## Layout Patterns — HIGH confidence

- **Three-pane desktop:** `flex h-screen` outer + `w-64 shrink-0` sidebar + `flex-1 flex flex-col` main
- **Mobile:** `lg:hidden` bottom nav, `hidden lg:flex` sidebar
- **File grid:** `grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4`
- **Floating action button:** `fixed bottom-20 right-4 lg:hidden`

## What NOT to Use

| Library | Why Not |
|---------|---------|
| Alpine.js | Unnecessary 15KB dependency; vanilla is sufficient for this scale |
| HTMX | New pattern the codebase doesn't need; overkill for simple fetch calls |
| Bootstrap CDN | Not utility-first, less expressive for custom layouts |
| Font Awesome | Requires account/kit for CDN usage |
| Pre-built Tailwind CLI output | Violates single-file embed constraint |
| React/Vue/Svelte | Requires build step; violates embedded HTML constraint |

## Key Considerations

1. **Tailwind CDN v4 vs v3:** The old `cdn.tailwindcss.com` URL serves v3. The new v4 URL is on jsdelivr. Config model changed entirely — CSS-first, no `tailwind.config.js` object.
2. **Go template compatibility:** Go template `{{ }}` syntax doesn't conflict with Tailwind utilities since Tailwind uses plain class strings, not curly-brace expressions.
3. **Offline resilience:** Consider inlining the Tailwind CDN script for homelab environments where WAN may be down. Bundle size is ~100KB.
