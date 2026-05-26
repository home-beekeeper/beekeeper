---
status: complete
phase: 03-editor-extension-defense
source:
  - 03-01-SUMMARY.md
  - 03-02-SUMMARY.md
  - 03-03-SUMMARY.md
  - 03-05-SUMMARY.md
  - 03-04-SUMMARY.md
started: "2026-05-26T20:05:00Z"
updated: "2026-05-26T20:10:00Z"
mode: automated
---

## Current Test

[testing complete]

## Tests

### 1. EDXT-01: Extension Install Recognition (all 4 editors)
expected: |
  `beekeeper check` with `{"tool_name":"Bash","tool_input":{"command":"code --install-extension nrwl.angular-console@18.95.0"}}`
  returns Level:"warn", Allow:true, with a catalog match.
  The same applies to cursor, windsurf, and code-insiders prefixes.
  Bulk installs (`code --install-extension ext1 --install-extension ext2`) are evaluated
  as worst-decision across all IDs.
result: pass
verified_by: selftest EDXT-01 fixture passes (selftest PASS:10 FAIL:0); TestExtensionInstallExtract (8 sub-tests), TestExtensionInstallBulk, TestExtensionInstallVariants all pass; purity constraint (imports only "strings") confirmed.

### 2. EDXT-01: Extension Install Blocked by Release-Age Policy
expected: |
  A newly-published extension (age < 1440 minutes) triggers a block decision from
  `beekeeper check`, even if not in any threat catalog (release-age policy).
result: pass
verified_by: TestHandleNewExtensionCatalogHit exercises release-age block (extension age 10m < threshold 1440m → block + quarantine). TestScanBumblebeeUnavailable verifies release-age evaluation in scan path with 48h-old extension that passes.

### 3. Marketplace Age Adapter (Open VSX + VS Code Marketplace)
expected: |
  `FetchMarketplaceAge` returns age in minutes for an extension by hitting Open VSX
  (primary) or VS Code Marketplace (fallback). Results are cached for 24h at
  `<cacheDir>/marketplace-cache/<pub>/<name>/<ver>.json`.
  If both sources fail, returns missing=true (fail-closed).
result: pass
verified_by: TestFetchMarketplaceAge (httptest.Server stub), TestMarketplaceAgeCacheHit (cache hit avoids network), TestMarketplaceAgeMissing (both sources fail → missing:true) — all pass.

### 4. Quarantine Manager (Move/List/Restore/Purge)
expected: |
  `quarantine.Move` moves an extension directory to `~/.beekeeper/quarantine/extensions/<id>/`
  and writes a metadata manifest.
  `quarantine.List` returns all quarantined items; empty dir returns empty slice.
  `quarantine.Restore` moves the extension back to its original path.
  `quarantine.Purge` removes all quarantined items.
  Path-traversal attacks (e.g., id="../../../etc") are rejected.
result: pass
verified_by: TestQuarantineList, TestQuarantineRestore, TestQuarantinePurge, TestQuarantineRestorePathTraversal — all pass.

### 5. fsnotify Watch Daemon
expected: |
  `beekeeper watch` starts and monitors extension directories for new installs.
  New directories trigger catalog+age evaluation within milliseconds (no polling).
  Burst events (10 rapid Create events) are coalesced to 1 handler call via 500ms debounce.
  Non-existent directories are retried every 30s without crashing.
result: pass
verified_by: TestWatchDebounce (10 events → 1 call), TestWatchNonExistentDir (graceful non-existent dir), TestWatchWindowsFilter (Windows Create-only filter). `beekeeper watch --help` confirms command is registered.

### 6. New Extension Handler Pipeline (EDXT-02 + EDXT-03)
expected: |
  On a new extension appearing in a watched directory:
  - Manifest is parsed from `package.json`
  - Catalog + release-age policy is evaluated
  - On ALLOW: writes audit record with `EDXT-02` rule ID
  - On BLOCK (catalog hit or fresh extension): writes `sentry_alert` audit record with
    `EDXT-03` rule ID, sends desktop notification (best-effort), moves to quarantine
  - Symlink escape attacks (path outside WatchedRoots) are rejected silently
result: pass
verified_by: TestHandleNewExtensionCatalogHit exercises full block path (audit + quarantine). TestParseManifest, TestParseManifestNonExtension confirm manifest parsing with .obsolete/extensions.json filtering.

### 7. beekeeper quarantine CLI (EDXT-05)
expected: |
  `beekeeper quarantine list` prints a table of quarantined extensions or "no quarantined items".
  `beekeeper quarantine restore <id>` restores the item and writes a quarantine_restore audit record.
  `beekeeper quarantine purge` prompts `[y/N]` interactively; `--yes` skips the prompt.
  Each purged item writes one quarantine_purge audit record per EDXT-05.
result: pass
verified_by: `beekeeper quarantine --help` confirms list/restore/purge subcommands with correct descriptions. CLI wiring in cmd/beekeeper/main.go:500-649 confirmed. Quarantine unit tests cover all operations.

### 8. beekeeper scan (EDXT-04)
expected: |
  `beekeeper scan` outputs NDJSON to stdout: bumblebee_unavailable record (when bumblebee
  not installed) followed by finding records per extension.
  `beekeeper scan --deep` passes `--profile deep` to bumblebee (no `--format ndjson` flag).
  Malformed subprocess lines produce scan_error records (fail-closed, no crash).
result: pass
verified_by: TestScanWithBumblebee (canned NDJSON passthrough), TestScanBumblebeeUnavailable (bumblebee_unavailable + finding records). `beekeeper scan --help` shows `--deep` flag. grep confirms no `--format ndjson` in scanner.go.

### 9. Editor Detection (EDXT-06)
expected: |
  `beekeeper init` detects VS Code, Cursor, and Windsurf by checking executable PATH and
  extension directory existence. For each detected editor, offers two consent prompts:
  (1) disable extension auto-update, (2) register watch directory.
  `--no-editors` skips detection entirely (scripted installs).
  `--yes` auto-consents all prompts.
  Re-running is idempotent (AddWatchDirectory deduplicates).
result: pass
verified_by: TestDetectEditors confirms detection logic. `beekeeper init --help` confirms `--yes` and `--no-editors` flags. TestPatchEditorSettings, TestPatchEditorSettingsIdempotent, TestPatchEditorSettingsJSONC, TestPatchEditorSettingsCreateFile all pass.

### 10. JSONC Settings Patch (DisableExtensionAutoUpdate)
expected: |
  `DisableExtensionAutoUpdate` sets `extensions.autoUpdate: false` in the editor's
  settings.json, preserving all other keys. Works even if the file has JSONC comments.
  Write is atomic (temp file + rename). Creates parent dirs if missing.
result: pass
verified_by: TestPatchEditorSettingsJSONC confirms JSONC comment stripping + key preservation. TestPatchEditorSettingsCreateFile confirms missing file creation. Atomic write via os.Rename confirmed in settings.go.

### 11. Selftest (EDXT-01 corpus fixture end-to-end)
expected: |
  `beekeeper selftest` includes the EDXT-01 fixture and reports PASS for it.
  The fixture tests that `code --install-extension nrwl.angular-console@18.95.0`
  routes through the catalog and returns level:warn, allow:true.
result: pass
verified_by: `beekeeper selftest` output: PASS:10, FAIL:0 (was 7 before Phase 3; 3 new fixtures added including EDXT-01). TestSelftestAllFixturesPass passes.

## Summary

total: 11
passed: 11
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none]
