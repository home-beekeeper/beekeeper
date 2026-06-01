# SECURITY.md — Phase 3: Editor Extension Defense

**Generated:** 2026-05-26
**ASVS Level:** 2
**Auditor:** gsd-secure-phase

---

## Audit Result: SECURED

**Threats Closed:** 20/20
**Threats Open:** 0/20
**Unregistered Flags:** 0

---

## Threat Verification

### Plan 03-01 — Extension Install Recognition (internal/policy/engine.go)

| Threat ID | STRIDE | Disposition | Status | Evidence |
|-----------|--------|-------------|--------|----------|
| T-03-01-01 | Tampering | mitigate | CLOSED | `engine.go:266` — `normalize(name)` applied to parsed token; `strings.Index`/`strings.Fields` only (no regex); publisher.name never used in filesystem path construction in this file. Pattern matching via `editorInstallPatterns` slice with `strings.Index`. |
| T-03-01-02 | Spoofing | accept | CLOSED | Accepted risk documented in plan threat register: case-insensitive prefix match via `strings.ToLower` at `engine.go:249`; shell-level obfuscation out of scope for Layer 1 (Layer 2 file-watcher provides defense-in-depth). Rationale sound. |
| T-03-01-03 | Elevation | mitigate | CLOSED | `engine.go:3` — `import "strings"` is the only import. Grep for `os\.|net/|time\.|sync\.|context\.` on `internal/policy/engine.go` returns zero matches. Purity gate confirmed. |

### Plan 03-02 — Marketplace Adapter + Quarantine Manager

| Threat ID | STRIDE | Disposition | Status | Evidence |
|-----------|--------|-------------|--------|----------|
| T-03-02-01 | Tampering | mitigate | CLOSED | `quarantine.go:81-83` — `filepath.Base(m.Publisher)`, `filepath.Base(m.Name)`, `filepath.Base(m.Version)` applied before id composition. `quarantine.go:89-93` — `filepath.Clean + strings.HasPrefix(cleanDest, cleanExt+string(filepath.Separator))` prefix guard on `Move`. `quarantine.go:168` — `filepath.Base(id)` in `Restore`. Same prefix guard at `quarantine.go:173-177`. |
| T-03-02-02 | Tampering | mitigate | CLOSED | `marketplace.go:18-19` — HTTPS-only base URLs (`https://open-vsx.org/api`, `https://marketplace.visualstudio.com/...`). 24h TTL enforced via `ageCacheTTL` constant (reused from `age_cache.go`). Both-API-failure path at `marketplace.go:167-176` writes `Missing:true` and returns `(0, true, nil)` — fail-closed. |
| T-03-02-03 | Information Disclosure | mitigate | CLOSED | `quarantine.go:114` — `os.WriteFile(manifestPath, data, 0o600)`. `quarantine.go:119` — `platform.SetOwnerOnly(manifestPath)` enforces owner-only permissions. |
| T-03-02-04 | Denial of Service | mitigate | CLOSED | `marketplace.go:102` — `io.LimitReader(resp.Body, 4<<20)` on VS Code Marketplace POST path. Open VSX path uses `fetchRegistryJSON` which applies `io.LimitReader(resp.Body, 4<<20)` at `registry.go:66`. Both API paths capped at 4 MiB. |
| T-03-02-05 | Tampering | accept | CLOSED | Accepted risk documented in plan threat register and in code comment at `quarantine.go:70-73`: "Cross-device moves … are not supported in Phase 3; os.Rename returns a cross-device error which is propagated to the caller." Rationale sound — Move returns error, no silent partial move. |

### Plan 03-03 — File-Watcher Daemon + Extension Handler

