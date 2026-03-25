---
phase: 03
slug: operations-tab-toggle
status: validated
nyquist_compliant: false
wave_0_complete: true
created: 2026-03-24
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package |
| **Config file** | none (Go built-in) |
| **Quick run command** | `go test -C client ./internal/ui/... -v` |
| **Full suite command** | `go test -C common ./... && go test -C client ./... && go test -C server ./...` |
| **Estimated runtime** | ~1 second (UI tests), ~3 seconds (full suite) |

---

## Sampling Rate

- **After every task commit:** Run `go test -C client ./internal/ui/... -v`
- **After every plan wave:** Run full suite
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 3 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-01-01 | 01 | 1 | LAYOUT-04 | html-content | — (escalated, see Manual-Only) | — | ⚠️ escalated |
| 03-01-01 | 01 | 1 | STAT-03 | html-content | `go test -C client ./internal/ui/... -run TestDashboardHTML_ErrorBanner` | ✅ | ✅ green |
| 03-02-01 | 02 | 2 | OPS-01 | integration | `go test -C client ./internal/ui/... -run TestHandleUpload` | ✅ | ✅ green |
| 03-02-01 | 02 | 2 | OPS-02 | integration | `go test -C client ./internal/ui/... -run TestHandleDownload` | ✅ | ✅ green |
| 03-02-01 | 02 | 2 | OPS-03 | integration | `go test -C client ./internal/ui/... -run "TestHandleDelete(Client\|Server)"` | ✅ | ✅ green |
| 03-02-01 | 02 | 2 | OPS-04 | html-content | `go test -C client ./internal/ui/... -run TestDashboardHTML_PerFileFeedback` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky/escalated*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No new framework or config needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Tab toggle visible above file panels with All Files / Local / Server buttons | LAYOUT-04 | Tab toggle markup was removed from working tree by subsequent uncommitted changes. Committed at HEAD (`2ebb91a`) but no longer in embedded HTML at build time. Needs re-evaluation: either restore tab toggle or update requirement status. | 1. Check `git show 2ebb91a:client/internal/ui/templates/dashboard.html` for `id="tab-toggle"` presence. 2. If tab toggle is intentionally removed, mark LAYOUT-04 as superseded by hamburger sidebar. |

---

## Validation Audit 2026-03-24

| Metric | Count |
|--------|-------|
| Gaps found | 8 |
| Resolved | 7 |
| Escalated | 1 |

### New Tests Added

| Test | Requirement | Description |
|------|-------------|-------------|
| `TestHandleDownload_PathValidation` | OPS-02 | Rejects empty, absolute, and traversal paths with 400 |
| `TestHandleDownload_HappyPath` | OPS-02 | Valid path passes validation, reaches TCP layer (502) |
| `TestHandleDeleteServer_PathValidation` | OPS-03 | Rejects empty, absolute, and traversal paths with 400 |
| `TestHandleDeleteServer_HappyPath` | OPS-03 | Valid path passes validation, reaches TCP layer (502) |
| `TestHandleUpload_HappyPath` | OPS-01 | Real file in sync dir passes validation, reaches TCP (502) |
| `TestDashboardHTML_ErrorBanner` | STAT-03 | Verifies banner element, helper functions, and wiring to 3 critical functions |
| `TestDashboardHTML_PerFileFeedback` | OPS-04 | Verifies showFeedback function and calls in all 4 operation handlers |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 3s
- [ ] `nyquist_compliant: true` set in frontmatter (blocked by LAYOUT-04 escalation)

**Approval:** partial 2026-03-24

---
*Phase: 03-operations-tab-toggle*
*Validated: 2026-03-24*
