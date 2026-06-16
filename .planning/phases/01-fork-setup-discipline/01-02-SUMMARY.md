---
phase: "01-fork-setup-discipline"
plan: "02"
subsystem: "pollen-fork"
tags: ["attribution", "license", "trademark", "upstream-tracking", "version"]
dependency_graph:
  requires: ["01-01 (pollen repo exists with cmd/pollen and module path rewrite)"]
  provides: ["NOTICE-verbatim-PRD-7.2", "UPSTREAM.md-pinned-SHA", "CHANGES.md-delta-log", "VERSION-0.1.1-pollen.1", "FORK-04-audit-green", "threat_intel-empty"]
  affects: ["01-03 (CI/build plans depend on NOTICE/VERSION being correct)", "01-04 (release signing attests the attribution files)", "Phase 5 (UPSTREAM.md is the sync workflow reference)"]
tech_stack:
  added: []
  patterns: ["verbatim-NOTICE-per-PRD", "UPSTREAM.md-pinned-commit-record", "CHANGES.md-added-renamed-modified-removed"]
key_files:
  created:
    - "../pollen/NOTICE"
    - "../pollen/UPSTREAM.md"
    - "../pollen/CHANGES.md"
    - "../pollen/threat_intel/README.md"
  modified:
    - "../pollen/README.md"
    - "../pollen/VERSION"
    - "../pollen/internal/model/model.go"
    - "../pollen/internal/walk/walk.go"
    - "../pollen/internal/output/httpsink_test.go"
    - "../pollen/.goreleaser.yaml"
    - "../pollen/.github/workflows/ci.yml"
decisions:
  - "ScannerName constant in model.go changed from 'bumblebee' to 'pollen' — output NDJSON scanner_name field is now 'pollen'"
  - "6 upstream catalog JSON files removed from threat_intel/ (ships empty per PRD §6.3 reference decision)"
  - ".goreleaser.yaml updated: project_name, binary id/name, archive id/template, windows added to goos (FORK-04 + future Phase 2–4 readiness)"
  - "ci.yml build/selftest step updated from cmd/bumblebee to cmd/pollen binary"
  - "UPSTREAM.md uses both table format and fenced-block format so grep for 'verified by: bantuson' (with colon) matches"
metrics:
  duration: "~25 minutes"
  completed: "2026-06-01"
  tasks_completed: 2
  files_changed: 14
---

# Phase 01 Plan 02: Attribution, VERSION, and FORK-04 Audit Summary

Apache-2.0 attribution complete for the Pollen fork: NOTICE reproduces PRD §7.2 verbatim, UPSTREAM.md records the pinned 40-char SHA with the Phase 5 sync workflow, CHANGES.md logs all deltas from the pinned commit, VERSION set to 0.1.1-pollen.1, threat_intel/ cleared to empty-by-design, and the full-repo FORK-04 trademark audit (source-code + README headline + README usage gates) is green.

## One-liner

Apache-2.0 attribution for Pollen fork: verbatim NOTICE (PRD §7.2), pinned-SHA UPSTREAM.md, CHANGES.md delta log, VERSION 0.1.1-pollen.1, empty threat_intel/, FORK-04 trademark audit green across source and markdown.

## Tasks Completed

| Task | Name | Status | Pollen Commit |
|------|------|--------|---------------|
| 1 | Write UPSTREAM.md, CHANGES.md, NOTICE; verify LICENSE (FORK-02) | Done | `943882b` |
| 2 | Write README, VERSION, threat_intel/README; FORK-04 full-repo audit | Done | `a3a85f3` |

## Verification Results

### Task 1 — FORK-02 acceptance criteria

| Check | Command | Result |
|-------|---------|--------|
| 40-char SHA in UPSTREAM.md | `grep -q "c24089804ee66ece4bec6f14638cb98985389cdb" UPSTREAM.md` | PASS |
| pinned tag in UPSTREAM.md | `grep -q "v0.1.1" UPSTREAM.md` | PASS |
| verifier in UPSTREAM.md | `grep -q "verified by: bantuson" UPSTREAM.md` | PASS |
| LICENSE file exists | `test -f LICENSE` | PASS |
| NOTICE file exists | `test -f NOTICE` | PASS |
| NOTICE non-affiliation statement | `grep -q "not affiliated with, endorsed by, or supported by Perplexity AI Inc." NOTICE` | PASS |
| CHANGES.md has Added/Renamed/Modified sections | `grep -qiE "Renamed\|Modified\|Added" CHANGES.md` | PASS |
| NOTICE contains github.com/perplexityai/bumblebee | confirmed present | PASS |
| NOTICE contains github.com/home-beekeeper/beekeeper | confirmed present | PASS |
| NOTICE contains "Changes from upstream are documented in CHANGES.md." | confirmed present | PASS |
| CHANGES.md names cmd/bumblebee → cmd/pollen | confirmed present | PASS |
| CHANGES.md names module path rename | confirmed present | PASS |
| LICENSE confirmation method | LICENSE was not regenerated — came verbatim from the upstream clone at c24089804ee66ece4bec6f14638cb98985389cdb. `git -C ../pollen show c240898:LICENSE` and current HEAD LICENSE are identical. The file was NOT added in the FORK-02 commit (it was already present from the clone commit). | PASS |

### UPSTREAM.md sync workflow steps captured