| Threat ID | STRIDE | Disposition | Status | Evidence |
|-----------|--------|-------------|--------|----------|
| T-03-03-01 | Tampering | mitigate | CLOSED | `watcher.go:115-118` — `shouldProcess` returns `event.Has(fsnotify.Create)` only on `runtime.GOOS == "windows"`, filtering Write events. 500ms debounce via `time.AfterFunc` at `watcher.go:106`; burst coalescing via timer reset at `watcher.go:102-104`. |
| T-03-03-02 | Tampering | mitigate | CLOSED | `handler.go:32` — `WatchedRoots []string` field on `Handler`. `handler.go:65-75` — `filepath.Dir(filepath.Clean(path))` compared against each element of `WatchedRoots`; non-matching paths cause early `return` before any processing. |
| T-03-03-03 | Denial of Service | mitigate | CLOSED | Debounce in `watcher.go:100-108` coalesces bursts to one handler invocation per path per debounce window. `notify.Notify` at `notify.go:41` — `_ = notifyFunc(...)` swallows all errors; notification is best-effort and never queued. |
| T-03-03-04 | Tampering | mitigate | CLOSED | `manifest.go:64-65` — empty Publisher or Name returns `ErrNoManifest`. Path construction for quarantine delegated to `quarantine.Move` (T-03-02-01 guards). `handler.go:78-82` — `ErrNoManifest` causes silent return before any quarantine action. |
| T-03-03-05 | Information Disclosure | accept | CLOSED | Accepted risk documented in plan threat register. `notify.go:36-39` — Linux headless guard: early return when `DISPLAY==""` and `WAYLAND_DISPLAY==""`. `notify.go:41` — all errors swallowed. No security impact; quarantine proceeds regardless. Rationale sound. |
| T-03-03-06 | Repudiation | mitigate | CLOSED | `handler.go:166` — `audit.NewWriter` + `w.Write(rec)` executed BEFORE `notify.Notify` at line 176 and BEFORE `quarantine.Move` at line 190. Audit-write failure is logged (`log.Printf`) but does not block quarantine. Ordering requirement satisfied. |

### Plan 03-04 — Scan Orchestrator + CLI Wiring

| Threat ID | STRIDE | Disposition | Status | Evidence |
|-----------|--------|-------------|--------|----------|
| T-03-04-01 | Tampering | mitigate | CLOSED | `scanner.go:115-125` — each bumblebee stdout line is validated via `json.Unmarshal` into `json.RawMessage`; invalid lines produce a `scan_error` record and `continue` (skip), never crash. Unknown `record_type` values pass through unmodified at `scanner.go:127` but never control flow. |
| T-03-04-02 | Denial of Service | mitigate | CLOSED | `scanner.go:75` — `exec.CommandContext(ctx, bin, args...)` binds subprocess to the command context. `scanner.go:59-61` — `runBumblebeeFn` wraps `defaultRunBumblebee` which uses `exec.CommandContext`; ctx cancellation terminates subprocess. Buffered channel (cap 64) at `scanner.go:83`. |
| T-03-04-03 | Tampering | mitigate | CLOSED | `main.go:557` — CLI passes `args[0]` directly to `quarantine.Restore(qDir, args[0])`. `quarantine.go:168` — `Restore` applies `filepath.Base(id)` as first operation, stripping any path components. Prefix guard at `quarantine.go:173-177` provides secondary check. No path construction in `main.go`. |
| T-03-04-04 | Elevation | mitigate | CLOSED | `main.go:147-155` — per-editor consent prompt for auto-update disable; `main.go:166-174` — per-editor consent prompt for watch-dir registration. `--yes` flag at `main.go:192` is explicit opt-in. `--no-editors` at `main.go:193` preserves zero-touch behavior. `DisableExtensionAutoUpdate` only called after `consent == true` at `main.go:156`. Settings write is atomic (`PatchSettings` atomic rename) and idempotent. |
| T-03-04-05 | Repudiation | mitigate | CLOSED | `main.go:562-579` — restore emits `quarantine_restore` audit record with `RuleIDs: []string{"EDXT-05"}` before returning. `main.go:622-639` — purge emits one `quarantine_purge` audit record per purged ID with `RuleIDs: []string{"EDXT-05"}`. |
| T-03-04-06 | Tampering | mitigate | CLOSED | Grep for `"scan --format ndjson"` in `internal/scan/scanner.go` returns zero matches. `scanner.go:71-74` — bumblebee args are `["scan"]` plus optional `["--profile","deep"]`; forbidden flag absent. |

