---
phase: 7
slug: sensitive-path-runtime-enforcement
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-06-03
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `07-RESEARCH.md` → "## Validation Architecture". Planner refines per-task IDs.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (standard Go test discovery) |
| **Config file** | none — existing Go test infrastructure |
| **Quick run command** | `go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -run "TestEvaluatePath|TestCanonicalize|TestExtract|TestMergeDecisions|TestRunCheckCredential|TestRunCheckTraversal|TestRunCheckBash|TestRunCheckEnv|TestRunCheckWindows" -count=1` |
| **Full suite command** | `go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -count=1 -timeout=60s` |
| **Estimated runtime** | ~5–15 seconds (no network, no mmap cold-load in unit paths) |

---

## Sampling Rate

- **After every task commit:** Run quick run command
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Requirement → Test Map

*(Task IDs assigned by planner; rows are the behavior contract each task must satisfy.)*

| Requirement | Behavior | Test Type | Automated Command | File | Status |
|-------------|----------|-----------|-------------------|------|--------|
| SPATH-01 | `RunCheck` blocks `Read` with `file_path:"~/.aws/credentials"` (exit 1 + block) | integration | `go test ./internal/check/... -run TestRunCheckCredentialFileBlocks -count=1` | handler_test.go (W0 add) | ⬜ pending |
| SPATH-01 | `EvaluatePath` blocks `/.ssh/`, `/.aws/`, `/.cursor/`, `/.windsurf/` | unit | `go test ./internal/policy/... -run TestEvaluatePath -count=1` | path_test.go (exists + add) | ⬜ pending |
| SPATH-01 | `policyloader` overlay fires for `file_path` key (extractTargetPath fix) | unit | `go test ./internal/policyloader/... -run TestExtractTargetPathFilePath -count=1` | enforce_test.go (W0 add) | ⬜ pending |
| SPATH-02 | `RunCheck` blocks traversal `file_path:"../../.aws/credentials"` | integration | `go test ./internal/check/... -run TestRunCheckTraversalBlocks -count=1` | handler_test.go (W0 add) | ⬜ pending |
| SPATH-02 | `canonicalizePath` resolves `~`, `..`, `%USERPROFILE%`, slash-normalizes | unit | `go test ./internal/check/... -run TestCanonicalizePath -count=1` | paths_test.go (W0 new) | ⬜ pending |
| SPATH-02 | `RunCheck` blocks Windows form `C:\Users\u\.aws\credentials` | integration | `go test ./internal/check/... -run TestRunCheckWindowsCredentialBlocks -count=1` | handler_test.go (W0 add) | ⬜ pending |
| SPATH-03 | `RunCheck` blocks `Bash` + `cat ~/.ssh/id_rsa` (exit 1 + block + rule_id) | integration | `go test ./internal/check/... -run TestRunCheckBashCatCredentialBlocks -count=1` | handler_test.go (W0 add) | ⬜ pending |
| SPATH-03 | `RunCheck` blocks `Bash` + `type %USERPROFILE%\.ssh\id_rsa` (SC2 full, D-01) | integration | `go test ./internal/check/... -run TestRunCheckBashTypeUserProfileBlocks -count=1` | handler_test.go (W0 add) | ⬜ pending |
| SPATH-03 | `extractBashCredentialPaths` extracts path after read-verbs incl. env-var expansion | unit | `go test ./internal/check/... -run TestExtractBashCredentialPaths -count=1` | paths_test.go (W0 new) | ⬜ pending |
| SPATH-04 | `RunCheck` allows `.env.example` / `.env.test` / `.env.schema` (exit 0 + allow) | integration | `go test ./internal/check/... -run "TestRunCheckEnvExampleAllowed|TestRunCheckEnvTestAllowed|TestRunCheckEnvSchemaAllowed" -count=1` | handler_test.go (W0 add) | ⬜ pending |
| SPATH-04 | `RunCheck` still blocks `.env.production` (`.env.*` glob intact) | integration | `go test ./internal/check/... -run TestRunCheckEnvProductionBlocked -count=1` | handler_test.go (W0 add) | ⬜ pending |
| SPATH-04 | `EvaluatePath` allows `.env.example` via basename allowlist (isAllowedPath fix) | unit | `go test ./internal/policy/... -run TestEvaluatePathBasenameAllowlist -count=1` | path_test.go (W0 add) | ⬜ pending |
| (constraint) | `internal/policy/path.go` stays pure (no `os`/`io`/etc. imports) | unit | `go test ./internal/policy/... -run TestPathImportsArePure -count=1` | path_test.go (exists) | ⬜ pending |

---

## Observable Signals Per Success Criterion

| SC | Observable signal | How to assert |
|----|-------------------|---------------|
| SC1: `~/.aws/credentials` and `../../.aws/credentials` block | `res.ExitCode == exitBlock && res.Decision.Level == "block"` | `RunCheck` integration test |
| SC2: `Bash` `cat ~/.ssh/id_rsa` AND `type %USERPROFILE%\.ssh\id_rsa` detected | block + `rec.RuleIDs` contains `"sensitive-path-policy"` | `RunCheck` + `readLastAuditRecord` |
| SC3: `.env.example`/`.env.test`/`.env.schema` NOT blocked | `res.ExitCode == exitAllow && res.Decision.Level == "allow"` | `RunCheck` integration test |
| SC4: exit code + `decision:"block"` audit record (wiring is live) | `res.ExitCode == exitBlock && readLastAuditRecord(...).Decision == "block"` | `RunCheck` + `readLastAuditRecord` |

---

## Wave 0 Requirements

- [ ] `internal/check/paths_test.go` (NEW) — `TestExtractPathTargets`, `TestCanonicalizePath`, `TestExtractBashCredentialPaths`, `TestMergeDecisions`
- [ ] `internal/check/handler_test.go` (additions) — `TestRunCheckCredentialFileBlocks`, `TestRunCheckTraversalBlocks`, `TestRunCheckWindowsCredentialBlocks`, `TestRunCheckBashCatCredentialBlocks`, `TestRunCheckBashTypeUserProfileBlocks`, `TestRunCheckEnvExampleAllowed`, `TestRunCheckEnvTestAllowed`, `TestRunCheckEnvSchemaAllowed`, `TestRunCheckEnvProductionBlocked`
- [ ] `internal/policy/path_test.go` (additions) — `TestEvaluatePathBasenameAllowlist`, `TestEvaluatePathCursorMCPBlocked`, `TestEvaluatePathWindsurfMCPBlocked`
- [ ] `internal/policyloader/enforce_test.go` (additions) — `TestExtractTargetPathFilePath`

*Existing `path_test.go`, `handler_test.go`, `integration_test.go` provide all helpers (`buildTestIndex`, `closedConfig`, `auditPathIn`, `readLastAuditRecord`) — additions only, no new framework.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live binary block | SPATH-01 | Confidence smoke (covered automated in Phase 8 E2E gate) | `echo '{"tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}' \| beekeeper check; echo $?` → expect 1 |

*All Phase-7 success criteria have automated `RunCheck` verification; the manual check above is an optional smoke, redundant with the Phase 8 live-binary E2E release gate.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies (all 6 tasks across 07-01/02/03 carry `<automated>` verify; behavior specs serve as the Wave 0 contract for `tdd: true` tasks — confirmed by plan-checker)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (new test files listed above)
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-06-03
