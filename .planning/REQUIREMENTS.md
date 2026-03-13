# Requirements: VaultSync Multi-Device

**Defined:** 2026-03-12
**Core Value:** Every device can reliably push its files to the homelab server and selectively pull files from other devices — without unwanted automatic downloads.

## v1 Requirements

### Schema & Migration

- [ ] **SCHM-01**: Database schema uses composite unique index `(rel_path, device_id)` instead of global `UNIQUE(rel_path)`
- [ ] **SCHM-02**: Migration script converts existing single-device data to new schema without data loss
- [ ] **SCHM-03**: Existing rows retain their current `device_id` after migration

### Database Layer

- [ ] **DBLR-01**: `UpsertFile` resolves conflicts on `(rel_path, device_id)` composite key
- [ ] **DBLR-02**: `FileExists` checks path+hash scoped to the requesting device
- [ ] **DBLR-03**: `GetFileHash` looks up hash scoped to the requesting device
- [ ] **DBLR-04**: `MarkDeleted` soft-deletes only the requesting device's file record
- [ ] **DBLR-05**: `GetFilesForDevice(deviceID)` returns only that device's files
- [ ] **DBLR-06**: `GetSharedFiles(excludeDevice)` returns files from all other devices
- [ ] **DBLR-07**: `HashRefCount` counts references across all devices (unchanged — already correct)
- [ ] **DBLR-08**: Server removes file metadata records when no device references a file (ref-counted cleanup)

### Server Handler

- [ ] **SRVR-01**: `CmdCheckFile` passes `device_id` to `FileExists` — only checks if this device has the file
- [ ] **SRVR-02**: `CmdSendFile`/`CmdFileChunk` upserts under the authenticated device's ID
- [ ] **SRVR-03**: `CmdDeleteFile` passes `device_id` to `MarkDeleted` — only soft-deletes this device's record
- [ ] **SRVR-04**: `CmdListServerFiles` returns all files with `DeviceID` field so client knows the source
- [ ] **SRVR-05**: Blob cleanup via `HashRefCount` remains correct across multi-device references
- [ ] **SRVR-06**: `CmdRequestFile` can serve files from any device's storage (for cross-device pull)

### Protocol

- [x] **PROT-01**: `ServerFileEntry` struct includes `DeviceID string` field
- [x] **PROT-02**: Protocol version bumped to reflect multi-device changes

### Client Sync

- [x] **SYNC-01**: Default sync mode is push-only (upload phase only, no automatic download)
- [x] **SYNC-02**: CLI command `vault-sync pull --from <device> <path>` downloads specific files from another device
- [x] **SYNC-03**: Deletion on client removes only this device's record on server
- [x] **SYNC-04**: `state.json` continues tracking local device's `relPath → hash` (no structural change needed)

### Web UI

- [ ] **WEBU-01**: Server panel groups files by device name (e.g., "MacBook/", "RaspberryPi/")
- [ ] **WEBU-02**: User can download individual files from any device's listing to local machine
- [ ] **WEBU-03**: Delete button clarifies "remove from this device" and only deletes the current device's record
- [ ] **WEBU-04**: Push (upload) button sends files under the current device's ID

## v2 Requirements

### Enhanced Multi-Device

- **EMDT-01**: Subscribe to another device's files for automatic pull on change
- **EMDT-02**: "Delete everywhere" option to remove a file from all devices
- **EMDT-03**: Device rename / device management UI
- **EMDT-04**: File conflict visualization when same path exists on multiple devices

## Out of Scope

| Feature | Reason |
|---------|--------|
| Automatic bidirectional sync | Deliberately removed — user controls what each device pulls |
| Filesystem watcher / continuous sync | Manual trigger model preserved |
| Device-to-device direct sync | All sync goes through central homelab server |
| Shared folders between devices | Each device is its own namespace |
| Encryption at rest | Not in this milestone |
| Token hashing in database | Future security improvement |
| Real-time notifications | Not needed for manual sync model |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| SCHM-01 | Phase 1 | Pending |
| SCHM-02 | Phase 1 | Pending |
| SCHM-03 | Phase 1 | Pending |
| DBLR-01 | Phase 1 | Pending |
| DBLR-02 | Phase 1 | Pending |
| DBLR-03 | Phase 1 | Pending |
| DBLR-04 | Phase 1 | Pending |
| DBLR-05 | Phase 1 | Pending |
| DBLR-06 | Phase 1 | Pending |
| DBLR-07 | Phase 1 | Pending |
| DBLR-08 | Phase 1 | Pending |
| PROT-01 | Phase 1 | Complete |
| PROT-02 | Phase 1 | Complete |
| SRVR-01 | Phase 2 | Pending |
| SRVR-02 | Phase 2 | Pending |
| SRVR-03 | Phase 2 | Pending |
| SRVR-04 | Phase 2 | Pending |
| SRVR-05 | Phase 2 | Pending |
| SRVR-06 | Phase 2 | Pending |
| SYNC-01 | Phase 2 | Complete |
| SYNC-02 | Phase 2 | Complete |
| SYNC-03 | Phase 2 | Complete |
| SYNC-04 | Phase 2 | Complete |
| WEBU-01 | Phase 3 | Pending |
| WEBU-02 | Phase 3 | Pending |
| WEBU-03 | Phase 3 | Pending |
| WEBU-04 | Phase 3 | Pending |

**Coverage:**
- v1 requirements: 27 total
- Mapped to phases: 27
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-12*
*Last updated: 2026-03-12 after roadmap creation*
