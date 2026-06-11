---
phase: 21
slug: full-system-validation-ci-calibration
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-11
---

# Phase 21 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> **This phase IS the validation phase — "the gate tests the gate."** Each deliverable validates itself; this doc records how. Derived from `21-RESEARCH.md` §Validation Architecture (HIGH confidence, code-grounded at `04857cc`).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + native fuzzing (`testing.F`), Go 1.25 |
| **Config file** | none (Go convention); CI in `.github/workflows/ci.yml` |
| **Quick run command** | `go test ./...` (Windows dev box — Tier A) |
| **Full suite command** | `go test -race ./...` + `go test -tags fuzz -fuzz ...` + `go test -tags e2e ...` (CI matrix — Tier B / e2e) |
| **Estimated runtime** | ~30–90s Tier-A locally; matrix is CI-bound |

---

## Sampling Rate

- **After every task commit:** Run `go test ./...` + `go vet ./...` (Windows-local Tier A)
- **After every plan wave:** Full Tier-A suite green on Windows + `go test -tags fuzz` smoke (`-fuzztime 5s`) for any new fuzz target
- **Before `/gsd-verify-work`:** Tier-A suite green; the e2e Claude Code `--hook` exit-2 case green locally (`-tags e2e`); the CI matrix YAML statically authored + locally build-verified
- **Max feedback latency:** ~90 seconds (Tier A). **Tier B (eBPF load, `-race`/CGO, eslogger, ETW, peer-cred) and the live GitHub matrix run are CI-only / post-push** — honest per D-05 / Phase-19 precedent: the repo has never been pushed, so the matrix gets its green badge at first push, not during this phase.

---

## Per-Task Verification Map

