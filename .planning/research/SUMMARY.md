# Research Summary: Beekeeper v1.2.0 "Runtime Behavioral Hardening"

**Project:** Beekeeper — agent runtime safety harness
**Domain:** Go security daemon / policy enforcement for autonomous coding agents
**Researched:** 2026-06-03
**Confidence:** HIGH (four research files derived from live codebase inspection + verified external sources)

---

## Executive Summary

v1.2.0 closes three enforcement gaps discovered through live agent testing. The gaps are not design flaws — the underlying engines exist and their unit tests pass. What is missing is **wiring, path normalization, and a per-severity escalation rule**. A hijacked agent can currently read `~/.aws/credentials` (PLCY-05 gap), install the Shai-Hulud malware package `ai-figure` with only a warning (PLCY-07 gap), and run `pnpm`/`bun` install commands that bypass catalog matching entirely (NUDGE/F3 gap). All three are closed by targeted changes plus a behavioral test suite that proves the wiring is **live**, not just that component functions return correct values.

Recommended strategy: **pure-logic first, I/O adapters second, wiring third, CLI and E2E last** — enforced by the CLAUDE.md purity constraint (`internal/policy` stays pure). The new `internal/nudge/` package mirrors the established `policy.EvaluateReleaseAge(ReleaseAgeInput, cfg)` pattern: `nudge.Evaluate` receives a caller-resolved `PMState`; detection I/O lives in `detect.go`. Exactly **one** new Go module dependency for the entire milestone: `golang.org/x/mod/semver`. No TOML/YAML library — hand-written line scanners (with fuzz targets) cover the two config values needed.

Two principal risks:
1. **NUDGE detection latency on the one-shot `beekeeper check` hot path.** The PRD's 60-second in-process cache is dead code in a one-shot process; three subprocess execs per hook add ~150ms, blowing the sub-100ms target. **Unresolved architectural decision** — must be settled before `detect.go`.
2. **PLCY-07 self-defense.** The severity-escalation threshold must be **gated on `catalog/sanity.go` not reporting a degraded state**; shipping escalation without the gate inverts the trust model and creates a single-catalog poisoning vector worse than the current warn-only state.

---

## Decision Flags (Roadmapper / Phase Planners MUST surface these)

### Flag 1 — One new dependency + hand-scanner fuzz targets
`golang.org/x/mod/semver` is the only new dep. pnpm/bun emit bare versions (`11.5.1`, `1.3.14`) without the `"v"` prefix `x/mod/semver.Compare` requires → a `normalize()` helper that prepends `v` is **load-bearing** (and must be tested with real pnpm/bun output; naive integer comparators misorder pre-releases like `12.0.0-rc.1`). `node --version` already emits `v`. **Do NOT add** `BurntSushi/toml`, `pelletier/go-toml`, `gopkg.in/yaml.v3`, or `Masterminds/semver` — `bunfig.toml` needs one key under `[install.security]` (~20-line scanner) and `pnpm-workspace.yaml` needs two scalar keys (`strings.HasPrefix`). Both hand scanners need dedicated **fuzz targets** (CI fuzz gate).

### Flag 2 — NUDGE detection on the one-shot `check` hot path — UNRESOLVED
The PRD §4 60-second in-process cache is dead code in `beekeeper check` (one-shot process). Three execs (`pnpm`/`bun`/`node --version`) ≈ 150ms/hook → demolishes the sub-100ms target. **The NUDGE phase plan MUST commit to one of:**
- **Position A — file-based cache** (`~/.beekeeper/state/nudge-detect.json` + TTL, atomic write, graceful read-error fallthrough). The only approach that survives the one-shot model. *(Architecture research recommendation.)*
- **Position B — gateway/shim only in v1.2.0**, check path gets a no-op `Proceed` stub; defer check-path nudge to v1.3.0. Simpler, zero hot-path latency. *(Pitfalls research recommendation.)*
- **Position C — lazy/async PROCEED-on-miss.** More complex; first-call `npm install` not nudged. Not recommended.

This choice determines `detect.go`'s signature, test strategy, and the BTEST E2E fixtures.

### Flag 3 — PLCY-07 self-defense: sanity gate is mandatory
Do **NOT** mark bundled bumblebee entries `Signed:true`. Correct fix: add `SeverityOverrides map[string]SeverityThreshold` to `CorroborationThresholds`, default `["critical"] = {BlockAt:1, QuarantineAt:2}`, **activating only when `catalog/sanity.go CheckSanity()` reports non-degraded** (existing `BlockDeltaEntries:10000` / `AlertDeltaEntries:1000` backstop). Escalation + sanity-gate are **one atomic deliverable**. Release gate: inject 1001 critical entries → catalog alerts → critical match still warns. `validateCorroborationThresholds` must reject `BlockAt < 1`. `CriticalBlockAt:1` applies **only** to version-specific entries (`Versions != ["*"]`) — an all-versions critical entry still needs 2-source corroboration (prevents a mis-tagged `severity:critical` + `versions:["*"]` from blocking all `react`/`typescript` installs).

