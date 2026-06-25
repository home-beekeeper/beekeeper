---
quick_id: 260625-svl
slug: catalog-sync-visibility-alerting
type: quick
run_mode: validate
planner: opus
executor: sonnet
scope: "Full (all 4 gaps): silent+logged scheduled sync (cross-platform log, Windows console-hide), persistent sync summary + `catalogs status`, desktop toast on sync-hit quarantine (cross-platform via beeep), TUI auto-incident for background catalog quarantines."
files_modified:
  - internal/platform/dirs.go
  - internal/platform/dirs_test.go
  - cmd/beekeeper/console_windows.go
  - cmd/beekeeper/console_other.go
  - cmd/beekeeper/synclog.go
  - cmd/beekeeper/synclog_test.go
  - cmd/beekeeper/catalogs_daemon.go
  - cmd/beekeeper/catalogs_daemon_test.go
  - cmd/beekeeper/catalogs_daemon_windows.go
  - cmd/beekeeper/catalogs_daemon_darwin.go
  - cmd/beekeeper/catalogs_daemon_linux.go
  - cmd/beekeeper/main.go
  - internal/catalog/state.go
  - internal/catalog/state_test.go
  - internal/watch/firstresponder.go
  - internal/watch/firstresponder_test.go
  - internal/tui/model.go
  - internal/tui/model_view_test.go
  - internal/tui/catalogs_panel.go
  - docs/install-posture.md
autonomous: true
must_haves:
  truths:
    - "The OS-scheduled hourly heartbeat no longer flashes a blank console on Windows: the schtasks /tr runs `catalogs sync --background`, and the binary hides its own console window (ShowWindow SW_HIDE) on the --background path; conhost --headless is used when available on Windows 11 for true zero-flash."
    - "On EVERY platform, a --background sync tees all of its stdout/stderr to a size-rotated `<StateDir>/logs/sync.log`, so the run is no longer invisible (macOS launchd previously discarded it; Linux only had journald)."
    - "The 2h-default, [2h,24h]-clamped configured interval is UNCHANGED — the heartbeat stays hourly by design (D-T1-interval); only its visibility/logging changes."
    - "After each sync, a structured summary (result: synced|unchanged|skipped|error, entry count, scan-hit count, quarantined count, pending count, last error, next-due time) is persisted to state.json and printed by a NEW `beekeeper catalogs status` command, which also shows the daemon registration state and the sync.log path."
    - "RunFirstResponder returns the counts it produced (hits / quarantined / pending / would-quarantine) so the sync summary reflects what actually happened, not a guess."
    - "A real (non-dry-run) catalog-sync quarantine fires a best-effort desktop notification via internal/notify (Windows toast / macOS Notification Center / Linux notify-send), exactly mirroring the watch daemon (handler.go:196). Headless Linux still no-ops (notify.go DISPLAY guard). Dry-run NEVER notifies a move it did not make."
    - "When the TUI is open, a background FRSP-01/FRSP-02 catalog_quarantine or pending-quarantine record raises the existing CatalogQuarantineIncidentFromRecord card (today only sentry_alert+critical auto-raises); the card's human-gated [R]estore/[P]urge/[A]cknowledge actions are wired to real quarantine ops."
    - "Honesty invariants preserved: scan READ-ONLY, quarantine REVERSIBLE-move-only (purge stays human-gated), Sentry DETECTION-ONLY, fail-closed (a console-hide/log/notify failure is non-fatal and never downgrades a block), internal/policy stays pure, notifications best-effort."
    - "No new dependencies (golang.org/x/sys v0.44.0 and gen2brain/beeep v0.11.2 are already in go.mod)."
    - "go build ./..., go vet ./..., go test ./... pass on Windows; GOOS=linux go vet ./... and GOOS=darwin go vet ./... compile-validate the OS-split files (console_windows.go + the three daemon installers)."
  artifacts:
    - path: "cmd/beekeeper/synclog.go"
      provides: "Gap 1: --background log tee + size-rotated sync.log opener (cross-platform)"
    - path: "cmd/beekeeper/console_windows.go"
      provides: "Gap 1: Windows-only console-window hide (ShowWindow SW_HIDE via x/sys/windows)"
    - path: "internal/catalog/state.go"
      provides: "Gap 2: SyncSummary struct + LastSync field on WatchState"
    - path: "internal/watch/firstresponder.go"
      provides: "Gap 2+3: RunFirstResponder returns FirstResponderResult counts; fires notify on a real quarantine"
    - path: "cmd/beekeeper/main.go"
      provides: "Gap 1+2: `catalogs sync --background` flag + `catalogs status` subcommand"
    - path: "internal/tui/model.go"
      provides: "Gap 4: auto-raise the catalog-quarantine incident card from background FRSP records"
  key_links:
    - from: "cmd/beekeeper/catalogs_daemon_windows.go installCatalogDaemon"
      to: "schtasks /tr"
      via: "`\"<exe>\" catalogs sync --background` (+ conhost --headless when available)"
    - from: "cmd/beekeeper/main.go catalogs sync RunE (--background)"
      to: "cmd/beekeeper/synclog.go + console_windows.go"
      via: "tee Out/Err to sync.log, then HideConsoleWindow() on Windows"
    - from: "cmd/beekeeper/catalogs_daemon.go runCatalogsSync"
      to: "internal/catalog/state.go SyncSummary"
      via: "record result/entries/hits/quarantined/pending/next-due after the pipeline"
    - from: "internal/watch/firstresponder.go RunFirstResponder"
      to: "internal/notify.Notify"
      via: "best-effort toast on a non-dry-run catalog_quarantine, gated by NotifyConfig"
    - from: "internal/tui/model.go newRecordsMsg"
      to: "internal/tui/incidents.go CatalogQuarantineIncidentFromRecord"
      via: "raise the card on RecordType catalog_quarantine|pending-quarantine"
