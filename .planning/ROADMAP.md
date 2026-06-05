# Roadmap: Beekeeper

## Milestones

- ✅ **v1.0.0 — Comprehensive Standalone Release** — Phases 1–10 (shipped 2026-06-01)
  Full per-phase detail archived in [`milestones/v1.0.0-ROADMAP.md`](milestones/v1.0.0-ROADMAP.md).
  Audit: PASSED — [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).

- 🔄 **v1.1.0 — "Pollen"** — Phases 1–5 (in progress)
  Goal: Own Windows inventory compatibility via a bounded Apache-2.0 Bumblebee derivative so the Windows CI matrix goes fully green.

- ✅ **v1.2.0 — "Runtime Behavioral Hardening"** — Phases 6–9 (shipped 2026-06-04)
  Full per-phase detail archived in [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).
  Audit: `tech_debt` (no blockers) — all findings cleared by the inserted Phase 9 — [`milestones/v1.2.0-MILESTONE-AUDIT.md`](milestones/v1.2.0-MILESTONE-AUDIT.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).

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

- [x] **Phase 1: Fork Setup & Discipline** — Pollen module established, Apache-2.0 attribution, reproducible builds + Sigstore, differential + selftest CI green; tag `v0.1.1-pollen.1`
- [~] **Phase 2: Windows Root Resolver** — All 8 ecosystems resolved on Windows, parity test passes (code complete & verified); tag `v0.1.1-pollen.2` **deferred to M2 close**
- [~] **Phase 3: Windows Path Representation** — Native backslash paths and Windows endpoint record in NDJSON, round-trip verified (code complete & verified 2026-06-02); tag `v0.1.1-pollen.3` **deferred to M2 close**
- [~] **Phase 4: Windows Extension & MCP Coverage** — Editor, browser, MCP config paths complete; beekeeper compat test runs on all 3 OSes with zero skips (code complete & verified 2026-06-02); tag `v0.1.1-pollen.4` **deferred to M2 close**
- [ ] **Phase 5: Contribution-Back & Milestone Close** — Upstream PRs prepared, beekeeper full CI green including honeypot E2E, `pollen-self` in catalog, sync workflow documented; tag `v0.1.1-pollen.5`

<details>
<summary>✅ v1.2.0 Runtime Behavioral Hardening (Phases 6–9) — SHIPPED 2026-06-04</summary>

- [x] Phase 6: Corroboration Severity Hardening (3/3 plans) — critical malware blocks at one trusted source, anti-poisoning sanity gate (F1)
- [x] Phase 7: Sensitive-Path Runtime Enforcement (3/3) — `EvaluatePath` wired live into `check`; canonicalization closes traversal/tilde/Windows-env bypasses (F2)
- [x] Phase 8: Package-Manager Nudge + Behavioral Test Suite (8/8) — `pkgparse` closes F3; nudge soft-advise/hard-rewrite + CLI; hermetic live-binary E2E release gate
- [x] Phase 9: v1.2.0 Tech-Debt Cleanup (5/5 +1 fix) — hermetic CORR E2E gate, LoadLayered Nudge merge, SPATH evasion hardening, live version_drift, Phase-6 Nyquist reconcile (inserted from audit; verified 9/9)

Full detail: [`milestones/v1.2.0-ROADMAP.md`](milestones/v1.2.0-ROADMAP.md).

</details>

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
- [x] 01-02-PLAN.md — Wave 1: Apache-2.0 attribution (LICENSE/NOTICE/CHANGES/UPSTREAM), VERSION, empty threat_intel, full-repo trademark audit (FORK-02, FORK-04)
- [x] 01-03-PLAN.md — Wave 1: NDJSON normalization harness + TestDifferential vs pinned upstream + selftest 3-finding regression (PTEST-02, PTEST-03)
- [x] 01-04-PLAN.md — Wave 2: reproducible Makefile + goreleaser (cosign + CycloneDX SBOM), 3-OS CI matrix + differential + govulncheck, release.yml SLSA L3, THREAT-MODEL (FORK-03, SDEF-02)
- [x] 01-05-PLAN.md — Wave 3: create bantuson/pollen repo, green CI, tag + signed v0.1.1-pollen.1 release, verify signature + SBOM (FORK-03, SDEF-02)

