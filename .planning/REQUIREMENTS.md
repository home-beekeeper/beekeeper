# Requirements: Beekeeper — Milestone v1.1.0 "Pollen"

**Defined:** 2026-06-01
**Core Value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Milestone Goal:** Own Windows inventory compatibility by forking Bumblebee into a bounded Apache-2.0 derivative ("Pollen"), so the Windows CI matrix goes fully green and cross-platform test discipline holds instead of rotting behind `t.Skip`.
**Scope source:** `beekeeper-m2-prd.md` (repo root).

## v1 Requirements

Requirements for milestone v1.1.0. Each maps to exactly one roadmap phase. Spans two repos: the new `github.com/bantuson/pollen` module and beekeeper's `internal/inventory/` integration.

### Fork Setup & Hygiene (FORK)

- [x] **FORK-01**: Pollen exists as a separate Go module `github.com/bantuson/pollen`, forked from `perplexityai/bumblebee` at a pinned commit (v0.1.1 tag), with `cmd/bumblebee/` renamed to `cmd/pollen/`; the `pollen` CLI binary builds on ubuntu/macos/windows
- [x] **FORK-02**: Apache-2.0 LICENSE preserved verbatim; NOTICE attributes upstream Perplexity/Bumblebee and states non-affiliation; CHANGES.md documents every delta from the pinned commit; UPSTREAM.md records the pinned commit (40-char SHA + tag + date + verifier) and the sync workflow
- [x] **FORK-03**: Reproducible builds (`-trimpath -buildvcs=false`) + Sigstore keyless signing via GitHub Actions OIDC wired into Pollen's release workflow; first tag `v0.1.1-pollen.1` proves fork hygiene before any Windows code lands
- [x] **FORK-04**: Trademark discipline — "Bumblebee" appears only in attribution contexts (NOTICE, README "derived from" paragraph, UPSTREAM.md), never in command names, package names, README headlines, or `cmd/`

### Windows Root Resolver (WRES)

- [x] **WRES-01**: `internal/resolver/resolver_windows.go` (build tag `windows`) discovers Windows package roots for the JS ecosystems — npm (global `%APPDATA%\npm\node_modules` + user cache), pnpm (`%LOCALAPPDATA%\pnpm\store`), Yarn (`%LOCALAPPDATA%\Yarn\Data\global`), Bun (`%USERPROFILE%\.bun\install\cache`) per PRD §8.1
- [x] **WRES-02**: Windows root discovery for PyPI (user `%APPDATA%\Python\...` + venv `<venv>\Lib\site-packages`), Go modules (`%USERPROFILE%\go\pkg\mod`), RubyGems (`%USERPROFILE%\.gem\...` + `%ProgramFiles%\Ruby*`), Composer (`%APPDATA%\Composer\vendor`)

### Windows Path Representation (WPATH)

- [ ] **WPATH-01**: NDJSON `project_path`/`source_file` preserve native Windows paths — backslash separators and drive letters retained (`C:\Users\...`, not `/c/Users/...`), no Unix-to-Windows conversion artifacts. (Live locus per Phase-3 RESEARCH: the `projectPath` join in `internal/ecosystem/npm/npm.go` and `internal/ecosystem/pnpm/pnpm.go` wrapped in `filepath.FromSlash` — a no-op on Unix so the PTEST-02 differential stays byte-identical; `source_file` is already native. The PRD §5.2 `internal/output/paths_windows.go` path does not exist in the live fork.)
- [ ] **WPATH-02**: `endpoint` record emits `os="windows"`, `arch` from `runtime.GOARCH`, `username` from the Windows environment, and empty `uid`; beekeeper's audit-log consumer handles Windows-shaped endpoint records (round-trip verified)

### Windows Extension & MCP Coverage (WEXT)

