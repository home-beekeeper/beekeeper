---
gsd_state_version: 1.0
milestone: v1.1.0
milestone_name: "Pollen" — Windows Inventory Compatibility
status: executing
stopped_at: Completed 01-03-PLAN.md
last_updated: "2026-06-02T10:04:40.085Z"
last_activity: 2026-06-02
progress:
  total_phases: 5
  completed_phases: 1
  total_plans: 9
  completed_plans: 6
  percent: 20
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-06-01)

**Core value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Current focus:** Phase 02 — windows-root-resolver

## Current Position

Phase: 02 (windows-root-resolver) — EXECUTING
Plan: 2 of 4
Status: Ready to execute
Last activity: 2026-06-02

Progress: `[██░░░░░░░░] 20%` (1/5 plans in phase 01)

Next action: Execute plan 01-02

## Phase Summary (v1.1.0)

| Phase | Name | Tag | Requirements | Status |
|-------|------|-----|--------------|--------|
| 1 | Fork Setup & Discipline | v0.1.1-pollen.1 | FORK-01–04, PTEST-02–03, SDEF-02 | Not started |
| 2 | Windows Root Resolver | v0.1.1-pollen.2 | WRES-01–02, PTEST-01 | Not started |
| 3 | Windows Path Representation | v0.1.1-pollen.3 | WPATH-01–02 | Not started |
| 4 | Windows Extension & MCP Coverage | v0.1.1-pollen.4 | WEXT-01–03, BKINT-01, PTEST-04 | Not started |
| 5 | Contribution-Back & Milestone Close | v0.1.1-pollen.5 | SYNC-01–02, BKINT-02, PTEST-05, SDEF-01 | Not started |

## Phase 1 Completion Summary (v1.0.0 reference)

### Plans completed (all 6/6)

| Wave | Plan | Title | Commit | Status |
|------|------|-------|--------|--------|
| 1 | 01 | Project Scaffold | 5c0c515 | ✅ Done |
| 2 | 02 | Catalog Sync + mmap index | 009284d | ✅ Done |
| 2 | 03 | Self-Defense (builds/signing) | e81b019 | ✅ Done |
| 3 | 04 | Pure Policy Engine (TDD) | afd5f67 | ✅ Done |
| 4 | 05 | NDJSON Audit Logging | f5d6489 | ✅ Done |
| 5 | 06 | Hook Handler + Selftest | 88c34bb | ✅ Done |

### UAT results (01-UAT.md — all automated)

- Build + vet: pass
- Full test suite (6 packages): pass
- `beekeeper version`: pass
- `beekeeper init`: pass
- `beekeeper catalogs sync` (654 entries, mmap index): pass
- `beekeeper check` clean package → allow, exit 0: pass
- `beekeeper check` nrwl.angular-console@18.95.0 → warn, exit 0: pass
- `beekeeper check` malformed JSON → block, exit 1 (fail-closed): pass
- `beekeeper selftest` → PASS: 7, FAIL: 0: pass
- `beekeeper audit tail` wired (no "not yet implemented"): pass
- Self-defense files (Makefile, goreleaser, SECURITY.md, Renovate, CI, release): pass
- `go mod verify` → all modules verified: pass

### Known constraints carried forward

- `go test -race` requires CGO + C compiler (not installed on Windows dev machine); race gate runs in CI
- `make verify-release` requires `make` (not installed on Windows); reproducibility logic validated manually; CI covers it
- Phase 1 is single-source warn semantics only; corroboration-based block enforcement is Phase 2

## Phase 2 Completion Summary (v1.0.0 reference)

### Plans completed (all 9/9)

| Wave | Plan | Title | Commit | Status |
|------|------|-------|--------|--------|
| 1 | 02-01 | Corroboration types + engine refactor | — | ✅ Done |
| 1 | 02-02 | Sensitive path policy + output credential filtering | — | ✅ Done |
| 1 | 02-03 | Network egress + multi-turn exfiltration + baseline | — | ✅ Done |
| 2 | 02-04 | OSV public REST API catalog adapter | — | ✅ Done |
| 2 | 02-05 | Socket PURL API adapter | — | ✅ Done |
| 2 | 02-06 | Release-age + lifecycle-script policies | — | ✅ Done |
| 3 | 02-07 | Catalog watch daemon + sanity bounds | — | ✅ Done |
| 4 | 02-08 | Multi-source aggregator + baseline + audit + CLI | — | ✅ Done |
| 5 | 02-09 | Fuzz CI release gates + corroboration integration tests | 6bf6f05 | ✅ Done |

