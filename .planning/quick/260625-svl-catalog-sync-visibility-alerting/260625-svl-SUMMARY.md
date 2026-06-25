---
quick_id: 260625-svl
slug: catalog-sync-visibility-alerting
status: complete
run_mode: validate
branch: feat/catalog-sync-visibility
date: 2026-06-25
---

# Summary — Catalog-sync visibility + alerting

Closed all four catalog-sync visibility gaps without changing the sync pipeline or
the 2h-default/[2h,24h] interval cadence. Six atomic commits on
`feat/catalog-sync-visibility`; full gate green (build, vet, `go test ./...` exit 0,
GOOS=linux/darwin/windows vet, em-dash check).

## What shipped

- **Gap 1 (silent + logged).** `catalogs sync --background` hides its console on
  Windows (`ShowWindow SW_HIDE`, lazy kernel32/user32; conhost `--headless` preferred
  on Win11 via the installer) and tees output to a size-rotated
  `<state>/logs/sync.log` on every OS. `--background` is wired into all three
  installers (schtasks, launchd, systemd). macOS/Linux had no window flash; the log
  is the cross-platform upgrade (macOS launchd previously discarded output).
- **Gap 2 (status).** `SyncSummary` persisted to state.json on every exit path
  (synced/unchanged/skipped/disabled/error); `RunFirstResponder` now returns
  `FirstResponderResult` counts; new `beekeeper catalogs status` reports result,
  counts, next-due, daemon registration, and the log path.
- **Gap 3 (toast).** A real (non-dry-run) sync-hit quarantine fires a best-effort
  desktop notification via `internal/notify` (Win toast / macOS Notification Center /
  Linux notify-send; headless Linux no-ops), routed through a swappable `notifyFn`
  seam so the call-site discipline is unit-tested.
- **Gap 4 (TUI card).** A background FRSP-01/02 record auto-raises the existing
  catalog-quarantine card via a dedicated lower-precedence `quarantineAlert` flag (a
  sentry critical still preempts); `[r]`/`[p]` route to the admin-gated quarantine
  panel, `[a]` dismisses. The catalogs panel shows a `last sync:` activity line.

## Honesty invariants preserved

Scan read-only; quarantine reversible-move-only (purge human-gated); Sentry
detection-only; `internal/policy` untouched; all new I/O (console-hide, log, notify,
summary) best-effort and fail-closed; cadence + heartbeat unchanged; no new deps
(`golang.org/x/sys`, `gen2brain/beeep` already present).

## Verification

- `go build ./...`, `go vet ./...`, `go test ./... -count=1` all exit 0.
- `GOOS=linux go vet ./...`, `GOOS=darwin go vet ./...` exit 0 (OS-split console +
  installers compile-validated).
- Live read-only smoke: `beekeeper catalogs status` renders interval 2h, daemon
  registered, log path, "never synced" (no run on the new binary yet).

## Notes / follow-ups

- The live `--background` sync was NOT run against the real install (it could fetch +
  quarantine). Behavior is covered by unit tests.
- Web/docs (separate beekeeper-web repo) NOT updated — out of Go scope, named in the
  plan's web_docs_blog_followup.
- `notify` uses `Enabled:true` parity with the watch daemon (no config knob added);
  could become a config toggle later if desired.
