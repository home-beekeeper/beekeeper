# Phase 8: Package-Manager Nudge + Behavioral Test Suite — Research

**Researched:** 2026-06-04
**Domain:** Go package-manager command parsing + subprocess version detection + pure-policy decision + behavioral test architecture (table-driven / integration / live-binary E2E / fuzz)
**Confidence:** HIGH (codebase analogs verified by file read; PM facts verified via web; Flag 5 confirmed against pnpm/Node release sources)

## Summary

Phase 8 ships the full **package-manager nudge** feature (PRD `.planning/specs/NUDGE-PRD.md`) and the **v1.2.0 behavioral test suite** that gates the release. The feature steers agent `npm install` calls toward locally-installed pnpm (>=11) or bun (>=1.3), soft-advising by default and hard-rewriting on opt-in, and — critically — closes the **F3 gap** by making pnpm/bun/yarn install commands *parseable* so the existing catalog-matching engine applies to them (they are silently unparsed today).

The architecture is a direct repeat of the Phase 7 / `EvaluateReleaseAge` pattern, which is already locked by CLAUDE.md: **a pure decision function in a no-I/O package, fed a caller-resolved input struct by an impure adapter.** `nudge.Evaluate(ParsedCommand, PMState, Config)` is the pure decision; `internal/nudge/detect.go` is the impure adapter that shells out to `pnpm --version` / `bun --version` / `node --version` and scans `bunfig.toml` / `pnpm-workspace.yaml`. Wiring into `runCheck` mirrors Phase 7's `mergeDecisions` most-restrictive-wins merge exactly. The behavioral test suite has three tiers already exemplified in the repo: table-driven pure tests (`release_age_test.go`), `RunCheck`/`runCheckWithIndex` integration tests (`integration_test.go`), and a NEW live-binary E2E battery (no precedent in `internal/check` — it is net-new and is the release gate).

**Primary recommendation:** Put `nudge.Evaluate` as a **pure** function in a NEW `internal/nudge/` package (NOT `internal/policy`, to avoid bloating the locked pure library and because nudge needs its own `Action`/`PMState`/`Config`/reason-enum types); confine ALL detection I/O to `internal/nudge/detect.go` guarded by a `TestNudgeEvaluateImportsArePure` test mirroring `TestReleaseAgeImportsArePure`. **Extract install-command parsing into a new shared `internal/pkgparse/` package** (Flag 4 → extract) consumed by `policy/engine.go`, `policyloader/enforce.go`, and `nudge/parse.go` — there are exactly two copies today and nudge would add a third. **The 60s detection cache lives ONLY in the long-lived gateway** (Flag 2 → Position B); the one-shot `beekeeper check` process runs detection fresh each call under the 2s timeout (no caching benefit possible in a process that exits after one decision).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Parse install command (npm/pnpm/bun/yarn/npx) | `internal/pkgparse` (pure) | — | Shared by engine, overlay, nudge; pure string logic, no I/O |
| Detect local PM versions (pnpm/bun/node) | `internal/nudge/detect.go` (impure adapter) | — | Shells out with 2s timeout; the only I/O in the feature |
| Scan `bunfig.toml` / `pnpm-workspace.yaml` | `internal/nudge/detect.go` (impure) | — | Hand-written scanners (no TOML/YAML dep, per REQUIREMENTS Out-of-Scope) |
| Nudge decision (Advise/Rewrite/Proceed/Block) | `internal/nudge/evaluate.go` (PURE) | — | Mirrors `policy.EvaluateReleaseAge`; decides over caller-resolved `PMState` |
| Command rewriting (npm→pnpm/bun verb mapping) | `internal/nudge/rewrite.go` (pure) | — | Pure string transform; tested table-driven |
| Catalog matching of pnpm/bun installs (F3) | `internal/policy/engine.go` (pure) | `internal/pkgparse` | Add pnpm/bun/yarn prefixes → ecosystem "npm" so `LookupAll` keys correctly |
| Wire nudge into live check | `internal/check/handler.go` (`runCheck`) | `internal/nudge` adapter | Same seam Phase 7 used; `mergeDecisions` merge |
| Wire nudge into gateway (+ 60s cache) | `internal/gateway/policy.go` (`applyPolicy`) | `internal/nudge` | Long-lived process — the ONLY place the cache is effective |
| Wire nudge into shim | `internal/shim/shim.go` | `internal/nudge` | npm shim calls Evaluate before proxying |
| `nudge` config block | `internal/config/config.go` + `internal/policyloader` | — | New `NudgeConfig` struct; bounds-validated like release_age |
| `beekeeper nudge` CLI | `cmd/beekeeper/nudge.go` (thin Cobra) | `internal/nudge` + `internal/audit` | status/check/audit subcommands; thin wiring only |
| Weekly drift check | `internal/nudge` (detect + emit) | gateway/daemon scheduler | `version_drift` audit record; does NOT auto-update floors |
| Audit `record_type:"nudge"` / `"version_drift"` | `internal/audit/types.go` | `internal/nudge` | New record types join `policy_decision`/`tool_result`/`llmf_alert` |

---

## Resolved Architectural Decisions (Flag 2 / Flag 4 / Flag 5)

These were to be resolved in `/gsd-discuss-phase 8`, which the user skipped. STATE.md says they "must be settled before `detect.go` is written" and "must not be deferred to implementation." The planner should treat the positions below as **locked decisions**.

### Flag 2 — NUDGE detection cache location → **Position B (gateway/shim-only cache; check hook runs fresh)**

**Evidence (verified):**
- `internal/check/handler.go` is a one-shot process: `RunCheck` reads one tool call from stdin, decides, writes one audit record, and the process exits (`exitCodeFor` → `os.Exit` in `cmd/beekeeper`). A 60s in-memory cache in a process that lives for one decision and then dies provides **zero** cache hits.
- A file-backed cache (Position A) in the check hook would add hot-path disk I/O on **every** `beekeeper check` invocation (read cache file → maybe parse → maybe write) directly contradicting CLAUDE.md: *"Hook handler loads catalog via mmap — never cold-load per invocation"* and the existing 5s hard budget / latency-tracking discipline (`GlobalHookTracker`, `appendHookLatency`). It buys nothing the timeout doesn't already bound.
- `internal/gateway/proxy.go` runs `applyPolicy` per-request inside a long-lived `Start` loop (`for { ... applyPolicy(msg, h.idx, h.cfg, ac) }`). This is the ONLY consumer where a session-scoped 60s cache produces hits.
- NUDGE-08 already states the resolution verbatim: *"the 60s detection cache lives only where it is effective (long-lived gateway)."*

**Decision:** The 60s detection cache is a property of the **long-lived gateway** (and a shim daemon if/when one is long-lived). The one-shot **check hook runs detection fresh every call**, bounded only by the 2s detection timeout. The shim, when invoked as a short-lived wrapper, behaves like the check hook (fresh detect, no cache).

**Exact `detect.go` signature implication for the check-hook path:**