## Phase 3 Completion Summary (v1.0.0 reference)

### Plans completed (all 5/5)

| Wave | Plan | Title | Commit | Status |
|------|------|-------|--------|--------|
| 1 | 03-01 | EDXT-01 extension install recognition + deps | 69536fd | ✅ Done |
| 1 | 03-02 | Marketplace adapter + quarantine manager | a84a742 | ✅ Done |
| 2 | 03-03 | fsnotify watch daemon + manifest parser + handler | e4e3ba4 | ✅ Done |
| 2 | 03-05 | Editor detection + JSONC settings patch + selftest fixture | 7b1aa9d | ✅ Done |
| 3 | 03-04 | Scan orchestrator + watch/scan/quarantine CLI + init | 9428ed3 | ✅ Done |

### Key deliverables

- `internal/policy`: EDXT-01 extension-install recognition (code --install-extension, cursor, windsurf)
- `internal/catalog`: Open VSX + VS Code Marketplace age adapters; marketplace-cache on disk
- `internal/quarantine`: list/restore/purge operations; per-item metadata NDJSON
- `internal/watch`: fsnotify daemon + manifest parser + quarantine/notify handler (EDXT-02, EDXT-03)
- `internal/editorinit`: editor detection (VS Code, Cursor, Windsurf) + JSONC settings patch
- `internal/scan`: Bumblebee subprocess orchestrator + Beekeeper-own per-extension scan (EDXT-04)
- `internal/config`: WatchSettings + AddWatchDirectory + Save
- `cmd/beekeeper/main.go`: watch, scan, quarantine, extended init commands (EDXT-04, EDXT-05, EDXT-06)

### Deviations from plan

- `evaluateExtension` in scanner.go duplicates adapter construction from handler.go (noted; minimal, not shared because handler.go is quarantine/notify coupled)
- `notify.Config{Enabled: true}` hardcoded in newWatchCmd (notification preferences deferred to a future phase)
- `quarantine_restore`/`quarantine_purge` audit RecordTypes differ from standard `policy_decision` schema (acceptable for Phase 3 audit trail)

## Phase 7 Completion Summary (v1.0.0 reference)

### Plans completed (all 5/5)

| Wave | Plan | Title | Commit | Status |
|------|------|-------|--------|--------|
| 1 | 07-01 | macOS eslogger subprocess + event drain | 08552c4 | ✅ Done |
| 1 | 07-02 | Windows ETW ingestion layer | 7030ce5 | ✅ Done |
| 2 | 07-03 | macOS Sentry daemon + launchd CLI | 61ae8e3 | ✅ Done |
| 3 | 07-04 | Windows Sentry daemon + named pipe IPC | e959585 | ✅ Done |
| 4 | 07-05 | SLSA Level 3 + CycloneDX SBOM + eslogger CI gate | 1e4d1ec | ✅ Done |

### Key deliverables

- `internal/sentry/darwin/` — eslogger subprocess drain, parser, launchd plist, daemon (RunDaemon + correlationEngineLoop)
- `internal/sentry/windows/` — tekert/golang-etw v0.6.2 ingestion, NT Kernel Logger conflict probe, Windows Service install/uninstall, RunDaemon via svc.Run
- `internal/ipc/pipe_windows.go` — go-winio named pipe IPC replaces ErrNotSupported stub; SDDL DACL restricted to installing-user SID
- `internal/ipc/peer_linux.go` + `peer_darwin.go` — SO_PEERCRED/LOCAL_PEERCRED platform split (fixed cross-compile bug)
- `cmd/beekeeper/protect_darwin.go` + `protect_windows.go` — platform CLIs with admin guard; SWIN-03 conflict + SWIN-04 EventsLost surfacing
- `.goreleaser.yaml` — CycloneDX SBOM via syft added; Phase 1 signs/builds/checksums preserved
- `.github/workflows/release.yml` — SLSA Level 3 provenance via `slsa-github-generator@v2.1.0` (full semver, locked)
- `.github/workflows/ci.yml` — `test-eslogger-fields` job on macos-latest; wired into release-gate
- `internal/sentry/darwin/eslogger_fields_test.go` — live eslogger schema validation; skips locally, blocks release on schema drift

