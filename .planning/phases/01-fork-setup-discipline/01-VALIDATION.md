---
phase: 1
slug: fork-setup-discipline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-01
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> **All commands run inside the Pollen repo** (`C:/Users/Bantu/mzansi-agentive/pollen`, i.e. `../pollen` from beekeeper), which Wave 0 creates. Pollen is pure stdlib (zero external deps at the pinned upstream SHA).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (no test-framework dependency) |
| **Config file** | `go.mod` (`go test` is the runner) — created Wave 0 |
| **Quick run command** | `go build -trimpath -buildvcs=false ./cmd/pollen && ./pollen selftest` |
| **Full suite command** | `go test -race -count=1 ./...` |
| **Estimated runtime** | ~30–60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go build ./cmd/pollen && ./pollen selftest` (in `../pollen`)
- **After every plan wave:** Run `go test -race -count=1 ./...` on Linux
- **Before `/gsd-verify-work`:** Full CI matrix green (ubuntu/macos/windows) **+** differential test green on Linux+macOS **+** `v0.1.1-pollen.1` tag signed
- **Max feedback latency:** ~60 seconds (local build+selftest)

---

## Per-Task Verification Map

> Task IDs are assigned by the planner (step 8). Rows below are the requirement-level verification contract every task must roll up to. `record_id` is upstream's stable SHA-256 content key — the safe sort anchor for the differential harness.

| Task (planner-assigned) | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---|---|---|---|---|---|---|---|---|
| TBD | 0 | FORK-01 | — | Forked at pinned SHA, no drift | build smoke | `go build -trimpath -buildvcs=false ./cmd/pollen` | ❌ W0 | ⬜ pending |
| TBD | — | FORK-01 | — | Cross-compiles to Windows (no compile-fail on `syscall`) | build | `GOOS=windows go build ./...` | ❌ W0 | ⬜ pending |
| TBD | — | FORK-02 | T-supply-chain | Pinned commit recorded + verifiable | grep audit | `grep c24089804ee66ece4bec6f14638cb98985389cdb UPSTREAM.md` | ❌ W0 | ⬜ pending |
| TBD | — | FORK-03 | T-build-integrity | Two builds hash-identical | repro verify | `make verify-release VERSION=0.1.1-pollen.1` (CI) | ❌ W0 | ⬜ pending |
| TBD | — | FORK-04 | T-trademark | "bumblebee" only in attribution | grep audit | `grep -ri "bumblebee" cmd/ go.mod --include="*.go" \| grep -v "NOTICE\|UPSTREAM\|attribution"` returns empty | ❌ W0 | ⬜ pending |
| TBD | — | PTEST-02 | — | pollen == upstream NDJSON on Linux+macOS (after normalization) | differential | `go test ./cmd/pollen/ -run TestDifferential -v` | ❌ W0 | ⬜ pending |
| TBD | — | PTEST-03 | — | selftest emits exactly 3 findings, exit 0, on all 3 OSes | selftest | `./pollen selftest` (exit 0) | ❌ W0 | ⬜ pending |
| TBD | — | PTEST-03 | — | Upstream inherited Go tests pass unchanged on Linux/macOS | unit | `go test -race -count=1 ./...` | ❌ W0 | ⬜ pending |
| TBD | — | SDEF-02 | T-supply-chain | CycloneDX SBOM in release artifacts | release CI | GoReleaser syft stanza produces `*.cdx.json` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Differential-test normalization contract (PTEST-02 — load-bearing):** raw NDJSON comparison ALWAYS fails. The harness MUST strip the 4 per-scan non-deterministic fields (`run_id`, `scan_time`, `end_time`, `duration_ms`), the 3 per-machine endpoint fields (`hostname`, `username`, `uid`), and sort records by `record_id` (stable SHA-256 content key) before asserting byte-for-byte equality between `pollen` and upstream `bumblebee`.

---

## Wave 0 Requirements

The Pollen repo itself is Wave 0 — all of the following are created during Phase 1 execution (in `../pollen`):

- [ ] `C:/Users/Bantu/mzansi-agentive/pollen/` — new git repo, `git init`, `upstream` remote → `perplexityai/bumblebee`
- [ ] Source imported at SHA `c24089804ee66ece4bec6f14638cb98985389cdb` (v0.1.1)
- [ ] `go.mod` — module path → `github.com/bantuson/pollen`
- [ ] `cmd/pollen/` — renamed from `cmd/bumblebee/`; `main.Version` ldflags path confirmed; `bumblebee-selftest-*` temp dir → `pollen-selftest-*`; help text binary name updated
- [ ] `LICENSE` (verbatim), `NOTICE`, `CHANGES.md`, `UPSTREAM.md`
- [ ] `VERSION` → `0.1.1-pollen.1`
- [ ] `Makefile` — mirrors beekeeper repro-build + `verify-release` targets
- [ ] `.goreleaser.yaml` — derived from **beekeeper's** (NOT upstream's, which lacks `-buildvcs=false`): repro flags, cosign keyless sign, syft CycloneDX SBOM, windows build target, `pollen` binary name
- [ ] `.github/workflows/ci.yml` — 3-OS matrix (go 1.25.x): vet + `go test -race` + selftest + build; differential (Linux+macOS); govulncheck; Windows tests explicitly `t.Skip` with structured reasons
- [ ] `.github/workflows/release.yml` — cosign keyless OIDC (`github.com/bantuson/pollen/...` identity) + SLSA L3 + SBOM
- [ ] Differential test + normalization harness — `go test` runnable

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Sigstore signature verifiable on the published tag | FORK-03 | Requires the GitHub Actions release run to have executed (no remote/CI yet) | After `v0.1.1-pollen.1` release: `cosign verify-blob --certificate-identity=https://github.com/bantuson/pollen/.github/workflows/release.yml@refs/tags/v0.1.1-pollen.1 ...` |
| Reproducible-build hash matches published release | FORK-03 | `make verify-release` needs `make` + CI build artifacts (not on Windows dev box) | CI `verify-release` job; documented in pollen `docs/THREAT-MODEL.md` |

---

## Validation Sign-Off

- [ ] All tasks have an automated verify or a Wave 0 dependency
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (the whole repo is Wave 0)
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