### Plan 03-05 — Editor Detection + Settings Patch + Selftest Fixture

| Threat ID | STRIDE | Disposition | Status | Evidence |
|-----------|--------|-------------|--------|----------|
| T-03-05-01 | Tampering | mitigate | CLOSED | `settings.go:61-65` — marshal fully into `out`, write to `tmpPath = path + ".tmp"` via `os.WriteFile`, then `os.Rename(tmpPath, path)`. Parse failure (`json.Unmarshal` returning error) causes `settings = nil` which is converted to empty map — no write is aborted by parse failure per se, but the marshal step at line 50 would surface encoding errors before the atomic write. Settings.json is fully marshaled before any file mutation. |
| T-03-05-02 | Information Disclosure | accept | CLOSED | Accepted risk documented in plan threat register. Detection is local-only: `detect.go:118-123` — `lookPath(alias)` (wraps `exec.LookPath`), `detect.go:127-129` — `statFunc(d.extensionDir)` (wraps `os.Stat`). Results stay in-process; no network exfiltration surface. Rationale sound. |
| T-03-05-03 | Tampering | mitigate | CLOSED | `settings.go:17-19` — doc comment records the known limitation: "existing // and /* */ comments in the file are removed on write." `main.go:151` — consent prompt text explicitly warns: "NOTE: comments in settings.json will be removed." User-visible warning present before any write. |
| T-03-05-04 | Spoofing | mitigate | CLOSED | `internal/check/corpus/fixtures.json:121-130` — EDXT-01 fixture present: `"command": "code --install-extension nrwl.angular-console@18.95.0"` with `expect_catalog_match: true`, `expect_level: "warn"`, `expect_rule_id: "bumblebee-catalog-match"`. Selftest corpus confirms `beekeeper selftest` would fail if `extractExtensionInstall` stopped routing to the catalog. |

---

## Accepted Risks Log

| Threat ID | Category | Rationale |
|-----------|----------|-----------|
| T-03-01-02 | Spoofing — command obfuscation bypass | Shell-level obfuscation (aliases, env-indirection) out of scope for Layer 1. Layer 2 (file-watcher) provides defense-in-depth. Risk accepted at design time. |
| T-03-02-05 | Tampering — cross-volume Windows rename | Cross-device quarantine unsupported in Phase 3. `os.Rename` returns a cross-device link error that is propagated; no silent partial move. Supported configuration: quarantine dir on same drive as %APPDATA%. |
| T-03-03-05 | Information Disclosure — headless notification error | Notification failure is always swallowed. Linux headless guard short-circuits before beeep call. No security impact; quarantine proceeds regardless of notification outcome. |
| T-03-05-02 | Information Disclosure — editor detection probing | Detection uses only local probes (`exec.LookPath` + `os.Stat`). Results never leave the process. Probe results inform consent prompt only. |

---

## Unregistered Flags

None. All threat flags from SUMMARY.md `## Threat Flags` sections map to existing threat register entries. No new unregistered attack surface was identified during this audit.

---

## Notes

- **T-03-05-01 partial caveat:** `PatchSettings` treats a corrupt/unmarshalable input file as an empty object (`settings = nil → make(map[string]any)`), silently discarding the existing content. This is consistent with the documented behavior ("treat ErrNotExist as existing=='{}'") but means a corrupt input file does not prevent a write. This is a documented design choice (fail-safe creation), not a security gap — the plan spec explicitly calls for this behavior. The atomic rename ensures no partial writes occur.

- **T-03-03-06 audit-before-quarantine ordering:** Verified by line-number comparison: audit write at handler.go:166, notification at 176, quarantine.Move at 190. The ordering invariant holds.

- **T-03-02-04 LimitReader coverage:** Both API paths confirmed capped — Open VSX via the shared `fetchRegistryJSON` helper (`registry.go:66`), VS Code Marketplace via explicit `io.LimitReader` at `marketplace.go:102`.
