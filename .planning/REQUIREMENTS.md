# Requirements: Beekeeper — Milestone v1.2.0 "Runtime Behavioral Hardening"

**Defined:** 2026-06-03
**Core Value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Milestone Goal:** Close the three runtime-enforcement gaps that live `beekeeper check` validation exposed — credential-file access flagging, package-manager nudging, and critical-severity corroboration — each locked in by a behavioral test suite.
**Scope source:** `.planning/specs/NUDGE-PRD.md` (nudge feature) + runtime-validation findings F1/F2/F3 + `.planning/research/SUMMARY.md`.

> ⏸ **Parked milestone:** v1.1.0 "Pollen" is paused at its release checkpoint (not closed). Its requirements are preserved at `.planning/milestones/v1.1.0-REQUIREMENTS.md`; release resume via `docs/release-runbook.md` + `HANDOFF.json`.

## v1 Requirements

Requirements for milestone v1.2.0. Each maps to exactly one roadmap phase. All work is in `beekeeper` core (not the Pollen fork). `internal/policy` stays a pure function library — detection/normalization I/O lives in adapters, mirroring `policy.EvaluateReleaseAge`.

### Sensitive-Path Runtime Enforcement (SPATH) — finding F2

- [ ] **SPATH-01**: `beekeeper check` blocks (fail-closed) an agent tool call whose `file_path` target (Read/Write/Edit) is a sensitive credential path — `~/.ssh/*`, `~/.aws/*`, `~/.gnupg/*`, `~/.npmrc`, `~/.pypirc`, `~/.cargo/credentials*`, `.env`/`.env.*`, and MCP host-config files — by wiring the existing `policy.EvaluatePath`/`DefaultSensitivePaths` engine into the live check pipeline (currently referenced only by its own test).
- [ ] **SPATH-02**: Path targets are canonicalized before evaluation (tilde expansion, `filepath.Abs`, `EvalSymlinks`, slash normalization) so `..`-traversal (`../../.aws/credentials`), relative paths, `~`, and Windows backslash/drive forms cannot bypass the blocklist.
- [ ] **SPATH-03**: Credential access via shell-command targets (`cat`/`type`/`Get-Content`/`gc` of a sensitive path inside a `Bash` tool call) is detected and flagged, not just direct `file_path` reads.
- [ ] **SPATH-04**: A default allowlist prevents false positives on safe lookalikes (`.env.example`, `.env.test`, `.env.schema`); built-in defaults merge with project/user policy-file `sensitive_path` rules and allowlist by most-restrictive-wins with an allowlist escape hatch.

### Package-Manager Nudge (NUDGE) — finding F3, spec `.planning/specs/NUDGE-PRD.md`

- [ ] **NUDGE-01**: npm install commands (`npm install`/`i`/`add`, `npx`) are recognized, and `pnpm`/`bun`/`yarn` install commands are likewise parsed so catalog matching applies to them too (closes the F3 bypass where pnpm/bun installs were unparsed).
- [ ] **NUDGE-02**: Beekeeper detects locally-installed pnpm (>=11), bun (>=1.3), node (>=22), and the `@socketsecurity/bun-security-scanner`, producing a `PMState` via a timeout-bounded (2s) detection adapter; `nudge.Evaluate(ParsedCommand, PMState, Config)` is a pure decision (no I/O).
- [ ] **NUDGE-03**: Soft mode (default) advises steering `npm install` toward the hardened equivalent — correct verb/flag mapping incl. no-arg `npm install`→`pnpm install`/`bun install` and `npx`→`pnpm dlx`/`bun x` — and proceeds (exit 0); at most one advisory per session; never blocks.
- [ ] **NUDGE-04**: Hard mode (opt-in `mode:"hard"`) rewrites the command to the pnpm/bun equivalent; `requireHardened` (opt-in) blocks `npm install` when no hardened PM is present, with a structured reason pointing to install guidance.
- [ ] **NUDGE-05**: The nudge flags unpinned installs (`@latest`, bare name, or wide `^`/`~` range) and recommends an exact-pinned spec, naming the detected risk pattern.
- [ ] **NUDGE-06**: Every nudge decision emits a `record_type:"nudge"` NDJSON audit record (original command, decision, closed-enum reason code, PMState); the weekly major-version drift check emits a `record_type:"version_drift"` record.
- [ ] **NUDGE-07**: `beekeeper nudge status | check <command> | audit [--since]` CLI surfaces current PM state + config, dry-runs a command, and queries nudge decisions from the audit log.
- [ ] **NUDGE-08**: The nudge is wired into all three enforcement consumers (check hook, MCP gateway, shim) and honors layered config (the `nudge` block; project `.beekeeper.json` `nudge.enabled:false` disables it); the 60s detection cache lives only where it is effective (long-lived gateway), per the resolved check-hot-path decision.

