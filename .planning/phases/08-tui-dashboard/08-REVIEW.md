---
phase: 08-tui-dashboard
reviewed: 2026-05-29T14:14:51Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/tui/watcher.go
  - internal/tui/watcher_test.go
  - internal/tui/policy_rules.go
  - internal/tui/policy_panel.go
  - internal/tui/policy_panel_test.go
  - internal/tui/alerts_panel.go
  - internal/tui/alerts_panel_test.go
  - internal/tui/health.go
  - internal/tui/pid_alive_unix.go
  - internal/tui/pid_alive_windows.go
  - internal/tui/model.go
  - internal/tui/base.go
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-05-29T14:14:51Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

This is a gap-closure review for Phase 8 (TUI Dashboard). Scope is the diff from
base `63c6a5f`: the CR-01 BLOCKER fix (`watcher.go` `tailFrom` rewrite), the new
TUI-owned policy rules persistence (`policy_rules.go`), cross-platform PID
liveness (`pid_alive_*.go`), and the alerts/policy panel feature additions
(agent column, expanded Sentry detail, admin-gated policy toggle).

**CR-01 is genuinely fixed.** The `tailFrom` rewrite replaces the
`bufio.Scanner` + `Seek(0,1)` offset arithmetic with a `bufio.Reader` /
`ReadString('\n')` loop that advances the offset by `len(line)` ONLY for
complete, newline-terminated lines. Tracing every path:

- **Partial trailing line** (mid-write record, no `\n`): `ReadString` returns
  `err != nil` with the partial data and the loop `break`s *before* advancing
  the offset — the fragment is held and re-read on the next tick. Verified by
  `TestTailFromPartialLine`, which asserts `offset1 == len(line1)` (not past the
  fragment) and that the second call emits the now-completed record exactly once.
- **Malformed-but-complete line**: offset advances past it (it ends in `\n`),
  `json.Unmarshal` fails, the record is skipped — no infinite re-read.
  Verified by `TestTailFromMalformedSkipped`.
- **Multiple complete lines / no double-emit**: verified by
  `TestTailFromCompleteLines` (second call returns 0 records, offset unchanged).
- **Missing file**: returns `(nil, offset)` with no panic. Verified by
  `TestTailFromMissingFile`.

The `(records, offset)` return contract is preserved for both callers
(`watchAuditLog` advances its persistent `offset` from the return; `probeLastBlock`
ignores the offset and only reads `recs`). The fix is not papered over.

Project conventions hold: `charm.land/bubbletea/v2` import path is used
throughout, health probes fail-soft (return `false`/degraded on any error), and
`pid_alive_*.go` uses pure-Go syscalls (`golang.org/x/sys/windows`, `syscall`) —
no CGO. `go vet ./internal/tui/` is clean and the package test suite passes.

Two WARNINGs remain in the gap code: a stale-selection-index crash in the policy
panel toggle path (reintroduced by the new `stateTick` reload), and a fail-soft
data-clobber window in `policy_rules.go`. Neither is a BLOCKER. Deferred prior
warnings (WR-03 full-file read in `probeLastBlock`, WR-05/WR-08 goroutine
lifecycle, WR-06 incident latch) are NOT re-flagged — the gap code did not
reintroduce them.

## Warnings

### WR-01: PolicyPanel toggle can panic on a stale selection index after a `stateTick` reload

**File:** `internal/tui/policy_panel.go:43-45, 76-81`
**Issue:**
`PolicyPanel.Update` reloads the rule slice on every `stateTick`
("Reload rules so external edits surface", line 45):

```go
case stateTick:
    p.rules = LoadPolicyRules(p.policiesDir)
```

`p.selIdx` is never re-clamped against the new `len(p.rules)`. `LoadPolicyRules`
returns whatever is on disk — `tui_rules.json` is operator/external-writable and
may legitimately contain fewer rules than before (or be edited down to 2 while a
5-rule panel is open). The admin toggle handler then indexes with the stale
index:

```go
case "e", "E", "t", "T":
    if len(p.rules) > 0 {                  // guards empty, NOT in-range
        p.rules[p.selIdx].Enabled = !p.rules[p.selIdx].Enabled  // panic if selIdx >= len
```

Repro: admin opens the panel, navigates to `selIdx = 4` (5 rules), an external
process trims the file to 2 rules, the 5 s `stateTick` reloads (`len == 2`,
`selIdx == 4`), the operator presses `e` → `p.rules[4]` is out of range and the
entire TUI crashes. The `len(p.rules) > 0` guard only rules out the empty case,
not the out-of-range case. The `Body` render at line 130 (`if i == p.selIdx`) is
harmless because it iterates, but the toggle write path is a hard crash.

**Fix:** Clamp `selIdx` into range immediately after every reload, and harden the
toggle guard:
```go
case stateTick:
    p.rules = LoadPolicyRules(p.policiesDir)
    if p.selIdx >= len(p.rules) {
        p.selIdx = len(p.rules) - 1
    }
    if p.selIdx < 0 {
        p.selIdx = 0
    }
...
case "e", "E", "t", "T":
    if p.selIdx >= 0 && p.selIdx < len(p.rules) {
        p.rules[p.selIdx].Enabled = !p.rules[p.selIdx].Enabled
        _ = ToggleRule(p.policiesDir, p.rules[p.selIdx].ID, p.rules[p.selIdx].Enabled)
        p.rules = LoadPolicyRules(p.policiesDir)
        if p.selIdx >= len(p.rules) { p.selIdx = len(p.rules) - 1 }
    }
```

### WR-02: `LoadPolicyRules` clobbers valid on-disk rules with defaults on a transient read error, and `writeRules` is non-atomic

**File:** `internal/tui/policy_rules.go:60-93`
**Issue:**
`writeRules` writes the rules file with a single non-atomic `os.WriteFile`
(truncate + write in place), and `LoadPolicyRules` treats *any* `os.ReadFile`
error as "first run" and re-seeds defaults, overwriting the file:

```go
data, err := os.ReadFile(rulesFilePath(policiesDir))
if err != nil {
    defaults := defaultPolicyRules()
    _ = writeRules(policiesDir, defaults) // overwrites whatever was there
    return defaults
}
```

These two facts combine into a small data-loss window. On Windows in particular,
a concurrent reader (e.g., a `stateTick` reload landing while another panel
instance is mid-`writeRules`) can hit a sharing-violation `ReadFile` error on an
*existing* file with real user toggles; `LoadPolicyRules` then interprets the
transient error as "absent" and overwrites the operator's customized rules with
all-enabled defaults. The malformed-JSON branch (line 88) correctly preserves
user data by returning defaults *without* writing — but the read-error branch
does not make that distinction. The non-atomic write also means a crash mid-write
can leave a truncated/partial file (which the malformed branch then masks with
defaults on the next load).

**Fix:** Distinguish "absent" from "exists but unreadable", and write atomically:
```go
data, err := os.ReadFile(rulesFilePath(policiesDir))
if errors.Is(err, os.ErrNotExist) {
    defaults := defaultPolicyRules()
    _ = writeRules(policiesDir, defaults) // genuine first run: seed
    return defaults
}
if err != nil {
    return defaultPolicyRules() // transient/permission error: do NOT overwrite
}
```
And in `writeRules`, write to a temp file + `os.Rename` for atomic replacement:
```go
tmp := rulesFilePath(policiesDir) + ".tmp"
if err := os.WriteFile(tmp, data, 0600); err != nil { return err }
return os.Rename(tmp, rulesFilePath(policiesDir))
```

## Info

### IN-01: `pidAlive` liveness is inconsistent between Windows and Unix for access-denied processes

