# Roadmap: Beekeeper

## Milestones

- ✅ **v1.0.0 — Comprehensive Standalone Release** — Phases 1–10 (shipped 2026-06-01)
  Full per-phase detail archived in [`milestones/v1.0.0-ROADMAP.md`](milestones/v1.0.0-ROADMAP.md).
  Audit: PASSED — [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).
- 🔄 **v1.1.0 — "Pollen"** — Phases 1–5 (in progress)
  Goal: Own Windows inventory compatibility via a bounded Apache-2.0 Bumblebee derivative so the Windows CI matrix goes fully green.

## Phases

<details>
<summary>✅ v1.0.0 Comprehensive Standalone Release (Phases 1–10) — SHIPPED 2026-06-01</summary>

- [x] Phase 1: Foundation + Hook Handler (6/6 plans) — fail-closed `beekeeper check`, Bumblebee mmap catalog, reproducible Sigstore builds
- [x] Phase 2: Policy Engine + Multi-Source Catalogs (9/9) — corroboration semantics, OSV + Socket adapters, lifecycle/path/egress/baseline policies, catalog watch daemon
- [x] Phase 3: Editor Extension Defense (5/5) — agent CLI intercept, fsnotify watcher, quarantine workflow, `beekeeper scan`
- [x] Phase 4: Integration Surfaces (5/5) — Claude Code/Cursor/Codex hook installers, MCP gateway, shim layer, multi-agent observability
- [x] Phase 5: Linux Sentry (5/5) — privileged systemd daemon, fanotify + cilium/ebpf ingestion, 5-rule correlation, 7-day baseline
- [x] Phase 6: LlamaFirewall + Audit Sinks (5/5) — supervised Python sidecar, syslog/OTLP/HTTPS sinks, audit query/export
- [x] Phase 7: Cross-Platform Sentry (5/5) — macOS eslogger, Windows ETW, SLSA Level 3 + CycloneDX SBOM
- [x] Phase 8: TUI Dashboard (9/9) — Bubble Tea v2, all panels, admin mode, Windows resize workaround
- [x] Phase 9: Policy as Code + Self-Defense Capstone (5/5) — declarative JSON policies, layered config, `beekeeper-self` catalog, `beekeeper diag`
- [x] Phase 10: Cross-Phase Integration Closure (1/1) — live corroboration_threshold, gateway corroboration, LlamaFirewall supervisor + scan wiring, diag sidecar latency, overlay coverage
- [x] Phase 11: v1.0.0 PRD-Gap Closure (pre-push) — gateway PromptGuard real tool name, layered config in enforcement, eBPF build-pipeline generation, delta-triggered scan, `catalogs diff`, real Ed25519 catalog signatures

</details>

### v1.1.0 "Pollen" — Windows Inventory Compatibility

- [ ] **Phase 1: Fork Setup & Discipline** — Pollen module established, Apache-2.0 attribution, reproducible builds + Sigstore, differential + selftest CI green; tag `v0.1.1-pollen.1`
- [ ] **Phase 2: Windows Root Resolver** — All 8 ecosystems resolved on Windows, parity test against Linux passes; tag `v0.1.1-pollen.2`
- [ ] **Phase 3: Windows Path Representation** — Native backslash paths and Windows endpoint record in NDJSON, round-trip verified; tag `v0.1.1-pollen.3`
- [ ] **Phase 4: Windows Extension & MCP Coverage** — Editor, browser, MCP config paths complete; beekeeper compat test runs on all 3 OSes with zero skips; tag `v0.1.1-pollen.4`
- [ ] **Phase 5: Contribution-Back & Milestone Close** — Upstream PRs prepared, beekeeper full CI green including honeypot E2E, `pollen-self` in catalog, sync workflow documented; tag `v0.1.1-pollen.5`

## Phase Details

### Phase 1: Fork Setup & Discipline
**Goal**: The `github.com/bantuson/pollen` module exists with correct Apache-2.0 attribution, renamed binary, reproducible builds + Sigstore signing, and CI that guards every subsequent change with a differential test and selftest on all three OSes. No Windows functionality yet — this phase proves fork hygiene before any Windows code lands.
**Repo locus**: Primarily `bantuson/pollen` (new repo). Beekeeper CI is not affected this phase.
**Depends on**: Nothing (first phase)
**Requirements**: FORK-01, FORK-02, FORK-03, FORK-04, PTEST-02, PTEST-03, SDEF-02
**Success Criteria** (what must be TRUE):
  1. `pollen` binary builds and runs on ubuntu/macos/windows from `go install github.com/bantuson/pollen/cmd/pollen@v0.1.1-pollen.1`; `pollen selftest` exits 0 on all three OSes
  2. The CI matrix (ubuntu/macos/windows, Go 1.25.x) runs `go vet`, `go test -race ./...`, and selftest green; the differential test asserts byte-for-byte identical NDJSON output between Pollen and upstream Bumblebee on Linux and macOS
  3. `LICENSE` is verbatim Apache-2.0; `NOTICE` names Perplexity/Bumblebee as origin; `CHANGES.md` records every delta; `UPSTREAM.md` records the pinned 40-char SHA with tag + date + verifier
  4. "Bumblebee" does not appear in any command name, package name, or README headline — only in NOTICE, README attribution paragraph, and UPSTREAM.md
  5. The `v0.1.1-pollen.1` GitHub release carries a Sigstore/cosign signature and a CycloneDX SBOM recording the upstream pinned commit
