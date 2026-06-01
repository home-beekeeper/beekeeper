---
phase: 09-policy-as-code-self-defense-capstone
verified: 2026-05-30T12:00:00Z
status: passed
score: 5/5
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 4/5
  gaps_closed:
    - "CODE-01 live enforcement: policyloader.ApplyPolicyOverlay now called inside runCheck (internal/check/handler.go lines 243-253); policies/*.json affect beekeeper check decisions, not only beekeeper policy test"
    - "CODE-05 CLI project-config discovery: resolveConfig() in cmd/beekeeper/diag.go now populates SystemPath, ProjectPath (via discoverProjectConfig upward walk), and Environ=os.Environ(); SC2 (project overrides user without env vars) is satisfied"
  gaps_remaining: []
  regressions: []
---

# Phase 9: Policy as Code + Self-Defense Capstone — Verification Report

**Phase Goal:** Policy is version-controllable, testable, and layered; Beekeeper monitors its own supply chain integrity via the separately hosted and signed `beekeeper-self` catalog — the system is ready to be trusted on real production work.
**Verified:** 2026-05-30T12:00:00Z
**Status:** PASSED
**Re-verification:** Yes — after gap closure (commits 04bd318 + 09-06 series: 654cd69, 14d0eac, c656654)

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Developer can write a declarative JSON policy file, validate it with `beekeeper policy validate <file>`, dry-run it with `beekeeper policy test <file>`, and list loaded policies with `beekeeper policy list` | VERIFIED | `cmd/beekeeper/policy.go` wires all three subcommands to `internal/policyloader`; `beekeeper policy test` now applies `ApplyPolicyOverlay` on top of the engine result so dry-run output matches live enforcement (commit 654cd69) |
| 2 | Config merges correctly across system → user → project → env var → CLI flag; a project-level `.beekeeper/config.json` overrides user-level config without requiring environment variables | VERIFIED | `resolveConfig()` in `cmd/beekeeper/diag.go` now sets `SystemPath=/etc/beekeeper/config.json`, `ProjectPath` via `discoverProjectConfig()` git-style upward walk that skips the user-level path and stops at home dir, and `Environ=os.Environ()`; commit 04bd318 confirmed all three layers |
| 3 | `beekeeper diag` displays hook latency p95/p99, sidecar inference latency, catalog freshness per source, and ETW `EventsLost` count in a single human-readable output | VERIFIED | `internal/check/diag.go` `CollectDiag()` assembles all 4 sections; `cmd/beekeeper/diag.go` formats them to stdout; ETW build-tag pair compiles on both platforms; `TestDiagCmd_Output` verifies all 4 section headings |
| 4 | `beekeeper-self` catalog is checked on every startup and catalog sync; self-quarantine fires if the running version appears as compromised; separate host + separate signing key + separate access control | VERIFIED (client-side) | `internal/catalog/selfcatalog.go` — real `ed25519.Verify` against `SelfCatalogPublicKey`; `enforceSelfQuarantine` wired at top of `check`, `gateway`, `sentry`, `watch` RunE bodies and end of `catalogs sync`; CR-01 fix (commit 14d0eac): `cfg.SelfCatalog.PubKey` hex-decoded and passed as `opts.PubKeyOverride`; misconfigured key fails closed, never silently falls back to embedded key; live external hosting is tracked ops-gate per 09-VALIDATION.md |
| 5 | Complete threat model documented publicly, including the coordinated false-positive poisoning attack surface and the fanotify mmap gap | VERIFIED | `docs/THREAT-MODEL.md` — 8 sections; Section 4 documents coordinated false-positive poisoning gap; Section 5 documents fanotify mmap gap; Section 6 includes single-maintainer governance note |