### Corroboration Severity Hardening (CORR) — finding F1

- [x] **CORR-01**: A *critical*-severity catalog match escalates to **block** at a single trusted source via a per-severity threshold override (`SeverityOverrides["critical"]={BlockAt:1}`), so a known critical malware package (e.g. `ai-figure` / Shai-Hulud, OSV `MAL-2026-4126`, currently warn-only) is blocked.
- [x] **CORR-02**: The escalation is gated on catalog sanity — it does NOT apply when `catalog/sanity.go` reports a degraded/alert state — and `validateCorroborationThresholds` rejects unsafe overrides (`BlockAt < 1`); a mis-tagged all-versions (`versions:["*"]`) critical entry still requires 2-source corroboration (anti-poisoning sanity bound; self-defense).

### Behavioral Test Suite (BTEST) — cross-cutting, the milestone's primary ask

- [ ] **BTEST-01**: Table-driven pure-policy tests cover each new behavior — sensitive-path decisions (traversal, allowlist, OS path forms); severity escalation incl. the degraded-catalog regression; nudge `Evaluate` over `PMState` (PRD §10 criteria 1–10, 14–17).
- [ ] **BTEST-02**: Check-handler integration tests drive `RunCheck` with raw stdin JSON and assert decision + exit code for credential reads, catalog-critical blocks, and pnpm/bun installs (proves wiring is live, not just that component functions return correct values).
- [ ] **BTEST-03**: A live-binary E2E battery invokes the compiled `beekeeper` against the real catalog (mirroring the validation run that surfaced F1/F2/F3), asserting exit codes + audit records — a release gate; hand-written config scanners (`bunfig.toml`, `pnpm-workspace.yaml`) carry fuzz targets per the CI fuzz gate.

## Future Requirements

Deferred to a later release (v1.3.0+). Tracked but not in this roadmap.

- **NUDGE-F1**: Hard-rewrite mode hardened + on by default — gated on soft-advise validation in production (agent-output-parsing compatibility risk).
- **NUDGE-F2**: Nudge coverage for Yarn Berry (`npmMinimalAge`) and pip/cargo/gem/composer ecosystems.
- **CORR-F1**: OSV/Socket consulted as an automatic second corroborating source on the hot path (currently rejected for added check-path latency).
- **NUDGE-F3**: Distinguish `GHSA-*` (patched CVEs in legit packages) from `MAL-*` (actively malicious) in the critical-escalation path.

## Out of Scope

Explicitly excluded for v1.2.0.

| Feature | Reason |
|---------|--------|
| Beekeeper installing or configuring pnpm/bun (editing `pnpm-workspace.yaml`/`bunfig.toml`) | Detect-and-report only; users own their package-manager config (PRD §2.2, §12) |
| Blocking on `@latest`/unpinned in soft mode | Soft mode advises + proceeds; agency preserved (PRD §4) |
| Treating the bundled Bumblebee catalog as cryptographically signed | Inverts the corroboration trust model → poisoning vector; use sanity-gated severity override instead (research Flag 3) |
| Auto-updating the pnpm/bun/node version floors on detected drift | Drift is logged for review; floor bumps require an explicit Beekeeper release (PRD §7.1) |
| New TOML/YAML library dependency | Two config values → hand scanners + fuzz targets; keep the dependency surface minimal (CLAUDE.md; research Flag 1) |

## Traceability

Which phases cover which requirements. Populated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CORR-01 | Phase 6 | Complete |
| CORR-02 | Phase 6 | Complete |
| SPATH-01 | Phase 7 | Pending |
| SPATH-02 | Phase 7 | Pending |
| SPATH-03 | Phase 7 | Pending |
| SPATH-04 | Phase 7 | Pending |
| NUDGE-01 | Phase 8 | Pending |
| NUDGE-02 | Phase 8 | Pending |
| NUDGE-03 | Phase 8 | Pending |
| NUDGE-04 | Phase 8 | Pending |
| NUDGE-05 | Phase 8 | Pending |
| NUDGE-06 | Phase 8 | Pending |
| NUDGE-07 | Phase 8 | Pending |
| NUDGE-08 | Phase 8 | Pending |
| BTEST-01 | Phase 8 | Pending |
| BTEST-02 | Phase 8 | Pending |
| BTEST-03 | Phase 8 | Pending |

**Coverage:**
- v1 requirements: 17 total (SPATH x4, NUDGE x8, CORR x2, BTEST x3)
- Mapped to phases: 17
- Unmapped: 0

---
*Requirements defined: 2026-06-03 — milestone v1.2.0 "Runtime Behavioral Hardening"*