### Phase 2: Windows Root Resolver

**Goal**: Pollen can discover all 8 package-manager roots on Windows — npm/pnpm/Yarn/Bun (JS ecosystems), PyPI, Go modules, RubyGems, and Composer — using `%APPDATA%`/`%LOCALAPPDATA%`/`%USERPROFILE%`/`%ProgramFiles%` environment variables, with the cross-platform parity test asserting equivalent detection counts against Linux.
**Repo locus**: Primarily `bantuson/pollen` — `cmd/pollen/roots_windows.go` (//go:build windows; in-place minimal-diff per RESEARCH Option B — the PRD-draft `internal/resolver/` path does not exist in the live repo).
**Depends on**: Phase 1
**Requirements**: WRES-01, WRES-02, PTEST-01
**Success Criteria** (what must be TRUE):

  1. On a Windows CI runner with the standard fake-package fixture tree, `pollen scan` returns inventory records for all 8 ecosystems with non-empty paths under `%USERPROFILE%` and `%APPDATA%`/`%LOCALAPPDATA%` as appropriate per PRD §8.1
  2. The cross-platform parity test passes on all three OSes: same packages detected, same severity matches, equivalent record counts (modulo OS path strings), `endpoint.os` differs correctly per platform
  3. The differential test continues to pass on Linux and macOS — Windows resolver additions have not drifted Pollen from upstream Bumblebee on the platforms upstream supports
  4. `v0.1.1-pollen.2` is tagged and signed; Windows CI no longer skips root-resolver tests

**Plans**: 4 plans (3 waves)

- [x] 02-01-PLAN.md — Wave 1: roots_windows.go (8-ecosystem Windows root table) + case "windows": wiring in roots.go + isBroadHomeRoot drive-root branch (WRES-01, WRES-02)
- [x] 02-02-PLAN.md — Wave 2: roots_windows_test.go (Windows unit tests) + flip the 6 Phase-2 skips in main_test.go (WRES-01, WRES-02 verification)
- [x] 02-03-PLAN.md — Wave 2: parity_test.go + testdata/parity-fixture/ 8-ecosystem fixture (PTEST-01)
- [~] 02-04-PLAN.md — Wave 3: VERSION bump 0.1.1-pollen.2 + CHANGES.md **prepared & committed locally** (`../pollen` HEAD `c94b271`); **tag + Sigstore signing DEFERRED to M2 close** by maintainer decision (Success Criterion 4 gate)

> **⏸ Pending release (future context):** Phase 2 code is complete and verified (SC1–SC3 ✅; SC4 skips-flipped ✅, signed tag deferred). The `v0.1.1-pollen.2` signed release is intentionally batched to **end of Milestone 2**. 4 commits sit unpushed on `../pollen` `main`. Cut it via: `git push origin main` → confirm 3-OS CI green → `git tag -a v0.1.1-pollen.2 …` + push → `cosign verify-blob`. Full commands in `.planning/phases/02-windows-root-resolver/02-04-SUMMARY.md` and the STATE.md Deferred Items table.

### Phase 3: Windows Path Representation

**Goal**: Every NDJSON record emitted by Pollen on Windows carries native Windows paths — backslash separators, drive letters, `endpoint.os="windows"`, correct `arch` and `username`, and empty `uid` — and beekeeper's audit-log consumer handles Windows-shaped endpoint records correctly on round-trip.
**Repo locus**: `bantuson/pollen` — WPATH-01 fix lives in the ecosystem scanners `internal/ecosystem/npm/npm.go` + `internal/ecosystem/pnpm/pnpm.go` (NOT `internal/output/` — that package is a thin JSON encoder; verified in 03-RESEARCH.md), WPATH-02 in `internal/endpoint/endpoint.go`; beekeeper round-trip test co-located in `internal/scan/scanner_test.go` (NOT a new `internal/inventory/` package — that boundary is Phase 4 BKINT-01).
**Depends on**: Phase 2
**Requirements**: WPATH-01, WPATH-02
**Success Criteria** (what must be TRUE):

  1. A Windows CI `pollen scan` run produces NDJSON where `project_path` and `source_file` fields contain backslash separators and drive letters (`C:\Users\...`) with no Unix-to-Windows conversion artifacts
  2. The `endpoint` record in every NDJSON output on Windows contains `os="windows"`, `arch` matching `runtime.GOARCH`, a non-empty `username`, and an empty `uid`; Linux/macOS records are unchanged
  3. Beekeeper's audit-log consumer parses a Windows-shaped Pollen NDJSON record without error and round-trips the endpoint fields correctly; a test in `internal/scan/` asserts this (locus corrected from `internal/inventory/` per 03-RESEARCH.md — `internal/inventory/` is Phase 4's BKINT-01 boundary)
  4. `v0.1.1-pollen.3` is tagged and signed (DEFERRED to M2 close per D-06 / Phase-2 precedent — plan 03-03 prepares VERSION + CHANGES.md locally; the signed git tag is batched to milestone close)

