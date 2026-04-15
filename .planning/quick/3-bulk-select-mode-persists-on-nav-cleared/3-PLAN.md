---
phase: quick-3
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - client/internal/ui/templates/dashboard.html
autonomous: true
requirements: [BULK-09-revised]

must_haves:
  truths:
    - "Bulk select mode stays active after switching tabs"
    - "Bulk select mode stays active after navigating into a subfolder"
    - "Clicking the Select button while in bulk mode exits bulk mode"
    - "Clicking the Select button while not in bulk mode enters bulk mode"
    - "Cancel button in bulk bar still exits bulk mode"
  artifacts:
    - path: client/internal/ui/templates/dashboard.html
      provides: "Updated switchTab, filesNavTo, serverNavTo, enterBulkMode, and Select button onclick"
  key_links:
    - from: "Select button (local-select-btn)"
      to: "toggleBulkMode('local')"
      via: "onclick attribute"
    - from: "Select button (server-select-btn)"
      to: "toggleBulkMode('server')"
      via: "onclick attribute"
---

<objective>
Remove the automatic bulk mode clear-on-navigation behavior so that bulk select mode persists across tab switches and subfolder navigation. Bulk mode is only exited when the user explicitly clicks the Select button again (toggle) or the Cancel button in the bulk bar.

Purpose: UX improvement — users in the middle of a bulk selection should not lose their mode context when they navigate around.
Output: Updated dashboard.html with 3 clearBulkMode call-sites removed from navigation functions and the Select button converted to a toggle.
</objective>

<execution_context>
@/Users/elijahatienza/.claude/get-shit-done/workflows/execute-plan.md
@/Users/elijahatienza/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Remove clearBulkMode calls from navigation functions</name>
  <files>client/internal/ui/templates/dashboard.html</files>
  <action>
Three navigation functions currently call clearBulkMode; remove all three calls:

1. `switchTab()` (around line 712): Remove the two lines `clearBulkMode('local');` and `clearBulkMode('server');` that appear at the top of the function body. The rest of the function (setting App.activeTab, toggling tab classes, toggling view display) stays unchanged.

2. `filesNavTo()` (around line 907): Remove the single line `clearBulkMode('local');` at the top of the function body. The rest (App.filesPath, renderBreadcrumb, renderFilesTable) stays unchanged.

3. `serverNavTo()` (around line 1118): Remove the single line `clearBulkMode('server');` at the top of the function body. The rest (App.serverPath, renderServerBreadcrumb, renderServerTable) stays unchanged.

Do NOT touch clearBulkMode calls anywhere else (bulk action handlers like bulkPush, bulkPull, bulkDeleteLocal, bulkDeleteServer keep their clearBulkMode calls — those are correct post-action cleanup).
  </action>
  <verify>
grep -n "clearBulkMode" client/internal/ui/templates/dashboard.html
# Should show: Cancel button onclicks (2), bulk action handlers (~4), clearBulkMode function definition
# Should NOT show calls inside switchTab, filesNavTo, or serverNavTo
  </verify>
  <done>switchTab, filesNavTo, and serverNavTo contain no clearBulkMode calls. All other clearBulkMode usages intact.</done>
</task>

<task type="auto">
  <name>Task 2: Convert Select button to toggle bulk mode</name>
  <files>client/internal/ui/templates/dashboard.html</files>
  <action>
Currently the Select button calls `enterBulkMode(view)` and then gets disabled (opacity 0.5, disabled=true) so the user cannot re-click it to exit. Change this to a toggle pattern:

1. Add a new function `toggleBulkMode(view)` near the `enterBulkMode` / `clearBulkMode` functions:

```javascript
function toggleBulkMode(view) {
  if (App.bulkMode[view + 'Active']) {
    clearBulkMode(view);
  } else {
    enterBulkMode(view);
  }
}
```

2. Update `enterBulkMode(view)`: Remove the lines that disable the Select button:
```javascript
// REMOVE these two lines:
if (selectBtn) { selectBtn.disabled = true; selectBtn.style.opacity = '0.5'; }
```
The button should remain enabled so the user can click it again to toggle off.

3. Update the two Select button onclick attributes in the HTML:
   - `id="local-select-btn"`: change `onclick="enterBulkMode('local')"` to `onclick="toggleBulkMode('local')"`
   - `id="server-select-btn"`: change `onclick="enterBulkMode('server')"` to `onclick="toggleBulkMode('server')"`

The `clearBulkMode` function already handles re-enabling the button (`selectBtn.disabled = false; selectBtn.style.opacity = ''`), so that cleanup path stays correct.
  </action>
  <verify>
grep -n "toggleBulkMode\|enterBulkMode\|local-select-btn\|server-select-btn" client/internal/ui/templates/dashboard.html
# Should show: toggleBulkMode function definition, both select-btn onclicks use toggleBulkMode, enterBulkMode no longer disables the button
  </verify>
  <done>Select button calls toggleBulkMode. enterBulkMode no longer disables the button. toggleBulkMode function exists and correctly delegates to enter or clear based on current state.</done>
</task>

</tasks>

<verification>
After both tasks:
1. Grep confirms no clearBulkMode in switchTab/filesNavTo/serverNavTo
2. Grep confirms toggleBulkMode function defined and used on both select buttons
3. Grep confirms enterBulkMode no longer has the disabled/opacity lines
4. Manual smoke test (or visual inspection): enter bulk mode on Local tab, switch to Server tab — local bulk bar should still be visible when switching back
</verification>

<success_criteria>
- Navigating between tabs does not exit bulk mode
- Navigating into a subfolder does not exit bulk mode
- Clicking Select while in bulk mode exits bulk mode (toggle off)
- Clicking Select while not in bulk mode enters bulk mode (toggle on)
- Cancel button in bulk bar still exits bulk mode
- Bulk actions (Push, Pull, Delete) still exit bulk mode after completion
</success_criteria>

<output>
After completion, create `.planning/quick/3-bulk-select-mode-persists-on-nav-cleared/3-SUMMARY.md`
</output>
