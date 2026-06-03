# Roadmap: Beekeeper

## Milestones

- ✅ **v1.0.0 — Comprehensive Standalone Release** — Phases 1–10 (shipped 2026-06-01)
  Full per-phase detail archived in [`milestones/v1.0.0-ROADMAP.md`](milestones/v1.0.0-ROADMAP.md).
  Audit: PASSED — [`milestones/v1.0.0-MILESTONE-AUDIT.md`](milestones/v1.0.0-MILESTONE-AUDIT.md).
  Summary: [`MILESTONES.md`](MILESTONES.md).
- 🔄 **v1.1.0 — "Pollen"** — Phases 1–5 (in progress)
  Goal: Own Windows inventory compatibility via a bounded Apache-2.0 Bumblebee derivative so the Windows CI matrix goes fully green.
- 📋 **v1.2.0 — "Runtime Behavioral Hardening"** — Phases 6–8 (planned)
  Goal: Close three runtime-enforcement gaps (credential reads, critical-malware warn-only, pnpm/bun bypass) locked in by a behavioral test suite.

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

### v1.2.0 "Runtime Behavioral Hardening"

- [x] **Phase 6: Corroboration Severity Hardening** — Per-severity escalation so critical malware blocks at one trusted source; anti-poisoning sanity gate (completed 2026-06-03)
- [ ] **Phase 7: Sensitive-Path Runtime Enforcement** — Wire existing path engine into live `beekeeper check`; canonicalization adapter closes traversal bypasses
- [ ] **Phase 8: Package-Manager Nudge + Behavioral Test Suite** — Full nudge feature (detect/evaluate/rewrite/CLI); PRD §10 acceptance tests; live-binary E2E release gate

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

---

## Milestone v1.2.0 — Runtime Behavioral Hardening

**Milestone Goal:** Close three runtime-enforcement gaps surfaced by live `beekeeper check` validation — a critical-severity malware package that warns instead of blocks (F1), credential files that return ALLOW to agent reads (F2), and pnpm/bun installs that bypass catalog matching entirely (F3) — each locked in by a behavioral test suite proving the wiring is live.

**Phases (continuing numbering from v1.1.0):**

- [x] **Phase 6: Corroboration Severity Hardening** — Per-severity escalation so critical malware blocks at one trusted source; anti-poisoning sanity gate (completed 2026-06-03)
- [ ] **Phase 7: Sensitive-Path Runtime Enforcement** — Wire existing path engine into live `beekeeper check`; canonicalization adapter closes traversal bypasses; integration tests prove check wiring is live
- [ ] **Phase 8: Package-Manager Nudge + Behavioral Test Suite** — Full nudge feature (detect/parse/evaluate/rewrite/CLI); PRD §10 acceptance tests; complete table-driven test suite; live-binary E2E release gate

### Phase 6: Corroboration Severity Hardening
**Goal**: A critical-severity catalog match blocks at a single trusted source — so `ai-figure` (Shai-Hulud / OSV `MAL-2026-4126`) is blocked, not warned — with an anti-poisoning sanity gate that prevents a degraded or flooded catalog from triggering false escalations.
**Depends on**: Phase 5 (v1.1.0) or Phase 10/11 (v1.0.0) — `internal/policy/corroboration.go` and `catalog/sanity.go` must be present
**Requirements**: CORR-01, CORR-02
**Success Criteria** (what must be TRUE):
  1. `beekeeper check` with a Bumblebee-matched + OSV-confirmed `ai-figure` install returns exit 1 (block) and audit `decision:"block"` — previously exit 0 warn-only
  2. When the catalog sanity check reports a degraded state (>1000 alert-delta entries injected), the critical-severity escalation does NOT activate and the same match returns warn-only — proving the anti-poisoning gate is live
  3. A catalog entry with `versions:["*"]` and `severity:"critical"` still requires 2-source corroboration to block — the all-versions guard prevents a mis-tagged wildcard entry from blocking all installs of a legitimate package (e.g. `react`, `typescript`)
  4. `validateCorroborationThresholds` rejects any configuration where `BlockAt < 1` with a descriptive error at startup
  5. Table-driven unit tests in `internal/policy/` cover the Shai-Hulud fixture (1-source critical → block), degraded-catalog regression (1001 entries → warn), and all-versions guard (wildcard + critical → 2-source required)
**Plans**: 3 plans (2 waves)
- [x] 06-01-PLAN.md - Wave 1: pure policy core - SeverityThreshold/SeverityOverrides/CatalogHealthy types, findSeverityOverride + all-versions guard, sanity-bound validateCorroborationThresholds, 6 unit tests, selftest fixture audit (CORR-01, CORR-02)
- [x] 06-02-PLAN.md - Wave 2: policy-file configurable critical_block_at (loader/validate/test merge + bound) + loader test (CORR-01)
- [x] 06-03-PLAN.md - Wave 2: resolveCatalogHealthy wired into all four policy.Evaluate consumers (check/gateway/watch/scan) + 3 RunCheck integration tests proving live wiring (CORR-02)