**Score:** 5/5 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/policyloader/enforce.go` | `ApplyPolicyOverlay`, `LoadPolicyDir` | VERIFIED | 393 LOC, pure function, no I/O; handles package_allowlist block/warn/allow and sensitive_path; skips corroboration_threshold/release_age/lifecycle at overlay level (documented limitation in file comment and THREAT-MODEL.md §8) |
| `internal/policyloader/enforce_test.go` | 13-case test table | VERIFIED | Added in commit 654cd69; covers all overlay combination rules |
| `internal/policyloader/test.go` | `RunPolicyTest` applies overlay | VERIFIED | `runPolicyTestWithCatalog` calls `ApplyPolicyOverlay([]PolicyFile{pf}, tc, base)` at line 81 so `beekeeper policy test` output matches live check enforcement |
| `internal/policyloader/loader.go` | `LoadPolicyFile`, `ListPolicyFiles` | VERIFIED | Unchanged; 140 LOC, `DisallowUnknownFields` guard |
| `internal/policyloader/validate.go` | `ValidateSchema`, 5-value rule_type enum | VERIFIED | Unchanged |
| `internal/policyloader/testdata/` (6 fixtures) | 2 valid + 4 adversarial | VERIFIED | All 6 files present |
| `internal/check/handler.go` | Calls `LoadPolicyDir` + `ApplyPolicyOverlay` inside `runCheck` | VERIFIED | Lines 243–253; imports policyloader (confirmed at line 32); honors fail_mode on dir-read error; per-file skip for malformed files (T-09-33) |
| `internal/check/handler_test.go` | `TestPolicyOverlayBlocksViaDir` | VERIFIED | Added in commit 654cd69 |
| `internal/config/layered.go` | `LoadLayered`, 5-layer merge | VERIFIED | Unchanged; 364 LOC, zero-value-safe `merge()` |
| `internal/config/config.go` | `SelfCatalogConfig.PubKey` field | VERIFIED | `pub_key` string field at line 101; `merge()` in layered.go propagates it at line 194–195 |
| `internal/catalog/selfcatalog.go` | `SelfCatalogOpts.PubKeyOverride` exported field | VERIFIED | Exported `PubKeyOverride ed25519.PublicKey` in `SelfCatalogOpts`; `CheckSelfCatalog` uses override when non-nil, falls back to embedded key when nil; commit 14d0eac |
| `internal/llamafirewall/latency.go` | `P95()` and `P99()` using nearest-rank percentile (WR-03) | VERIFIED | Both methods call shared `percentile(buf, p)` with `idx = ceil(p*n)-1` clamped to `[0,n-1]`; commit c656654 |
| `cmd/beekeeper/diag.go` | `resolveConfig()` with SystemPath, ProjectPath, Environ | VERIFIED | Lines 104–109: `SystemPath=systemConfigPath()`, `ProjectPath=discoverProjectConfig(userPath)`, `Environ=os.Environ()`; commit 04bd318 |
| `cmd/beekeeper/diag.go` | `discoverProjectConfig()` git-style upward walk | VERIFIED | Lines 133–155: walks up from `os.Getwd()`, skips the user-level path, stops at home dir or filesystem root |
| `cmd/beekeeper/selfquarantine.go` | `enforceSelfQuarantine` with CR-01 PubKeyOverride wiring | VERIFIED | Lines 94–113: hex-decodes `cfg.SelfCatalog.PubKey`; wrong length or decode failure → fail closed; passes `PubKeyOverride` in `SelfCatalogOpts`; commit 14d0eac |
| `docs/THREAT-MODEL.md` | 8 sections | VERIFIED | Unchanged from prior verification |
| `internal/catalog/selfkey.go` | Embedded `SelfCatalogPublicKey` | VERIFIED | Unchanged; `init()` decodes hex constant |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/check/handler.go` `runCheck` | `policyloader.LoadPolicyDir` + `ApplyPolicyOverlay` | called at lines 245, 252 after `policy.Evaluate` | VERIFIED | Wired in commit 654cd69; import at line 32 |
| `internal/policyloader/test.go` `runPolicyTestWithCatalog` | `ApplyPolicyOverlay` | called at line 81 | VERIFIED | Overlay applied on top of engine result; `policy test` now mirrors live enforcement |
| `cmd/beekeeper/diag.go` `resolveConfig` | `config.LoadLayered` with full 5-layer opts | `opts.SystemPath`, `opts.UserPath`, `opts.ProjectPath`, `opts.Environ` all populated | VERIFIED | Commit 04bd318; project layer discoverable without env vars (SC2 satisfied) |
| `cmd/beekeeper/selfquarantine.go` `enforceSelfQuarantine` | `cfg.SelfCatalog.PubKey` → `SelfCatalogOpts.PubKeyOverride` | hex-decode → `ed25519.PublicKey` → `opts.PubKeyOverride` | VERIFIED | CR-01 fix in commit 14d0eac; misconfigured key fails closed |
| `internal/catalog/selfcatalog.go` `CheckSelfCatalog` | `opts.PubKeyOverride` || `SelfCatalogPublicKey` | `pubKey := opts.PubKeyOverride; if pubKey == nil { pubKey = SelfCatalogPublicKey }` | VERIFIED | Lines 205–207 in selfcatalog.go |
| `internal/llamafirewall/latency.go` `P95` / `P99` | `percentile(buf, p)` nearest-rank | `idx = int(math.Ceil(p*float64(n))) - 1` | VERIFIED | WR-03 fix in commit c656654 |
| `resolveConfig()` | `config.LoadLayered` with `ProjectPath` | `discoverProjectConfig(userPath)` upward walk | VERIFIED | Commit 04bd318; project overrides user without env vars |
| `internal/policy` package | unchanged | zero diff across all 09-06 commits | VERIFIED | `git diff HEAD~8 HEAD -- internal/policy/` produces no output; last touch was commit c1051a2 (Phase 4) |