```go
// internal/nudge/detect.go  (IMPURE adapter — shells out, reads files)

// DetectState resolves the local PM state. It runs `pnpm/bun/node --version`
// each with a 2s hard timeout and scans bunfig.toml / pnpm-workspace.yaml.
// On any timeout or error a PM is treated as "not installed" (graceful
// fallback — never blocks on detection failure, PRD §10 criterion 12).
//
// The check hook calls DetectState directly (no cache) on every invocation.
func DetectState(ctx context.Context, cfg Config) PMState

// Cache wraps DetectState with a 60s TTL. It is constructed ONCE by the
// gateway at startup and reused across requests. NEVER constructed in the
// check hook (a one-shot process gets no cache hits — Flag 2 Position B).
type Cache struct { /* mu sync.Mutex; state PMState; expiresAt time.Time */ }
func NewCache(d func(context.Context, Config) PMState, ttl time.Duration) *Cache
func (c *Cache) State(ctx context.Context, cfg Config) PMState
```

- `Cache` is the only place `sync` and `time.Now()` live (impure, gateway-only).
- `DetectState` uses `context.WithTimeout(ctx, 2*time.Second)` per `exec.CommandContext`.
- `nudge.Evaluate(parsed, state, cfg)` never touches either — it receives the resolved `PMState`.

**How tests assert it (resolves PRD §10 criterion 11):**
- Cache behavior is tested at the `Cache` level with an injectable detect-fn counter: call `State` twice within the TTL, assert the underlying fn ran once; advance a fake clock past TTL, assert it ran again. (The `release_age` adapter and `gateway` already use injected funcs / fake clocks — `scanOnDeltaFn` pattern, STATE.md Phase 11 decisions.)
- The check-hook path is tested for *graceful fresh detection*, not caching: criterion 12 (2s timeout → treated as not installed) via an injected detect-fn that sleeps/errs.
- **Planner note:** PRD §10 criterion 11 ("cache prevents re-running within 60s in the same session") is satisfied by the *gateway* Cache test, NOT a check-hook test. The plan must state this explicitly so the verifier does not expect a check-hook cache.

### Flag 4 — installPrefixes extraction → **EXTRACT into new `internal/pkgparse/`**

**Evidence (verified — count the copies):**
- **Copy 1:** `internal/policy/engine.go` — `installPrefixes` table (lines 31-47) + `extractFromCommand` + `firstPackageToken` + `splitVersion` + `normalize`.
- **Copy 2:** `internal/policyloader/enforce.go` — `installPrefixesOverlay` table (lines 240-253, *identical entries*) + `extractEcoPackageFromCommand` + `firstNonFlagToken` + `stripVersionSuffix`. The code comment literally says: *"duplicated here to keep internal/policy untouched."*
- A nudge parser would be **Copy 3**. Three hand-maintained copies of supply-chain-security-critical parsing (which package/version an agent is installing) is a correctness and drift hazard: the F3 fix requires adding `pnpm`/`bun`/`yarn` prefixes, and that addition would have to be made — identically — in three places or the catalog-match path and the nudge path would disagree about what counts as an install.

**Decision:** Create **`internal/pkgparse/`** — a pure (no-I/O) package exposing the canonical install-command parser. Both existing copies are refactored to consume it; the new nudge parser consumes it too.

**Proposed `internal/pkgparse` public surface:**

```go
package pkgparse  // PURE — no os/net/io/time/sync imports (add TestPkgparseImportsArePure)

type ParsedCommand struct {
    Raw        string   // original command verbatim
    Manager    string   // "npm" | "pnpm" | "bun" | "yarn" | "npx" | "pip" | ...
    Ecosystem  string   // catalog key: npm/pnpm/bun/yarn → "npm"; pip → "pypi"; etc.
    Verb       string   // "install" | "i" | "add" | "" (no-arg) | "dlx" | "x"
    Package    string   // normalized (lowercased, trimmed); "" for no-arg install
    Version    string   // from trailing @version; "" if none
    IsInstall  bool     // true only for install-class verbs (NUDGE-01/§6.4)
    IsExec     bool     // npx / pnpm dlx / bun x
    Sudo       bool     // leading "sudo " stripped (PRD §6.4, criterion 10)
    Unpinned   bool     // @latest, bare name, or wide ^/~ range (NUDGE-05)
}

func Parse(cmd string) (ParsedCommand, bool)   // ok=false when not an install/exec command
```

**Files affected (Flag 4 → extract):**
| File | Change |
|------|--------|
| `internal/pkgparse/pkgparse.go` (NEW) | Canonical prefix table (incl. pnpm/bun/yarn — closes F3), `Parse`, token/version/sudo/unpinned helpers |
| `internal/pkgparse/pkgparse_test.go` (NEW) | Table-driven parse tests + `TestPkgparseImportsArePure` |
| `internal/pkgparse/fuzz_test.go` (NEW) | `FuzzParse` (must never panic) — feeds BTEST-03 fuzz gate alongside config scanners |
| `internal/policy/engine.go` (EDIT) | Replace `installPrefixes`/`extractFromCommand` with `pkgparse.Parse`; keep editor-extension path as-is. **Adds pnpm/bun/yarn → ecosystem "npm" so `LookupAll("npm", pkg)` matches (NUDGE-01, SC1)** |
| `internal/policyloader/enforce.go` (EDIT) | Replace `installPrefixesOverlay`/`extractEcoPackageFromCommand` with `pkgparse.Parse` |
| `internal/nudge/parse.go` (NEW, thin) | Wrap/re-export `pkgparse.Parse`; nudge-specific verb→nudge mapping lives in `rewrite.go` |

**Risk/cost note for planner:** `internal/policy` currently imports only `fmt`/`strings`. `internal/pkgparse` must be equally pure so `policy` can import it without breaking `TestPathImportsArePure`/`TestReleaseAgeImportsArePure`-style purity (pkgparse imports only `strings`). This is a refactor of TWO existing files plus their tests — the plan should sequence it as Wave 0 / Wave 1 *before* the engine adds pnpm/bun prefixes, and re-run the full `internal/policy` and `internal/policyloader` suites to prove behavior is byte-identical (the existing engine_test.go and enforce_test.go are the regression net).

### Flag 5 — PRD corrections to APPLY (confirmed, not investigate)

These are **confirmed corrections** the planner must bake in. Both verified against current sources (June 2026).

1. **`minimumReleaseAge` default = 1440 minutes (NOT 60).** pnpm 11 ships `minimumReleaseAge: 1440` (1 day) on by default `[VERIFIED: pnpm.io/blog/releases/11.0, pnpm.io/settings]`. The repo's own `policy.DefaultReleaseAgeConfig()` already uses `1440`. **Concrete edits:**
   - PRD §6.3 step 2 currently reads *"if explicitly set to a value less than **60**, treat as a configuration weakness."* → change threshold reference to **1440** (a value materially below the 1440-minute default is the weakness signal). The nudge config's `versionFloors`/hardening-check logic must compare against 1440, not 60.
   - PRD §10 criterion 16 (`minimumReleaseAge` set to 0 → warn but `pnpm_hardened` stays true) is unaffected by the number; keep the test, the "weakness" comparison just uses 1440 as the baseline.
   - Any `nudge` default config the planner writes for `config.json` should NOT introduce a `minimumReleaseAge` field for Beekeeper itself (out of scope: Beekeeper does not configure pnpm — REQUIREMENTS Out-of-Scope). The 1440 value matters only for the *hardening-weakness detection* comparison in `detect.go`.

