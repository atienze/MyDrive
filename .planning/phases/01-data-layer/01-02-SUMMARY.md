---
phase: 01-data-layer
plan: 02
subsystem: protocol
tags: [gob, protocol, handshake, version-check, multi-device]

# Dependency graph
requires: []
provides:
  - ServerFileEntry with DeviceID string field for multi-device file ownership
  - protocol.Version constant == 3 (bumped from 2)
  - Explicit protocol version check in handshake handler rejecting stale clients
affects:
  - 01-03 (DB layer — populates DeviceID when responding to CmdListServerFiles)
  - 02-sync-behavior (client sender must handshake with Version=3)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - TDD red/green for protocol struct changes
    - Version-gated handshake: magic number check then version check before token auth

key-files:
  created:
    - common/protocol/packet_test.go
  modified:
    - common/protocol/packet.go
    - common/protocol/handshake.go
    - server/internal/receiver/handler.go

key-decisions:
  - "DeviceID added as last field of ServerFileEntry so old gob decoders that omit it still decode cleanly"
  - "Version bumped to 3 immediately; both client and server must be rebuilt together to reconnect"
  - "Magic number check added alongside version check — defensive belt-and-suspenders rejection of non-VaultSync traffic"

patterns-established:
  - "Protocol version check: reject connections where shake.Version != protocol.Version before token auth"
  - "TDD for protocol struct changes: write tests first that fail to compile, then add field, confirm GREEN"

requirements-completed: [PROT-01, PROT-02]

# Metrics
duration: 2min
completed: 2026-03-12
---

# Phase 1 Plan 02: Protocol Extension Summary

**ServerFileEntry gains DeviceID field, protocol bumped to Version 3, and handler now explicitly rejects version-mismatched clients before token auth**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-12T00:16:03Z
- **Completed:** 2026-03-12T00:17:50Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Added `DeviceID string` to `ServerFileEntry` enabling per-device file ownership tracking in the manifest
- Bumped `protocol.Version` from 2 to 3, signaling the multi-device namespaced protocol
- Added explicit magic number and version checks in `handler.go` — old clients are now cleanly rejected with a logged reason before reaching token auth

## Task Commits

Each task was committed atomically:

1. **Task 1: Write failing tests for protocol changes** - `a76de3a` (test - RED)
2. **Task 2: Implement protocol changes and version check** - `e8574de` (feat - GREEN)

## Files Created/Modified
- `common/protocol/packet_test.go` - Three TDD tests: DeviceID round-trip, Version==3 assertion, ServerFileListResponse multi-device round-trip
- `common/protocol/packet.go` - `ServerFileEntry` extended with `DeviceID string` as final field
- `common/protocol/handshake.go` - `Version` constant changed from 2 to 3, comment updated
- `server/internal/receiver/handler.go` - Magic number check and version mismatch check added after handshake decode, before token auth

## Decisions Made
- DeviceID placed as the last field in `ServerFileEntry` so forward-compatibility is maintained: old gob decoders that don't know about the field decode the struct without error (gob ignores unknown fields)
- Magic number check added alongside the version check even though it was not explicitly required by PROT-02 — the plan said "check if a magic number check already exists; if not, add both", and none existed

## Deviations from Plan

None — plan executed exactly as written. The magic number check was explicitly called for by Task 2's action block ("if not, add both").

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Protocol definitions are stable for Plan 03 (DB layer) to reference `ServerFileEntry.DeviceID`
- Handler version gate is live — any test harness connecting to the server must now send Version=3
- Client sender (`sender/client.go`) still sends Version=2 in its handshake — will need updating in 02-sync-behavior phase before end-to-end sync can work

---
*Phase: 01-data-layer*
*Completed: 2026-03-12*

## Self-Check: PASSED

- common/protocol/packet_test.go: FOUND
- common/protocol/packet.go: FOUND
- common/protocol/handshake.go: FOUND
- server/internal/receiver/handler.go: FOUND
- Commit a76de3a (test RED): FOUND
- Commit e8574de (feat GREEN): FOUND
