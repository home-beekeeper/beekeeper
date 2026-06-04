# Requirements: Beekeeper — Milestone v1.2.0 "Runtime Behavioral Hardening"

**Defined:** 2026-06-03
**Core Value:** A hijacked or off-task agent cannot successfully act on the developer's machine without Beekeeper deciding to permit it.
**Milestone Goal:** Close the three runtime-enforcement gaps that live `beekeeper check` validation exposed — credential-file access flagging, package-manager nudging, and critical-severity corroboration — each locked in by a behavioral test suite.
**Scope source:** `.planning/specs/NUDGE-PRD.md` (nudge feature) + runtime-validation findings F1/F2/F3 + `.planning/research/SUMMARY.md`.

> ⏸ **Parked milestone:** v1.1.0 "Pollen" is paused at its release checkpoint (not closed). Its requirements are preserved at `.planning/milestones/v1.1.0-REQUIREMENTS.md`; release resume via `docs/release-runbook.md` + `HANDOFF.json`.

## v1 Requirements

Requirements for milestone v1.2.0. Each maps to exactly one roadmap phase. All work is in `beekeeper` core (not the Pollen fork). `internal/policy` stays a pure function library — detection/normalization I/O lives in adapters, mirroring `policy.EvaluateReleaseAge`.

### Sensitive-Path Runtime Enforcement (SPATH) — finding F2

- [x] **SPATH-01**: `beekeeper check` blocks (fail-closed) an agent tool call whose `file_path` target (Read/Write/Edit) is a sensitive credential path — `~/.ssh/*`, `~/.aws/*`, `~/.gnupg/*`, `~/.npmrc`, `~/.pypirc`, `~/.cargo/credentials*`, `.env`/`.env.*`, and MCP host-config files — by wiring the existing `policy.EvaluatePath`/`DefaultSensitivePaths` engine into the live check pipeline (currently referenced only by its own test).
- [x] **SPATH-02**: Path targets are canonicalized before evaluation (tilde expansion, `filepath.Abs`, `EvalSymlinks`, slash normalization) so `..`-traversal (`../../.aws/credentials`), relative paths, `~`, and Windows backslash/drive forms cannot bypass the blocklist.
- [x] **SPATH-03**: Credential access via shell-command targets (`cat`/`type`/`Get-Content`/`gc` of a sensitive path inside a `Bash` tool call) is detected and flagged, not just direct `file_path` reads.
- [x] **SPATH-04**: A default allowlist prevents false positives on safe lookalikes (`.env.example`, `.env.test`, `.env.schema`); built-in defaults merge with project/user policy-file `sensitive_path` rules and allowlist by most-restrictive-wins with an allowlist escape hatch.

### Package-Manager Nudge (NUDGE) — finding F3, spec `.planning/specs/NUDGE-PRD.md`

- [x] **NUDGE-01**: npm install commands (`npm install`/`i`/`add`, `npx`) are recognized, and `pnpm`/`bun`/`yarn` install commands are likewise parsed so catalog matching applies to them too (closes the F3 bypass where pnpm/bun installs were unparsed).
- [x] **NUDGE-02**: Beekeeper detects locally-installed pnpm (>=11), bun (>=1.3), node (>=22), and the `@socketsecurity/bun-security-scanner`, producing a `PMState` via a timeout-bounded (2s) detection adapter; `nudge.Evaluate(ParsedCommand, PMState, Config)` is a pure decision (no I/O).
- [x] **NUDGE-03**: Soft mode (default) advises steering `npm install` toward the hardened equivalent — correct verb/flag mapping incl. no-arg `npm install`→`pnpm install`/`bun install` and `npx`→`pnpm dlx`/`bun x` — and proceeds (exit 0); at most one advisory per session; never blocks.
- [x] **NUDGE-04**: Hard mode (opt-in `mode:"hard"`) rewrites the command to the pnpm/bun equivalent; `requireHardened` (opt-in) blocks `npm install` when no hardened PM is present, with a structured reason pointing to install guidance.
- [x] **NUDGE-05**: The nudge flags unpinned installs (`@latest`, bare name, or wide `^`/`~` range) and recommends an exact-pinned spec, naming the detected risk pattern.
- [x] **NUDGE-06**: Every nudge decision emits a `record_type:"nudge"` NDJSON audit record (original command, decision, closed-enum reason code, PMState); the weekly major-version drift check emits a `record_type:"version_drift"` record.
- [x] **NUDGE-07**: `beekeeper nudge status | check <command> | audit [--since]` CLI surfaces current PM state + config, dry-runs a command, and queries nudge decisions from the audit log.
- [x] **NUDGE-08**: The nudge is wired into all three enforcement consumers (check hook, MCP gateway, shim) and honors layered config (the `nudge` block; project `.beekeeper.json` `nudge.enabled:false` disables it); the 60s detection cache lives only where it is effective (long-lived gateway), per the resolved check-hot-path decision.

### Corroboration Severity Hardening (CORR) — finding F1

- [x] **CORR-01**: A *critical*-severity catalog match escalates to **block** at a single trusted source via a per-severity threshold override (`SeverityOverrides["critical"]={BlockAt:1}`), so a known critical malware package (e.g. `ai-figure` / Shai-Hulud, OSV `MAL-2026-4126`, currently warn-only) is blocked.
- [x] **CORR-02**: The escalation is gated on catalog sanity — it does NOT apply when `catalog/sanity.go` reports a degraded/alert state — and `validateCorroborationThresholds` rejects unsafe overrides (`BlockAt < 1`); a mis-tagged all-versions (`versions:["*"]`) critical entry still requires 2-source corroboration (anti-poisoning sanity bound; self-defense).