2. **Node 22 = Maintenance LTS; Node 24 = Active LTS (Node 26 = Current).** `[VERIFIED: nodejs.org release schedule, endoflife.date/nodejs — June 2026]`. **Concrete edits:**
   - PRD §7 version matrix row "Node.js (for pnpm 11)" currently says recommended *"latest 22.x LTS"* and reason *"pnpm 11 requires Node 22+; ESM-only."* The **floor** (22.0.0) is correct and stays — pnpm 11 requires Node 22+ `[VERIFIED]`. But the **recommended** text should read *"Node 24.x (Active LTS); Node 22.x is Maintenance LTS — still supported through 2027-04 but no longer the recommended target."*
   - PRD §2.1 "Node.js >= 22 required for pnpm 11" stays correct as a floor.
   - The `node-incompatible-with-pnpm-11` reason code (criterion 6) fires when active Node < 22 — unchanged; the floor is 22, not 24.

**Net:** Floors are unchanged (pnpm 11.0.0, bun 1.3.0, node 22.0.0). Only (a) the hardening-weakness comparison baseline (60 → 1440) and (b) the human-facing "recommended Node" guidance (22 → 24 Active LTS) change.

---

## Standard Stack

This phase is **pure Go + stdlib** — no new third-party dependencies (REQUIREMENTS Out-of-Scope: *"No TOML/YAML library dependency — two config values → hand scanners + fuzz targets"*).

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `os/exec` | go1.25 | `exec.CommandContext("pnpm","--version")` with 2s timeout | Already used in `internal/shim`; `CommandContext` is the canonical timeout idiom |
| Go stdlib `context` | go1.25 | 2s per-detect deadline + 5s outer check budget | Already the timeout mechanism in `handler.go` |
| Go stdlib `strings` | go1.25 | Command parsing, hand-written `bunfig.toml`/`pnpm-workspace.yaml` scanners | Keeps `pkgparse`/`nudge.Evaluate` pure |
| `github.com/spf13/cobra` | (existing) | `beekeeper nudge status\|check\|audit` CLI wiring | Project CLI framework; thin wiring only (CLAUDE.md) |

### Supporting (existing internal packages — consumed, not added)
| Package | Purpose | When to Use |
|---------|---------|-------------|
| `internal/audit` | `record_type:"nudge"` + `"version_drift"` records; `audit.Query` for `nudge audit` | Add new record types + fields to `types.go`; reuse `Query`/`QueryOpts` |
| `internal/policy` | Catalog matching of pnpm/bun installs (F3) via `Evaluate`→`LookupAll` | Add pnpm/bun/yarn prefixes (via pkgparse) → ecosystem "npm" |
| `internal/policyloader` | Config-block load/validate/merge, bounds validation | Mirror `validateCorroborationThresholds` for `nudge` block bounds |
| `internal/config` | New `NudgeConfig` struct on `Config` | Add `Nudge NudgeConfig json:"nudge,omitempty"` |
| `internal/check` | `runCheck` wiring + `runCheckWithIndex` integration harness | Mirror Phase 7 `mergeDecisions` block |
| `internal/gateway` | `applyPolicy` wiring + the 60s `nudge.Cache` (Flag 2) | The ONLY long-lived cache home |
| `internal/shim` | npm shim calls `nudge.Evaluate` before proxy | Already lists npm/pnpm/bun-relevant `DefaultTools` |
| `internal/platform` | `StateDir()`/`AuditDir()` resolution for CLI | Reuse for `nudge status`/`audit` path resolution |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-written TOML/YAML scanners | `BurntSushi/toml` + `goccy/go-yaml` | REQUIREMENTS explicitly forbids new dep for 2 config values; adds supply-chain footprint to a *security* tool (self-defense §12 tension). Hand scanners + fuzz targets are the locked choice. |
| New `internal/pkgparse` (Flag 4) | Third copy with cross-ref comments | Rejected: three copies of install-parsing that must stay in lockstep for the F3 fix is the larger risk. Extraction is the locked choice. |
| `nudge.Evaluate` in `internal/policy` | Keep it in policy | Rejected: nudge needs its own `Action`/`PMState`/`Config`/reason-enum; PRD §3.1 mandates `internal/nudge/`; keeps the locked pure library lean. The purity *discipline* is preserved via `TestNudgeEvaluateImportsArePure`. |

**No `npm install` needed** — Go module only. Detection tools (pnpm/bun/node) are detected at runtime, never installed by Beekeeper (PRD §2.2).

---

## Architecture Patterns

### System Architecture Diagram

```
agent tool call (Bash: "npm install foo@latest")
        │
        ├──────────────► check hook (one-shot)        ├──► gateway (long-lived)      ├──► npm shim
        │   internal/check/handler.go runCheck         │   internal/gateway/policy.go  │   internal/shim
        │                                              │   applyPolicy                 │
        ▼                                              ▼                               ▼
   ┌────────────────────────────────────────────────────────────────────────────────────┐
   │ STEP 1  pkgparse.Parse(cmd) → ParsedCommand{Manager,Ecosystem,Verb,Package,Version,  │
   │         IsInstall, Sudo, Unpinned}        (PURE, shared by all 3 consumers + engine) │
   └────────────────────────────────────────────────────────────────────────────────────┘
        │ IsInstall?
        ▼
   ┌──────────────────────────────┐        ┌─────────────────────────────────────────────┐
   │ STEP 2a  catalog match (F3)  │        │ STEP 2b  nudge.DetectState / Cache.State      │
   │ policy.Evaluate via LookupAll│        │ (IMPURE — exec pnpm/bun/node --version, 2s    │
   │ now keyed for pnpm/bun too   │        │  timeout; scan bunfig.toml/pnpm-workspace)    │
   │ → catalog Decision           │        │ → PMState   [check: fresh; gateway: 60s cache]│
   └──────────────────────────────┘        └─────────────────────────────────────────────┘
        │                                          │
        │                                          ▼
        │                              ┌──────────────────────────────────────────┐
        │                              │ STEP 3  nudge.Evaluate(parsed,state,cfg)   │
        │                              │ (PURE) → Decision{Action,Reason,Rewritten} │
        │                              │ Advise | Rewrite | Proceed | Block          │
        │                              └──────────────────────────────────────────┘
        │                                          │
        ▼                                          ▼
   ┌────────────────────────────────────────────────────────────────────────────┐
   │ STEP 4  mergeDecisions(catalogDecision, nudgeDecision)  most-restrictive-wins │
   │         (mirrors Phase 7 path-block merge; block > warn/advise > allow)        │
   └────────────────────────────────────────────────────────────────────────────┘
        │
        ▼
   exit code + audit record_type:"nudge" (+ "version_drift" weekly, async)
```

