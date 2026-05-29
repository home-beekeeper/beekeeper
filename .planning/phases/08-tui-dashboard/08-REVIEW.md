---
phase: 08-tui-dashboard
reviewed: 2026-05-29T00:00:00Z
depth: standard
files_reviewed: 23
files_reviewed_list:
  - cmd/beekeeper/main.go
  - internal/tui/alerts_panel.go
  - internal/tui/alerts_panel_test.go
  - internal/tui/audit_panel.go
  - internal/tui/base.go
  - internal/tui/catalogs_panel.go
  - internal/tui/catalogs_panel_test.go
  - internal/tui/health.go
  - internal/tui/help_panel.go
  - internal/tui/incidents.go
  - internal/tui/model.go
  - internal/tui/model_test.go
  - internal/tui/palette.go
  - internal/tui/panel.go
  - internal/tui/policy_panel.go
  - internal/tui/quarantine_panel.go
  - internal/tui/quarantine_panel_test.go
  - internal/tui/resize_other.go
  - internal/tui/resize_windows.go
  - internal/tui/scan_panel.go
  - internal/tui/scan_panel_test.go
  - internal/tui/styles.go
  - internal/tui/toast.go
  - internal/tui/watcher.go
findings:
  critical: 1
  warning: 8
  info: 6
  total: 15
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-05-29T00:00:00Z
**Depth:** standard
**Files Reviewed:** 23
**Status:** issues_found

## Summary

Phase 8 implements the Bubble Tea v2 TUI dashboard: a calm-mode base screen,
command palette, seven panel overlays, an incident card, a toast system, health
probes, and an fsnotify-backed audit-log watcher goroutine. The code is generally
well structured, the `charm.land/bubbletea/v2` import path is honored, the
Windows resize poller is present, and health probes degrade gracefully per the
documented read-only philosophy.

