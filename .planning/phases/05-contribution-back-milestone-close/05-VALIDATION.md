---
phase: 5
slug: contribution-back-milestone-close
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-03
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from 05-RESEARCH.md "Validation Architecture". Three repos: Pollen (`../pollen`), Beekeeper (this repo), and (no outward action) upstream.
> Note: several deliverables are doc/release artifacts (UPSTREAM.md, signed tags) verified by review/runbook, not unit tests; the GitHub-facing release steps are checkpointed (CONTEXT D-5).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard testing (`go test`) — both repos |
| **Config file** | none (go.mod, `go 1.25.0`) |
| **Quick run command** | `go test ./internal/sentry/... ./internal/catalog/... ./internal/check/... ./internal/scan/...` |
| **Full suite command** | `go test ./...` (local; `-race` is CI-only — needs CGO per CLAUDE.md) |
| **Pollen sanity** | `cd ../pollen && go build ./... && go test ./...` |
| **Estimated runtime** | ~30s per repo |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/sentry/... ./internal/catalog/... ./internal/check/... ./internal/scan/...`
- **After every plan wave:** `go test ./...` (beekeeper) + `cd ../pollen && go test ./...`
- **Before `/gsd-verify-work 5`:** Full suite green in both repos; Windows inventory-test skip baseline = **zero**
- **Max feedback latency:** ~30s

---

## Per-Task Verification Map

> Task IDs (`5-PP-TT`) assigned by the planner. Threat refs from the Security Domain register. GitHub-facing release steps (D-5) are **checkpointed (autonomous:false)** — verified by runbook execution + `cosign verify`, not by `go test`.

| Behavior | Requirement | Threat Ref | Test Type | Automated Command | File Exists | Status |
|----------|-------------|------------|-----------|-------------------|-------------|--------|
| `isSensitivePath` matches Windows backslash paths (`C:\...\.aws\credentials`) — the exfil-rule Windows bug fix | PTEST-05 | Evasion (ETW backslash bypass) | unit (all OS) | `go test ./internal/sentry/ -run TestIsSensitivePathWindows` | ❌ W0 | ⬜ pending |
| Windows honeypot: synthetic `.aws/credentials` read + outbound connect fires the exfil-signature-fusion rule (SENTRY-005) | PTEST-05 | — | unit (windows) | `go test ./internal/sentry/windows/ -run TestHoneypotExfilFusion` | ❌ W0 | ⬜ pending |
| `selfCatalogAdapter.LookupAll("beekeeper","pollen")` returns a match for a known-bad Pollen version | SDEF-01 | Tampering (compromised Pollen release) | unit | `go test ./internal/catalog/ -run TestSelfCatalogAdapter_PollenEntries` | ❌ W0 | ⬜ pending |
| `beekeeper selftest` passes with the pollen-self fixture (non-production version string — no false alarm) | SDEF-01 | Tampering | integration | `go test ./internal/check/ -run TestRunSelftest` | ❌ W0 (fixture) | ⬜ pending |
| Beekeeper inventory/scan tests pass with `go install`-pinned Pollen; **zero Windows `t.Skip`** | BKINT-02 | Spoofing (typosquat/GOPROXY) | integration (CI) | `go test ./internal/scan/ -v 2>&1 \| Select-String SKIP` is empty | ✅ existing (verify) | ⬜ pending |
| Pollen builds + full suite green after pollen.5 VERSION/CHANGES bump | (release) | — | regression | `cd ../pollen && go build ./... && go test ./...` | ✅ existing | ⬜ pending |
| UPSTREAM.md contains the §6.2 8-step sync workflow + version history (pollen.2/3/4/5) + contribution-deferred note | SYNC-01 | — | manual/review | doc review | ✅ (`../pollen/UPSTREAM.md` extend) | ⬜ pending |
| Prepared Windows patch set + SYNC-02-deferred rationale documented (no PRs opened) | SYNC-02 (descoped) | — | manual/review | doc review | ✅ (UPSTREAM.md) | ⬜ pending |
| Four signed tags cut + verified: pollen.2/.3/.4/.5 | (release SC6) | Tampering (release integrity) | checkpoint + runbook | `cosign verify-blob ...` (capital-B `Bantuson` identity) | N/A (auth-gated, D-5) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/sentry/rules_test.go` (or `_windows_test.go`) — `TestIsSensitivePathWindows` (backslash-path regression for the `filepath.ToSlash` fix)
- [ ] `internal/sentry/windows/honeypot_test.go` (`//go:build windows`) — `TestHoneypotExfilFusion` (synthetic `EventFileAccess` `.aws/credentials` → `EventNetworkConnect` RFC-5737 IP → rule fires)
- [ ] `internal/catalog/selfcatalog_test.go` addition — `TestSelfCatalogAdapter_PollenEntries`
- [ ] `internal/check/corpus/fixtures.json` — pollen-self fixture entry (non-production version string, e.g. `pollen-test-v0.0.1`, to avoid false quarantine)
- [ ] (optional) `internal/catalog/testdata/selfcatalog_match_pollen.json` — extends the existing self-catalog feed fixture pattern

*Framework already present — both repos use stdlib `testing` only.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Push both repos to GitHub; cut + cosign-sign tags pollen.2/.3/.4/.5; verify | release SC6 / D-4 | Auth-gated (GitHub `gh`/OIDC) + irreversible outward action — checkpointed per CONTEXT D-5; signing runs in GitHub Actions, not locally | Follow the executor-produced release runbook: confirm 3-OS CI green → `git tag -a v0.1.1-pollen.N` → push tag → wait for release workflow → `cosign verify-blob` against `^https://github.com/Bantuson/pollen/` |
| UPSTREAM.md is followable cold by a second maintainer (SC1) | SYNC-01 | Documentation quality is a human judgment | Read UPSTREAM.md top-to-bottom; confirm every step is a concrete command, not prose |
| Full 3-OS CI matrix green incl. Windows honeypot E2E | BKINT-02 / PTEST-05 | Cross-OS CI is CI-only (Windows is the dev box; Linux/macOS in CI) | Push beekeeper; confirm GitHub Actions matrix green with zero Windows inventory `t.Skip` |

*All unit/integration behaviors above have automated verification; only cross-OS CI confirmation + the auth-gated signed release are environment/human-gated.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies (doc/release tasks use review/runbook checks)
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all ❌ W0 references above
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