### Recommended Project Structure
```
internal/
├── pkgparse/                 # NEW — pure shared install-command parser (Flag 4)
│   ├── pkgparse.go
│   ├── pkgparse_test.go      # table-driven + TestPkgparseImportsArePure
│   └── fuzz_test.go          # FuzzParse (//go:build fuzz) — BTEST-03 fuzz gate
├── nudge/                    # NEW — the feature
│   ├── evaluate.go           # PURE: nudge.Evaluate(ParsedCommand, PMState, Config) Decision
│   ├── detect.go             # IMPURE: DetectState (exec + file scan, 2s timeout) + Cache (gateway-only)
│   ├── rewrite.go            # PURE: npm→pnpm/bun verb mapping (add/install/dlx/x); no-arg + npx forms
│   ├── version.go            # PURE: semver floor checks, drift detection
│   ├── reasons.go            # closed reason-code enum (PRD §9)
│   ├── config.go             # NudgeConfig defaults + bounds (mirrors DefaultReleaseAgeConfig)
│   ├── scanners.go           # IMPURE: hand-written bunfig.toml / pnpm-workspace.yaml scanners
│   ├── evaluate_test.go      # table-driven PRD §10 criteria 1-10,14-17 (BTEST-01)
│   ├── detect_test.go        # injected detect-fn: timeout fallback (12), cache TTL (11), parse-fail (13)
│   ├── rewrite_test.go
│   ├── version_test.go
│   └── scanners_fuzz_test.go # FuzzBunfig + FuzzPnpmWorkspace (//go:build fuzz) — BTEST-03
├── check/
│   ├── handler.go            # EDIT: wire nudge into runCheck (after overlay, mergeDecisions)
│   ├── nudge_adapter.go      # NEW (optional): impure glue calling nudge.DetectState in check path
│   ├── integration_test.go   # EDIT: add pnpm/bun install integration cases (BTEST-02)
│   └── e2e_test.go           # NEW (//go:build e2e): compiled-binary battery (BTEST-03 release gate)
├── policy/engine.go          # EDIT: use pkgparse; pnpm/bun/yarn → "npm" ecosystem (F3, SC1)
├── policyloader/enforce.go   # EDIT: use pkgparse
├── audit/types.go            # EDIT: nudge + version_drift record types/fields
└── config/config.go          # EDIT: add Nudge NudgeConfig
cmd/beekeeper/
├── main.go                   # EDIT: root.AddCommand(newNudgeCmd())
└── nudge.go                  # NEW: thin Cobra wiring for status/check/audit
docs/
└── nudge.md                  # NEW (PRD §13)
```

### Pattern 1: Pure decision over caller-resolved input (THE locked pattern)
**What:** The decision function takes a fully-resolved input struct; all I/O happens in a separate adapter.
**When to use:** `nudge.Evaluate` — mandatory per CLAUDE.md and PRD §3.2 editor's note.
**Example:**
```go
// Source: internal/policy/release_age.go (verbatim pattern — the canonical analog)
func EvaluateReleaseAge(input ReleaseAgeInput, cfg ReleaseAgeConfig) Decision {
    // ... no time.Now(), no exec, no os — pure over input + cfg
}
// nudge mirror:
func Evaluate(cmd pkgparse.ParsedCommand, state PMState, cfg Config) Decision {
    // ... no exec, no file reads — state was resolved by detect.go
}
```

### Pattern 2: Most-restrictive-wins merge into the live pipeline (Phase 7 precedent)
**What:** Compute an independent decision, then merge it into the catalog decision with `mergeDecisions` (block > warn > allow).
**When to use:** Wiring nudge into `runCheck`/`applyPolicy`. Run AFTER `ApplyPolicyOverlay` so a `package_allowlist` allow cannot downgrade a nudge `Block` (mirrors CR-02 path-block ordering exactly).
**Example:**
```go
// Source: internal/check/handler.go lines 280-288 (Phase 7 path block) — repeat for nudge
nudgeDecision := nudgeAdapter.Evaluate(ctx, toolCall, nudgeCfg) // adapter resolves PMState
decision = mergeDecisions(decision, nudgeDecision)              // most-restrictive-wins
```
**Subtlety:** nudge `Advise` and `Rewrite` are NOT "block" — they keep `Allow=true` / exit 0 (soft mode never blocks, NUDGE-03). Only `Block` (requireHardened) is restrictive. `mergeDecisions` uses a `rank` map keyed on `Level` ("allow"/"warn"/"block"); nudge `Advise` must map to a non-blocking level (use "warn" so it surfaces, or "allow" with the advisory in `Reason`). **Decision for planner:** map `Advise`→level "warn" (exit 0, surfaces in audit), `Rewrite`→"warn" (exit 0; the rewrite is in the audit `rewritten_command`), `Block`→"block", `Proceed`→"allow". This keeps `exitCodeFor` semantics intact (only `Allow=false` blocks).

### Pattern 3: Injected function for testable I/O (existing repo idiom)
**What:** Adapter I/O is wrapped in a package-level `var fn = realImpl` so tests substitute a fake.
**When to use:** `detect.go` exec calls + `Cache` clock. Precedents: `osLookPath = exec.LookPath` (shim.go:35), `scanOnDeltaFn` / `runBumblebeeFn` (STATE.md Phase 11), `catalogOpener` injection (handler.go).
```go
// Source: internal/shim/shim.go:35
var osLookPath = exec.LookPath          // tests override with a fake
// nudge mirror:
var pnpmVersionFn = func(ctx context.Context) (string, error) { /* exec */ }
```

### Anti-Patterns to Avoid
- **Exec from the pure decision.** `nudge.Evaluate` must NEVER call `exec`/`os`/`time` — enforced by `TestNudgeEvaluateImportsArePure` (copy `TestReleaseAgeImportsArePure` AST-import check).
- **A cache in the check hook.** One-shot process → no hits, only hot-path I/O cost (Flag 2).
- **A third copy of install-prefix parsing.** Extract to `pkgparse` (Flag 4).
- **Blocking on detection failure.** Timeout/error → treat PM as "not installed", proceed (PRD §10 criterion 12). This is the ONE place the feature is *fail-OPEN by design* (a detection failure must not block the agent) — distinct from the catalog/path fail-closed rule. Document this explicitly so it doesn't look like a fail-closed violation.
- **More than one advisory per session.** NUDGE-03 caps advisories at one per session — this is *gateway-session* state (lives with the Cache), not check-hook state.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Install-command parsing | A nudge-private parser (3rd copy) | `internal/pkgparse.Parse` (Flag 4) | Drift between catalog-match and nudge views of "what is being installed" is a security bug |
| Subprocess timeout | manual `time.AfterFunc` + kill | `exec.CommandContext` + `context.WithTimeout(2s)` | Stdlib handles process-group kill on Windows/Unix; already the repo idiom |
| Audit query for `nudge audit` | new NDJSON scanner | `audit.Query` + `audit.QueryOpts{Since,...}` | `cmd/beekeeper/main.go` audit query already does exactly this; reuse + filter `record_type:"nudge"` |
| Config bounds validation | ad-hoc checks | mirror `policyloader.validateCorroborationThresholds` | Same fail-closed-on-bad-config discipline (CORR-02 precedent) |
| Most-restrictive merge | new merge logic | `check.mergeDecisions` (Phase 7) | Already correct + tested |
| Semver compare for floors | regex | small pure `version.go` (major.minor int compare) | NO new dep allowed; floors are simple major.minor gates (11.0 / 1.3 / 22) — full semver lib is overkill and adds supply-chain footprint to a security tool |

