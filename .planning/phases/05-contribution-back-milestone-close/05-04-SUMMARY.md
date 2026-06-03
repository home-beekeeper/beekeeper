---
phase: 05-contribution-back-milestone-close
plan: "04"
subsystem: ci-pin + release-runbook
tags: [BKINT-02, release-runbook, pollen-pin, subprocess-boundary, D-5]
dependency_graph:
  requires: ["05-03"]
  provides: ["BKINT-02-local-prep", "D-5-release-runbook"]
  affects:
    - ".github/workflows/ci.yml"
    - "internal/scan/pollen_version.go"
    - "docs/release-runbook.md"
tech_stack:
  added: []
  patterns:
    - "go install @pinned-version in CI (subprocess-boundary pin, not go.mod import)"
    - "source const for auditable version tracking (PinnedPollenVersion)"
key_files:
  created:
    - "internal/scan/pollen_version.go"
    - "docs/release-runbook.md"
  modified:
    - ".github/workflows/ci.yml"
decisions:
  - "BKINT-02 pin expressed as CI go install step + source const; no go.mod module import (subprocess boundary BKINT-01 intact)"
  - "pollen.4 tagged at b906404 (recommended) over a9db7b3 — includes WR-01 VSCodium labelling fix"
  - "Release runbook sequences pollen push+tags before beekeeper push (Pitfall 3 mitigation)"
metrics:
  duration: "~5 minutes"
  completed: "2026-06-03"
  tasks_completed: 2
  tasks_total: 2
  files_created: 2
  files_modified: 1
---

# Phase 05 Plan 04: BKINT-02 CI Pin + Release Runbook Summary

**One-liner:** Beekeeper CI pins Pollen binary at v0.1.1-pollen.4 via `go install` on all 3 OS runners; PinnedPollenVersion const records the pin; docs/release-runbook.md is the copy-paste D-5 maintainer procedure for cutting four signed tags and pushing both repos.

---

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | BKINT-02 — CI Install Pollen step + PinnedPollenVersion const | `6539854` | `.github/workflows/ci.yml`, `internal/scan/pollen_version.go` |
| 2 | Author docs/release-runbook.md (D-5 maintainer hand-off procedure) | `985b755` | `docs/release-runbook.md` |

---

## Task 1: BKINT-02 — CI Install Pollen Step + Source Const

**Files:** `.github/workflows/ci.yml`, `internal/scan/pollen_version.go`

**CI change:** Added step `Install Pollen (BKINT-02 — pinned binary for inventory tests)` to
the `test` job in `.github/workflows/ci.yml`, placed after `Verify dependencies` and before
`Build` (before `Test`), with no `if:` guard — runs on all three OS matrix runners
(`ubuntu-latest`, `macos-latest`, `windows-latest`):

```yaml
- name: Install Pollen (BKINT-02 — pinned binary for inventory tests)
  run: go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.4
```

`actions/setup-go@v5` adds `GOPATH/bin` to PATH (Assumption A2 from 05-RESEARCH.md),
so `exec.LookPath("pollen")` in `scanner.go` will find the installed binary during CI test runs.

**Source const:** Created `internal/scan/pollen_version.go` (package `scan`) with:

```go
const PinnedPollenVersion = "v0.1.1-pollen.4"
```

The doc comment explains: this is a subprocess-boundary pin (BKINT-02), NOT a Go-module
dependency. No `import "github.com/bantuson/pollen/..."` was added (prohibited — BKINT-01).
The file explicitly directs maintainers to update both this const AND the CI step together.

**Subprocess boundary verification:**
- `go.mod` has no `github.com/bantuson/pollen` require directive (verified)
- `internal/scan/pollen_version.go` mentions `github.com/bantuson/pollen` only in doc comments,
  never in an import block — the Go compiler ignores it
- `go build ./...` clean; `go vet ./internal/scan/` clean

**Zero-skip baseline:** `go test ./internal/scan/ -v` produces zero SKIP lines. The four scan
tests (`TestScanWithBumblebee`, `TestScanWindowsShapedRecord`, `TestScanPollenUnavailable`,
`TestPollenCompatibility`) all use injectable `runPollenFn`/`lookPollenFn` — they do not
require the real binary and have zero `t.Skip`.

**Note:** `go install @v0.1.1-pollen.4` will fail in CI until the pollen tag is live on
GitHub (Pitfall 3, 05-RESEARCH.md). The CI step is local prep; it resolves after the
plan-05 checkpoint when the maintainer pushes pollen + cuts the tags (Step 7 of the runbook).