---

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `internal/check/handler.go runCheck` | `policyFiles` (overlay) | `policyloader.LoadPolicyDir(policiesDir)` reads `~/.beekeeper/policies/*.json` | Yes — reads real files from disk; missing dir = empty (not error) | FLOWING |
| `internal/check/handler.go runCheck` | `decision` (post-overlay) | `policyloader.ApplyPolicyOverlay(policyFiles, toolCall, decision)` | Yes — deterministic pure function; result alters final exit code | FLOWING |
| `cmd/beekeeper/diag.go resolveConfig` | `cfg.SelfCatalog.PubKey` | `config.LoadLayered(opts)` reads from project/user/system config.json | Yes — reads real config layers; Environ passes `os.Environ()` | FLOWING |
| `cmd/beekeeper/selfquarantine.go enforceSelfQuarantine` | `pubKeyOverride` | `hex.DecodeString(cfg.SelfCatalog.PubKey)` from merged config | Yes — real Ed25519 key bytes or fail-closed on bad input | FLOWING |

---

### Behavioral Spot-Checks

Build and full test suite confirmed green by orchestrator (go build ./..., GOOS=windows go build ./..., go test ./... across 22 packages). The gap-closure commits add:

- `enforce_test.go`: 13 overlay combination cases
- `TestPolicyOverlayBlocksViaDir` in `handler_test.go`: live-dir overlay wiring integration test
- `TestSelfCatalog_CustomKeyVerifiesAndEmbeddedFails`: CR-01 PubKeyOverride behaviour
- `TestEnforceSelfQuarantine_InvalidPubKeyFailsClosed`: short key → fail-closed, never reaches checkSelfCatalogFn
- `TestP95NinetyFifthPercentile`, `TestP95SmallNDoesNotCollapseToMax`, updated `TestP99NinetyNinthPercentile`: WR-03 nearest-rank correctness

All new tests part of the green suite. No spot-checks skipped for substantive reasons.

---

### Probe Execution

