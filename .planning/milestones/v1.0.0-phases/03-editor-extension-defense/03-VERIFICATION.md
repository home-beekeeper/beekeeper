---
phase: 03-editor-extension-defense
verified: 2026-05-26T12:00:00Z
status: passed
score: 5/5
overrides_applied: 0
---

# Phase 3: Editor Extension Defense — Verification Report

**Phase Goal:** Agent-initiated extension installs and silently dropped extension directories are intercepted and evaluated before they can execute, closing the Nx Console-class attack surface.
**Verified:** 2026-05-26T12:00:00Z
**Status:** PASSED
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `code --install-extension` (and cursor/windsurf/code-insiders variants, including bulk) are intercepted before install | VERIFIED | `internal/policy/engine.go:15-20` — `editorInstallPatterns` covers all four editors; `extractExtensionInstall()` at line 247 is called in `extract()` at line 217 before `extractFromCommand()`; bulk path in `Evaluate()` at lines 69-91. Three named tests confirmed in 03-01-SUMMARY. |
| 2 | `beekeeper watch` detects new extension directories via OS-native filesystem events without polling | VERIFIED | `internal/watch/watcher.go:36-96` — `Watch()` uses `fsnotify.NewWatcher()` with event-driven `select` loop, 30s retry for non-existent dirs, 500ms debounce. No polling ticker on events. Tests: `TestWatchNonExistentDir`, `TestWatchDebounce` in `internal/watch/watcher_test.go`. |
| 3 | On catalog hit or release-age block, Beekeeper emits a critical audit record, optionally sends a desktop notification, and moves the extension to `~/.beekeeper/quarantine/extensions/` | VERIFIED | `internal/watch/handler.go:154-202` — on `hit`, writes `sentry_alert` audit record, calls `notify.Notify()`, calls `quarantine.Move()`. `TestHandleNewExtensionCatalogHit` (handler_test.go:18-131) asserts all three outcomes with a live temp-dir test exercising the full pipeline. |
| 4 | Developer can run `beekeeper quarantine list`, `quarantine restore <id>`, and `quarantine purge` | VERIFIED | `cmd/beekeeper/main.go:500-649` — `newQuarantineCmd()` registers all three subcommands. `quarantine.List/Restore/Purge` are substantive in `internal/quarantine/quarantine.go`. Purge requires `--yes` or interactive `[y/N]` prompt. |
| 5 | Developer can run `beekeeper init` to detect installed editors and offer (with consent) to disable extension auto-update and enable the file-watcher | VERIFIED | `cmd/beekeeper/main.go:83-195` — `newInitCmd()` calls `editorinit.DetectEditors()`, presents per-editor per-action consent prompts (two prompts per editor), calls `editorinit.DisableExtensionAutoUpdate()` and `cfg.AddWatchDirectory()` on consent. `--yes` and `--no-editors` flags present. Idempotent via `AddWatchDirectory()` dedup. |

**Score: 5/5 truths verified**

---

### Deferred Items

None.

---

## Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/policy/engine.go` | `extractExtensionInstall` pure function + `editorInstallPatterns` | VERIFIED | Lines 15-20, 247-268. Only `"strings"` imported. All four editor prefixes present. |
| `internal/policy/engine_test.go` | `TestExtensionInstallExtract`, `TestExtensionInstallBulk`, `TestExtensionInstallVariants` | VERIFIED | All three functions confirmed at lines 430, 524, 569. |
| `go.mod` | `fsnotify v1.10.1`, `beeep v0.11.2`, `tidwall/jsonc v0.3.3` | VERIFIED | go.mod lines 17, 18, 29 — all three at exact pinned versions. |
| `internal/catalog/marketplace.go` | `FetchMarketplaceAge` + Open VSX + VS Code Marketplace fallback + 24h cache | VERIFIED | Full implementation lines 1-218. Cache-first, both adapters, fail-closed on both failing. `marketplaceCachePath` present. |
| `internal/quarantine/quarantine.go` | `Move`, `List`, `Restore`, `Purge` + `Manifest` with `OriginalPath` | VERIFIED | All four functions present (lines 77-232). Path traversal guard in `Move` (lines 89-93) and `Restore` (lines 174-177). `platform.SetOwnerOnly` called on manifest. |
| `internal/watch/watcher.go` | `Watch()` daemon with non-existent-dir retry, debounce, Windows Create-only filter | VERIFIED | Lines 36-96. `shouldProcess()` at line 114 uses `runtime.GOOS == "windows"`. `processEvent()` factored for test isolation. |
| `internal/watch/manifest.go` | `ParseManifest` + `ExtensionManifest` + `ErrNoManifest` | VERIFIED | All three present. 1MB size cap enforced. `Publisher == "" || Name == ""` → `ErrNoManifest`. |
| `internal/watch/handler.go` | `HandleNewExtension` with full pipeline: parse → catalog → release-age → audit/notify/quarantine | VERIFIED | Lines 61-221. Symlink escape guard present (lines 65-75). `WatchedRoots` field held on `Handler`. |
| `internal/notify/notify.go` | Best-effort `Notify()`, error swallowed, Linux headless guard | VERIFIED | Lines 27-42. `_ = notifyFunc(...)` swallows error. Linux DISPLAY/WAYLAND check at lines 35-39. |
| `internal/scan/scanner.go` | `Scan()` orchestrator: Bumblebee subprocess + Beekeeper-own + NDJSON merge + graceful degradation | VERIFIED | Lines 98-150. `exec.LookPath` used (line 55). No `--format` flag. `bumblebee_unavailable:true` on absence. `beekeeperScan()` runs regardless. |
| `cmd/beekeeper/main.go` | `watch`, `scan`, `quarantine` subcommands + extended `init` | VERIFIED | Lines 54-57 register all four. All use thin wiring with business logic in `internal/` packages. |
| `internal/editorinit/detect.go` | `DetectEditors` + `Editor` struct + platform-aware paths | VERIFIED | Lines 108-147. `lookPath` and `statFunc` injectable vars. `knownEditors()` evaluates at call time. |
| `internal/editorinit/settings.go` | `PatchSettings` JSONC-safe + `DisableExtensionAutoUpdate` | VERIFIED | Lines 23-73. Uses `jsonc.ToJSON` (line 34). Atomic write via temp file + `os.Rename` (lines 63-65). MkdirAll for missing parent (line 56). |
| `internal/check/corpus/fixtures.json` | EDXT-01 selftest fixture for `code --install-extension nrwl.angular-console@18.95.0` | VERIFIED | Lines 121-133. `expect_level: "warn"`, `expect_allow: true`, `expect_catalog_match: true`, `expect_rule_id: "bumblebee-catalog-match"`. |
| `internal/watch/testdata/valid-extension/package.json` | ms-python.python@2026.4.0 | VERIFIED | File exists with correct content. |
| `internal/watch/testdata/malicious-extension/package.json` | nrwl.angular-console@18.95.0 | VERIFIED | File exists with correct content. Used in `TestHandleNewExtensionCatalogHit`. |
| `internal/config/config.go` | `WatchSettings`, `AddWatchDirectory`, `Save` | VERIFIED | Lines 39-89. All three present. `AddWatchDirectory` is idempotent (dedup loop at lines 82-88). |

---

## Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/policy/engine.go extract()` | `extractExtensionInstall()` | Called before `extractFromCommand()` in command-shape branch | VERIFIED | `engine.go:216-219` — editor check is first in command branch |
| `internal/watch/watcher.go shouldProcess` | `runtime.GOOS == "windows"` Create-only filter | `event.Has(fsnotify.Create)` | VERIFIED | `watcher.go:115` — `runtime.GOOS == "windows"` guard present |
| `internal/watch/handler.go HandleNewExtension` | `quarantine.Move` + `notify.Notify` + `audit.Writer` | On catalog hit or release-age block | VERIFIED | `handler.go:165-201` — all three called in hit path |
| `internal/scan/scanner.go runBumblebee` | `exec.LookPath("bumblebee")` then `exec.CommandContext` | No `--format` flag | VERIFIED | `scanner.go:55,71-75` — `LookPath` used; args = `["scan"]` or `["scan","--profile","deep"]` |
| `cmd/beekeeper/main.go newWatchCmd` | `watch.Watch` + `watch.NewHandler` | `signal.NotifyContext` foreground daemon | VERIFIED | `main.go:413-437` — `NewHandler` constructed, `signal.NotifyContext` wraps context, `watch.Watch` called |
| `cmd/beekeeper/main.go init` | `editorinit.DetectEditors` + `DisableExtensionAutoUpdate` | Per-editor consent prompt | VERIFIED | `main.go:131-187` — `DetectEditors()` called, consent gated, `DisableExtensionAutoUpdate` called on consent |
| `internal/editorinit/settings.go PatchSettings` | `tidwall/jsonc.ToJSON` for comment stripping | read → strip → unmarshal → set → marshal → atomic write | VERIFIED | `settings.go:34` — `jsonc.ToJSON(data)` present |
| `internal/catalog/marketplace.go FetchMarketplaceAge` | `marketplace-cache` disk dir | Cache-first read then HTTP then write, 24h TTL | VERIFIED | `marketplace.go:139-218` — `marketplaceCachePath` used; `ageCacheTTL` (24h) check at line 143 |

---

## Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `internal/watch/handler.go HandleNewExtension` | `catalogDecision` / `ageDecision` | `policy.Evaluate` over `catalog.NewMultiIndex(bbIdx, osvAdapter, socketAdapter)` + `catalog.FetchMarketplaceAge` | Yes — mmap index lookup + marketplace HTTP with cache | FLOWING |
| `internal/scan/scanner.go beekeeperScan` | `catalogDecision` / `ageDecision` | Same multi-source path as handler; `watch.ParseManifest` provides manifest identity | Yes — catalog lookup + marketplace cache | FLOWING |
| `internal/quarantine/quarantine.go List` | `manifests []Manifest` | `os.ReadDir` + `os.ReadFile` of `beekeeper-manifest.json` per entry | Yes — reads actual quarantine directory | FLOWING |

---

## Behavioral Spot-Checks

The Go toolchain is not available in the current shell environment (Bash in Windows sandbox does not expose the PATH-mounted Go installation). The user confirmed `go test ./... -count=1` with 13 packages passing before this verification was initiated. Step 7b is noted as requires-human for runtime confirmation. Static code analysis above provides confidence.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| `code --install-extension` recognized | `go test ./internal/policy/... -run TestExtensionInstall` | Confirmed in SUMMARY and code inspection | PASS (static) |
| Watch daemon compiles and links | `go build ./...` | Confirmed in SUMMARY; all imports resolve | PASS (static) |
| Quarantine CLI wired | `go run ./cmd/beekeeper quarantine --help` | Confirmed in SUMMARY; `newQuarantineCmd` registered | PASS (static) |
| EDXT-01 selftest fixture active | `go test ./internal/check/... -run TestSelftest` | Fixture present in `corpus/fixtures.json:121-133` | PASS (static) |

---

## Probe Execution

No `scripts/*/tests/probe-*.sh` files declared or present for this phase. Step 7c: SKIPPED (no probes defined for Phase 3).

---

## Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| EDXT-01 | 03-01, 03-05 | Editor-extension CLI install recognition (code/cursor/windsurf/code-insiders, bulk) | SATISFIED | `extractExtensionInstall` in engine.go; `extractAllExtensionInstalls` for bulk; selftest corpus fixture. |
| EDXT-02 | 03-03, 03-04 | File-watcher detects new extension directory without polling | SATISFIED | `internal/watch/watcher.go` fsnotify event-driven loop; allow audit record written with `EDXT-02` rule ID. |
| EDXT-03 | 03-03 | Catalog hit triggers critical audit + notification + quarantine | SATISFIED | `internal/watch/handler.go` hit path; `TestHandleNewExtensionCatalogHit` asserts quarantine + sentry_alert. |
| EDXT-04 | 03-04 | `beekeeper scan` Bumblebee + Beekeeper-own scan | SATISFIED | `internal/scan/scanner.go`; both tests pass; no `--format ndjson` flag; graceful degradation. |
| EDXT-05 | 03-02, 03-04 | Quarantine list/restore/purge with path-traversal guard | SATISFIED | `internal/quarantine/quarantine.go`; `filepath.Base + strings.HasPrefix` guards; CLI in main.go. |
| EDXT-06 | 03-05, 03-04 | Editor detection + consent-gated auto-update disable + watch-dir registration | SATISFIED | `internal/editorinit/detect.go` + `settings.go`; `newInitCmd` consent prompts; idempotent. |