### API deviations discovered

- tekert/golang-etw: `c.EventCallback = func(*etw.Event) error` (not `ProcessEvents`); `c.Start()` non-blocking; `etw.ERROR_ALREADY_EXISTS` not `windows.ERROR_ALREADY_EXISTS`; `e.System.Provider.Guid` is `etw.GUID` type
- go-winio: import path is `github.com/Microsoft/go-winio` (capital M); lowercase fails at go get with module path mismatch
- Windows elevation: `GetCurrentProcessToken().IsElevated()` (no unsafe.Pointer dance needed)

## Phase 6 Completion Summary (v1.0.0 reference)

### Plans completed (5/5)

| Wave | Plan | Title | Commit | Status |
|------|------|-------|--------|--------|
| 1 | 06-01 | Audit Rotation, Query, Export | 07b2c66 | ✅ Done |
| 1 | 06-02 | LlamaFirewall IPC Protocol + Fuzz | — | ✅ Done |
| 2 | 06-03 | Audit Sinks + Config Extensions | — | ✅ Done |
| 2 | 06-04 | LlamaFirewall Supervisor + Client + Sidecar | 546d94c | ✅ Done |
| 3 | 06-05 | LLMF Handler+Gateway Integration | 006edb7 | ✅ Done |

### UAT results (06-UAT.md — all automated)

- Cold start smoke test (version + audit query): pass
- Audit log rotation (numbered archives, retention, no-op below threshold): pass
- Audit query command (filter by since/agent/tool/decision, skip malformed): pass
- Audit export CSV (fixed header + data rows): pass
- Audit export OTLP (resourceLogs envelope): pass
- Audit tail --no-follow (print once, exit): pass
- LlamaFirewall IPC protocol (9 unit tests, 1MB cap, fuzz CI gate): pass
- Audit multi-sink fan-out (10 tests, fire-and-forget remote errors): pass
- AuditConfig + LlamaFirewallConfig in config (accessor methods): pass
- LlamaFirewall CLI enable/disable/status: pass
- Supervisor fail-closed/open after MaxRetries: pass
- LatencyTracker P95 ring-buffer: pass
- LLMF hook handler integration (injection alert exit 0, fail-closed exit 1): pass
- LLMF gateway integration (CodeShield block/warn wired): pass
- LLMF AuditRecord fields (LLMFScanned et al.): pass

### Key decisions from Phase 6

- Remote sink errors are fire-and-forget; local NDJSON write is never blocked
- AuditConfig imported by audit package (no import cycle; config imports stdlib only)
- Injection detection (LLMF-02) exits 0 — PostToolUse hooks must not block agent flow
- scan_code / scan_alignment are stubs in Python sidecar (CodeShield model integration is a follow-on)

## Performance Metrics

**Velocity (v1.0.0):**

- Total plans completed: 51
- Average duration: ~10 min/plan

**By Phase (v1.0.0):**

| Phase | Plans | Avg/Plan |
|-------|-------|----------|
| 1 (Foundation + Hook Handler) | 6/6 | ~10 min |
| 2 (Policy Engine + Multi-Source Catalogs) | 9/9 | ~10 min |
| 3 (Editor Extension Defense) | 5/5 | ~10 min |
| Phase 09 P04 | 20min | 2 tasks | 9 files |
| Phase 09 P05 | 45min | 3 tasks | 8 files |
| Phase 10 P10-01 | 45 | 5 tasks | 13 files |
| Phase 11 P11-01 | 27min | 6 tasks | 15 files |
| Phase 01 P01 | 19min | 3 tasks | 42 files |
| Phase 01-fork-setup-discipline P03 | 35 | 2 tasks | 7 files |
| Phase 02-windows-root-resolver P01 | 15 | 3 tasks | 3 files |

## Accumulated Context

### Decisions

Recent decisions from Phase 01 (v1.1.0 Pollen - plan 01-01):

