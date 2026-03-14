# Roadmap: VaultSync Multi-Device

## Overview

The existing single-device sync is fully functional. This milestone transforms the global file namespace into per-device manifests, replaces automatic bidirectional sync with explicit push/pull, and surfaces device ownership in the Web UI. Changes flow in one direction: schema first, then the DB and protocol layer, then server and client behavior, then the UI that exposes it all.

## Phases

- [ ] **Phase 1: Data Layer** - Migrate schema to per-device namespaces and scope all DB + protocol primitives to device_id
- [ ] **Phase 2: Sync Behavior** - Scope server commands to device ownership, replace bidirectional sync with push-only + explicit pull CLI
- [ ] **Phase 3: Web UI** - Surface device ownership in the browser, enable cross-device file downloads

## Phase Details

### Phase 1: Data Layer
**Goal**: The server stores and retrieves files per device — no two devices share a namespace
**Depends on**: Nothing (first phase)
**Requirements**: SCHM-01, SCHM-02, SCHM-03, DBLR-01, DBLR-02, DBLR-03, DBLR-04, DBLR-05, DBLR-06, DBLR-07, DBLR-08, PROT-01, PROT-02
**Success Criteria** (what must be TRUE):
  1. Two devices can sync files with the same relative path without overwriting each other's records
  2. Existing single-device data survives migration intact with device_id preserved
  3. DB queries for file existence, hash lookup, and deletion are scoped to the querying device
  4. `ServerFileEntry` over the wire carries a DeviceID field
  5. Protocol version reflects the multi-device change
**Plans**: 4 plans

Plans:
- [ ] 01-01-PLAN.md — Schema migration binary (migrate-v3): composite unique constraint
- [ ] 01-02-PLAN.md — Protocol changes: ServerFileEntry.DeviceID + Version=3 + handshake version check
- [ ] 01-03-PLAN.md — DB layer: device-scoped query methods + handler call site updates
- [ ] 01-04-PLAN.md — Gap closure: hard-delete file metadata rows when no device references a file (DBLR-08)

### Phase 2: Sync Behavior
**Goal**: Push sends files under the authenticated device's ID; pull is an explicit user action; deletions only affect the requesting device
**Depends on**: Phase 1
**Requirements**: SRVR-01, SRVR-02, SRVR-03, SRVR-04, SRVR-05, SRVR-06, SYNC-01, SYNC-02, SYNC-03, SYNC-04
**Success Criteria** (what must be TRUE):
  1. Running `vault-sync sync` uploads local files and does not download anything automatically
  2. Running `vault-sync pull --from <device> <path>` downloads that specific file from the named device
  3. Deleting a file locally and syncing removes only this device's server record, not other devices' copies
  4. Blob garbage collection does not remove a shared blob while any device still references it
**Plans**: 3 plans

Plans:
- [ ] 02-01-PLAN.md — Server handler call-site updates: device-scoped commands + cross-device hash lookup
- [ ] 02-02-PLAN.md — Client push-only sync + pull CLI subcommand + caller updates
- [ ] 02-03-PLAN.md — Gap closure: fix CmdListServerFiles to return all devices' files (unblocks SRVR-04, SYNC-02)

### Phase 3: Web UI
**Goal**: The browser shows which device owns each file and lets the user pull any file to the local machine
**Depends on**: Phase 2
**Requirements**: WEBU-01, WEBU-02, WEBU-03, WEBU-04
**Success Criteria** (what must be TRUE):
  1. Server panel groups files under headings for each device (e.g., "MacBook", "RaspberryPi")
  2. User can click a download button next to any file in any device's section to pull it locally
  3. Delete button label and behavior makes clear it removes only this device's copy
  4. Push button uploads under the current device's ID (not a global namespace)
**Plans**: 2 plans

Plans:
- [ ] 03-01-PLAN.md — Go backend: DeviceID in file list JSON, handlePull endpoint, DeviceName in status API
- [ ] 03-02-PLAN.md — Dashboard HTML: device-grouped server panel, pull-device action, conditional delete buttons

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Data Layer | 3/4 | Gap closure needed |  |
| 2. Sync Behavior | 2/3 | Gap closure needed | |
| 3. Web UI | 0/2 | Not started | - |
