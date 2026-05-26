---
phase: 3
slug: editor-extension-defense
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-26
---

# Phase 3 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` package + `go test` |
| **Config file** | None — `go test ./...` discovers automatically |
| **Quick run command** | `go test ./internal/... -count=1` |
| **Full suite command** | `go test -race -count=1 ./...` (CI-only; requires CGO) |
| **Estimated runtime** | ~15–30 seconds (quick), ~60–90s (full, CI) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/... -count=1`
- **After every plan wave:** Run `go test ./internal/... -count=1` (full suite is CI-only)
- **Before `/gsd-verify-work`:** Full test suite green on all 3 platforms
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| extension-install-extract | policy | 1 | EDXT-01 | — | `code --install-extension a.b@v` → ecosystem="editor-extension", pkg="a.b" | unit | `go test ./internal/policy/... -run TestExtensionInstallExtract -v` | ❌ W0 | ⬜ pending |
| extension-install-bulk | policy | 1 | EDXT-01 | — | Bulk multi-flag form: all IDs extracted | unit | `go test ./internal/policy/... -run TestExtensionInstallBulk -v` | ❌ W0 | ⬜ pending |
| extension-install-variants | policy | 1 | EDXT-01 | — | cursor/windsurf/code-insiders recognized | unit | `go test ./internal/policy/... -run TestExtensionInstallVariants -v` | ❌ W0 | ⬜ pending |
| watch-nonexistent-dir | watch | 2 | EDXT-02 | — | Daemon starts; skips non-existent dir; no crash | unit | `go test ./internal/watch/... -run TestWatchNonExistentDir -v` | ❌ W0 | ⬜ pending |
| watch-create-event | watch | 2 | EDXT-02 | — | Create event on child dir → handleNewExtension called | unit | `go test ./internal/watch/... -run TestWatchCreateEvent -v` | ❌ W0 | ⬜ pending |
| watch-windows-filter | watch | 2 | EDXT-02 | T-windows-write | Windows Write events on watched dir filtered | unit | `go test ./internal/watch/... -run TestWatchWindowsFilter -v` | ❌ W0 | ⬜ pending |
| watch-debounce | watch | 2 | EDXT-02 | — | Burst of 10 Create events → one callback | unit | `go test ./internal/watch/... -run TestWatchDebounce -v` | ❌ W0 | ⬜ pending |
| parse-manifest | watch | 2 | EDXT-03 | — | Valid package.json → publisher/name/version parsed | unit | `go test ./internal/watch/... -run TestParseManifest -v` | ❌ W0 | ⬜ pending |
| parse-manifest-invalid | watch | 2 | EDXT-03 | T-path-traversal | .obsolete/extensions.json → ErrNoManifest | unit | `go test ./internal/watch/... -run TestParseManifestNonExtension -v` | ❌ W0 | ⬜ pending |
| marketplace-age-fetch | catalog | 2 | EDXT-03 | — | Open VSX timestamp → correct ageMinutes | unit (httptest) | `go test ./internal/catalog/... -run TestFetchMarketplaceAge -v` | ❌ W0 | ⬜ pending |
| marketplace-age-cache | catalog | 2 | EDXT-03 | — | 24h cache hit → no HTTP request | unit | `go test ./internal/catalog/... -run TestMarketplaceAgeCacheHit -v` | ❌ W0 | ⬜ pending |
| marketplace-missing-failclosed | catalog | 2 | EDXT-03 | T-timestamp-mitm | Both APIs fail → missing=true → block | unit | `go test ./internal/catalog/... -run TestMarketplaceAgeMissing -v` | ❌ W0 | ⬜ pending |
| catalog-hit-quarantine | watch | 2 | EDXT-03 | — | Catalog hit → quarantine move + audit record | integration | `go test ./internal/watch/... -run TestHandleNewExtensionCatalogHit -v` | ❌ W0 | ⬜ pending |
| scan-with-bumblebee | scan | 3 | EDXT-04 | — | Bumblebee in PATH → subprocess runs; NDJSON merged | integration | `go test ./internal/scan/... -run TestScanWithBumblebee -v` | ❌ W0 | ⬜ pending |
| scan-bumblebee-unavailable | scan | 3 | EDXT-04 | — | Bumblebee not in PATH → Beekeeper-only scan; logs bumblebee_unavailable:true | unit | `go test ./internal/scan/... -run TestScanBumblebeeUnavailable -v` | ❌ W0 | ⬜ pending |
| quarantine-list | quarantine | 3 | EDXT-05 | — | Reads all beekeeper-manifest.json; prints table | unit | `go test ./internal/quarantine/... -run TestQuarantineList -v` | ❌ W0 | ⬜ pending |
| quarantine-restore | quarantine | 3 | EDXT-05 | — | Restore moves dir back to original_path | unit | `go test ./internal/quarantine/... -run TestQuarantineRestore -v` | ❌ W0 | ⬜ pending |
| quarantine-purge | quarantine | 3 | EDXT-05 | — | Purge --yes removes all items; audit records emitted | unit | `go test ./internal/quarantine/... -run TestQuarantinePurge -v` | ❌ W0 | ⬜ pending |
| init-patch-settings | init | 4 | EDXT-06 | — | `extensions.autoUpdate:false` written; other keys preserved | unit | `go test ./... -run TestPatchEditorSettings -v` | ❌ W0 | ⬜ pending |
| init-patch-idempotent | init | 4 | EDXT-06 | — | Re-running patch does not duplicate key | unit | `go test ./... -run TestPatchEditorSettingsIdempotent -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/policy/engine_test.go` — extend: TestExtensionInstallExtract, TestExtensionInstallBulk, TestExtensionInstallVariants
- [ ] `internal/watch/watcher_test.go` — TestWatchNonExistentDir, TestWatchCreateEvent, TestWatchWindowsFilter, TestWatchDebounce
- [ ] `internal/watch/manifest_test.go` — TestParseManifest, TestParseManifestNonExtension
- [ ] `internal/watch/handler_test.go` — TestHandleNewExtensionCatalogHit (integration with tempdir)
- [ ] `internal/catalog/marketplace_test.go` — TestFetchMarketplaceAge, TestMarketplaceAgeCacheHit, TestMarketplaceAgeMissing (httptest.Server stubs)
- [ ] `internal/scan/scanner_test.go` — TestScanWithBumblebee, TestScanBumblebeeUnavailable
- [ ] `internal/quarantine/quarantine_test.go` — TestQuarantineList, TestQuarantineRestore, TestQuarantinePurge
- [ ] `internal/watch/testdata/valid-extension/package.json` — synthetic fixture (publisher, name, version fields)
- [ ] `internal/watch/testdata/malicious-extension/package.json` — synthetic fixture (matches adversarial corpus pattern)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| fsnotify actually fires on new extension directory (live install) | EDXT-02 | Requires a real VS Code/Cursor install + real extension install | Start `beekeeper watch`; install an extension via GUI; verify audit record emitted |
| Desktop notification appears on extension hit | EDXT-03 | Requires a display server; headless CI cannot verify | On Linux desktop: trigger a release-age block; verify toast notification appears |
| `beekeeper init` writes `extensions.autoUpdate:false` to real VS Code settings.json | EDXT-06 | Requires VS Code installed with real settings.json | Run `beekeeper init`; consent to disable auto-update; verify `~/.config/Code/User/settings.json` contains key |
| Cursor Windows extension path discovery | EDXT-02 (open question) | Cursor Windows path is LOW confidence; needs empirical validation | On Windows+Cursor: verify `%USERPROFILE%\.cursor\extensions\` or `%APPDATA%\Cursor\extensions\` is correct |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