- GOOS=windows go build ./... passes clean — no non-test files needed Windows fixes (Open Question 1 resolved)
- 6 Unix-root-resolver tests in cmd/pollen/main_test.go get t.Skip with "Phase 2 (v0.1.1-pollen.2)" structured reasons (not build tags — allows other tests in the file to run)
- scanner_test.go TestEndToEndScan: path separator bug fixed via filepath.Separator (not a skip; test passes on all OSes)
- BUMBLEBEE_ env var names renamed to POLLEN_ in roots.go + main_test.go (FORK-04 trademark)
- upstream remote configured at pollen repo clone; origin binding to github.com/bantuson/pollen deferred to plan 05

Recent decisions from Phase 11:

- VerifySignatureWithKey(entry, pubKey) added alongside VerifySignature — presence-only path unchanged for backward compat
- Dissent sentinels (CatalogMatch{Dissented:true}) emitted by MultiIndex.LookupAll for configured-but-no-match sources; corroborate() filters them into SourcesDissented — import cycle avoided
- scanOnDeltaFn injectable var follows runBumblebeeFn pattern for test-time mock without real scan binary
- GoReleaser before.hooks uses sh -c guard so non-Linux environments skip eBPF generate gracefully
- -buildvcs=false added to goreleaser build flags (reproducibility gap closure)

Recent decisions from Phase 7:

- go-winio import path is github.com/Microsoft/go-winio (capital M); lowercase fails at go get with module path mismatch
- PipePath is var not const to enable test-time substitution; production value unchanged
- GetCurrentProcessToken().IsElevated() replaces manual TOKEN_ELEVATION unsafe pointer dance
- ETW EnableProvider is the actual API (not AddProvider); Provider struct needs GUID value type from *MustParseGUID dereference
- TestQueryServiceWhenNotInstalled skips on non-admin (mgr.Connect returns Access Denied); covered by CI admin runners

Recent decisions from Phase 6:

- Remote sink errors are fire-and-forget (nil returned); local NDJSON write is never blocked by remote collector outage
- AuditConfig imported by audit/sink.go from internal/config — no import cycle (config imports only stdlib)
- LlamaFirewall injection detection (LLMF-02) exits 0 in hook handler — PostToolUse hooks must not block agent flow; llmf_alert is the forensic signal
- scan_code / scan_alignment are Python sidecar stubs; CodeShield model integration deferred
- Scannable interface in check package, GatewayScanner in gateway/policy.go — avoids circular imports; supervisor satisfies at runtime, mocks in tests

Earlier decisions from Phase 3:

- Injectable `runBumblebeeFn` package var for test isolation without real binary (Windows portability)
- Pre-seeded marketplace cache pattern for tests (avoids live network; same as handler_test.go)
- `catalog.NewMultiIndex` nil-safe: nil first arg skips Bumblebee source gracefully
- `WatchSettings` added to Config as pointer field (omitempty — backward-compatible)
- `quarantine_restore`/`quarantine_purge` NDJSON records use custom RecordType values (outside standard schema — acceptable for Phase 3 completeness)

Earlier decisions from Phase 1 (full log in PROJECT.md):