---

<objective>
Close the four catalog-sync VISIBILITY gaps the maintainer identified, without changing the
already-correct sync pipeline (cross-reference scan -> reversible quarantine -> audit -> Sentry
targets -> overlay) or the 2h-default interval cadence:

  Gap 1 — the hourly Windows heartbeat flashes a blank console and discards its output; mac/linux run
          silently but mac discards output too. FIX: `catalogs sync --background` self-hides the
          console on Windows and tees output to a size-rotated cross-platform `sync.log`.
  Gap 2 — there is no "what did the last sync do / when is the next one" surface. FIX: persist a
          SyncSummary to state.json and add `beekeeper catalogs status`.
  Gap 3 — a background quarantine produces no proactive alert. FIX: fire a best-effort desktop toast
          via the existing internal/notify (already wired into the watch daemon, NOT the sync path).
  Gap 4 — the TUI never auto-raises the catalog-quarantine card for a background sync hit (only
          sentry_alert+critical auto-raises today). FIX: raise CatalogQuarantineIncidentFromRecord
          from new FRSP records and wire its [R]/[P]/[A] actions.

The pipeline that does the SCAN + QUARANTINE already exists (260612-f80 / Phase 24 FRB). This task is
purely about making the SCHEDULED (background) run observable and alerting. Web/docs accuracy in the
separate beekeeper-web repo is OUT of this plan's Go scope (see closing note).
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/quick.md
</execution_context>

<context>
@CLAUDE.md
@cmd/beekeeper/catalogs_daemon.go
@cmd/beekeeper/catalogs_daemon_windows.go
@cmd/beekeeper/catalogs_daemon_darwin.go
@cmd/beekeeper/catalogs_daemon_linux.go
@cmd/beekeeper/main.go
@internal/catalog/state.go
@internal/config/config.go
@internal/watch/firstresponder.go
@internal/watch/handler.go
@internal/notify/notify.go
@internal/tui/model.go
@internal/tui/incidents.go
@internal/tui/catalogs_panel.go
@internal/platform/dirs.go
</context>

<reconciliation_notes>
## The interval is correct — do NOT touch the cadence
DefaultCatalogSyncConfig is {Enabled:true, Interval:"2h"}, clamped [2h,24h] (config.go:128). The
hourly schtasks / launchd StartInterval 3600 / systemd OnUnitActiveSec=1h is a deliberate HEARTBEAT
decoupled from the fetch cadence (the interval gate in runCatalogsSync:191 makes most heartbeats a
no-op so the OS schedule never needs rewriting when the user changes the interval). This task makes
the heartbeat SILENT + LOGGED; it must NOT change the heartbeat frequency or the interval clamp.