**File:** `internal/tui/pid_alive_windows.go:14-24`, `internal/tui/pid_alive_unix.go:16-33`
**Issue:** On Unix, a live process the caller lacks permission to signal returns
`EPERM`, which is explicitly treated as **alive** (`true`). On Windows,
`OpenProcess(SYNCHRONIZE, ...)` against a live process owned by another user or
an elevated process can fail with `ERROR_ACCESS_DENIED`, and the code treats any
`OpenProcess` error as **not alive** (`false`). So the same situation — a running
LlamaFirewall sidecar the dashboard cannot open — reports `LlamaFirewallOK=false`
on Windows but `true` on Unix. The Windows direction is fail-soft toward
"degraded" (acceptable per the fail-closed posture), but the cross-platform
divergence can produce a false-red health pip on Windows when the sidecar is
actually up under a different security context.
**Fix:** Treat `ERROR_ACCESS_DENIED` as alive on Windows to match the Unix EPERM
semantics:
```go
handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
if err != nil {
    if err == windows.ERROR_ACCESS_DENIED {
        return true // exists but we lack access — treat as alive (matches Unix EPERM)
    }
    return false
}
defer windows.CloseHandle(handle)
return true
```

### IN-02: `renderExpandedDetail` truncates attacker-influenced strings on byte boundaries, not rune boundaries

**File:** `internal/tui/alerts_panel.go:324-327, 339-342, 354-357`
**Issue:** Each detail section truncates with `truncated[:width-6]` on the raw
byte slice. `ParentChain`/`FilesAccessed`/`NetworkDests` are Sentry-sourced,
attacker-influenceable strings (file paths, process args, network hosts). A
multi-byte UTF-8 rune straddling the `width-6` cut point yields invalid UTF-8 in
the rendered cell. This does not panic (byte slicing within range is always
safe, and the bounds `width > 6 && len > width-6` keep the index valid), and the
comment correctly notes these strings are never exec'd/eval'd (T-08-06-01), so
this is purely a display-fidelity nit, not a security or crash issue.
**Fix:** Truncate on runes, e.g.:
```go
r := []rune(entry)
if len(r) > width-6 && width > 6 {
    truncated = string(r[:width-6])
}
```

### IN-03: `minInt` / `max` package-local helpers duplicate Go 1.25 builtins (carried from prior IN-01)

**File:** `internal/tui/alerts_panel.go:220-225, 380-385`
**Issue:** `minInt` and `max` are defined locally. Go 1.25 (project minimum per
CLAUDE.md) provides builtin `min`/`max`. The local `max` shadows the builtin and
`minInt` duplicates `min`. This was flagged in the prior review (IN-01) and the
gap code continues to rely on these helpers (e.g., the new
`renderExpandedDetail` uses `max(0, width-6)`). Not a defect; noted for cleanup.
**Fix:** Delete `minInt`/`max` and use the builtins `min`/`max`.

### IN-04: Non-admin gate for PolicyPanel is verified only structurally, not through the real `Update` dispatch

**File:** `internal/tui/policy_panel_test.go:85-131`
**Issue:** `TestPolicyPanelNonAdminNoToggle` documents that it verifies the
admin gate "structurally" and then manually replicates the non-admin navigation
branch (`if p.selIdx < len(p.rules)-1 { p.selIdx++ }`) rather than driving
`p.Update(tea.KeyPressMsg{...})`. The actual gate — the `if !p.adminMode { ...
return }` short-circuit at `policy_panel.go:50-63` that prevents reaching the
`e/t` toggle — is never exercised by the test. The admin-gate correctness is
sound on inspection (the non-admin branch has no toggle case and returns early),
but the test would not catch a regression that, say, moved the toggle case above
the gate. The test file itself acknowledges this is "consistent with the test
style ... which avoid version-dependent KeyPressMsg construction." Low priority
given the gate is simple and correct as written.
**Fix:** If `tea.KeyPressMsg` construction is now feasible under
`charm.land/bubbletea/v2`, drive at least one test through
`p.Update(keyPress("e"))` with `adminMode=false` and assert disk state is
unchanged, so the real dispatch gate is covered.

---

_Reviewed: 2026-05-29T14:14:51Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