- `CatalogLookup` interface decouples policy engine from concrete mmap type (testability)
- `internal/audit/FromDecision` takes caller-supplied recordID/timestamp (purity)
- Selftest uses `//go:embed corpus/fixtures.json` — no runtime file dependency
- `npm install` command shape maps to `npm` ecosystem; editor extensions tested via direct `editor-extension` shape
- BEEI v1 mmap index format (magic 0x42454549, sorted 48-byte records, sha256 key, LE offsets)
- [Phase ?]: ThresholdsFromPolicyFiles exported from policyloader for shared use by check, gateway, watch, scan
- [Phase ?]: gateway.Config.Scanner GatewayScanner field (nil=disabled) for LLMF; lifecycle owned by main.go
- [Phase ?]: llmfClientScanner adapter bridges *llamafirewall.Client to check.Scannable for one-shot audit-record
- [Phase ?]: watch/scan policy overlay errors are non-fatal; handler.go is fail-closed as primary enforcement point
- [Phase ?]: scanner_name stripped alongside 7 non-det fields in normalize() — differential asserts detection-logic parity not self-identification-string parity
- [Phase ?]: TestDifferential name LOCKED in cmd/pollen/differential_test.go — plan 04 CI invokes it via go test ./cmd/pollen/ -run '^TestDifferential$'
- [Phase ?]: normalize() uses map[string]any (not struct) — tolerates additive upstream fields without requiring schema updates; Go json.Marshal sorts map keys deterministically
- [Phase ?]: test decision
- [Phase ?]: Phase 02-01: roots_notwindows.go (//go:build !windows) stubs required — Go compiles all switch case bodies at build time; case windows: bodies need !windows stubs for tri-GOOS build
- [Phase ?]: Phase 02-01: env-var guards are per-variable (if appdata := ...) not single upfront check — Windows has multiple independent env vars unlike Unix HOME
- [Phase ?]: Phase 02-01: GOPATH not consulted for Go modules root — mirrors upstream Unix behavior using %USERPROFILE%\go\pkg\mod only

### Open Research Flags (v1.1.0)

- **Phase 2**: fsnotify Windows junction point behavior with package roots under `%LOCALAPPDATA%` / `%APPDATA%` — needs live testing on Windows CI runner
- **Phase 2**: `%ProgramFiles%` Ruby enumeration — glob pattern for versioned directories (`Ruby*`) requires validation against an actual Windows Ruby installation
- **Phase 4**: Cursor extension directory on Windows confirmed `%USERPROFILE%\.cursor\extensions\` (from Deviations: "Cursor Windows extension-dir path (Assumption A1) — Needs live validation" carried from Phase 3)
- **Phase 5**: Upstream PR submission timing — coordinated (all 4 PRs together at M2.5) vs incremental per sub-phase; coordinated is cleaner; confirm before Phase 5 plan

### Blockers/Concerns

- Phase 4: MCP client differences (Claude Code vs Cursor) expose different edge cases; July 2026 spec SDK lag
- Phase 4: MCP message parser must be fuzz-tested before v0.6.0 as a release gate (not backlog item)
- Phase 5: eBPF CI matrix needs Ubuntu 20.04 (kernel 5.4) and 22.04 (kernel 5.15) — ubuntu-latest alone is insufficient
- ~~Phase 7: eslogger field coverage incomplete from documentation~~ — RESOLVED: `test-eslogger-fields` CI gate on macos-latest validates field paths against live eslogger output; blocks release on schema drift

## Deferred Items

Items acknowledged and carried forward:

| Category | Item | Status | Deferred At |
|----------|------|--------|-------------|
| Testing | `go test -race` requires CGO/C compiler | CI-only | Phase 1 (v1.0.0) |
| Build | `make verify-release` requires make on Windows | CI-only | Phase 1 (v1.0.0) |
| Audit | Log rotation, sinks (syslog/OTLP/HTTPS) | Shipped Phase 6 | Phase 1 (v1.0.0) |
| Hooks | `beekeeper hooks install` (INTG-01) | Shipped Phase 4 | Phase 1 (v1.0.0) |
| Watch | `notify.Config` wired to config preferences | Future phase | Phase 3 (v1.0.0) |
| Cursor | Windows extension-dir path (Assumption A1) | Live validation in v1.1.0 Phase 4 | Phase 3 (v1.0.0) |
| Distribution | Pollen binary releases (DIST-01) | v2 requirement | v1.1.0 scoping |
| Self-catalog | Separate `pollen-self` catalog (SELF-02) | v2 requirement | v1.1.0 scoping |

## Session Continuity

Last session: 2026-06-02T10:04:40.068Z
Stopped at: Completed 01-03-PLAN.md
Resume file: None

## Operator Next Steps

- **Plan Phase 1** with `/gsd-plan-phase 1` — Fork Setup & Discipline (FORK-01–04, PTEST-02–03, SDEF-02). Work happens in the NEW `bantuson/pollen` repo (create it first).
- **Create the pollen repo** at `github.com/bantuson/pollen` before or during Phase 1 plan execution — the fork target must exist.
- **Still pending (from v1.0.0 close):** push `bantuson/beekeeper` to GitHub remote if not done; verify CI fires correctly.
