---
status: complete
phase: 01-fork-setup-discipline
source: [01-01-SUMMARY.md, 01-02-SUMMARY.md, 01-03-SUMMARY.md, 01-04-SUMMARY.md, 01-05-SUMMARY.md]
started: 2026-06-02
updated: 2026-06-02
note: Auto-approved by user — every test backed by concrete CI/release evidence (green 3-OS CI run + cosign-verified signed release), not manual re-run.
---

## Current Test

[testing complete]

## Tests

### 1. Pollen repo exists and builds (FORK-01)
expected: `github.com/home-beekeeper/pollen` is a public repo, forked at pinned SHA `c240898`, module `github.com/home-beekeeper/pollen`, `cmd/pollen`; `pollen` binary builds on ubuntu/macos/windows.
result: pass
evidence: Repo live + public; CI `test` job green on all 3 OSes (build step ✓); local `go build ./cmd/pollen` + `GOOS=windows go build ./...` clean.

### 2. Apache-2.0 attribution + trademark discipline (FORK-02, FORK-04)
expected: LICENSE (verbatim), NOTICE (verbatim PRD §7.2), CHANGES.md, UPSTREAM.md (40-char SHA + sync workflow), VERSION=0.1.1-pollen.1, empty threat_intel/; "Bumblebee" only in attribution contexts.
result: pass
evidence: All files present + verified; FORK-04 trademark grep audit (incl. README headline) green in plan 01-02; scanner_name renamed to "pollen".

### 3. Selftest regression — 3 findings (PTEST-03)
expected: `pollen selftest` emits exactly 3 findings (npm, pypi, mcp), exit 0, on all 3 OSes.
result: pass
evidence: CI `test` selftest step green ×3 OS; local `pollen selftest` → "selftest OK (3 findings)".

### 4. Differential guard — pollen ≡ upstream (PTEST-02)
expected: `TestDifferential` proves byte-identical NDJSON vs upstream bumblebee (after normalizing non-deterministic + build-identity fields) on Linux+macOS; structured Windows skip.
result: pass
evidence: CI `differential` job green on ubuntu + macos (after fixing the scanner_version normalization gap that only surfaced in CI); Windows skip is the intentional one.

### 5. Reproducible + cosign-signed release (FORK-03)
expected: `v0.1.1-pollen.1` built reproducibly (`-trimpath -buildvcs=false`) and cosign-keyless-signed; `cosign verify-blob` returns Verified OK against the home-beekeeper/pollen OIDC identity.
result: pass
evidence: Release published; `cosign verify-blob` → **Verified OK** against `^https://github.com/home-beekeeper/pollen/` (A1: canonical casing `Bantuson`); snapshot build reproducible across 6 targets.

### 6. SBOM + SLSA provenance published (SDEF-02)
expected: CycloneDX SBOM per archive + SLSA L3 provenance attached to the release.
result: pass
evidence: 6× `*.cdx.json` (CycloneDX 1.6) + `multiple.intoto.jsonl` (SLSA L3) on the release. Upstream-SHA pin-of-record is UPSTREAM.md (user Accept decision; syft SBOM = artifact composition).

### 7. CI matrix green on 3 OSes (phase gate)
expected: The full ci.yml matrix (ubuntu/macos/windows test + differential + govulncheck) is green — the "green guards before code lands" gate.
result: pass
evidence: CI run 26785689550 (+ doc/release follow-ups) all green; only skip is the intentional Windows differential skip.

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
