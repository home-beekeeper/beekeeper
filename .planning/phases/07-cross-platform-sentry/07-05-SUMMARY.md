---
phase: 07-cross-platform-sentry
plan: "05"
subsystem: release-pipeline
tags: [slsa, sbom, cyclonedx, syft, provenance, eslogger, ci-gate]
dependency_graph:
  requires: [07-01, 07-02, 07-03, 07-04]
  provides: [SFDF-05, SMAC-02-gate]
  affects: [.goreleaser.yaml, .github/workflows/release.yml, .github/workflows/ci.yml]
tech_stack:
  added:
    - "syft (anchore/sbom-action/download-syft@v0) — CycloneDX SBOM generation"
    - "slsa-github-generator@v2.1.0 — SLSA Level 3 provenance attestation"
  patterns:
    - "GoReleaser sboms section with syft as external tool producing cyclonedx-json per archive"
    - "Goreleaser job exposes base64-encoded checksums.txt hash via $GITHUB_OUTPUT for SLSA provenance job"
    - "darwin build-tagged test that skips locally and runs against live eslogger on macos-latest CI"
key_files:
  created:
    - internal/sentry/darwin/eslogger_fields_test.go
  modified:
    - .goreleaser.yaml
    - .github/workflows/release.yml
    - .github/workflows/ci.yml
decisions:
  - "slsa-github-generator pinned to @v2.1.0 (full semver) per CLAUDE.md constraint — never @v2 or @main"
  - "syft installed via anchore/sbom-action/download-syft@v0 before GoReleaser step so GoReleaser can invoke it as sboms cmd"
  - "hashes export step fails loudly if checksums.txt is missing — prevents silent empty-string provenance"
  - "eslogger test uses t.Skip (not t.Fatal) when BEEKEEPER_ESLOGGER_FIXTURE unset — dev machines not blocked"
  - "test-eslogger-fields added to release-gate needs list — parser schema drift blocks releases"
metrics:
  duration: "~10 minutes"
  completed: "2026-05-28"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
  files_created: 1
---

# Phase 7 Plan 05: SLSA Level 3 + CycloneDX SBOM + eslogger CI Gate Summary

SLSA Level 3 provenance via slsa-github-generator@v2.1.0 and CycloneDX SBOM via syft wired into the GoReleaser release pipeline; macos-latest eslogger field validation gate added to CI release-gate aggregator.

## Tasks Completed

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | SLSA Level 3 + CycloneDX SBOM in GoReleaser pipeline | 1e4d1ec | .goreleaser.yaml, .github/workflows/release.yml |
| 2 | eslogger field validation test + CI gate | d99a31e | internal/sentry/darwin/eslogger_fields_test.go, .github/workflows/ci.yml |

## What Was Built

### Task 1: SLSA Level 3 + CycloneDX SBOM (SFDF-05)

**.goreleaser.yaml** — Added `sboms:` section after the `signs:` section:
- Invokes `syft` as an external command per archive
- Outputs CycloneDX JSON format (`cyclonedx-json=$document`) per CONTEXT.md locked decision
- Document template: `${artifact}.cdx.json`
- Updated header comment to reference SFDF-05 (Phase 7) replacing the "intentionally NOT configured" placeholder

**.github/workflows/release.yml** — Full rewrite extending the Phase 1 pipeline:
- Added `anchore/sbom-action/download-syft@v0` install step before GoReleaser so syft is on PATH when GoReleaser runs the sboms section
- Added `id: run-goreleaser` to the GoReleaser step to capture its `outputs.artifacts` JSON
- Added "Generate artifact hashes for SLSA provenance" step that extracts checksums.txt from GoReleaser artifacts and base64-encodes it — fails loudly if checksums.txt is missing (Rule 2: explicit error beats silent empty provenance)
- Added `outputs: hashes` to the goreleaser job so the provenance job can consume it
- Added `provenance` job using `slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml@v2.1.0` — **full semver, not @v2** (CLAUDE.md constraint, RESEARCH Pitfall 7)
- Moved `id-token: write` to per-job permissions; top-level defaults to `contents: read`