- [x] **WEXT-01**: Windows editor-extension paths — VS Code, Code Insiders, Cursor, Windsurf, VSCodium (`%USERPROFILE%\.vscode\extensions\` and equivalents)
- [x] **WEXT-02**: Windows browser-extension paths — Chrome, Chromium, Edge, Brave (`%LOCALAPPDATA%\...\User Data\<Profile>\Extensions\`) and Firefox (`%APPDATA%\Mozilla\Firefox\Profiles\<profile>\extensions.json`), per profile
- [x] **WEXT-03**: Windows MCP host-config paths — Claude Desktop (`%APPDATA%\Claude\...`), Cursor MCP, Windsurf MCP, Cline (`%APPDATA%\cline\...`), Gemini CLI (`%USERPROFILE%\.gemini\settings.json`)

### Testing & Compatibility (PTEST)

- [x] **PTEST-01**: Cross-platform parity test — identical fake-package fixtures (`internal/testfixtures/`) produce equivalent inventory records on Linux/macOS/Windows (same packages, same severity matches, equivalent counts modulo OS path strings); `endpoint.os` differs correctly per platform
- [x] **PTEST-02**: Differential test — Pollen output is byte-for-byte identical to upstream Bumblebee on Linux and macOS for a fixed fixture; runs on every Pollen PR; re-run manually against any new upstream tag before absorbing
- [x] **PTEST-03**: `pollen selftest` passes on all three OSes; Pollen CI matrix (ubuntu/macos/windows, go 1.25.x) runs `go vet`, `go test -race ./...`, selftest, and a versioned build green; upstream's inherited Go test suite passes unchanged on Linux/macOS
- [x] **PTEST-04**: The Pollen compatibility test runs from beekeeper's harness on all three OSes — invokes `pollen scan`, asserts NDJSON schema-consistency with beekeeper's audit schema, runs beekeeper rules on top, asserts no double-counting and correct `scanner_name`; the Windows skip baseline for these tests is zero
- [x] **PTEST-05**: Windows honeypot E2E — a planted process tree that reads synthetic `%USERPROFILE%\.aws\credentials` and makes an outbound connection fires beekeeper's Sentry exfil-signature-fusion rule on Windows

### Beekeeper Integration (BKINT)

- [x] **BKINT-01**: Beekeeper consumes Pollen across the `internal/inventory/` boundary — invokes `pollen scan`, parses NDJSON, applies beekeeper rules; the `bumblebee` subprocess call in `internal/scan` (`runBumblebeeFn`) is swapped for `pollen` behind a mockable interface so beekeeper unit tests don't require Pollen to run
- [ ] **BKINT-02**: Beekeeper's `go.mod` pins Pollen at an explicit version (no auto-update; bumps require explicit beekeeper PRs); beekeeper CI installs Pollen and runs the compatibility + honeypot tests, flipping Windows CI from "skipped Bumblebee tests" to fully green

### Upstream Sync & Contribution-Back (SYNC)

- [ ] **SYNC-01**: Documented, repeatable upstream sync workflow in UPSTREAM.md — `git remote update upstream`, diff-review (new files / NDJSON schema / root resolver / LICENSE+NOTICE), run upstream tests on Linux+macOS, cherry-pick preserving Windows code paths, re-run the differential test, update pinned commit + CHANGES.md, tag a new `v0.1.1-pollen.N`
- [ ] **SYNC-02**: Windows additions prepared as upstream-shaped PRs against `perplexityai/bumblebee` — root resolver (build-tag pattern like PR #4), path representation (closes upstream #1), extension/MCP coverage — each referencing the equivalent Pollen tag and linking parity tests as evidence

### Self-Defense (SDEF)

- [ ] **SDEF-01**: `pollen-self` entries added to the `beekeeper-self` catalog so a compromised Pollen release is detectable by beekeeper itself (recursive self-quarantine across the dependency boundary)
- [x] **SDEF-02**: CycloneDX SBOM published per Pollen release, recording the source upstream commit and the Windows additions

## v2 Requirements

Deferred to a future release. Tracked but not in the current roadmap.

### Distribution (DIST)

- **DIST-01**: Pollen binary releases (currently source-only via `go install`) — revisit if external users without a Go toolchain request them (PRD §13 open question)

### Self-Catalog Separation (SELF)

- **SELF-02**: A separate `pollen-self` catalog distinct from `beekeeper-self` — leaning unified for v0.1; revisit if operational separation is warranted (PRD §13 open question)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| New ecosystems beyond upstream (Cargo, etc.) | Pollen is parity-plus-Windows, not feature-extension; stays an upstream concern (PRD §4.2) |
| SARIF export; live OSV.dev source (upstream #21) | Upstream features, not in Pollen's scope |
| Changing matching semantics / NDJSON schema / CLI surface | `schema_version` stays `0.1.0`; Pollen is a behavioral fork, not a protocol fork |
| Pollen as a standalone product | Exists to serve beekeeper; README points general users to upstream Bumblebee |
| Public binary distribution of Pollen | For our CI / internal use; not advertised as a Bumblebee alternative (revisit → DIST-01) |
| Indefinite maintenance of Pollen | Planned EOL — Pollen retires or becomes a thin shim when upstream merges equivalent Windows support (PRD §12.3) |
| Mirroring upstream `threat_intel/` catalogs into Pollen | Single-source: catalogs flow through beekeeper's own `catalogs sync` (PRD §6.3 "reference") |

## Traceability

Which phases cover which requirements. Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FORK-01 | Phase 1 | Complete |
| FORK-02 | Phase 1 | Complete |
| FORK-03 | Phase 1 | Complete |
| FORK-04 | Phase 1 | Complete |
| PTEST-02 | Phase 1 | Complete |
| PTEST-03 | Phase 1 | Complete |
| SDEF-02 | Phase 1 | Complete |
| WRES-01 | Phase 2 | Complete |
| WRES-02 | Phase 2 | Complete |
| PTEST-01 | Phase 2 | Complete |
| WPATH-01 | Phase 3 | Pending |
| WPATH-02 | Phase 3 | Pending |
| WEXT-01 | Phase 4 | Complete |
| WEXT-02 | Phase 4 | Complete |
| WEXT-03 | Phase 4 | Complete |
| BKINT-01 | Phase 4 | Complete |
| PTEST-04 | Phase 4 | Complete |
| SYNC-01 | Phase 5 | Pending |
| SYNC-02 | Phase 5 | Pending |
| BKINT-02 | Phase 5 | Pending |
| PTEST-05 | Phase 5 | Complete |
| SDEF-01 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 22 total
- Mapped to phases: 22
- Unmapped: 0 ✓

**Phase breakdown:**
- Phase 1 (Fork Setup & Discipline): FORK-01, FORK-02, FORK-03, FORK-04, PTEST-02, PTEST-03, SDEF-02 (7 requirements)
- Phase 2 (Windows Root Resolver): WRES-01, WRES-02, PTEST-01 (3 requirements)
- Phase 3 (Windows Path Representation): WPATH-01, WPATH-02 (2 requirements)
- Phase 4 (Windows Extension & MCP Coverage): WEXT-01, WEXT-02, WEXT-03, BKINT-01, PTEST-04 (5 requirements)
- Phase 5 (Contribution-Back & Milestone Close): SYNC-01, SYNC-02, BKINT-02, PTEST-05, SDEF-01 (5 requirements)

---
*Requirements defined: 2026-06-01*
*Last updated: 2026-06-01 — traceability filled, 22/22 mapped*