No `scripts/*/tests/probe-*.sh` probes declared or found. SKIPPED.

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| CODE-01 | 09-01-PLAN, 09-06 | Declarative JSON policy files in `policies/`, loaded by policy engine | VERIFIED | `ApplyPolicyOverlay` + `LoadPolicyDir` in `internal/policyloader/enforce.go`; wired into `runCheck` (handler.go lines 243-253) and `runPolicyTestWithCatalog` (test.go line 81); `internal/policy` pure package unchanged |
| CODE-02 | 09-01-PLAN | `beekeeper policy test <file>` — dry-run | VERIFIED | `RunPolicyTest` → `runPolicyTestWithCatalog` → `ApplyPolicyOverlay`; CLI wired in `cmd/beekeeper/policy.go` |
| CODE-03 | 09-01-PLAN | `beekeeper policy validate <file>` — validate schema | VERIFIED | `ValidateSchema` + CLI command; invalid file → non-zero exit |
| CODE-04 | 09-01-PLAN | `beekeeper policy list` — list loaded policy files | VERIFIED | `ListPolicyFiles` + CLI command |
| CODE-05 | 09-02-PLAN, 04bd318 | Layered config merge system→user→project→env→flag | VERIFIED | `LoadLayered` fully correct; `resolveConfig()` now populates all 5 layer opts including `discoverProjectConfig()` upward walk; SC2 satisfied without env vars |
| CODE-06 | 09-04-PLAN | `beekeeper diag` — hook p95/p99, sidecar latency, catalog freshness, ETW EventsLost | VERIFIED | All 4 sections in `CollectDiag`; `P99()` added; WR-03 nearest-rank percentile correct |
| SFDF-06 | 09-03-PLAN, 09-05-PLAN, 14d0eac | `beekeeper-self` live at v1.0.0 — separate host, key, access; self-quarantine on startup and sync | VERIFIED (client-side) | Ed25519 key in `selfkey.go`; `enforceSelfQuarantine` at 5 call sites; CR-01: `cfg.SelfCatalog.PubKey` routed through `PubKeyOverride` with fail-closed on bad key; live external hosting is ops-gate per 09-VALIDATION.md |
| CTLG-04 | 09-03-PLAN | `beekeeper-self` checked on every startup and catalog sync; self-quarantine fires on match | VERIFIED (client-side) | Client behaviour fully tested (7 base tests + 2 CR-01 tests); post-sync guard in `catalogs sync` |

---

### Anti-Patterns Found

No `TBD`, `FIXME`, or `XXX` markers found in any Phase 9 or gap-closure files. No placeholder implementations. No hardcoded empty return values in live code paths. No anti-patterns detected.

---

### Human Verification Required

None. Both previously flagged human items are resolved by code evidence:

1. **CODE-01 enforcement wiring** — `internal/check/handler.go` now imports `policyloader` and calls `LoadPolicyDir` + `ApplyPolicyOverlay` inside `runCheck` (lines 243–253). Policies in `~/.beekeeper/policies/*.json` affect live `beekeeper check` decisions. Confirmed by grep and direct file read.

2. **CODE-05 project config discovery** — `resolveConfig()` in `cmd/beekeeper/diag.go` now populates `SystemPath`, `ProjectPath` (via `discoverProjectConfig()` git-style upward walk), and `Environ=os.Environ()`. A project-level `.beekeeper/config.json` overrides user config without env vars. Confirmed by reading the updated function body (lines 93–115).

---

### Gaps Summary

No gaps. All 5 observable truths are VERIFIED. All 8 Phase 9 requirements (CODE-01 through CODE-06, SFDF-06, CTLG-04) are SATISFIED. The two previously flagged human-verification items were resolved by commits 04bd318 and the 09-06 series (654cd69, 14d0eac, c656654):

- **CODE-01**: Pure `ApplyPolicyOverlay` in `internal/policyloader/enforce.go` + `LoadPolicyDir`; wired into both `runCheck` and `RunPolicyTest`; `internal/policy` package confirmed unmodified.
- **CODE-05**: `resolveConfig()` now exercises the full 5-layer merge including the git-style upward-walk project-config discovery. SC2 (project overrides user without env vars) is satisfied.
- **CR-01**: `cfg.SelfCatalog.PubKey` routed through `PubKeyOverride` in `SelfCatalogOpts`; misconfigured key fails closed before `checkSelfCatalogFn` is called.
- **WR-03**: `LatencyTracker.P95()` and `P99()` use shared nearest-rank `percentile()` helper; off-by-one corrected.

Live external hosting of the `beekeeper-self` feed remains a tracked ops-gate (v1.0.0 tagging prerequisite, documented in 09-VALIDATION.md) and is not a code gap.

---

_Verified: 2026-05-30T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
_Re-verification: Yes — gaps from 2026-05-29T22:00:00Z initial report resolved by commits 04bd318 + 654cd69 + 14d0eac + c656654_