---

## Task 2: docs/release-runbook.md

**File:** `docs/release-runbook.md`

The D-5 maintainer hand-off procedure — 7 numbered steps with copy-paste command blocks:

1. **Preconditions** — local commits verified in both repos; `gh auth status` green; cosign v3 available
2. **Create beekeeper GitHub repo** — `gh repo create bantuson/beekeeper --public --source=. --push` (lowercase matches go.mod module path)
3. **Push pollen main** — `git -C .../pollen push origin main`; wait for 3-OS CI green via `gh -R Bantuson/pollen run watch`
4. **Cut pollen.2** at `c94b271` — `git tag -a v0.1.1-pollen.2 c94b271 -m "..."` + push + wait for release job
5. **Cut pollen.3** at `19695e3` — same pattern
6. **Cut pollen.4** — decision point documented: `b906404` recommended (includes WR-01 VSCodium fix) over `a9db7b3` (Phase 4 release-prep commit that predates the fix)
7. **Cut pollen.5** at HEAD after Phase 5 Plan 01 commits (VERSION/CHANGES/UPSTREAM.md) — explicitly NOT `a9db7b3` (Pitfall 6)
8. **Cosign verify** each release: `cosign verify-blob --bundle checksums.txt.sigstore.json --certificate-identity-regexp '^https://github\.com/Bantuson/pollen/' --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' checksums.txt`
9. **Push beekeeper main** (Step 6 in runbook) — after all pollen tags live (Pitfall 3 sequencing)

**Key safeguards in runbook:**
- Capital-B `Bantuson` in cosign identity regexp (Pitfall 4 — lowercase fails)
- Explicit sequencing note: pollen push+tags BEFORE beekeeper push (Pitfall 3)
- Decision-point table for pollen.4 with recommended `b906404` and fallback `a9db7b3`
- Every GitHub-facing step marked auth-gated / maintainer-executed
- Post-release checklist (four releases on GitHub, cosign OK, beekeeper CI green, zero skips)
- Tag-to-commit reference table for all four releases
- Troubleshooting section for the three most likely failure modes

---

## Verification Results

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build ./...` | PASS |
| Vet | `go vet ./internal/scan/` | PASS |
| CI step pattern | `Select-String -Path .github/workflows/ci.yml -Pattern "go install .../pollen@v0.1.1-pollen.4"` | True |
| No go.mod pollen import | `Select-String -Path go.mod -Pattern "bantuson/pollen"` | empty (PASS) |
| Zero scan test skips | `go test ./internal/scan/ -v \| Select-String SKIP` | empty (PASS) |
| cosign verify-blob in runbook | `Select-String -Path docs/release-runbook.md -Pattern "cosign verify-blob"` | True |
| pollen.5 in runbook | `Select-String -Path docs/release-runbook.md -Pattern "v0.1.1-pollen.5"` | True |
| capital-B Bantuson in runbook | `Select-String -Path docs/release-runbook.md -Pattern "Bantuson/pollen"` | True |
| All 4 commit hashes in runbook | c94b271, 19695e3, b906404, a9db7b3 | All present |

---

## Deviations from Plan

None — plan executed exactly as written.

The plan specified `b906404` as the recommended pollen.4 commit (over `a9db7b3`) and the
runbook presents this as a decision point with the recommended default, consistent with
the plan's `<action>` block.

---

## Threat Surface Scan

No new network endpoints, auth paths, or file access patterns introduced. The CI step
installs a public Go binary at a pinned version — covered by threat T-05-10 (typosquat
mitigation via full-version pin) and T-05-11 (release integrity via cosign + SLSA L3)
already in the plan's threat model. No new threat flags.

---

## Known Stubs

None. The CI step and PinnedPollenVersion const are complete as authored. The runbook
is the maintainer hand-off artifact; it is not wired to a data source.

## Self-Check: PASSED

- `internal/scan/pollen_version.go` exists with `PinnedPollenVersion = "v0.1.1-pollen.4"`
- `.github/workflows/ci.yml` contains the Install Pollen step with `@v0.1.1-pollen.4`
- `docs/release-runbook.md` exists with cosign verify-blob, Bantuson identity, four commit hashes
- Commits `6539854` (task 1) and `985b755` (task 2) confirmed in git log
- `go build ./...` and `go vet ./internal/scan/` clean
- `go.mod` has no `bantuson/pollen` import