The UPSTREAM.md documents the following Phase 5 sync procedure (exact steps):
1. `git remote update upstream` — fetch latest upstream main
2. Diff-review: new files, NDJSON schema changes, root resolver conflicts, LICENSE/NOTICE changes
3. Run upstream tests on Linux and macOS
4. Cherry-pick or merge preserving Windows code paths
5. Re-run Pollen CI matrix (all three OSes)
6. Run the differential test (byte-for-byte NDJSON identity on Linux)
7. Bump pinned commit in UPSTREAM.md + append to CHANGES.md
8. Tag new `v0.1.1-pollen.N`

Sync execution is deferred to Phase 5; UPSTREAM.md documents the workflow now as the fixed reference.

### Task 2 — FORK-04 acceptance criteria

| Check | Gate command | Result |
|-------|--------------|--------|
| VERSION exact | `grep -qx "0.1.1-pollen.1" VERSION` | PASS |
| threat_intel/README.md exists | `test -f threat_intel/README.md` | PASS |
| threat_intel/ no JSON | `find threat_intel -name '*.json'` returns nothing | PASS (empty) |
| Source-code FORK-04 gate | `grep -rIi "bumblebee" . --include="*.go" --include="*.yaml" --include="*.yml" --include="Makefile" \| grep -v selftest` | PASS (empty) |
| README headline gate | `grep -i "^#.*bumblebee" README.md` | PASS (empty) |
| README usage gate | `grep -i "go install.*bumblebee\|bumblebee " README.md` | PASS (empty) |
| README has attribution paragraph | `grep -q "perplexityai/bumblebee" README.md` | PASS |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Fixed ScannerName constant in model.go**
- **Found during:** Task 2 FORK-04 audit
- **Issue:** `ScannerName = "bumblebee"` in `internal/model/model.go` — every NDJSON record would emit `"scanner_name": "bumblebee"`, which is a trademark violation and incorrect identity in the output
- **Fix:** Changed to `ScannerName = "pollen"`
- **Files modified:** `internal/model/model.go`
- **Commit:** `a3a85f3`

**2. [Rule 2 - Missing Critical] Removed 6 upstream catalog JSON files from threat_intel/**
- **Found during:** Task 2 (threat_intel/ setup)
- **Issue:** `threat_intel/` contained 6 upstream catalog JSON files (antv-mini-shai-hulud.json, gemstuffer.json, mini-shai-hulud.json, node-ipc-credential-stealer.json, nx-console-vscode-2026-05-18.json, shopsprint-decimal-typosquat.json) that were included in the upstream clone. PRD §6.3 "reference" decision: threat_intel/ ships empty; catalogs flow through beekeeper catalogs sync.
- **Fix:** `git rm` all 6 JSON files; rewrote threat_intel/README.md to document the empty-by-design rationale
- **Files modified:** `threat_intel/README.md` + 6 deleted JSON files
- **Commit:** `a3a85f3`

**3. [Rule 2 - Missing Critical] Fixed trademark strings in 4 additional source files**
- **Found during:** Task 2 FORK-04 source-code audit
- **Issue:** FORK-04 audit revealed `bumblebee` in `.goreleaser.yaml` (project_name, binary id/name, archive id/template), `.github/workflows/ci.yml` (build/selftest step), `internal/walk/walk.go` (comment referencing cmd/bumblebee), and `internal/output/httpsink_test.go` (UserAgent test string `"bumblebee/test"`)
- **Fix:** Updated all 4 files to use `pollen` names
- **Files modified:** `.goreleaser.yaml`, `.github/workflows/ci.yml`, `internal/walk/walk.go`, `internal/output/httpsink_test.go`
- **Commit:** `a3a85f3`

**4. [Rule 2 - Missing Critical] Added windows to goreleaser goos list**
- **Found during:** Task 2 .goreleaser.yaml update
- **Issue:** Upstream .goreleaser.yaml built only for `darwin` and `linux`. Pollen targets Windows as a first-class platform (its whole purpose). The goreleaser goos list needed `windows` added.
- **Fix:** Added `windows` to goos in the builds stanza
- **Files modified:** `.goreleaser.yaml`
- **Commit:** `a3a85f3`

## Known Stubs

None — all attribution, version, and catalog files are complete. threat_intel/ intentional empty status is documented in its README and is by design (PRD §6.3), not a stub.

## Threat Surface Scan

No new network endpoints, auth paths, or schema changes introduced. The threat surface reductions in this plan:
- `threat_intel/` catalog JSON files removed — eliminates the risk of stale upstream intel being accidentally used via `--exposure-catalog threat_intel/`
- `ScannerName` corrected from `bumblebee` to `pollen` — output records now correctly identify their source
- `NOTICE` non-affiliation statement explicitly disclaims Perplexity AI endorsement — reduces spoofing risk (T-01-05)
- `UPSTREAM.md` records the verifiable 40-char SHA — provides auditable provenance chain (T-01-06)

## Self-Check: PASSED

Files exist in pollen repo:
- `../pollen/NOTICE` — FOUND
- `../pollen/UPSTREAM.md` — FOUND
- `../pollen/CHANGES.md` — FOUND
- `../pollen/VERSION` (contains `0.1.1-pollen.1`) — FOUND
- `../pollen/threat_intel/README.md` — FOUND
- `../pollen/README.md` (Pollen headline, single attribution para) — FOUND

Pollen commits verified:
- `943882b` — FOUND (FORK-02 attribution commit)
- `a3a85f3` — FOUND (FORK-04 README/VERSION/trademark commit)

No JSON files in threat_intel/ — CONFIRMED

Full-repo FORK-04 gates:
- Source-code gate: PASS (empty)
- README headline gate: PASS (empty)
- README usage gate: PASS (empty)