**SLSA generator version pinned:** `v2.1.0` (exact full semver as required by CLAUDE.md and the SLSA framework's trusted-builder verification rules).

**Syft installation pattern:** `anchore/sbom-action/download-syft@v0` — this is the anchore-maintained action that installs syft into PATH. GoReleaser's sboms section then invokes it via `cmd: syft`.

**Phase 1 preservation:** All existing `builds:`, `checksums:`, `signs:`, `archives:`, `release:` sections of `.goreleaser.yaml` are unchanged. The `sigstore/cosign-installer@v3` and `goreleaser/goreleaser-action@v7` steps are preserved in the updated release.yml.

### Task 2: eslogger Field Validation Test + CI Job (SMAC-02)

**internal/sentry/darwin/eslogger_fields_test.go**:
- Build tag `//go:build darwin` — excluded from Windows and Linux builds
- `TestEsloggerFieldValidation` reads `BEEKEEPER_ESLOGGER_FIXTURE` env var; skips if unset
- Parses every NDJSON line via `parseEsloggerLine` (the 07-01 parser)
- Asserts: total > 0, parsed > 0, execCount > 0, withPID > 0, withExe > 0
- Each failure message includes the specific field path that may be wrong (e.g., "wrong audit_token path") to aid schema drift diagnosis

**.github/workflows/ci.yml** — Added `test-eslogger-fields` job:
- Runs on `macos-latest` (current macOS image, tracks latest stable)
- Captures live eslogger output in background with `sudo -n eslogger exec open create network_flow fork`
- Generates warmup process activity (5 x `/bin/echo` + `/usr/bin/whoami`) to guarantee at least one exec event
- Exports fixture path via `$GITHUB_ENV` (`BEEKEEPER_ESLOGGER_FIXTURE=$FIXTURE`)
- Runs `go test -tags darwin -count=1 -v -run TestEsloggerFieldValidation ./internal/sentry/darwin/`
- Updated `release-gate` needs list: added `test-eslogger-fields`

## eslogger Field Paths Validated

The parser in `internal/sentry/darwin/parser.go` uses the following field paths:

| Event Type | PID Source | Exe Source |
|------------|-----------|------------|
| exec | `.event.exec.target.audit_token.pid` | `.event.exec.target.executable.path` |
| open | `.process.audit_token.pid` | `.process.executable.path` |
| create | `.process.audit_token.pid` | `.process.executable.path` |
| network_flow | `.process.audit_token.pid` | `.process.executable.path` |

These are [ASSUMED] field paths from RESEARCH §2.2 that are now verified at every release by the macos-latest CI job. The `TestEsloggerFieldValidation` test will fail with an explicit message if Apple changes these paths in a future macOS version.

**Note on CI validation status:** This plan was executed on a Windows development machine. Live eslogger validation against a real macOS runner is performed in CI. The test is structured to fail the release gate if the assumed field paths do not match the actual eslogger schema — this is the closure of the CLAUDE.md research note "eslogger field names partially undocumented."

## Deviations from Plan

None — plan executed exactly as written. The test file field names (`PID`, `Exe`, `Kind`, `sentry.EventProcessCreate`, `sentry.EventFileAccess`, `sentry.EventNetworkConnect`) were confirmed by reading `internal/sentry/types.go` before writing the test.

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, or trust boundary surfaces introduced. The release pipeline additions are GitHub Actions-side only.

## Self-Check: PASSED

- `.goreleaser.yaml` exists and contains `sboms:`, `cyclonedx-json`, `syft`, `cosign`, `mod_timestamp`
- `.github/workflows/release.yml` exists and contains `generator_generic_slsa3.yml@v2.1.0`, `anchore/sbom-action/download-syft@v0`, `base64-subjects`, 2x `id-token: write`
- `.github/workflows/ci.yml` exists and contains 2x `test-eslogger-fields`, `sudo -n eslogger`, `BEEKEEPER_ESLOGGER_FIXTURE`
- `internal/sentry/darwin/eslogger_fields_test.go` exists, starts with `//go:build darwin`, contains `TestEsloggerFieldValidation`
- `go build ./...` passes
- `GOOS=darwin go vet ./internal/sentry/darwin/...` passes
- Task 1 commit: 1e4d1ec
- Task 2 commit: d99a31e