**Plans**: 5 plans (4 waves)
- [x] 01-01-PLAN.md — Wave 0: fork upstream @ pinned SHA, rewrite module path, rename cmd/bumblebee→cmd/pollen, trademark fixes, build + Windows cross-compile + selftest (FORK-01, FORK-04)
- [ ] 01-02-PLAN.md — Wave 1: Apache-2.0 attribution (LICENSE/NOTICE/CHANGES/UPSTREAM), VERSION, empty threat_intel, full-repo trademark audit (FORK-02, FORK-04)
- [ ] 01-03-PLAN.md — Wave 1: NDJSON normalization harness + TestDifferential vs pinned upstream + selftest 3-finding regression (PTEST-02, PTEST-03)
- [ ] 01-04-PLAN.md — Wave 2: reproducible Makefile + goreleaser (cosign + CycloneDX SBOM), 3-OS CI matrix + differential + govulncheck, release.yml SLSA L3, THREAT-MODEL (FORK-03, SDEF-02)
- [ ] 01-05-PLAN.md — Wave 3: create bantuson/pollen repo, green CI, tag + signed v0.1.1-pollen.1 release, verify signature + SBOM (FORK-03, SDEF-02)

### Phase 2: Windows Root Resolver
**Goal**: Pollen can discover all 8 package-manager roots on Windows — npm/pnpm/Yarn/Bun (JS ecosystems), PyPI, Go modules, RubyGems, and Composer — using `%APPDATA%`/`%LOCALAPPDATA%`/`%USERPROFILE%`/`%ProgramFiles%` environment variables, with the cross-platform parity test asserting equivalent detection counts against Linux.
**Repo locus**: Primarily `bantuson/pollen` — `internal/resolver/resolver_windows.go`.
**Depends on**: Phase 1
**Requirements**: WRES-01, WRES-02, PTEST-01
**Success Criteria** (what must be TRUE):
  1. On a Windows CI runner with the standard fake-package fixture tree, `pollen scan` returns inventory records for all 8 ecosystems with non-empty paths under `%USERPROFILE%` and `%APPDATA%`/`%LOCALAPPDATA%` as appropriate per PRD §8.1
  2. The cross-platform parity test passes on all three OSes: same packages detected, same severity matches, equivalent record counts (modulo OS path strings), `endpoint.os` differs correctly per platform
  3. The differential test continues to pass on Linux and macOS — Windows resolver additions have not drifted Pollen from upstream Bumblebee on the platforms upstream supports
  4. `v0.1.1-pollen.2` is tagged and signed; Windows CI no longer skips root-resolver tests
**Plans**: TBD

### Phase 3: Windows Path Representation
**Goal**: Every NDJSON record emitted by Pollen on Windows carries native Windows paths — backslash separators, drive letters, `endpoint.os="windows"`, correct `arch` and `username`, and empty `uid` — and beekeeper's audit-log consumer handles Windows-shaped endpoint records correctly on round-trip.
**Repo locus**: `bantuson/pollen` (`internal/output/paths_windows.go`) and beekeeper `internal/inventory/` (consumer-side round-trip verification).
**Depends on**: Phase 2
**Requirements**: WPATH-01, WPATH-02
**Success Criteria** (what must be TRUE):
  1. A Windows CI `pollen scan` run produces NDJSON where `project_path` and `source_file` fields contain backslash separators and drive letters (`C:\Users\...`) with no Unix-to-Windows conversion artifacts
  2. The `endpoint` record in every NDJSON output on Windows contains `os="windows"`, `arch` matching `runtime.GOARCH`, a non-empty `username`, and an empty `uid`; Linux/macOS records are unchanged
  3. Beekeeper's audit-log consumer parses a Windows-shaped Pollen NDJSON record without error and round-trips the endpoint fields correctly; a test in `internal/inventory/` asserts this
  4. `v0.1.1-pollen.3` is tagged and signed
**Plans**: TBD