**Plans**: 3 plans (2 waves)

- [x] 03-01-PLAN.md — Wave 1 (Pollen): WPATH-01 — wrap npm.go + pnpm.go projectPath join in filepath.FromSlash (backslash on Windows, Unix-identity) + Windows-gated unit tests
- [x] 03-02-PLAN.md — Wave 1 (Pollen): WPATH-02 — guard both endpoint.Current() UID assignments with runtime.GOOS != "windows" (empty uid on Windows, unchanged on Unix) + endpoint tests
- [x] 03-03-PLAN.md — Wave 2 (both repos): parity_test.go Windows path-shape + empty-uid assertions, beekeeper internal/scan round-trip test (D-03), VERSION/CHANGES bump to 0.1.1-pollen.3 (no tag)

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
  5. `v0.1.1-pollen.4` is tagged and signed (DEFERRED to M2 close per D-06 — plan 04-03 prepares VERSION + CHANGES.md locally; the signed git tag is batched to milestone close)

**Plans**: 3 plans (2 waves)

- [x] 04-01-PLAN.md — Wave 1 (Pollen): WEXT-01/02/03 — fill browser `case "windows":` skeletons + Windows MCP roots (%APPDATA%\Claude, %APPDATA%\cline) + unconditional .windsurf MCP root + .vscode-oss editor segment; TestWindowsExtensionMCPRoots
- [x] 04-02-PLAN.md — Wave 1 (beekeeper): BKINT-01 in-place rename runBumblebeeFn→runPollenFn in internal/scan/scanner.go + PTEST-04 TestPollenCompatibility (5 record types, scanner_name=pollen, no double-counting, zero t.Skip)
- [~] 04-03-PLAN.md — Wave 2 (Pollen): VERSION bump 0.1.1-pollen.4 + CHANGES.md WEXT section **prepared & committed locally** (`../pollen` HEAD `a9db7b3`); **tag + Sigstore signing DEFERRED to M2 close** per D-06 (batched with pollen.2 + pollen.3)

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

**Plans**: 5 plans (3 waves)

- [x] 05-01-PLAN.md — Wave 1 (beekeeper): PTEST-05 — fix isSensitivePath (filepath.ToSlash, Windows backslash bug) + Windows honeypot E2E (TestHoneypotExfilFusion fires SENTRY-005)
- [x] 05-02-PLAN.md — Wave 1 (beekeeper): SDEF-01 — pollen-self entries in the unified beekeeper-self catalog + selftest fixture (non-production version)
- [x] 05-03-PLAN.md — Wave 1 (pollen): SYNC-01 UPSTREAM.md sync workflow + version history; SYNC-02 contribution-back-deferred rationale (D-2); pollen.5 VERSION/CHANGES prep
- [x] 05-04-PLAN.md — Wave 2 (beekeeper): BKINT-02 — CI go install Pollen pin (v0.1.1-pollen.4) + PinnedPollenVersion const + D-5 release runbook
- [ ] 05-05-PLAN.md — Wave 3 (CHECKPOINT, autonomous:false): maintainer pushes both repos + cuts four signed tags (pollen.2/.3/.4/.5) + cosign verify; CI-green confirmation; milestone close