**Key insight:** Almost every building block already exists in the repo from Phases 1-7. The genuinely new code is (1) the `pkgparse` extraction, (2) `nudge.Evaluate` + detection adapter, (3) hand-written config scanners + their fuzz targets, and (4) the live-binary E2E harness. Everything else is wiring that copies an existing pattern verbatim.

---

## Common Pitfalls

### Pitfall 1: Windows corepack-shimmed pnpm `cmd.exe` startup vs the 2s timeout
**What goes wrong:** On this dev machine `pnpm` resolves to `~/AppData/Roaming/npm/pnpm` and `corepack` is present `[VERIFIED: command -v probe]`. A corepack-shimmed `pnpm --version` spawns `cmd.exe` → corepack → Node → pnpm, which can be slow on a cold Windows process, risking the 2s detection timeout.
**Why it happens:** Windows process spawn + corepack's Node bootstrap is heavier than a native binary; first call in a session is slowest.
**How to avoid:** (a) Run `pnpm --version` via `exec.CommandContext(ctx, ...)` with the 2s deadline; on timeout treat as "not installed" and proceed (never block). (b) The gateway 60s cache amortizes the cost (one slow call per minute, not per request). (c) The check hook eats the cost fresh each call — acceptable because soft mode just means "no advisory this time" on a slow box, never a wrong block.
**Warning signs:** `version_drift`/`nudge` records intermittently showing `pnpm_version:""` on Windows CI. STATE.md flags this as needing live CI timing — the plan should add a Windows-CI timing assertion or a tunable timeout (default 2s, documented).
**Note:** `bun` is NOT installed on this machine `[VERIFIED]` — bun-path tests must use injected detect-fns, not a real `bun` binary, or they'll be skipped on the dev box.

### Pitfall 2: Hot-path I/O in the one-shot check hook
**What goes wrong:** Adding file-cache reads/writes or unbounded detection to `runCheck` regresses the carefully-bounded 5s/256MB check budget (CLAUDE.md, `GlobalHookTracker`).
**How to avoid:** Flag 2 Position B — no cache in the check hook; detection is the only added I/O and it's 2s-bounded and runs after the cheap `pkgparse.Parse` gate (only when `IsInstall`). Non-install commands (`npm ls`, `npm run`) never trigger detection (criterion 7).

### Pitfall 3: Parse-failure safety of hand-written config scanners
**What goes wrong:** A malformed `bunfig.toml` or `pnpm-workspace.yaml` crashes the scanner → crashes the nudge module → (worst case) fails the whole check.
**How to avoid:** Scanners return `(value, ok)` and NEVER panic; on any parse error default to the safe value (`BunScannerOK=false`, `pnpm_hardened` per defaults) and log a warning (PRD §6.2/§6.3, criterion 13). Enforce with `FuzzBunfig`/`FuzzPnpmWorkspace` (BTEST-03 requires fuzz targets for these scanners). Mirror the `gateway/parser_fuzz_test.go` contract: "must NEVER panic on any input."

### Pitfall 4: At-most-one-advisory-per-session state placement
**What goes wrong:** Implementing the "one advisory per session" cap (NUDGE-03) as global/process state in the one-shot check hook does nothing (each call is a new process) or, worse, as a file lock adds hot-path I/O.
**How to avoid:** Session-scoped state lives with the gateway `Cache` (the only long-lived session). In the check hook, "session" is effectively one call — every install gets at most one advisory naturally. Document that the per-session cap is a gateway property; the check hook is inherently one-advisory-per-call.

### Pitfall 5: sudo passthrough must parse but not rewrite
**What goes wrong:** `sudo npm install foo` gets rewritten to `sudo pnpm add foo` (wrong — sudo is a separate threat surface).
**How to avoid:** `pkgparse.Parse` sets `Sudo=true` and strips the prefix for *parsing/logging*, but `nudge.Evaluate` returns `Proceed`/`Advise` (logged) and NEVER `Rewrite` when `Sudo` is true (PRD §6.4, criterion 10). Test it explicitly.