The review found one BLOCKER (a data-loss bug in the audit tail watcher that can
silently drop audit records — the dashboard's core live data feed) plus a cluster
of WARNINGs around selection-index handling, an animation command that is dropped
when launched from the palette, an unbounded full-file read in a hot probe path,
and several correctness/robustness gaps. The test suite is shallow: many tests
bypass the real `Update` message path by mutating struct fields directly, so they
do not exercise the dispatch logic where most of the defects live.

Note on scope: per the supplied config, the TUI is read-only and health probes
intentionally return `false` on error — those are not flagged. Performance is out
of scope for v1 except where it causes a correctness problem.

## Critical Issues

### CR-01: `tailFrom` advances the offset past unparsed bytes, silently dropping audit records

**File:** `internal/tui/watcher.go:43-83`
**Issue:**
`tailFrom` computes the new offset from the underlying file position
(`f.Seek(0, 1)`) *after* draining a `bufio.Scanner`, then has a fallback that, when
that position reads as `0`, resets the offset to the full file size:

```go
scanner := bufio.NewScanner(f)
...
newOffset, _ := f.Seek(0, 1)
if newOffset == 0 {
    info, err := f.Stat()
    if err == nil {
        newOffset = info.Size()
    } ...
}
return records, newOffset
```

Two distinct data-loss problems:

1. **Partial final line is skipped on the next read.** `bufio.Scanner` reads the
   file in large chunks into its 1 MB buffer (via the underlying `Read`), so
   `f.Seek(0,1)` returns the position of the *last byte the reader buffered*, which
   is typically the current end-of-file — not the end of the last *complete* line
   the scanner returned. When an audit record is mid-write (no trailing `\n` yet,
   which is the common case for an append-in-progress NDJSON log), the scanner
   discards that trailing partial line but the returned offset still jumps past it.
   On the next tick the offset starts beyond the now-completed record, so that
   record is **never emitted to the panel**. Because audit writes are concurrent
   with the 1 s ticker/fsnotify reads, this is a routine occurrence, not an edge
   case — the dashboard's primary live feed loses events.

2. **The `newOffset == 0` fallback is both wrong and unreachable-as-intended.**
   After `Seek(offset, 0)` plus scanning, `Seek(0,1)` cannot legitimately return 0
   unless the file is empty *and* `offset` was 0. If it ever did (e.g., the
   seek/read produced a position of 0 on a non-empty file), resetting the offset to
   `info.Size()` would skip the entire remaining tail in one jump. The fallback
   masks the real position and can lose every pending record at once.

The correct approach is to track the offset by the bytes of the lines actually
consumed (sum `len(scanner.Bytes()) + 1` per scanned line starting from `offset`),
or read with a `bufio.Reader` using `ReadString('\n')` and only advance the offset
past complete, newline-terminated lines — leaving partial trailing lines for the
next tick.

**Fix:**
```go
func tailFrom(auditPath string, offset int64) ([]audit.AuditRecord, int64) {
    f, err := os.Open(auditPath)
    if err != nil {
        return nil, offset // includes os.IsNotExist
    }
    defer f.Close()

    if _, err := f.Seek(offset, 0); err != nil {
        return nil, offset
    }

    r := bufio.NewReader(f)
    var records []audit.AuditRecord
    for {
        line, err := r.ReadString('\n')
        if err != nil {
            // No trailing newline yet: do NOT advance past this partial line.
            break
        }
        offset += int64(len(line)) // only complete lines advance the offset
        var rec audit.AuditRecord
        if json.Unmarshal([]byte(strings.TrimRight(line, "\n")), &rec) == nil {
            records = append(records, rec)
        }
    }
    return records, offset
}
```
This guarantees a partial trailing record is re-read (and emitted) once its
newline lands, and removes the incorrect `newOffset == 0` size-jump.

## Warnings

### WR-01: Scan animation never starts when launched from the command palette

**File:** `internal/tui/model.go:322-332` (and `293-347`, `277-289`)
**Issue:**
`openPanel` returns a `stepTickCmd()` for the scan panel to drive the step
animation. But `runPaletteSelection` discards that command:

```go
case "scan now":
    m, _ := a.openPanel(panelScan, NewScanPanel("deep")) // cmd dropped
    return func() interface{} { return m }
```

The caller in `handleKey` (palette `enter` branch) returns a `nil` cmd:
`m := fn(); if app, ok := m.(App); ok { return app, nil }`. So when the user opens
"scan now" / "scan --quick" via the palette (the documented primary entry point),
no `stepTickMsg` is ever scheduled and the progress view stays frozen at step 0
forever. The animation only works via the direct keybind path that preserves the
cmd. This is a user-visible functional regression on the main launch path.

**Fix:** Plumb the command through the palette dispatch. For example, have
`runPaletteSelection` return both the model and the command (change the closure
type to `func() (tea.Model, tea.Cmd)`), and in the `enter` handler return that
cmd instead of `nil`:
```go
if fn := a.runPaletteSelection(); fn != nil {
    m, cmd := fn()
    if app, ok := m.(App); ok {
        return app, cmd
    }
}
```

### WR-02: LlamaFirewall `status` panics on a malformed/partial state.json

**File:** `cmd/beekeeper/main.go:1349-1350`
**Issue:**
`status` reads `state.json` into `map[string]any` and then performs unchecked type
assertions:
```go
pid := int(lfState["pid"].(float64))
startedAt := lfState["started_at"].(string)
```
If `llamafirewall` exists in state.json but `pid` is absent (nil), is a JSON string,
or `started_at` is missing/non-string, the unchecked `.(float64)` / `.(string)`
panics, crashing the CLI with a stack trace instead of reporting "Not running".
A daemon that wrote a partial/older-schema state file (or any corruption) turns a
status query into a crash. This is the same fail-soft expectation the rest of the
status path already honors (it returns "Not running" on read/parse failure).

**Fix:** Use comma-ok assertions and bail to the "Not running" message on any
missing/wrong-typed field:
```go
pidF, ok1 := lfState["pid"].(float64)
startedAt, ok2 := lfState["started_at"].(string)
if !ok1 || !ok2 {
    fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running")
    return nil
}
pid := int(pidF)
```

### WR-03: `probeLastBlock` reads and JSON-parses the entire audit log on every 10 s health tick

**File:** `internal/tui/health.go:93-122` (via `tailFrom(auditPath, 0)`)
**Issue:**
`probeLastBlock` calls `tailFrom(auditPath, 0)`, which scans the whole NDJSON file
from byte 0, unmarshalling every record, just to find the most recent `block`. This
runs inside `refreshHealthState`, which is invoked synchronously in `App.Update`
on every `healthTick` (every 10 s). On a long-lived, rotated-but-large audit log
this re-parses the full file on the UI thread repeatedly. Beyond the latency, it is
a correctness concern under the rotation model: `tailFrom`'s 1 MB scanner buffer
will silently `continue`-skip any line longer than 1 MB and, combined with reading
from offset 0 each time, there is no bound on the work performed in the render loop.
This is the one place where the otherwise out-of-scope performance issue crosses
into UI responsiveness/blocking behavior.

**Fix:** Read the tail only. Stat the file size and seek to e.g. `max(0, size-64KB)`,
scan forward discarding the first (possibly partial) line, and scan from there; or
maintain a persistent offset like the watcher does and only look at the new records.
At minimum, cap the scan to the last N KB rather than the entire file.

### WR-04: fsnotify `event.Name == auditPath` comparison is fragile across path normalization

**File:** `internal/tui/watcher.go:121-126`
**Issue:**
The watcher watches the parent directory and filters events with
`event.Name == auditPath`. fsnotify emits `event.Name` using the path form it
observed (which may differ from `auditPath` in separators, symlink resolution, or
case on Windows/macOS). `auditPath` comes from `platform.AuditDir()` joined with the
filename and is never normalized against `event.Name`. A mismatch means write
events are ignored and the panel falls back to the 1 s ticker only — degraded but
also racy with WR-01. On case-insensitive filesystems an exact string compare is
especially brittle.

**Fix:** Compare the cleaned base names and directories, e.g.
`filepath.Clean(event.Name) == filepath.Clean(auditPath)`, or compare
`filepath.Base(event.Name) == filepath.Base(auditPath)` since the watch is already
scoped to the parent directory.

### WR-05: Watcher goroutine leaks and never stops; `Run` ignores its context

**File:** `internal/tui/model.go:391-399`, `internal/tui/watcher.go:87-141`
**Issue:**
`Run` takes a `ctx` but immediately discards it (`_ = ctx`) and launches
`go watchAuditLog(p, m.auditPath)` with no shutdown signal. `watchAuditLog` loops
forever on `watcher.Events` / `fallback.C` with no `ctx.Done()` or quit channel; it
only returns if the fsnotify channels close. When the Bubble Tea program exits
(user quits), the goroutine keeps running, holding the fsnotify watcher and ticker
open until process exit. While process exit reclaims it here, the leaked goroutine
plus the ignored context means the dashboard cannot be cancelled by its caller
(e.g., a parent context timeout) and the watcher cannot be cleanly torn down — a
robustness/leak defect for a command intended to be embeddable.

**Fix:** Thread the context into the watcher and select on `ctx.Done()` in both the
fsnotify and ticker-only loops; return when it fires. Use
`tea.NewProgram(m, tea.WithContext(ctx))` (or equivalent) so program shutdown and
the watcher share a lifecycle.

### WR-06: Audit-record critical-detection only fires for the first record in a batch and never re-arms

**File:** `internal/tui/model.go:96-111`
**Issue:**
On `newRecordsMsg` the model scans the batch and escalates to critical on a
`sentry_alert`/`critical` record, but guarded by `&& !a.critical`. Once critical,
the dashboard latches and a *second, distinct* critical alert arriving later (after
the operator resolved the first, or a different rule firing) updates nothing —
`a.status`/`a.incident` are only set on the `!a.critical` transition. Combined with
`DefaultIncident()` being a hard-coded prototype incident (R5 exfil-signature-fusion)
regardless of the actual record's `SentryRuleID/Name/FilesAccessed`, the incident
card never reflects the real triggering alert. For a security dashboard this means
the displayed incident can be entirely unrelated to the alert that fired.

**Fix:** Build the `IncidentModel` from the triggering `audit.AuditRecord`
(rule id/name, files accessed, network dests, severity) instead of the static
`DefaultIncident()`, and define behavior for subsequent criticals (e.g., a queue or
count) rather than silently ignoring them. At minimum, document the latch as
intentional and surface a "+N more" indicator.

### WR-07: `pipColor` / `probeCatalogs` freshness thresholds are inconsistent and partly unreachable

**File:** `internal/tui/catalogs_panel.go:40-55`, `internal/tui/health.go:78-89`
**Issue:**
The catalogs panel treats a source as red at `age > 24h`, amber at `age > 2h`, green
otherwise; the health pip (`probeCatalogs`) treats catalogs as OK while
`age < 25h`. The two surfaces therefore disagree at the boundary (e.g., a 24.5 h-old
index shows a red pip in the panel but a green "catalogs fresh" health pip).
Additionally, for the `socket` and `self` sources `buildBody` hard-codes the
sync-info string and never consults mtime or `ss.Count`, while `Count()` still uses
`p.indexMtimes[src.Name].IsZero()` to decide whether those sources are "enforcing" —
so a `socket`/`self` source with no on-disk index is reported as not enforcing even
though the panel always renders it as live/clean. The enforce count and the per-row
display can contradict each other.

**Fix:** Centralize the freshness threshold in one constant shared by both surfaces,
and make `Count()` consistent with how each source's status is actually derived in
`buildBody` (don't gate `socket`/`self` on a nonexistent index file).

### WR-08: `Run` swallows AltScreen/teardown by ignoring `p.Run()` cleanup ordering on error

**File:** `internal/tui/model.go:391-399`
**Issue:**
`Run` calls `StartResizePoller(p)` and `go watchAuditLog(...)` *before* `p.Run()`,
and returns `p.Run()`'s error directly. The resize poller goroutine
(`resize_windows.go`) loops forever sending `tea.WindowSizeMsg` with no stop signal
and no check that the program is still alive; after `p.Run()` returns it continues
calling `p.Send` on a finished program. `tea.Program.Send` after shutdown is a
no-op in practice, but the goroutine spins a 500 ms ticker for the life of the
process. Same leak class as WR-05. Because both background goroutines outlive the
program and there is no `defer`/cancellation, an error return from `p.Run()` leaves
two orphaned goroutines.

**Fix:** Give both pollers a stop channel/context tied to program shutdown and
ensure they exit when `p.Run()` returns.

## Info

### IN-01: Duplicated `minInt`/`max` helpers and `Padding` is the only divergence

**File:** `internal/tui/alerts_panel.go:192-197`, `272-277`
**Issue:** `minInt` and `max` are defined locally in `alerts_panel.go`. Go 1.25
provides builtin `min`/`max`; the package-local `max` shadows nothing but is
redundant, and `minInt` duplicates the builtin `min`. Prefer the builtins for
clarity and to avoid confusion with the shadowed name.
**Fix:** Delete `minInt`/`max` and use builtin `min`/`max`.

### IN-02: `renderBaseDimmed` computes a dimmed string that is then discarded

**File:** `internal/tui/model.go:357-359`, `368-370`
**Issue:** In `View`, both palette and panel branches do
`dimmed := renderBaseDimmed(a); _ = dimmed`. The dimmed background is computed and
immediately thrown away — dead work and dead code that signals an unfinished
overlay-compositing intent. The background is never actually rendered behind the
overlay.
**Fix:** Either composite `dimmed` behind the overlay (the apparent intent) or
remove the `renderBaseDimmed` calls and the function if unused.

### IN-03: `viewString` test helper is unused by any test

**File:** `internal/tui/model.go:401-405`
**Issue:** `viewString` is documented as a test helper but no test references it
(model_test.go inspects fields directly). Dead code.
**Fix:** Remove it, or use it in tests that assert rendered state.

### IN-04: Hard-coded prototype/demo strings shipped in production paths

**File:** `internal/tui/model.go:58,105,236`; `internal/tui/incidents.go:37-63`;
`internal/tui/policy_panel.go:48-55`; `internal/tui/scan_panel.go:27-30`
**Issue:** The default status ("protecting 4 agents · 0 open criticals today"),
the static `DefaultIncident()` (fixed IPs, PIDs, file paths), the policy panel
values, and the scan completion line ("312 packages, 47 extensions") are
hard-coded prototype fixtures presented as live data. These are marked LOCKED from
the prototype, but as shipped they display fabricated security state to the
operator. Acceptable for Phase 8 if clearly scoped, but it is a correctness/trust
risk if the phase is considered "done" without wiring real data (tracked for
Phase 9 per comments). Flagging so it is not lost.
**Fix:** Confirm Phase 9 tickets exist to replace each with live data; consider a
visible "demo data" affordance until then.

### IN-05: Tests bypass the real `Update`/key-dispatch path

**File:** `internal/tui/model_test.go:20-40,104-126,128-156,184-195`
**Issue:** Many tests mutate `a.mode`, `a.palette`, `a.critical` directly with a
comment that "tea.KeyPressMsg construction is version-dependent," and
`TestAppCommandDispatch` hard-codes `selIdx: 3` to mean "alerts," coupling the test
to the literal command-slice order. These tests assert reachable end-states but do
not exercise `handleKey`/`Update`, which is exactly where CR-01-adjacent and
WR-01/WR-06 defects live. They will not catch dispatch regressions.
**Fix:** Drive tests through `App.Update(tea.KeyPressMsg{...})` so the real key
routing, palette filtering, and command dispatch are covered; assert on
`filtered()[selIdx]` rather than a magic index.

### IN-06: `probeHooks` substring match is overly broad

**File:** `internal/tui/health.go:31-43`
**Issue:** `probeHooks` reports the hook as installed if the raw bytes of
`~/.claude/settings.json` contain the substring "beekeeper" anywhere — including in
an unrelated comment, a disabled/commented config, or a different setting that
merely mentions the word. This can show a green "hooks" pip when no functional hook
is registered. Low severity given the read-only/degrade-gracefully posture, but it
is a false-positive health signal.
**Fix:** Parse the JSON and check for a beekeeper command under the actual
`hooks.PreToolUse`/`PostToolUse` structure rather than a raw substring scan.

---

_Reviewed: 2026-05-29T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