## Only console-hide is Windows-specific
Gaps 2, 3, 4 and the sync.log of Gap 1 are cross-platform. Pass `--background` in ALL THREE installers
(schtasks /tr, the launchd ProgramArguments, the systemd ExecStart) so mac/linux also tee to sync.log.
HideConsoleWindow() is a no-op on non-Windows (console_other.go) so the --background path compiles and
runs identically everywhere; only Windows actually hides a window.

## RunFirstResponder signature change ripples to TWO seams
RunFirstResponder currently returns only `error`. To feed the sync summary (Gap 2) it must also return
counts. Change it to `(FirstResponderResult, error)` and update BOTH injectable seams:
  - cmd/beekeeper/catalogs_daemon.go `firstResponderFn` (line ~29) and its test doubles.
  - internal/watch/firstresponder.go `firstResponderFn`/`defaultFirstResponder` (line ~67) and tests.
The watch-daemon caller (cmd/beekeeper/main.go onDelta path) ignores the new result (it already has its
own notify). Keep the change additive: result is a value struct, error semantics unchanged
(per-hit failures still never returned; CrossReference error still propagated).

## notify parity, not a new knob
Mirror the watch daemon EXACTLY: it passes `notify.Config{Enabled: true}` (main.go:789) and calls
notify.Notify on quarantine (handler.go:196). Thread a `NotifyConfig notify.Config` field into
FirstResponderConfig and have runCatalogsSync set `{Enabled: true}`. Do NOT add a config.json knob in
this slice (keeps scope tight + matches existing behavior). DryRun MUST NOT notify (only the real
MoveTyped success path notifies — same place the "catalog_quarantine" audit is written).

## Honesty invariants (bake into EVERY task)
- Scan READ-ONLY; quarantine REVERSIBLE move only; Purge stays human-gated (TUI [P] / CLI).
- Sentry target list DETECTION-ONLY (untouched here).
- Fail-closed/best-effort: a HideConsoleWindow, sync.log open, summary write, or notify failure is
  logged-and-continue and NEVER downgrades a block or aborts the sync.
- internal/policy stays pure (untouched).
- notify is fire-and-forget (notify.go already swallows errors + guards headless Linux).

## Windows / CRLF gotchas
- console_windows.go must carry `//go:build windows`; console_other.go `//go:build !windows`.
- Any byte-exact golden fixture (none expected here) needs a `.gitattributes` `text eol=lf` line.
- `go build` does NOT compile test files or the inactive-GOOS files — use `GOOS=linux go vet ./...`
  and `GOOS=darwin go vet ./...` to compile-validate the OS-split sources.
</reconciliation_notes>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1 (Gap 1 — log): LogDir helper + cross-platform sync.log tee + size rotation</name>
  <files>internal/platform/dirs.go, internal/platform/dirs_test.go, cmd/beekeeper/synclog.go, cmd/beekeeper/synclog_test.go</files>
  <behavior>
    - platform.LogDir() returns `<StateDir>/logs` (mirrors AuditDir/CatalogDir), created 0o700 by the caller.
    - synclog.go provides openSyncLog() -> (io.WriteCloser, error): opens `<LogDir>/sync.log` for append,
      and if the existing file exceeds a cap (e.g. 1 MiB) rotates it to `sync.log.1` (single backup) before opening.
    - synclog.go provides a teeWriter helper that wraps a cobra command's Out/Err to ALSO write to the log file,
      so `--background` runs persist their human-readable output while a manual `--force` run still prints to the terminal.
    - All failures are non-fatal: if the log cannot be opened, the sync proceeds writing only to the original Out/Err.
  </behavior>
  <action>Add platform.LogDir() to internal/platform/dirs.go + a dirs_test.go assertion it sits under StateDir. Add cmd/beekeeper/synclog.go with openSyncLog() (append + 1MiB single-backup rotation, owner-only 0o600 file under 0o700 dir) and a small tee helper that returns an io.Writer fanning out to [original, logfile]. Add synclog_test.go: a fresh log is created + written; a >1MiB log rotates to sync.log.1 then starts fresh; an unwritable dir degrades to original-only without error.</action>
  <verify>
    <automated>go test ./internal/platform/... ./cmd/beekeeper/ -run 'SyncLog|LogDir' -count=1</automated>
  </verify>
  <done>LogDir resolves under StateDir; openSyncLog rotates at the cap and degrades non-fatally; the tee writes to both sinks.</done>
  <commit>feat(platform,cmd): cross-platform sync.log dir + size-rotated tee (Gap 1)</commit>
