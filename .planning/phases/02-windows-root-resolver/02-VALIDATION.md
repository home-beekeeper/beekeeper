---
phase: 2
slug: windows-root-resolver
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-02
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Code under test lives in the sibling repo `../pollen` (`C:\Users\Bantu\mzansi-agentive\pollen`).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing package (stdlib), Go 1.25 |
| **Config file** | none (`go.mod` specifies `go 1.25`) |
| **Quick run command** | `go test ./cmd/pollen/ -run '^TestWindowsBaseline' -v` (run from `../pollen`; Windows-tagged tests only fire on windows-latest / a Windows host) |
| **Full suite command** | `go test -race ./...` with `CGO_ENABLED=1` (run from `../pollen`) |
| **Estimated runtime** | ~30–90 seconds full suite per OS |

---

## Sampling Rate

- **After every task commit:** `go build -trimpath -buildvcs=false ./cmd/pollen/` from `../pollen` (compile check — the Windows-tagged tests only execute on a Windows host, so local compile is the fast signal on the dev machine).
- **After every plan wave:** `go test -race ./...` (`CGO_ENABLED=1`) — green on all three OS runners in CI.
- **Before `/gsd-verify-work 2`:** Full suite green on ubuntu-latest, macos-latest, AND windows-latest; differential job still green on Linux+macOS.
- **Max feedback latency:** ~90 seconds (CI per-OS job).

---

## Per-Task Verification Map

> Task IDs are finalized by the planner. The requirement→test mapping below is the contract every plan task must satisfy.

| Task (plan) | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-------------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| Windows JS roots (npm/pnpm/Yarn/Bun) | 1 | WRES-01 | — | Roots resolve under `%APPDATA%`/`%LOCALAPPDATA%`/`%USERPROFILE%`; absent roots dropped, never errored | unit | `go test ./cmd/pollen/ -run '^TestWindowsBaseline' -v` | ❌ W0 | ⬜ pending |
| Windows PyPI/Go/Ruby/Composer roots | 1 | WRES-02 | — | Roots resolve incl. `%ProgramFiles%\Ruby*` / `%APPDATA%\Python\Python*` globs; venv `Lib\site-packages` | unit | `go test ./cmd/pollen/ -run '^TestWindowsBaseline' -v` | ❌ W0 | ⬜ pending |
| Cross-platform parity test | 2 | PTEST-01 | — | Same fixture → equivalent normalized records on all 3 OSes; `endpoint.os` differs correctly | integration | `go test ./cmd/pollen/ -run '^TestParityAllEcosystems' -v` | ❌ W0 | ⬜ pending |
| Flip Windows root-resolver skips | 2 | WRES-01/WRES-02 | — | `main_test.go` Phase-2 skips removed; Windows path actually exercised in CI | unit | `go test ./cmd/pollen/ -v` (windows-latest) | ✅ exists (edit) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `../pollen/cmd/pollen/roots_windows.go` (`//go:build windows`) — production code for WRES-01, WRES-02
- [ ] `../pollen/cmd/pollen/roots_windows_test.go` (`//go:build windows`) — unit tests for WRES-01, WRES-02
- [ ] `../pollen/cmd/pollen/parity_test.go` — PTEST-01 parity test (reuses `buildCurrentPollen`/`normalize` from `normalize_diff.go`)
- [ ] `../pollen/cmd/pollen/testdata/parity-fixture/` — single fake-package fixture tree consumed by all 3 OS runs
- [ ] `normalizeForParity()` helper (in `parity_test.go`, or reuse existing `normalize()`) — PTEST-01 determinism (strip non-deterministic fields, sort by record_id, allow OS-path-string divergence)

*Existing 3-OS `test` matrix job in `ci.yml` picks up the new tests automatically — no new CI job required.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `v0.1.1-pollen.2` tag is Sigstore-signed | (release gate) | Signing fires in GitHub Actions on tag push (OIDC); not reproducible in unit tests | After tagging, verify the release workflow's cosign step succeeded and the SBOM/signature artifacts are attached to the GitHub release |

*All functional phase behaviors (WRES-01/02, PTEST-01) have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