> Task IDs are placeholders until the planner assigns them; rows are keyed by requirement + expected wave. `❌ W0` = Wave-0 dependency (file does not yet exist).

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 21-01-* | 01 | 1 | VAL-01 | T-21 silent-coverage-erosion | Every prod file linked-tested or reason-coded allowlisted; gate fails on an unaccounted file | unit (gate) | `go test -run TestCoverageManifest ./internal/coveragegate/` | ❌ W0 | ⬜ pending |
| 21-01-* | 01 | 1 | VAL-08 | T-21 tampering | Allowlist parser rejects bare-path + unknown-reason-code lines | unit (meta) | `go test -run TestAllowlistFailsClosed ./internal/coveragegate/` | ❌ W0 | ⬜ pending |
| 21-01-* | 01 | 1 | VAL-01 | — | `catalog.ResolveHealthy` fail-open-on-read paths (covers the 4 `sanity.go` delegators) | unit | `go test ./internal/catalog/` | ❌ W0 | ⬜ pending |
| 21-01-* | 01 | 1 | VAL-01 | T-21 peer-spoof | IPC server 0600 owner socket + unauth peer closed pre-handler; client deadline path | unit (Tier-B `linux\|\|darwin`) | `go test ./internal/ipc/` (CI Linux/macOS legs) | ❌ W0 | ⬜ pending |
| 21-01-* | 01 | 1 | VAL-01 | — | `hooks/protected.go` markers + home-rooted config paths; `tui/model.go` `computeStatus`/`recentAuditRecords` | unit | `go test ./internal/hooks/ ./internal/tui/` | ⚠ partial | ⬜ pending |
| 21-02-* | 02 | 2 | VAL-02 | T-21 fail-open-deny | 17-harness deny contract byte-exact (incl. Hermes exit-0 + Kilo/Trae UNGUARDED) | unit (golden) | `go test -run TestRenderDeny ./internal/check/` | ✅ convert→golden | ⬜ pending |
| 21-02-* | 02 | 2 | VAL-02 | — | Installer config keys/idempotent/backup-on-overwrite for all 17 targets | unit | `go test -run TestInstall ./internal/hooks/` | ✅ fill to 17 | ⬜ pending |
| 21-03-* | 03 | 3 | VAL-04 | T-21 parser-panic | `EvaluateEvent` never panics; only `critical`/`high` severities | fuzz | `go test -tags fuzz -fuzz FuzzEvaluateEvent ./internal/sentry/` | ❌ W0 | ⬜ pending |
| 21-03-* | 03 | 3 | VAL-03 | T-21 config/build | Matrix: build native+3×GOOS, vet, test, `-race`, eBPF gen+load, eslogger, ETW, peer-cred | CI matrix | (CI only — statically authored + build-verified) | ✅ ci.yml extend | ⬜ pending |
| 21-04-* | 04 | 4 | VAL-05 | T-21 malicious-code | `beekeeper check --hook claude-code` canary `~/.ssh`+`~/.aws` read → **exit 2** + Family-A deny JSON + audit `block` | e2e | `go test -tags e2e -run TestE2E... ./internal/check/` | ⚠ exit-2 case new | ⬜ pending |
| 21-04-* | 04 | 4 | VAL-06 | — | `docs/validation-register.md`: 16 harness procedures + gated-model entry, each with sign-off | manual/doc | (human register; presence grep) | ❌ W0 | ⬜ pending |
| 21-04-* | 04 | 4 | VAL-07 | — | README harness count 15→17 / 14→16; Tier A/B/C posture + allowlist mechanism documented | doc | `grep -c "17"` README + posture section present | ⚠ README edit | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/coveragegate/` package — prod-file walker + reason-coded allowlist parser + `coverage-allowlist.txt` + `TestCoverageManifest` + `TestAllowlistFailsClosed` (VAL-01 / VAL-08)
- [ ] `internal/catalog/health_test.go` — covers `ResolveHealthy` (the 4 `sanity.go` delegators' real logic) (VAL-01)
- [ ] `internal/ipc/server_test.go` + `client_test.go` + `peer_linux_test.go`/`peer_darwin_test.go` (`//go:build linux||darwin`, Tier-B) (VAL-01)
- [ ] `internal/hooks/protected_test.go`; `internal/tui` function-level model tests (VAL-01)
- [ ] `internal/check/deny_render_test.go` golden conversion + `internal/check/testdata/deny/*.golden` + `-update` flag (VAL-02)
- [ ] `internal/sentry/fuzz_test.go` `FuzzEvaluateEvent` (`//go:build fuzz`) (VAL-04)
- [ ] `internal/check/e2e_test.go` `--hook claude-code` exit-2 canary case (`//go:build e2e`) (VAL-05)
- [ ] `docs/validation-register.md`; README count fix; validation-posture section (VAL-06/07)
- [ ] `.github/workflows/ci.yml` matrix extensions: 3×GOOS cross-build step, `fuzz-sentry` job → `release-gate.needs`, eBPF load smoke (VAL-03/04)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live block on the 16 non-Claude-Code harnesses | VAL-06 | Each needs its real client installed + driven; cannot be automated without the third-party harness | Follow each `docs/validation-register.md` row: install harness, `beekeeper hooks install --target <name>`, drive the canary read, confirm the expected exit code + deny family, sign off |
| LlamaFirewall gated-22M-model e2e | VAL-06 | Blocked on a human HF-license web action (accept Llama-Prompt-Guard-2 license + `huggingface-cli login`) | Accept license; `beekeeper llamafirewall install`; `BEEKEEPER_LLMF_E2E=1 go test -tags e2e -run TestLlamaFirewallE2E ./internal/llamafirewall/`; sign-off may stay PENDING past phase close (honest — D-07) |
| Live GitHub CI matrix green badge | VAL-03 | The repo has never been pushed; the matrix YAML is statically authored + locally build-verified, but only runs live after `git push` | Confirmed at first push (part of the deferred v1.1.0 release runbook); Phase 21 verifies YAML validity + build, not a live run |

---

## Validation Sign-Off

- [ ] All tasks have an `<automated>` verify or a Wave-0 dependency
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s (Tier A); Tier-B/e2e documented as CI-only
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending (set on plan-check pass)