</task>

<task type="auto" tdd="true">
  <name>Task 2 (Gap 1 — silent): --background flag, Windows console-hide, installer wiring (all 3 OSes)</name>
  <files>cmd/beekeeper/console_windows.go, cmd/beekeeper/console_other.go, cmd/beekeeper/main.go, cmd/beekeeper/catalogs_daemon_windows.go, cmd/beekeeper/catalogs_daemon_darwin.go, cmd/beekeeper/catalogs_daemon_linux.go</files>
  <behavior>
    - `beekeeper catalogs sync` gains a `--background` bool flag. When set: tee Out/Err to sync.log (Task 1) and call HideConsoleWindow().
    - console_windows.go (//go:build windows): HideConsoleWindow() calls GetConsoleWindow + ShowWindow(hwnd, SW_HIDE) via golang.org/x/sys/windows; a zero HWND or any error is ignored (best-effort).
    - console_other.go (//go:build !windows): HideConsoleWindow() is a no-op returning nil.
    - schtasks /tr becomes `"<exe>" catalogs sync --background`; when conhost.exe supports --headless (Windows 11) the installer prefers `conhost.exe --headless "<exe>" catalogs sync --background` for true zero-flash, else falls back to the plain self-hiding form.
    - launchd ProgramArguments and systemd ExecStart both append `--background` so mac/linux tee to sync.log too.
  </behavior>
  <action>Add the `--background` flag to syncCmd in newCatalogsCmd (main.go ~469-482) and thread it into runCatalogsSync (Task 3 changes the signature; for now have the RunE set up the tee + HideConsoleWindow before calling runCatalogsSync). Add console_windows.go + console_other.go with HideConsoleWindow(). In catalogs_daemon_windows.go set tr to the --background form and add best-effort conhost-headless detection (probe `conhost.exe --headless` availability; on failure use the plain form). In catalogs_daemon_darwin.go add `<string>--background</string>` to ProgramArguments; in catalogs_daemon_linux.go append ` --background` to ExecStart. No new test file required for the OS installers (they shell out); the flag plumbing is covered by Task 3's catalogs_daemon_test.go.</action>
  <verify>
    <automated>go build ./... && go vet ./... && env GOOS=linux go vet ./... && env GOOS=darwin go vet ./...</automated>
  </verify>
  <done>`catalogs sync --background` hides the console on Windows (no-op elsewhere) and logs to sync.log; all three installers pass --background; cross-OS vet is clean.</done>
  <commit>feat(cmd): silent --background scheduled sync + console-hide + 3-OS installer wiring (Gap 1)</commit>
</task>

<task type="auto" tdd="true">
  <name>Task 3 (Gap 2): SyncSummary in state.json + RunFirstResponder returns counts + `catalogs status`</name>
  <files>internal/catalog/state.go, internal/catalog/state_test.go, internal/watch/firstresponder.go, internal/watch/firstresponder_test.go, cmd/beekeeper/catalogs_daemon.go, cmd/beekeeper/catalogs_daemon_test.go, cmd/beekeeper/main.go</files>
  <behavior>
    - state.go: add SyncSummary{At time.Time, Result string, Entries int, ScanHits int, Quarantined int, Pending int, WouldQuarantine int, LastError string, NextDue time.Time} and `LastSync *SyncSummary json:"last_sync,omitempty"` on WatchState. Back-compat: absent field parses as nil.
    - firstresponder.go: RunFirstResponder returns (FirstResponderResult, error) where FirstResponderResult{ScanHits, Quarantined, Pending, WouldQuarantine int} counts both the scan-hit and corpus paths. Error semantics unchanged.
    - catalogs_daemon.go: runCatalogsSync records a SyncSummary after the pipeline+fetch: Result is "skipped" (interval gate), "unchanged" (304), "synced" (200), or "error"; NextDue = LastSuccess+interval; counts from FirstResponderResult.
    - main.go: `beekeeper catalogs status` prints the last summary (or "never synced"), the configured interval + next-due, the daemon registration state (reuse catalogDaemonStatus()), and the sync.log path.
  </behavior>
  <action>Add SyncSummary + LastSync to internal/catalog/state.go with a state_test.go round-trip (write summary, reload, fields intact; absent last_sync -> nil). Change RunFirstResponder to return FirstResponderResult; accumulate counts in both the scan-hit loop and the corpus loop; update defaultFirstResponder + the firstResponderFn seam + all firstresponder_test.go call sites. In catalogs_daemon.go update the package-level firstResponderFn seam type to return the result, capture it, and write the SyncSummary in every exit path (skip / 304 / 200 / error) BEFORE returning; update catalogs_daemon_test.go doubles. Add the `catalogs status` cobra subcommand in newCatalogsCmd. Status output must be honest: counts come from the recorded summary, "next due" from LastSuccess+interval, and it never fabricates a sync that did not run.</action>
  <verify>
    <automated>go test ./internal/catalog/... ./internal/watch/... ./cmd/beekeeper/ -count=1</automated>
  </verify>
  <done>SyncSummary round-trips in state.json; RunFirstResponder returns accurate counts; runCatalogsSync records a summary on every path; `catalogs status` prints last-run/result/counts/next-due/daemon-state/log-path.</done>
  <commit>feat(catalog,watch,cmd): persist sync summary + `catalogs status` surface (Gap 2)</commit>
</task>

<task type="auto" tdd="true">
  <name>Task 4 (Gap 3): desktop toast on a real sync-hit quarantine (cross-platform, dry-run-safe)</name>
  <files>internal/watch/firstresponder.go, internal/watch/firstresponder_test.go, cmd/beekeeper/catalogs_daemon.go</files>
  <behavior>
    - FirstResponderConfig gains `NotifyConfig notify.Config`. On a non-dry-run catalog_quarantine (both the scan-hit and corpus MoveTyped success branches), call notify.Notify(cfg.NotifyConfig, "Beekeeper: package quarantined", "<eco>/<pkg>@<ver> — <reason>").
    - DryRun ("would-quarantine") and pending-quarantine paths do NOT claim a move via toast (pending MAY emit a softer "review needed" notification — keep it honest, no "quarantined" wording).
    - runCatalogsSync sets NotifyConfig{Enabled: true}, mirroring the watch daemon.
    - notify stays best-effort: a notify failure never affects the quarantine/audit outcome.
  </behavior>
  <action>Add NotifyConfig to FirstResponderConfig. At the two MoveTyped success sites (scan-hit ~line 184 and corpus ~line 282) call notify.Notify after the catalog_quarantine audit write. Use the same injectable-stub discipline the notify package already supports (notify.notifyFunc) so firstresponder_test.go can assert: a real move with NotifyConfig{Enabled:true} calls notify exactly once with the package in the message; a DryRun move calls it zero times; NotifyConfig{Enabled:false} calls it zero times. In catalogs_daemon.go set NotifyConfig{Enabled:true} on the FirstResponderConfig built in runCatalogsSync.</action>
  <verify>
    <automated>go test ./internal/watch/... -count=1</automated>
  </verify>
  <done>A real catalog-sync quarantine fires exactly one best-effort toast naming the package; dry-run and disabled paths fire none; notify failure is non-fatal.</done>
  <commit>feat(watch,cmd): desktop notification on sync-hit quarantine (Gap 3)</commit>
</task>

<task type="auto" tdd="true">
  <name>Task 5 (Gap 4): TUI auto-raises the catalog-quarantine card from background FRSP records</name>
  <files>internal/tui/model.go, internal/tui/model_view_test.go, internal/tui/catalogs_panel.go</files>
  <behavior>
    - model.go newRecordsMsg: when a record's RecordType is "catalog_quarantine" or "pending-quarantine" (RuleIDs include FRSP-01/FRSP-02) and no critical sentry incident is already showing, raise CatalogQuarantineIncidentFromRecord(rec, pending) and set the status banner (e.g. "⚠ package quarantined: <pkg>").
    - The incident card's existing [R]estore / [P]urge / [A]cknowledge actions are wired to real ops: [R] -> quarantine.Restore, [P] -> human-gated purge confirm (reuse the QuarantinePanel confirm flow), [A] -> dismiss. Purge is NEVER auto-fired.
    - catalogs_panel.go: surface the last-sync summary (result + counts + next-due from state.json LastSync) as a one-line footer/status in the catalogs panel so the dashboard shows sync activity at rest.
    - A sentry_alert+critical record still takes precedence (do not overwrite a live critical banner).
  </behavior>
  <action>In model.go newRecordsMsg, extend the loop that currently handles only sentry_alert+critical: add a branch for catalog_quarantine/pending-quarantine that sets a.incident = CatalogQuarantineIncidentFromRecord(rec, pending) and a non-critical attention status (don't set a.critical=true so a real sentry event can still preempt). Wire the action keys via the existing incident Update + App key handling to quarantine.Restore / the purge-confirm path / acknowledge. In catalogs_panel.go read WatchState.LastSync in refresh() and render a "last sync: <result> · <n> hits · next <time>" line. Add model_view_test.go cases: a catalog_quarantine record raises the card with [R]/[P]/[A]; a pending-quarantine raises acknowledge-only; a sentry critical still wins precedence.</action>
  <verify>
    <automated>go test ./internal/tui/... -count=1</automated>
  </verify>
  <done>A background FRSP quarantine record auto-raises the catalog-quarantine card with real human-gated actions; the catalogs panel shows last-sync activity; a critical sentry alert still preempts.</done>
  <commit>feat(tui): auto-raise catalog-quarantine incident + sync-activity line (Gap 4)</commit>
</task>

<task type="auto">
  <name>Task 6 (docs + full cross-OS gate)</name>
  <files>docs/install-posture.md, cmd/beekeeper/main.go</files>
  <action>Document the new visibility surface in docs (the `--background` flag, the cross-platform `<StateDir>/logs/sync.log`, `beekeeper catalogs status`, and the sync-hit desktop notification + TUI card) — concise, no em-dashes (repo prose style). Confirm `catalogs status` help text is honest about the enforcement boundary (it reports, it does not itself block). Then run the full validation gate.</action>
  <verify>
    <automated>go build ./... && go vet ./... && go test ./... -count=1 && env GOOS=linux go vet ./... && env GOOS=darwin go vet ./... && env GOOS=windows go vet ./...</automated>
  </verify>
  <done>Docs describe the four new surfaces; go build/vet/test green on Windows; GOOS=linux/darwin/windows go vet all clean (console split + 3 installers compile-validate).</done>
  <commit>docs(install-posture): document sync visibility (background log, status, toast, TUI card) + full gate green</commit>
</task>

</tasks>

<verification>
Run from the repo root on Windows:
- `go build ./...` exit 0
- `go vet ./...` exit 0
- `go test ./...` exit 0 (new tests: sync.log rotation/tee, SyncSummary round-trip, RunFirstResponder counts, notify-on-real-move / not-on-dry-run, TUI catalog-incident auto-raise + precedence)
- `$env:GOOS='linux'; go vet ./...` then `'darwin'` then `'windows'` each exit 0 (compile-validates console_windows.go/console_other.go + the three daemon installers; go build does NOT compile inactive-GOOS files)
Confirm honesty invariants by inspection: no package mutation added to the scan/cross-ref path; no
kill/isolate/network-cut added; Purge reachable only via human [P]/CLI; internal/policy untouched;
notify + console-hide + log + summary writes are all best-effort/non-fatal; the 2h interval clamp and
hourly heartbeat frequency are unchanged.
</verification>

<success_criteria>
- Gap 1: the hourly Windows heartbeat no longer flashes a blank console (self-hide, conhost-headless on Win11); every --background run on every OS tees to a size-rotated sync.log.
- Gap 2: state.json carries a SyncSummary and `beekeeper catalogs status` reports last-run/result/counts/next-due/daemon-state/log-path.
- Gap 3: a real sync-hit quarantine fires a cross-platform best-effort desktop toast (dry-run/disabled fire none).
- Gap 4: the TUI auto-raises the catalog-quarantine card from background FRSP records with human-gated [R]/[P]/[A], and shows a sync-activity line; sentry-critical still preempts.
- The configured interval (2h default, [2h,24h]) and the hourly heartbeat are unchanged; no new deps; full gate + 3×GOOS vet green; honesty invariants intact.
</success_criteria>

<web_docs_blog_followup>
OUT of this plan's Go scope (separate beekeeper-web repo, resolves source_doc against ../beekeeper).
AFTER the Go work is green, a separate step updates web copy to mention the sync-visibility surface
(catalogs status, background sync.log, the sync-hit toast + TUI card). Web honesty rules: NO em-dashes
in visible copy, NO colored left-edge accent stripes; re-run the web accuracy + build gate.
</web_docs_blog_followup>

<output>
Create `.planning/quick/260625-svl-catalog-sync-visibility-alerting/260625-svl-SUMMARY.md` when done.
</output>