### Flag 4 — installPrefixes duplication: extract or accept a third copy
`internal/policy/engine.go` and `internal/policyloader/enforce.go` already have **diverged** install-prefix tables. `internal/nudge/parse.go` would be a third (cannot import `internal/policy` — circular). Decide before `parse.go`: extract a pure `internal/pkgparse/` (cleaner long-term) vs accept a third copy with cross-reference comments (narrower v1.2.0 scope).

### Flag 5 — PRD corrections to apply before implementation
1. **`minimumReleaseAge` default is 1440 minutes, not 60** (PRD §6.3). Use 1440 in `detect.go`; otherwise `minimumReleaseAge:120` users get spurious warnings.
2. **Node 22 is Maintenance LTS** (Node 24 is Active LTS, EOL 2028-04-30). Floor `22.0.0` stays correct for pnpm 11; UX copy: "Node 22 or later (Node 24 is the current Active LTS)".

---

## Key Findings

### Stack (v1.2.0 delta)
Existing stack (Go 1.25, no CGO, stdlib `encoding/json`, `cilium/ebpf`, Bubble Tea v2) unchanged. Add only `golang.org/x/mod/semver` (`go get golang.org/x/mod@latest`; not currently in the module graph; zero transitive deps). Versions verified 2026-06-03 (HIGH): pnpm floor `11.0.0` correct (latest 11.5.1, pure-ESM, Node 22 hard min); bun floor `1.3.0` correct (latest 1.3.14, Scanner API stable); `@socketsecurity/bun-security-scanner` name/publisher/`[install.security]` key all confirmed.

### Features
**Must-have (P1):** PLCY-05 wiring (`EvaluatePath`+`DefaultSensitivePaths` into `handler.go`; tilde+`Abs`+`EvalSymlinks`+`ToSlash`; fix `extractTargetPath` to read `file_path`; add `.env.example` to allowlist); PLCY-07 `SeverityOverrides["critical"]={BlockAt:1}` gated on sanity + all-versions guard; NUDGE full package (detect/parse/rewrite/evaluate/version/reasons), soft-advise default, wired into check+gateway+shim, `record_type:"nudge"`; BTEST (table tests + `RunCheck` integration + exec-based E2E = release gate).
**Should-have (P2):** `beekeeper nudge status|check|audit`; Node-22 compat reason code; weekly drift check (`version_drift` record + TUI badge).
**Defer to v1.3.0:** hard-rewrite mode (agent-output-parsing risk; validate soft-advise in prod first); Yarn/pip/cargo/gem nudge; OSV as auto-corroboration second source (hot-path latency).
**Anti-features:** auto-install pnpm/bun; editing `pnpm-workspace.yaml`/`bunfig.toml`; blocking on `@latest` in soft mode.

**Command equivalence (for rewrite.go):** no-arg `npm install` → `pnpm install`/`bun install` (verb changes); `npx <x>` → `pnpm dlx`/`bun x` (these inherit PM security gates; `npx` has none); flag divergences (`--save-optional`→bun `--optional`; `--production`→pnpm `--prod`). **Pinning risk:** `@latest`/bare = CRITICAL; `^x.y.z` = HIGH (caret = Axios attack vector); `~x.y.z` = MEDIUM; `exact` = LOW — message should name the detected spec pattern.

**Soft vs hard UX:** Claude Code hooks have no native "rewrite" — soft-advise = exit 0 + advisory text (default; one advisory per session, not per package); hard-rewrite = exit 2 (block) + rewritten command in stdout (agent re-issues).

### Architecture
`runCheck` (`internal/check/handler.go`) gets two new blocks between `policy.Evaluate` (~line 251) and `ApplyPolicyOverlay` (line 267):
```
[policy.Evaluate]
  ↓  PLCY-05: extractPathTargets → EvaluatePath loop → mergeDecisions (most-restrictive)
  ↓  NUDGE:   ParseCommand → DetectPMState → Evaluate → writeNudgeAuditRecord
[ApplyPolicyOverlay]
```
PLCY-07 is internal to `corroborate()` (no pipeline change). Nudge pattern mirrors `EvaluateReleaseAge`: pure `nudge.Evaluate(ParsedCommand, PMState, Config)` + impure `nudge.DetectPMState()`; 60s cache lives on the **gateway handler struct** (`internal/gateway/nudge_cache.go`), never a package global. Audit extends **additively**: new `internal/audit/nudge_types.go` (`NudgeRecord`, `VersionDriftRecord`); `AuditRecord`/`FromDecision` untouched; only nudge `Block` escalates the main `policy_decision` (Advise/Rewrite write a separate `nudge` record). Verify `internal/audit/writer.go` accepts `any` (else add `WriteAny`).