### Phase 7: Sensitive-Path Runtime Enforcement
**Goal**: `beekeeper check` blocks agent reads of credential files — `~/.aws/credentials`, `~/.ssh/id_rsa`, `.env`, and MCP config files — via the already-built `policy.EvaluatePath`/`DefaultSensitivePaths` engine wired into the live check pipeline, with path canonicalization that closes `..`-traversal and tilde-expansion bypasses.
**Depends on**: Phase 6
**Requirements**: SPATH-01, SPATH-02, SPATH-03, SPATH-04
**Success Criteria** (what must be TRUE):
  1. `beekeeper check` with stdin `{"tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}` returns exit 1 (block) — previously exit 0 allow; the same test with `{"file_path":"../../.aws/credentials"}` also blocks (traversal closed)
  2. A `Bash` tool call containing `cat ~/.ssh/id_rsa` or `type %USERPROFILE%\.ssh\id_rsa` is detected as a credential-access attempt and flagged — shell-command target extraction is live, not just direct `file_path` reads
  3. Safe lookalikes `.env.example`, `.env.test`, and `.env.schema` are NOT blocked — the built-in allowlist prevents false positives on development fixture files; project-level policy-file `sensitive_path.allow` rules merge by most-restrictive-wins
  4. `RunCheck` integration tests drive the full stdin-to-exit-code path for credential reads (SPATH-01/02/03) and assert both the exit code and the presence of a `decision:"block"` audit record — wiring is proven live, not just that `EvaluatePath` returns the correct value in isolation
**Plans**: 3 plans (2 waves)

**Wave 1** *(parallel — no inter-plan `files_modified` overlap: `internal/policy`+`internal/policyloader` vs new `internal/check/paths.go`)*
- [ ] 07-01-PLAN.md — Pure policy + policyloader fixes (DefaultSensitivePaths block/allow entries, isAllowedPath basename match, extractTargetPath file_path key)
- [ ] 07-02-PLAN.md — Impure canonicalization adapter (internal/check/paths.go: extract / canonicalize / %USERPROFILE% expand / Bash credential detection / mergeDecisions)

**Wave 2** *(blocked on Wave 1 — imports the 07-02 adapter and depends on the 07-01 allowlist/basename fix)*
- [ ] 07-03-PLAN.md — Wire path block into runCheck + runCheckWithIndex; RunCheck integration tests for SC1-SC4 with audit-record assertions

**Cross-cutting constraints** *(truths appearing in ≥2 plans):*
- `internal/policy/path.go` stays pure (no I/O imports — `TestPathImportsArePure`); all FS/env I/O confined to `internal/check/paths.go` (07-01, 07-02)
- The path block fires independently of catalog matching and emits an NDJSON `decision:"block"` audit record (SC4 — 07-02 adapter + 07-03 wiring/tests)
**UI hint**: no

### Phase 8: Package-Manager Nudge + Behavioral Test Suite
**Goal**: Agents running `npm install` are steered toward pnpm (>=11) or bun (>=1.3) — soft-advise by default, hard-rewrite on opt-in — with pnpm/bun install commands now parsed and catalog-matched; the full behavioral test suite (PRD §10, table-driven pure-policy tests, check-handler integration, live-binary E2E) passes as the v1.2.0 release gate.
**Depends on**: Phase 7
**Requirements**: NUDGE-01, NUDGE-02, NUDGE-03, NUDGE-04, NUDGE-05, NUDGE-06, NUDGE-07, NUDGE-08, BTEST-01, BTEST-02, BTEST-03
**Success Criteria** (what must be TRUE):
  1. `beekeeper check` parses `pnpm add malware-pkg` and `bun add malware-pkg` install commands and applies catalog matching — packages that were previously bypassed (F3 gap) now surface in corroboration decisions and audit records
  2. When pnpm >= 11 is locally installed, an agent `npm install foo` call receives an advisory message and proceeds (soft mode, default); when `nudge.mode:"hard"` is set, the command is rewritten to `pnpm add foo` and the agent reissues it; when no hardened PM is installed and `requireHardened:true`, the call is blocked with a structured reason
  3. PRD §10 acceptance criteria 1–10 and 14–17 pass as table-driven tests against `nudge.Evaluate` — covering Advise/Rewrite/Proceed/Block decisions, reason codes, sudo passthrough, detection timeout graceful fallback, `bunfig.toml` parse failure safety, audit record schema, and config-change audit logging
  4. A live-binary E2E test executes the compiled `beekeeper` binary against the real catalog with raw stdin JSON for credential reads (SPATH), the `ai-figure` critical install (CORR), and a `pnpm add` / `bun add` install (NUDGE); all three assert the correct exit code and a well-formed `decision` audit record — this test is the v1.2.0 release gate and must pass before any release tag is cut
  5. `beekeeper nudge status` outputs human-readable current PM state and config; `beekeeper nudge check "npm install chalk"` dry-runs the nudge decision; `beekeeper nudge audit --since=1h` queries nudge records from the audit log
**Plans**: TBD
**UI hint**: no

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
| **7. Sensitive-Path Runtime Enforcement** | **v1.2.0** | **0/3** | **Planned** | **—** |
| **8. Package-Manager Nudge + Behavioral Test Suite** | **v1.2.0** | **0/TBD** | **Not started** | **—** |