### Pitfall 6: The `version_drift` weekly record is async and must not block
**What goes wrong:** Running `pnpm view pnpm version` / `bun upgrade --check` on the hot path (it's a network call) blows the budget.
**How to avoid:** The weekly drift check (PRD §7.1, criterion 15) is a scheduled task in the long-lived gateway/daemon, NEVER on the check hot path. It emits `record_type:"version_drift"` severity `info` and does NOT auto-update floors (Out-of-Scope). Test it with an injected metadata-fetch fn returning a hypothetical "pnpm 12.0.0".

### Pitfall 7: F3 catalog-keying — pnpm/bun installs must map to ecosystem "npm"
**What goes wrong:** Adding `{"pnpm add","pnpm"}` to the prefix table makes `LookupAll("pnpm", pkg)` miss because the catalog is keyed by ecosystem `"npm"` for the JS registry.
**How to avoid:** Map pnpm/bun/yarn install prefixes to `Ecosystem:"npm"` (they all install from the npm registry). SC1 ("`pnpm add malware-pkg` ... surface in corroboration decisions") only works if the lookup key is `"npm"`. The `Manager` field stays "pnpm"/"bun" for the nudge/audit view; `Ecosystem` is the catalog key.

---

## Code Examples

### Detection adapter with 2s timeout + graceful fallback (PRD §6.1, criterion 12)
```go
// internal/nudge/detect.go (IMPURE)
// Source pattern: internal/shim/shim.go injected var + internal/check/handler.go context budget
var pnpmVersionFn = func(ctx context.Context) (string, error) {
    cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    out, err := exec.CommandContext(cctx, "pnpm", "--version").Output()
    if err != nil {
        return "", err // timeout or not-found → caller treats as "not installed"
    }
    return strings.TrimSpace(string(out)), nil
}
```

### Pure nudge decision skeleton (mirrors EvaluateReleaseAge)
```go
// internal/nudge/evaluate.go (PURE — no os/exec/time/sync/net/io)
func Evaluate(cmd pkgparse.ParsedCommand, state PMState, cfg Config) Decision {
    if !cfg.Enabled || !cmd.IsInstall {
        return Decision{Action: Proceed, Reason: ReasonNotApplicable}
    }
    if cmd.Sudo { // criterion 10 — parse + log, never rewrite
        return Decision{Action: Advise, Reason: ReasonSudoPassthrough, Original: cmd.Raw}
    }
    if state.PnpmInstalled && state.PnpmHardened {
        if !state.NodeOK() { // criterion 6
            return Decision{Action: Advise, Reason: ReasonNodeIncompatiblePnpm11}
        }
        if cfg.Mode == "hard" { // criterion 2
            return Decision{Action: Rewrite, Rewritten: rewriteToPnpm(cmd), Reason: ReasonPnpmHard}
        }
        return Decision{Action: Advise, Reason: ReasonPnpmAvailableSoft} // criterion 1
    }
    // ... bun branch (criterion 5: bun-available-no-scanner), then:
    if cfg.RequireHardened { // criterion 4
        return Decision{Action: Block, Reason: ReasonNoHardenedPM}
    }
    return Decision{Action: Proceed, Reason: ReasonNoHardenedPM} // criterion 3
}
```

### Live-binary E2E harness (NEW — BTEST-03 release gate; no repo precedent)
```go
//go:build e2e
// internal/check/e2e_test.go  (or a top-level e2e/ package)
func TestE2ELiveBinary(t *testing.T) {
    bin := filepath.Join(t.TempDir(), "beekeeper.exe")
    build := exec.Command("go", "build", "-o", bin, "github.com/bantuson/beekeeper/cmd/beekeeper")
    if out, err := build.CombinedOutput(); err != nil { t.Fatalf("build: %v\n%s", err, out) }
    // SPATH: credential read → exit 1; CORR: ai-figure critical → exit 1; NUDGE: pnpm add → parsed/audited
    cases := []struct{ name, stdin string; wantExit int }{ /* ... */ }
    for _, c := range cases {
        cmd := exec.Command(bin, "check")
        cmd.Stdin = strings.NewReader(c.stdin)
        // assert exit code AND read audit NDJSON record_type+decision (mirror readLastAuditRecord)
    }
}
```
**Note:** The `audit` log path must be redirected to a temp dir via env/flag for hermetic E2E; check how `newCheckCmd` resolves `auditPath` (it uses `platform.AuditDir()`) and whether a `BEEKEEPER_*` env override exists, or run with `HOME`/state-dir pointed at `t.TempDir()`. The planner must confirm the binary honors an overridable state dir for the E2E to be hermetic.

---

## Runtime State Inventory

This phase is net-new code/config (a feature), not a rename/refactor/migration. **Omitting the full inventory** — there is no stored data, OS-registered state, or build-artifact rename involved. Two adjacent runtime-state notes worth flagging for the planner:

- **Live service config:** the gateway 60s `nudge.Cache` is in-memory session state in the long-lived gateway process — not persisted, no migration. The weekly-drift scheduler is also gateway/daemon runtime state. Neither touches `~/.beekeeper/state.json` unless the planner chooses to persist last-drift-check timestamp there (recommended to avoid re-checking on every restart).
- **Config:** the new `nudge` block in `~/.beekeeper/config.json` is additive; absent block → `NudgeConfig` zero-value must mean `enabled:false`-safe OR explicit defaults. **Decision for planner:** default `enabled:true` per PRD §5.1, but a *missing* `nudge` key in an existing config must resolve to the documented defaults via the loader (mirror how `FailMode==""` → `FailModeClosed` in `config.Load`). Project `.beekeeper.json` `nudge.enabled:false` disables it (NUDGE-08, PRD §11) — layered config wins, same as existing layered merge.

---

## Validation Architecture

`workflow.nyquist_validation` is not disabled in config (treated as enabled). This section maps every Success Criterion / requirement to its validation tier so a VALIDATION.md can be derived directly.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (+ `testing/fuzz`) — go1.25 |
| Config file | none (Go convention); fuzz behind `//go:build fuzz`, E2E behind `//go:build e2e` |
| Quick run command | `go test ./internal/nudge/... ./internal/pkgparse/...` |
| Full suite command | `go test ./...` |
| Fuzz gate command | `go test -tags fuzz -run=Fuzz ./internal/nudge/... ./internal/pkgparse/... ./internal/gateway/...` |
| E2E gate command | `go test -tags e2e -run=TestE2ELiveBinary ./internal/check/...` |

### Phase Requirements → Test Map
| Req / SC | Behavior | Test Type | Automated Command | File |
|----------|----------|-----------|-------------------|------|
| NUDGE-01 / SC1 | pnpm/bun/yarn installs parsed + catalog-matched (F3) | unit + integration | `go test ./internal/pkgparse/... ./internal/policy/...` + integration case | pkgparse_test.go, engine_test.go (EDIT), integration_test.go |
| NUDGE-02 | timeout-bounded detection → PMState; Evaluate pure | unit | `go test ./internal/nudge/...` | detect_test.go, evaluate_test.go, `TestNudgeEvaluateImportsArePure` |
| NUDGE-03 / §10-1 | soft Advise + proceed (exit 0); ≤1 advisory/session | table-driven + gateway test | `go test ./internal/nudge/...` | evaluate_test.go |
| NUDGE-04 / §10-2,4 | hard Rewrite; requireHardened Block | table-driven | `go test ./internal/nudge/...` | evaluate_test.go (criteria 2, 4) |
| NUDGE-05 | unpinned (@latest/bare/wide range) flagged | unit | `go test ./internal/pkgparse/...` | pkgparse_test.go (Unpinned field) |
| NUDGE-06 / §10-14,15 | `record_type:"nudge"` schema; `version_drift` record | unit + audit-shape | `go test ./internal/nudge/... ./internal/audit/...` | evaluate_test.go, version_test.go, types_test.go (EDIT) |
| NUDGE-07 / SC5 | `nudge status\|check\|audit` CLI | CLI unit | `go test ./cmd/beekeeper/...` | nudge_test.go (NEW, mirror policy_test.go) |
| NUDGE-08 | wired into check+gateway+shim; layered config; cache gateway-only | integration | `go test ./internal/check/... ./internal/gateway/... ./internal/shim/...` | integration_test.go, gateway_test.go, shim_test.go |
| §10-5 | bun-available-no-scanner reason | table-driven | nudge | evaluate_test.go |
| §10-6 | node-incompatible-with-pnpm-11 | table-driven | nudge | evaluate_test.go |
| §10-7 | npm ls/run/publish NOT nudged | table-driven | pkgparse + nudge | pkgparse_test.go, evaluate_test.go |
| §10-8 | no-arg `npm install` softer reason | table-driven | nudge | evaluate_test.go |
| §10-9 | `npx` parsed as install+execute | table-driven | pkgparse | pkgparse_test.go |
| §10-10 | sudo parsed, NOT rewritten | table-driven | nudge | evaluate_test.go |
| §10-11 | 60s cache (gateway session) | gateway unit (injected clock) | `go test ./internal/nudge/... ./internal/gateway/...` | detect_test.go (Cache), gateway_test.go |
| §10-12 | 2s timeout → graceful fallback (not installed) | unit (injected slow fn) | nudge | detect_test.go |
| §10-13 | bunfig.toml parse failure → BunScannerOK=false, no crash | unit + fuzz | `go test ./internal/nudge/...` + fuzz | scanners_test.go, scanners_fuzz_test.go |
| §10-16 | minimumReleaseAge=0 → warn, pnpm_hardened stays true | unit | nudge | scanners_test.go / version_test.go |
| §10-17 | config change logged to audit | CLI + audit | `go test ./cmd/beekeeper/...` | nudge_test.go / config audit hook |
| BTEST-01 / SC3 | table-driven pure tests cover §10 1-10,14-17 | table-driven | `go test ./internal/nudge/... ./internal/policy/...` | evaluate_test.go (+ existing path/corroboration tests) |
| BTEST-02 | RunCheck integration: credential read, critical block, pnpm/bun install | integration | `go test -run TestIntegration ./internal/check/...` | integration_test.go (EDIT) |
| BTEST-03 / SC4 | live-binary E2E: SPATH+CORR+NUDGE exit codes + audit records | E2E (release gate) | `go test -tags e2e ./internal/check/...` | e2e_test.go (NEW) |
| BTEST-03 (fuzz) | bunfig.toml + pnpm-workspace.yaml + pkgparse fuzz never panic | fuzz (release gate) | `go test -tags fuzz -run Fuzz ./...` | scanners_fuzz_test.go, pkgparse/fuzz_test.go |

### Three CLI surfaces (SC5 / NUDGE-07)
| Surface | Validation |
|---------|------------|
| `beekeeper nudge status` | CLI test asserts human-readable PM state + config block printed (PRD §13: not just NDJSON) |
| `beekeeper nudge check "npm install chalk"` | CLI test asserts dry-run decision printed (mirror `policy test` output: decision/reason) |
| `beekeeper nudge audit --since=1h` | CLI test asserts `audit.Query` filtered to `record_type:"nudge"` returns records |

### Sampling Rate
- **Per task commit:** `go test ./internal/nudge/... ./internal/pkgparse/...`
- **Per wave merge:** `go test ./...` (full unit + integration)
- **Phase gate (release gate):** full suite green + `go test -tags fuzz -run Fuzz ./...` (seed corpus) + `go test -tags e2e ./internal/check/...` — ALL must pass before any v1.2.0 tag (SC4).

### Wave 0 Gaps
- [ ] `internal/pkgparse/` package + tests + fuzz — does not exist (Flag 4 extraction is Wave 0/1 prerequisite)
- [ ] `internal/nudge/` package (all files) — does not exist
- [ ] `internal/check/e2e_test.go` — no live-binary E2E precedent in repo; net-new harness
- [ ] `internal/nudge/scanners_fuzz_test.go` + `pkgparse/fuzz_test.go` — new fuzz targets (gateway fuzz exists as the template)
- [ ] `cmd/beekeeper/nudge.go` + `nudge_test.go` — new CLI command (policy.go / policy_test.go are the template)
- [ ] `audit.AuditRecord` fields for nudge (`original_command`, `rewritten_command`, `reason_code`, `pm_state`) — types.go EDIT
- [ ] Confirm binary honors overridable state/audit dir for hermetic E2E (env or flag) — VERIFY in `newCheckCmd`/`platform`

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| npm default install (no release-age guard) | pnpm 11 `minimumReleaseAge:1440` + `blockExoticSubdeps:true` ON by default | pnpm 11.0, 2026-04-28 `[VERIFIED]` | The nudge target: pnpm 11 is "hardened" because these defaults are on |
| No structural defense in Bun | Bun 1.3 Security Scanner API + `@socketsecurity/bun-security-scanner` | Bun 1.3 (2025-10), scanner integration 2025-10 `[VERIFIED]` | Bun "hardened" only WITH the Socket scanner in `bunfig.toml [install.security]` |
| Node 22 = Active LTS (PRD draft assumption) | Node 24 = Active LTS, Node 22 = Maintenance LTS, Node 26 = Current | 2026 LTS rotation `[VERIFIED]` | Flag 5 correction: floor stays 22, recommended target is 24 |

**Deprecated/outdated:**
- PRD §6.3 "less than 60" minimumReleaseAge threshold → corrected to 1440 (Flag 5).
- PRD §7 "latest 22.x LTS" recommended → corrected to "24.x Active LTS" (Flag 5).
- Current versions: pnpm **11.5.1**, bun **1.3.14** `[VERIFIED: registries, June 2026]` — both above PRD floors.

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Mapping nudge `Advise`/`Rewrite` to level `"warn"` (exit 0) is the right merge integration | Pattern 2 | If planner prefers a dedicated nudge level, `mergeDecisions`/`exitCodeFor` need extension; low risk (warn already = exit 0) |
| A2 | Live-binary E2E can be made hermetic via an overridable state/audit dir on the compiled binary | Code Examples, Wave 0 | If the binary hardcodes `platform.AuditDir()` with no override, E2E needs a HOME/env shim or a small flag addition — VERIFY before planning the E2E task |
| A3 | `version.go` simple major.minor int compare suffices for floors (11.0/1.3/22) | Don't Hand-Roll | If floors ever need patch-level or pre-release semantics, a fuller parser is needed; current floors are major.minor only |
| A4 | The weekly drift check belongs in the gateway/daemon scheduler (not a separate cron) | Pitfall 6 | If there's no always-on daemon in a given deployment, drift check never runs; acceptable (info-only, Out-of-Scope to auto-update) |
| A5 | `pkgparse` extraction will be byte-behavior-identical for existing engine/overlay tests | Flag 4 | If subtle differences exist (e.g. `npm i ` trailing-space handling), existing engine_test.go/enforce_test.go catch it — regression net is in place |

---

## Open Questions

1. **Hermetic E2E state dir (A2).**
   - What we know: `newCheckCmd` resolves `auditPath`/`indexPath` via `platform.*Dir()`; integration tests use `auditPathIn(t)` temp paths but that's the in-process harness, not the compiled binary.
   - What's unclear: whether the compiled `beekeeper` honors an env var (e.g. `BEEKEEPER_HOME`/`XDG`-style) to redirect state for E2E.
   - Recommendation: Wave 0 task — verify/add an overridable state dir env var; the E2E release gate depends on it.

2. **Drift-check scheduler home.**
   - What we know: PRD §7.1 wants a weekly check; the gateway is the long-lived process.
   - What's unclear: whether to persist `last_drift_check` in `state.json` (avoid re-check on restart) or keep purely in-memory.
   - Recommendation: persist a timestamp in `state.json` (gateway already reads/writes it); info-only record, no security impact.

3. **"Session" definition for the one-advisory cap in the gateway.**
   - What we know: cap is gateway-session state with the Cache.
   - What's unclear: is "session" the gateway process lifetime, or per-agent-id (lineage header)?
   - Recommendation: key the advisory-seen set by agent-id when present, else process-global; document in the plan.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build + all tests | ✓ | go1.25.0 windows/amd64 `[VERIFIED]` | — |
| node | detection + pnpm 11 compat check | ✓ | present (`/c/Program Files/nodejs`) `[VERIFIED]` | n/a (detected at runtime) |
| npm | the command being nudged | ✓ | present (bundled w/ node) `[VERIFIED]` | n/a |
| corepack | shims pnpm on Windows | ✓ | present `[VERIFIED]` | n/a |
| pnpm | detection target / hard-mode rewrite | ✓ | corepack-shimmed at `~/AppData/Roaming/npm/pnpm` `[VERIFIED]` | injected detect-fn in tests |
| bun | detection target / bun nudge | ✗ | NOT installed `[VERIFIED]` | bun-path tests MUST use injected detect-fns (no real binary on dev box) |
| clang/llvm | (eBPF only — N/A this phase) | ✗ | NOT installed `[VERIFIED]` | irrelevant to Phase 8 |

**Missing dependencies with no fallback:** none (Phase 8 needs no tool that isn't present or injectable).
**Missing dependencies with fallback:** `bun` — all bun-branch unit tests use injected detect-fns (`pnpmVersionFn`-style vars); a real `bun` is only needed for the live-binary E2E *if* a bun case is included — make the bun E2E case skip-if-absent (`t.Skip` when `exec.LookPath("bun")` fails) so the dev box and Windows CI without bun stay green; pnpm E2E case is the primary NUDGE assertion (pnpm IS present).

---

## Security Domain

`security_enforcement` is not disabled (treated as enabled).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V5 Input Validation | yes | `pkgparse.Parse` + hand-written `bunfig.toml`/`pnpm-workspace.yaml` scanners must be parse-failure-safe (never panic) — enforced by fuzz targets (BTEST-03) |
| V6 Cryptography | no | Phase 8 adds no crypto; catalog signature verification is unchanged (Phase 2/11) |
| V12 Files/Resources | yes | Detection reads `bunfig.toml`/`pnpm-workspace.yaml` from project root + `~`; bounded reads, no write, no exec of file contents |
| V13/V14 (config / build) | yes | New `nudge` config block validated with fail-closed bounds (mirror `validateCorroborationThresholds`); E2E + fuzz are release gates |
| V2/V3/V4 (authn/session/access) | no | No auth/session surface added |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Command injection via crafted install command | Tampering/Elevation | `pkgparse` is string-matching only; the command is NEVER passed to a shell or executed (mirrors Phase 7 `canonicalizePath` "never executed" rule). Detection execs fixed argv (`pnpm`,`--version`) — no user input in argv. |
| Malformed config file crashes the security tool (DoS / fail-bypass) | Denial of Service | Scanners return safe defaults on parse error + fuzz targets (criterion 13, BTEST-03) |
| Rewrite injects attacker-controlled tokens (`sudo`, `;`, `&&`) | Tampering | Rewrite operates on the parsed package/version tokens only; sudo → never rewrite (criterion 10); reject/proceed on shell-metacharacter tokens rather than rewrite them |
| Trust-footprint expansion (recommending pnpm / Socket scanner) | (Supply chain) | PRD §12 self-defense: Beekeeper never installs either; recommendation text is explicit; audit records every nudge for forensic trace. The drift check does NOT auto-update floors (no silent trust expansion). |
| F3 bypass: pnpm/bun installs of known-malicious packages evade catalog | Tampering | NUDGE-01: pnpm/bun/yarn now parsed → ecosystem "npm" → catalog corroboration applies (SC1) |

**Phase-specific self-defense note (CLAUDE.md disambiguation):** Per CLAUDE.md, v1.2.0 Phase 8's self-defense IS the behavioral test suite + live-binary E2E as the release gate (NOT SLSA/SBOM, which was v1.0.0 Phase 7). The fuzz targets on the hand-written scanners + pkgparse, plus the E2E gate, are the deliverable. No separate self-defense artifact is required.

---

## Project Constraints (from CLAUDE.md)

The planner must honor these with the same authority as locked decisions:
- **`internal/policy` is a PURE function library** — no I/O, no goroutines, no side effects. `nudge.Evaluate` mirrors this (in `internal/nudge`, enforced by an imports-purity test). Detection I/O lives ONLY in `detect.go`/adapter.
- **Fail closed by default** — catalog/path decisions fail closed. EXCEPTION (documented): nudge *detection failure* is fail-open by design (a slow/absent PM must not block the agent — PRD §10 criterion 12). This is not a fail-closed violation; it is the soft-nudge contract. The plan must state this distinction explicitly.
- **Hook handler loads catalog via mmap, never cold-load per invocation** — reinforces Flag 2 Position B (no per-call cache file I/O in the check hook).
- **Single static Go binary, Go 1.25+, no new CGO** — no new third-party deps (hand scanners, not TOML/YAML libs).
- **`cmd/beekeeper/main.go` thin Cobra wiring only** — `nudge.go` is thin wiring; all logic in `internal/nudge`.
- **MCP gateway: stateless per-request proxy** — the 60s cache is process-level detection memoization, NOT per-request session state; it does not violate stateless-proxy (it caches local-machine PM versions, identical across requests).

---

## Sources

### Primary (HIGH confidence)
- `internal/policy/release_age.go` + `release_age_test.go` — the canonical pure-decision-over-input + `TestReleaseAgeImportsArePure` pattern
- `internal/check/handler.go` (runCheck), `paths.go` (Phase 7 impure adapter + mergeDecisions), `integration_test.go` (runCheckWithIndex + readLastAuditRecord) — wiring + integration-test templates
- `internal/policy/engine.go` (`installPrefixes`, `extractFromCommand`) + `internal/policyloader/enforce.go` (`installPrefixesOverlay`, duplicated) — Flag 4 two-copy evidence
- `internal/gateway/proxy.go` + `policy.go` (long-lived `Start` loop, per-request `applyPolicy`) — Flag 2 gateway-is-the-cache-home evidence
- `internal/audit/types.go` (AuditRecord, FromDecision) — record-type extension point
- `internal/gateway/parser_fuzz_test.go` — fuzz-gate template (never-panic contract)
- `internal/shim/shim.go` (`osLookPath` injected var) + `cmd/beekeeper/policy.go` / `main.go` (CLI + audit query) — CLI + injectable-I/O templates
- `internal/config/config.go` — config struct + `Load` defaulting pattern
- `.planning/specs/NUDGE-PRD.md`, `.planning/STATE.md`, `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md` — phase scope, Flags, SCs

### Secondary (MEDIUM-HIGH confidence — official docs)
- pnpm 11 release / settings — `[VERIFIED: pnpm.io/blog/releases/11.0, pnpm.io/settings]` (minimumReleaseAge=1440, blockExoticSubdeps=true, Node 22+ required)
- Node.js release schedule — `[VERIFIED: nodejs.org, endoflife.date/nodejs]` (22 Maintenance LTS, 24 Active LTS, 26 Current)
- Bun 1.3 Security Scanner API — `[VERIFIED: bun.com/docs/pm/security-scanner-api, github.com/SocketDev/bun-security-scanner]` (`[install.security] scanner = "@socketsecurity/bun-security-scanner"`)
- Current versions — `[VERIFIED: registries June 2026]` pnpm 11.5.1, bun 1.3.14

### Local environment probe
- `go version` + `command -v` for go/npm/pnpm/bun/node/corepack/clang on the Windows dev machine

---

## Metadata

**Confidence breakdown:**
- Resolved decisions (Flag 2/4/5): HIGH — Flag 2/4 verified by reading the actual gateway loop and the two duplicate parser copies; Flag 5 verified against current pnpm/Node sources and the repo's own `DefaultReleaseAgeConfig`
- Standard stack: HIGH — pure Go stdlib + existing internal packages; no new deps (confirmed by REQUIREMENTS Out-of-Scope)
- Architecture: HIGH — every pattern is a verbatim repeat of an existing, tested phase (release_age purity, Phase 7 merge, gateway applyPolicy, fuzz template, CLI template)
- Pitfalls: HIGH for Windows corepack timing (live-probed corepack present), parse-safety, F3 keying; MEDIUM for exact session-cap semantics (Open Q3)

**Research date:** 2026-06-04
**Valid until:** 2026-07-04 (stable — Go stdlib + internal patterns; PM version facts may bump minor versions but floors/LTS status are stable for ~30 days)
