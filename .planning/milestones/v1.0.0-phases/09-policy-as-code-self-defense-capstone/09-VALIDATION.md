---
phase: 9
slug: policy-as-code-self-defense-capstone
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-05-29
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from 09-RESEARCH.md § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` + testify (existing project pattern) |
| **Config file** | none — `go test ./...` |
| **Quick run command** | `go test ./internal/policyloader/... ./internal/config/... ./internal/catalog/... ./internal/check/... -count=1` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | ~30–60 seconds (full suite, no `-race` on Windows dev) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/policyloader/... ./internal/config/... -count=1`
- **After every plan wave:** Run `go test ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~60 seconds

---

## Per-Task Verification Map

> Task IDs finalize when plans are written; mapping below is keyed by requirement and will be bound to task IDs during planning. Source: 09-RESEARCH.md test map.

| Requirement | Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|----------|-----------|-------------------|-------------|--------|
| CODE-01 | Policy file loads → engine inputs | unit | `go test ./internal/policyloader/... -run TestLoadPolicyFile` | ❌ W0 | ⬜ pending |
| CODE-01 | Adversarial policy (url/exec field) rejected | unit | `go test ./internal/policyloader/... -run TestValidateSchema_RejectsExec` | ❌ W0 | ⬜ pending |
| CODE-01 | Unknown `rule_type` fails validation | unit | `go test ./internal/policyloader/... -run TestValidateSchema_UnknownRuleType` | ❌ W0 | ⬜ pending |
| CODE-02 | `policy test` dry-run → expected block Decision | unit | `go test ./internal/policyloader/... -run TestPolicyTest_BlockRule` | ❌ W0 | ⬜ pending |
| CODE-02 | `policy test` allowlist override → allow | unit | `go test ./internal/policyloader/... -run TestPolicyTest_AllowlistOverride` | ❌ W0 | ⬜ pending |
| CODE-03 | `policy validate` exits non-zero + field errors on invalid | integration | `go test ./cmd/... -run TestPolicyValidateCmd_Invalid` | ❌ W0 | ⬜ pending |
| CODE-03 | `policy validate` exits 0 on valid | integration | `go test ./cmd/... -run TestPolicyValidateCmd_Valid` | ❌ W0 | ⬜ pending |
| CODE-04 | `policy list` correct rule counts per file | unit | `go test ./internal/policyloader/... -run TestListPolicyFiles` | ❌ W0 | ⬜ pending |
| CODE-04 | `policy list` empty (not error) when dir missing | unit | `go test ./internal/policyloader/... -run TestListPolicyFiles_MissingDir` | ❌ W0 | ⬜ pending |
| CODE-05 | user > system; project > user precedence | unit | `go test ./internal/config/... -run TestLoadLayered_PrecedenceOrder` | ❌ W0 | ⬜ pending |
| CODE-05 | `BEEKEEPER_*` env overrides JSON file | unit | `go test ./internal/config/... -run TestLoadLayered_EnvVarOverride` | ❌ W0 | ⬜ pending |
| CODE-05 | missing optional layers silently skipped | unit | `go test ./internal/config/... -run TestLoadLayered_MissingOptionalLayers` | ❌ W0 | ⬜ pending |
| CODE-05 | zero-value project field does NOT reset user field | unit | `go test ./internal/config/... -run TestMerge_ZeroValuePreservation` | ❌ W0 | ⬜ pending |
| CODE-06 | `diag` outputs all four sections | integration | `go test ./cmd/... -run TestDiagCmd_Output` | ❌ W0 | ⬜ pending |
| CODE-06 | hook latency p95/p99 accumulated | unit | `go test ./internal/check/... -run TestGlobalHookTracker` | ❌ W0 | ⬜ pending |
| CODE-06 | ETW EventsLost reports 0 on non-Windows | unit | `go test ./internal/check/... -run TestEventsLost_NonWindows` | ❌ W0 | ⬜ pending |
| CTLG-04/SFDF-06 | version match → self-quarantine | unit | `go test ./internal/catalog/... -run TestSelfCatalog_VersionMatch` | ❌ W0 | ⬜ pending |
| CTLG-04/SFDF-06 | invalid signature → fail closed | unit | `go test ./internal/catalog/... -run TestSelfCatalog_InvalidSignature` | ❌ W0 | ⬜ pending |
| CTLG-04/SFDF-06 | network error + no cache → warn, continue | unit | `go test ./internal/catalog/... -run TestSelfCatalog_NetworkError_NoCache` | ❌ W0 | ⬜ pending |
| CTLG-04/SFDF-06 | network error + fresh cache → use cache, continue | unit | `go test ./internal/catalog/... -run TestSelfCatalog_NetworkError_FreshCache` | ❌ W0 | ⬜ pending |
| CTLG-04/SFDF-06 | self-quarantine state persisted + read back | unit | `go test ./internal/catalog/... -run TestSelfQuarantineState_Persistence` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/policyloader/` package — does not exist yet (file I/O layer; keeps `internal/policy` pure)
- [ ] `internal/policyloader/testdata/` — policy fixtures: `valid_release_age.json`, `valid_allowlist.json`, `invalid_url_field.json`, `invalid_exec_action.json`, `invalid_unknown_rule_type.json`, `invalid_schema_version.json`
- [ ] `internal/catalog/selfcatalog.go` + `selfcatalog_test.go` + `testdata/selfcatalog_{match,no_match,invalid_sig}.json`
- [ ] `internal/check/diag.go` + `diag_windows.go` + `diag_other.go` (build-tagged EventsLost source)
- [ ] `internal/check` `GlobalHookTracker` + `LatencyTracker.P99()` (extend Phase 6 tracker)
- [ ] `internal/config/config_test.go` additions for `LoadLayered` precedence + zero-value-preservation matrix
- [ ] `docs/THREAT-MODEL.md` — does not exist yet

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `beekeeper-self` live at a real separate host with separate signing key | SFDF-06 | External hosting/ops infra is outside the build; code is tested against fixtures | After deploy, run `beekeeper catalogs sync` and confirm the self-catalog source appears in `beekeeper diag` catalog-freshness with a verified signature |
| Self-quarantine refuses to run + prints verification path | SFDF-06 | Requires the running binary to match a live compromised-version entry | Point self-catalog at a feed listing the running version; confirm `beekeeper check` refuses and prints the §12.7 verification path |
| Threat-model doc is published/linked from release announcement | success criterion 5 | Publication is a docs/process step | Confirm `docs/THREAT-MODEL.md` exists, covers corroboration-poisoning + fanotify mmap gap, links verification path |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