### Critical Pitfalls
1. **(CRITICAL) NUDGE exec overhead on one-shot check** — resolve Flag 2 before `detect.go`. Warning sign: no state-file design when Position A chosen.
2. **(CRITICAL) PLCY-07 without sanity gate** — poisoning vector. Warning sign: a `forceSigned=true` shortcut in `corroborate()` without sanity gating.
3. **(HIGH) PLCY-05 path normalization** — unresolved `~` bypasses allowlist; `../../.aws/credentials` has no `/.aws/` fragment → bypasses block. Require `filepath.Abs`+`EvalSymlinks` in the wiring adapter (`internal/check/paths.go`).
4. **(HIGH) `.env.example` false positive** — `DefaultSensitivePaths()` `AllowPatterns: nil`; `.env.*` glob blocks `.env.example`. Add `.env.example`/`.env.test`/`.env.schema` to allowlist before wiring is live.
5. **(CRITICAL) BTEST mocking too much** — F1/F2/F3 were missed because tests mocked at the wrong layer. ≥1 test MUST exec the compiled binary with raw stdin JSON and assert exit code + audit record. Release gate.
6. **(MEDIUM) Hard-mode agent output parsing** — pnpm/npm output differ; keep hard mode opt-in, soft default unbreakable.
7. **(MEDIUM) Severity inflation** — `CriticalBlockAt:1` only for version-specific entries.

---

## Suggested Phase Structure (continuing numbering at 06)

> Pure-before-impure is mandatory (purity constraint). PLCY-07 escalation + sanity gate are one unit. The detection-cache decision (Flag 2) and installPrefixes decision (Flag 4) must be committed before the nudge phases are planned.

- **Phase 06 — PLCY-07 corroboration hardening (pure).** `SeverityThreshold` type + `SeverityOverrides`; extend `corroborate()` + `validateCorroborationThresholds` (BlockAt≥1, override≤global); sanity-gate; all-versions guard. Tests: Shai-Hulud fixture (bumblebee-unsigned + OSV-signed + critical → block), degraded-catalog regression (1001 entries → warn), all-versions guard. Files: `internal/policy/types.go`, `corroboration.go`. *No research-phase needed.*
- **Phase 07 — PLCY-05 sensitive-path enforcement (wiring + normalization).** `internal/check/paths.go` normalization adapter (tilde+Abs+EvalSymlinks+ToSlash + Bash `cat`/`type`/`Get-Content` target scan); insert eval block in `handler.go`; fix `extractTargetPath` in `enforce.go` to read `file_path`; add `.env.example` allowlist. Integration test: `RunCheck` stdin `{"tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}` → exit 1.
- **Phase 08 — NUDGE feature (the PRD).** Resolve Flag 2 + Flag 4 in discuss/plan. Pure core first (`nudge.go`/`parse.go`/`rewrite.go`/`version.go`/`reasons.go` + `x/mod/semver`), then I/O (`detect.go` + chosen cache, `audit/nudge_types.go`, hand scanners + fuzz), then wiring (check/gateway/shim/config), then CLI (`beekeeper nudge status|check|audit`). PRD §10 criteria as acceptance tests.
- **(cross-cutting) BTEST** in every phase, culminating in the exec-based E2E battery as the v1.2.0 release gate.

*(Final phase count/split is the roadmapper's call — NUDGE may warrant splitting pure vs wiring across two phases.)*

### Research flags for phase planning
- **Before Phase 08 plan:** commit Flag 2 (cache: A vs B) and Flag 4 (pkgparse extract vs third copy).
- **During Phase 08:** Windows corepack-shimmed pnpm `cmd.exe` startup under the 2s detection timeout (live CI timing).
- **Before Phase 07 wiring:** confirm `internal/audit/writer.go` accepts `any`.

---

## Confidence Assessment
| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versions verified against official sources; `x/mod/semver` `"v"`-prefix requirement confirmed |
| Features | HIGH | Command mappings from pnpm.io/bun.sh docs; CVSS from FIRST.org; blocklist cross-referenced |
| Architecture | HIGH | Direct inspection of live source with exact line numbers (handler.go, enforce.go, corroboration.go) |
| Pitfalls | HIGH | Derived from live source reading + PROJECT.md F1/F2/F3 findings, not inferred |

**Overall: HIGH.** Ready for roadmap: yes.

---
*Research completed 2026-06-03. Sources: live codebase (engine.go, path.go, corroboration.go, types.go, handler.go, enforce.go, sanity.go, verify.go, audit/types.go, shim.go); pkg.go.dev x/mod/semver; pnpm.io/settings + /blog/releases/11.0; bun.com/blog/bun-v1.3; github.com/SocketDev/bun-security-scanner; nodejs.org releases; ossf OSV schema; FIRST.org CVSS.*