---

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | — | No TBD/FIXME/XXX markers found across all Phase 3 files | — | — |
| (none) | — | No stub implementations (empty returns, placeholder strings) found | — | — |

No debt markers or stubs detected. All implementations are substantive.

**Notable design note (not a blocker):** `internal/scan/scanner.go evaluateExtension` duplicates the multi-source adapter construction from `internal/watch/handler.go`. The duplication is minimal and documented in 03-04-SUMMARY as an intentional Phase 3 decision. The two paths serve different pipelines (scan vs. watch) and the duplication does not affect correctness.

---

## Human Verification Required

### 1. Live watcher behavior on actual filesystem events

**Test:** Start `beekeeper watch` with a real VS Code extensions directory, then install an extension via `code --install-extension ms-python.python`. Observe whether the new directory is detected within a few seconds.
**Expected:** Within ~500ms of the extension directory appearing, the handler fires; a message is printed or an audit record is written.
**Why human:** Requires a real editor installed, a running fsnotify watcher, and observing real-time filesystem event delivery — cannot verify programmatically without a live OS environment.

### 2. Desktop notification delivery

**Test:** Run `beekeeper watch` in a session with a display. Trigger a known-bad extension install (e.g., manually create a `nrwl.angular-console-18.95.0/package.json` in the watched dir). Observe whether a desktop notification appears.
**Expected:** A native desktop notification with "Beekeeper: extension quarantined" appears.
**Why human:** Notification delivery depends on the desktop session environment (dbus/libnotify/Windows toast). Cannot verify in a headless CI/sandbox.

### 3. Cross-device quarantine error surfacing

**Test:** With the quarantine directory on a different filesystem/volume from the extension directory, trigger a quarantine action. Verify the user receives a meaningful error.
**Expected:** An error message indicating cross-device quarantine is unsupported (not a silent failure).
**Why human:** Requires specific filesystem configuration (two separate volumes) to reproduce the cross-device rename error.

---

## Gaps Summary

No gaps identified. All five phase success criteria are substantively implemented:

1. **SC-1 (EDXT-01):** `extractExtensionInstall` in `internal/policy/engine.go` covers all four editor prefixes with case-insensitive matching; `extractAllExtensionInstalls` + `Evaluate` bulk path handles multi-flag commands with worst-decision-wins semantics. The `internal/policy` package maintains its purity guarantee (imports only `"strings"`). The EDXT-01 selftest fixture closes the end-to-end verification loop.

2. **SC-2 (EDXT-02):** `internal/watch/watcher.go` uses fsnotify OS-native events exclusively. Non-existent directories are retried on a 30-second ticker without blocking startup. The 500ms debounce coalesces burst events to a single handler invocation.

3. **SC-3 (EDXT-03):** `internal/watch/handler.go HandleNewExtension` implements the complete hit pipeline: `sentry_alert` audit record with `EDXT-03` rule ID, best-effort `notify.Notify`, and `quarantine.Move`. The integration test (`TestHandleNewExtensionCatalogHit`) exercises the full chain against a real temp-dir quarantine, real mmap index, and pre-seeded marketplace cache — no mocks of the security-critical path.

4. **SC-4 (EDXT-05):** `internal/quarantine/quarantine.go` provides all three operations with path-traversal guards. `cmd/beekeeper/main.go` wires `quarantine list/restore/purge` with EDXT-05 audit records and a `--yes`/interactive confirmation on purge.

5. **SC-5 (EDXT-06):** `internal/editorinit/detect.go` and `settings.go` provide injectable-testable editor detection and JSONC-safe idempotent settings patching. `newInitCmd` enforces explicit per-editor per-action consent (or `--yes` for non-interactive use). Phase 3 runtime directories (`quarantine/extensions`, `marketplace-cache`) are created by `beekeeper init`.

---

_Verified: 2026-05-26T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