### Behavioral Test Suite (BTEST) — cross-cutting, the milestone's primary ask

- [x] **BTEST-01**: Table-driven pure-policy tests cover each new behavior — sensitive-path decisions (traversal, allowlist, OS path forms); severity escalation incl. the degraded-catalog regression; nudge `Evaluate` over `PMState` (PRD §10 criteria 1–10, 14–17).
- [x] **BTEST-02**: Check-handler integration tests drive `RunCheck` with raw stdin JSON and assert decision + exit code for credential reads, catalog-critical blocks, and pnpm/bun installs (proves wiring is live, not just that component functions return correct values).
- [x] **BTEST-03**: A live-binary E2E battery invokes the compiled `beekeeper` against the real catalog (mirroring the validation run that surfaced F1/F2/F3), asserting exit codes + audit records — a release gate; hand-written config scanners (`bunfig.toml`, `pnpm-workspace.yaml`) carry fuzz targets per the CI fuzz gate.

### v1.2.0 Tech-Debt Cleanup (CLEAN / HARDEN / DRIFT) — Phase 9

> Inserted 2026-06-04 from the milestone audit (`.planning/v1.2.0-MILESTONE-AUDIT.md`, status `tech_debt`). Scope choice "Everything" (maintainer, 2026-06-04). These remediate audit findings; none reopen a failed v1 requirement (all 17 above are satisfied) — they harden the gate, fix a config root cause, close SPATH evasion edges, and complete the drift path. See `09-CONTEXT.md`.

- [x] **CLEAN-01**: The CORR live-binary E2E release gate is hermetic — `TestE2ELiveBinary/CORR_aifigure_critical_block` blocks with OSV unreachable via a signed, non-wildcard local `ai-figure` fixture (today it blocks only because the binary reaches live OSV; offline CI would flake exit 0 vs 1).
- [x] **CLEAN-02**: `config.LoadLayered` merges the `Config.Nudge *NudgeConfig` pointer at its root; the consumer-layer nil-workarounds (`defaultNudgeConfigHelper`, `handler.go` guard) are removed or documented as defense-in-depth; a layered-config test proves defaulting + project override without a consumer helper.
- [x] **CLEAN-03**: Phase 6 Nyquist is reconciled — `06-VALIDATION.md` is COMPLIANT, consistent with its passed `06-VERIFICATION.md` (stale `draft`/`nyquist_compliant:false` frontmatter corrected via `/gsd-validate-phase 6` or evidence-backed update).
- [x] **CLEAN-04**: The stale `internal/check/handler.go` decision-merge comment ("sensitive-path block is merged LAST") is corrected to the real order (overlay → SPATH → NUDGE).
- [x] **HARDEN-01**: An ancestor-symlink credential path still blocks — the sensitive-path match evaluates both pre- and post-`EvalSymlinks` forms so an ancestor symlink cannot strip a `/.aws/` or `/.ssh/` fragment (IN-01); regression test fails on pre-fix code.
- [x] **HARDEN-02**: Windows ADS (`id_rsa:stream`) and trailing-dot/space basenames (`credentials.`) are normalized before blocklist evaluation and block fail-closed (IN-02); Windows-gated regression tests.
- [x] **HARDEN-03**: Bash read-verb extraction is word-boundary-anchored — verb-substring tokens don't false-trigger while real `more ~/.ssh/id_rsa` still flags (IN-03).
- [x] **DRIFT-01**: Production `realMetadataFetch` performs a real registry query so the gateway weekly drift check emits live `record_type:"version_drift"` records; fail-open on fetch error preserved; pnpm/bun/node floors are never auto-bumped (auto-update stays Out-of-Scope).

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
| SPATH-01 | Phase 7 | Complete |
| SPATH-02 | Phase 7 | Complete |
| SPATH-03 | Phase 7 | Complete |
| SPATH-04 | Phase 7 | Complete |
| NUDGE-01 | Phase 8 | Complete |
| NUDGE-02 | Phase 8 | Complete |
| NUDGE-03 | Phase 8 | Complete |
| NUDGE-04 | Phase 8 | Complete |
| NUDGE-05 | Phase 8 | Complete |
| NUDGE-06 | Phase 8 | Complete |
| NUDGE-07 | Phase 8 | Complete |
| NUDGE-08 | Phase 8 | Complete |
| BTEST-01 | Phase 8 | Complete |
| BTEST-02 | Phase 8 | Complete |
| BTEST-03 | Phase 8 | Complete |
| CLEAN-01 | Phase 9 | Complete |
| CLEAN-02 | Phase 9 | Complete |
| CLEAN-03 | Phase 9 | Complete |
| CLEAN-04 | Phase 9 | Complete |
| HARDEN-01 | Phase 9 | Complete |
| HARDEN-02 | Phase 9 | Complete |
| HARDEN-03 | Phase 9 | Complete |
| DRIFT-01 | Phase 9 | Complete |

**Coverage:**
- v1 requirements: 17 total satisfied (SPATH x4, NUDGE x8, CORR x2, BTEST x3) — all Complete
- Phase 9 cleanup requirements: 8 (CLEAN x4, HARDEN x3, DRIFT x1) — inserted from milestone audit, all Complete & verified (2026-06-04; Phase 9 status passed 9/9)
- Mapped to phases: 25 (17 + 8)
- Unmapped: 0

---
*Requirements defined: 2026-06-03 — milestone v1.2.0 "Runtime Behavioral Hardening"*