> **SC2 relaxed (D-2):** No upstream contribution-back PRs against perplexityai/bumblebee this milestone (upstream Windows-support path unviable; PRs #3/#4 ignored). SYNC-02 is satisfied-by-documented-deferral in UPSTREAM.md (05-03). The verifier MUST NOT flag the absence of an upstream PR as a Phase-5 gap.

## Progress

| Phase | Milestone | Plans | Status | Completed |
|-------|-----------|-------|--------|-----------|
| 1. Foundation + Hook Handler | v1.0.0 | 4/5 | In Progress|  |
| 2. Policy Engine + Multi-Source Catalogs | v1.0.0 | 3/4 | In Progress|  |
| 3. Editor Extension Defense | v1.0.0 | 5/5 | Complete | 2026-05-26 |
| 4. Integration Surfaces | v1.0.0 | 2/3 | In Progress|  |
| 5. Linux Sentry | v1.0.0 | 4/5 | In Progress|  |
| 6. LlamaFirewall + Audit Sinks | v1.0.0 | 3/3 | Complete    | 2026-06-03 |
| 7. Cross-Platform Sentry | v1.0.0 | 5/5 | Complete | 2026-05-28 |
| 8. TUI Dashboard | v1.0.0 | 9/9 | Complete | 2026-05-29 |
| 9. Policy as Code + Self-Defense Capstone | v1.0.0 | 5/5 | Complete | 2026-05-29 |
| 10. Cross-Phase Integration Closure | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| 11. v1.0.0 PRD-Gap Closure (pre-push) | v1.0.0 | 1/1 | Complete | 2026-06-01 |
| **1. Fork Setup & Discipline** | **v1.1.0** | **0/TBD** | **Not started** | **—** |
| **2. Windows Root Resolver** | **v1.1.0** | **3/4** | **Code complete — release deferred to M2 close** | **—** |
| **3. Windows Path Representation** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred to M2 close** | **2026-06-02** |
| **4. Windows Extension & MCP Coverage** | **v1.1.0** | **3/3** | **Code complete & verified — release deferred to M2 close** | **2026-06-02** |
| **5. Contribution-Back & Milestone Close** | **v1.1.0** | **0/5** | **Planned** | **—** |
| **6. Corroboration Severity Hardening** | **v1.2.0** | **3/3** | **Complete** | **2026-06-03** |
| **7. Sensitive-Path Runtime Enforcement** | **v1.2.0** | **3/3** | **Complete** | **2026-06-04** |
| **8. Package-Manager Nudge + Behavioral Test Suite** | **v1.2.0** | **8/8** | **Complete** | **2026-06-04** |
| **9. v1.2.0 Tech-Debt Cleanup** | **v1.2.0** | **5/5** | **Complete & verified (9/9)** | **2026-06-04** |
| **10. Hook-Block Protocol Compliance & Multi-Harness Enforcement** | **v1.3.0 (next)** | **2/6** | **Executing** | **—** |

### Phase 10: Hook-Block Protocol Compliance & Multi-Harness Enforcement

> **Seeds the next milestone (v1.3.0 — proposed).** v1.2.0 is archived (phases 6–9) and v1.1.0 is parked; this phase continues the live `.planning/phases/` numbering (→ 10) but is NOT part of shipped v1.2.0. Reframe under a v1.3.0 milestone heading via `/gsd-new-milestone` if desired before planning.

**Goal**: Beekeeper's PreToolUse hook must ACTUALLY block a denied tool call across supported agent harnesses — not merely detect + audit it. A live dogfood (2026-06-05) proved the shipped hook fires and decides "block" but the harness runs the tool anyway, because `beekeeper check` exits `1` (+ its own JSON) while every harness requires exit code `2` or a per-harness deny JSON. This phase adds a `beekeeper check --hook <harness>` deny adapter, fixes/extends the per-harness installers (15 harnesses), routes no-hook harnesses to the MCP gateway, and adds the missing release gate that asserts the *harness* deny contract.

**Repo locus**: `internal/check` (hook-output adapter + tests), `cmd/beekeeper` (`--hook` flag), `internal/hooks/*` (per-harness installers), `internal/gateway` (routing for no-hook harnesses), `docs/` (support matrix).
**Depends on**: Phase 9 (shipped). Folds in 4 prerequisite fixes already done & tested this session but UNCOMMITTED (detectionTimeout 2→3s, execTimeout 5→8s, `claude_code.go` clobber→merge + regression test).
**Research**: `.planning/phases/10-hook-block-protocol-compliance-and-multi-harness-enforcement/10-RESEARCH.md` — the bug, live evidence, and the full 15-harness deny-contract matrix + citations. **Read it before planning.** Phase context: `10-CONTEXT.md` (same folder).

**Requirements**:

- **HPC-01** `beekeeper check --hook <harness>` adapter: on block emit exit 2 + stderr reason (universal baseline) plus the per-harness JSON; non-block → exit 0 (defer to the harness's own permission flow; never emit "allow"). Default (no flag) unchanged = raw Decision JSON + exit 1 (shim/gateway/tests). Pure, table-driven.
- **HPC-02** Per-harness installer correctness: correct command (`--hook <name>`), event name(s), config format/location, feature flags. MUST fix the Cursor event-name bug (`preToolUse` → `beforeShellExecution`/`beforeMCPExecution`/`beforeReadFile`) and Codex `[features] hooks=true`; merge-not-clobber + targeted uninstall for every settings.json-style target; add the new harnesses.
- **HPC-03** Per-harness deny-contract regression tests — the gate that was missing (assert the harness deny output + exit code, not just Beekeeper's internal exit code).
- **HPC-04** Live re-verification on Claude Code (only locally-installed harness): a credential read is actually DENIED end-to-end after install — not just audited.
- **HPC-05** No-hook harnesses → MCP gateway (Kilo, Trae); ship an OpenCode plugin (`tool.execute.before` → throw); document Hermes fail-open + Cline-no-Windows caveats; publish a Tier 1/2/3 support matrix (no overclaiming).
- **HPC-06** Self-defense: a release-gate test proving the harness deny contract holds for the shipped binary.

**Success Criteria** (what must be TRUE):

  1. On Claude Code, a credential-read tool call is BLOCKED live (tool does not execute), verified end-to-end — not just an audit record.
  2. `beekeeper check --hook <harness>` emits the correct deny signal (exit 2 + per-harness JSON) for each Tier-1 harness, proven by unit tests; default mode still exits 1 with raw JSON (shim/gateway/tests unbroken).
  3. Installers write the correct event names + config + feature flags per harness and never clobber a user's existing hooks.
  4. No-hook harnesses (Kilo, Trae) documented + routed to the MCP gateway; OpenCode plugin shipped; Hermes/Cline/Windows caveats documented; a Tier 1/2/3 support matrix published.
  5. A release-gate test asserts the harness deny contract (exit 2 / deny JSON), closing the gap that let exit-1 ship.

**Plans**: 6 plans across 5 waves (planned 2026-06-05).
**Wave 1**

- [x] 10-01-PLAN.md — RenderDeny pure adapter + --hook flag + fixed Claude installer + the missing deny-contract regression gate (HPC-01/03/06) [wave 1] ✅ 2026-06-05 (d7bffe3, 76029a3, 342d78a)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 10-02-PLAN.md — Live Claude Code end-to-end block re-proof (HPC-04) [wave 2, checkpoint]
- [x] 10-03-PLAN.md — Fix Cursor event-name bug + Codex features flag; add Augment/CodeBuddy/Qwen installers (HPC-02/03) [wave 2] ✅ 2026-06-05 (95ee9ed, a4211d0, 978fc4e)

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 10-04-PLAN.md — Copilot/Gemini/Antigravity/Windsurf installers, non-Claude deny families (HPC-02/03) [wave 3]

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 10-05-PLAN.md — Hermes (fail-open JSON-only) + Cline (no-Windows) + OpenCode plugin (HPC-02/03/05) [wave 4]

**Wave 5** *(blocked on Wave 4 completion)*

- [ ] 10-06-PLAN.md — Kilo/Trae MCP-gateway routing + honest Tier 1/2/3 support matrix (HPC-05) [wave 5]