### Phase 4: Windows Extension & MCP Coverage + Beekeeper Compat Test
**Goal**: Pollen enumerates all Windows editor-extension directories (VS Code family), browser-extension profile paths (Chromium + Firefox), and MCP host-config files (Claude Desktop, Cursor, Windsurf, Cline, Gemini CLI); and beekeeper's Pollen compatibility test runs on all three OSes with a Windows skip count of zero.
**Repo locus**: `bantuson/pollen` for extension/MCP paths; beekeeper `internal/inventory/` for the subprocess boundary + compat test (BKINT-01, PTEST-04).
**Depends on**: Phase 3
**Requirements**: WEXT-01, WEXT-02, WEXT-03, BKINT-01, PTEST-04
**Success Criteria** (what must be TRUE):
  1. On Windows CI, `pollen scan` detects fake VS Code, Code Insiders, Cursor, Windsurf, and VSCodium extensions planted under `%USERPROFILE%` fixture trees; all five editor paths return inventory records
  2. On Windows CI, `pollen scan` detects fake Chrome/Chromium/Edge/Brave (per-profile) and Firefox (per-profile `extensions.json`) browser extensions; all Chromium-family and Firefox paths return records
  3. On Windows CI, `pollen scan` finds fake MCP config files at `%APPDATA%\Claude\`, `%USERPROFILE%\.cursor\mcp.json`, `%USERPROFILE%\.windsurf\mcp.json`, `%APPDATA%\cline\`, and `%USERPROFILE%\.gemini\settings.json`
  4. Beekeeper's Pollen compatibility test (invoking `pollen scan`, asserting NDJSON schema-consistency, running beekeeper rules, asserting `scanner_name`) runs green on ubuntu/macos/windows with zero `t.Skip` calls — the beekeeper Windows CI skip baseline for inventory tests is zero
  5. `v0.1.1-pollen.4` is tagged and signed
**Plans**: TBD
**UI hint**: no

### Phase 5: Contribution-Back & Milestone Close
**Goal**: Windows additions are prepared as upstream-shaped PRs against `perplexityai/bumblebee`; beekeeper's full CI matrix is green on all three OSes including the Sentry honeypot E2E test on Windows; `pollen-self` entries protect against compromised Pollen releases; the upstream sync workflow is documented and operational.
**Repo locus**: `bantuson/pollen` (sync workflow, UPSTREAM.md, upstream PRs); beekeeper (BKINT-02, PTEST-05, SDEF-01 entry in `beekeeper-self` catalog).
**Depends on**: Phase 4
**Requirements**: SYNC-01, SYNC-02, BKINT-02, PTEST-05, SDEF-01
**Success Criteria** (what must be TRUE):
  1. `UPSTREAM.md` documents a repeatable, step-by-step sync workflow (fetch upstream, diff review, test on Linux/macOS, cherry-pick preserving Windows paths, re-run differential test, update pinned commit, tag new Pollen release) that a second maintainer could follow without prior context
  2. At least one upstream-shaped PR is open against `perplexityai/bumblebee` covering the Windows root resolver (or an existing PR such as #4/#16 is commented with test evidence and Windows CI results from this work); the PR references the Pollen tag and links parity-test output as evidence
  3. Beekeeper's `go.mod` pins Pollen at an explicit version; beekeeper CI installs Pollen and all inventory-related tests pass on ubuntu/macos/windows with zero skips — Windows Bumblebee-dependency tests that previously required `t.Skip` now run green via Pollen
  4. The Windows Sentry honeypot E2E test — a planted process tree reading synthetic `%USERPROFILE%\.aws\credentials` and making an outbound connection — fires beekeeper's exfil-signature-fusion rule on the Windows CI runner and the test asserts the expected alert is emitted
  5. `beekeeper-self` catalog contains `pollen-self` entries so that a known-bad Pollen release is detectable by beekeeper's self-quarantine mechanism; `beekeeper selftest` passes with the extended catalog
  6. `v0.1.1-pollen.5` is tagged and signed — the milestone-complete tag
**Plans**: TBD

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Foundation + Hook Handler | v1.0.0 | 1/5 | In Progress|  |
| 2. Policy Engine + Multi-Source Catalogs | v1.0.0 | 9/9 | Complete | 2026-05-26 |
| 3. Editor Extension Defense | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 4. Integration Surfaces | v1.0.0 | 5/5 | Complete | 2026-05-27 |
| 5. Linux Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 6. LlamaFirewall + Audit Sinks | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 7. Cross-Platform Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 8. TUI Dashboard | v1.0.0 | 9/9 | Complete | 2026-05-29 |
| 9. Policy as Code + Self-Defense Capstone | v1.0.0 | 5/5 | Complete | 2026-05-29 |
| 10. Cross-Phase Integration Closure | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| 11. v1.0.0 PRD-Gap Closure (pre-push) | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| **1. Fork Setup & Discipline** | **v1.1.0** | **0/TBD** | **Not started** | **—** |
| **2. Windows Root Resolver** | **v1.1.0** | **0/TBD** | **Not started** | **—** |
| **3. Windows Path Representation** | **v1.1.0** | **0/TBD** | **Not started** | **—** |
| **4. Windows Extension & MCP Coverage** | **v1.1.0** | **0/TBD** | **Not started** | **—** |
| **5. Contribution-Back & Milestone Close** | **v1.1.0** | **0/TBD** | **Not started** | **—** |
