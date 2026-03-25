---
phase: quick-4
plan: 1
type: execute
wave: 1
depends_on: []
files_modified:
  - client/cmd/main.go
  - client/internal/config/config.go
autonomous: true
must_haves:
  truths:
    - "Server file cards uploaded by the current device show the Remove button"
    - "Server file cards uploaded by the current device show the (you) badge"
    - "isMine evaluates to true when device_name in config.toml matches the server file's device field"
  artifacts:
    - path: "client/cmd/main.go"
      provides: "SetDeviceName call wiring"
      contains: "appStatus.SetDeviceName"
    - path: "client/internal/config/config.go"
      provides: "device_name field documentation in error message"
  key_links:
    - from: "client/cmd/main.go"
      to: "client/internal/status/status.go"
      via: "appStatus.SetDeviceName(cfg.DeviceName)"
      pattern: "SetDeviceName"
---

<objective>
Wire device_name from config.toml through to the /api/status response so the dashboard JS can identify which server files belong to the current device.

Purpose: Fix the bug where isMine is always false because SetDeviceName is never called, causing server cards from the user's own device to lack the "Remove" button and "(you)" badge.
Output: Two modified Go files — one-line wiring fix in main.go, config error message update in config.go.
</objective>

<execution_context>
@/Users/elijahatienza/.claude/get-shit-done/workflows/execute-plan.md
@/Users/elijahatienza/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@client/cmd/main.go
@client/internal/config/config.go
@client/internal/status/status.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Wire SetDeviceName and update config documentation</name>
  <files>client/cmd/main.go, client/internal/config/config.go</files>
  <action>
In client/cmd/main.go, inside runDaemon(), add `appStatus.SetDeviceName(cfg.DeviceName)` immediately after line 105 (`appStatus := status.New()`). This ensures the /api/status JSON response includes the device_name, which the dashboard JS uses to compute isMine for server file cards.

In client/internal/config/config.go, update the config-not-found error message (the fmt.Errorf around line 48) to include `device_name` in the example config snippet, so new users know to add it:

```
  server_addr  = "<server-ip>:9000"
  token        = "<64-char-token>"
  sync_dir     = "<path-to-sync>"
  device_name  = "<device-name>"
```

Do NOT add a validation check requiring device_name — it should remain optional (empty string is acceptable, the UI just won't show the (you) badge).
  </action>
  <verify>
    <automated>cd /Users/elijahatienza/Desktop/IndependentProjects/HomelabSecureSync && grep -n "SetDeviceName" client/cmd/main.go && grep -n "device_name" client/internal/config/config.go && go build -C client ./...</automated>
  </verify>
  <done>appStatus.SetDeviceName(cfg.DeviceName) is called in runDaemon(), config error message shows device_name field, client builds without errors</done>
</task>

</tasks>

<verification>
- `go build -C client ./...` succeeds
- `grep "SetDeviceName" client/cmd/main.go` shows the new call
- Config error message includes device_name in the example snippet
</verification>

<success_criteria>
When a user has `device_name = "MyLaptop"` in their config.toml and runs `vault-sync daemon`, the /api/status endpoint returns `"device_name": "MyLaptop"` in its JSON, enabling the dashboard to correctly identify owned server files and show Remove buttons and (you) badges.
</success_criteria>

<output>
After completion, create `.planning/quick/4-fix-push-pull-button-logic-or-complete-p/4-SUMMARY.md`
</output>
