# Phase 9: Policy as Code + Self-Defense Capstone - Research

**Researched:** 2026-05-29
**Domain:** Policy schema design, layered config, self-quarantine catalog, diagnostic assembly, threat-model documentation
**Confidence:** HIGH (all claims verified against live codebase or locked PRD decisions)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- CODE-01: Declarative JSON policy files in `policies/` — loaded OUTSIDE the pure engine, passed in as data. No I/O in `internal/policy`. No code execution surface (no URL fetching, no eval, no module loading). Separate from `config.json`.
- CODE-02: `beekeeper policy test <file>` — dry-run against sample tool-call JSON, reuses `policy.Evaluate`.
- CODE-03: `beekeeper policy validate <file>` — schema check, exit non-zero on invalid.
- CODE-04: `beekeeper policy list` — list loaded policy files with rule counts.
- CODE-05: Layered config merge order: `/etc/beekeeper/config.json` → `~/.beekeeper/config.json` → `<project>/.beekeeper/config.json` → `BEEKEEPER_*` env vars → CLI flags. Project layer overrides user WITHOUT env vars.
- CODE-06: `beekeeper diag` — hook latency p95/p99, sidecar inference latency, catalog freshness per source, ETW `EventsLost`.
- CTLG-04/SFDF-06: `beekeeper-self` catalog — separate host, separate signing key, checked on startup + every catalog sync. Self-quarantine on match (refuse to run, surface warning, point to verification path). Same static-JSON/signature discipline as all other sources.
- Threat-model docs: publish Section 12 self-defense writeup including coordinated false-positive poisoning surface and fanotify mmap gap. Link PRD §12.7 verification path.

### Claude's Discretion
- Policy file JSON schema design (must map onto existing pure engine inputs/outputs without code execution surface)
- Where loaded policy files compose with built-in policies (override vs. additive)
- `beekeeper-self` transport/host details (configurable separate base URL + separate trusted signing key)
- `diag` output formatting (table vs. sections — human-readable is the only constraint)
- Self-quarantine state representation (marker in `~/.beekeeper/state.json` or quarantine dir)

### Deferred Ideas (OUT OF SCOPE)
- Distributed mode / team-shared catalogs
- Independent external security review / bug-bounty / VDP publication
- Weighted corroboration
- `beekeeper-self` separate maintainer set (documented single-maintainer for v1.0.0)
- macOS notarization / Windows trusted-publisher code signing
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CODE-01 | Declarative JSON policy files in `policies/` — separate from config, version-controllable, loaded outside pure engine | Schema design §§ below; existing engine signature confirmed |
| CODE-02 | `beekeeper policy test <file>` — dry-run policy against sample tool-call JSON | `policy.Evaluate` signature documented; test harness pattern clear |
| CODE-03 | `beekeeper policy validate <file>` — validate policy file schema | JSON schema validation approach documented |
| CODE-04 | `beekeeper policy list` — list loaded policy files with rule counts | Loader + rule-count aggregation pattern documented |
| CODE-05 | Layered config merge: system → user → project → env vars → CLI flags | Existing `config.Load` documented; missing layers identified |
| CODE-06 | `beekeeper diag` — hook latency p95/p99, sidecar latency, catalog freshness, ETW EventsLost | All data sources located in codebase; cross-platform stubs pattern documented |
| SFDF-06 | `beekeeper-self` catalog live — separate host, separate key, self-quarantine on startup + sync | Integration point in `catalog.MultiIndex` and `catalog.Watch` documented |
| CTLG-04 | `beekeeper-self` catalog — self-quarantine feed, checked on every startup + catalog sync | Same as SFDF-06; implementation pattern detailed below |
</phase_requirements>

---

## Summary

Phase 9 is the v1.0.0 capstone. Eight requirements are scoped across five coherent workstreams: (1) a declarative policy-as-code system with `policy validate|test|list` commands, (2) a complete 5-layer config merge, (3) a `beekeeper diag` health output, (4) the `beekeeper-self` self-quarantine catalog, and (5) public threat-model documentation. All workstreams build on existing packages with no new heavy dependencies.

**The codebase is in very good shape for this phase.** The pure `policy.Evaluate` function signature is stable and clean. The `catalog.MultiIndex` aggregator already has an extensible adapter pattern for plugging in a fourth source. The `config.Load` function has explicit comments marking where CODE-05 layers belong. The LlamaFirewall `LatencyTracker` exposes `P95()` and `Mean()` accessors. The ETW `EventsLost` counter is a package-level `uint64` read via `atomic.LoadUint64`. Hook-handler latency is NOT currently tracked as a persistent time-series — `diag` must add a new `LatencyTracker` instance for hook calls.

**Self-quarantine (CTLG-04/SFDF-06) is the highest-complexity item.** The tension between "fail closed on integrity failure" and "don't brick on transient network failure" is non-trivial and requires an explicit design decision (documented below under Architecture Patterns).

**Primary recommendation:** Implement the five workstreams as five independent plan groups, sequenced so that the policy schema (CODE-01..04) and layered config (CODE-05) land in Wave 1, `diag` (CODE-06) lands in Wave 2 reusing already-built hook latency tracking, `beekeeper-self` (CTLG-04/SFDF-06) lands in Wave 3 as the most safety-critical item, and threat-model docs land in Wave 4 as the final release gate.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Policy file parsing/loading | CLI/I/O tier (`internal/policyloader` or `cmd/`) | — | CLAUDE.md: `internal/policy` must be pure; all I/O is outside the engine |
| Policy schema validation | CLI/I/O tier | — | JSON schema check is I/O that produces an error or nil |
| Policy evaluation (test dry-run) | `internal/policy` (pure) | Loaded by CLI | Reuses `policy.Evaluate` unchanged |
| Layered config merge | `internal/config` | `cmd/` (flag binding) | Already has `Load(path)` pattern; project + env + flag layers added here |
| Hook latency tracking | `internal/check` | — | `RunCheck` is the measurement point; new `LatencyTracker` instance |
| Sidecar latency | `internal/llamafirewall` | — | `LatencyTracker` already exists; expose via accessor |
| ETW EventsLost | `internal/sentry/windows` (Windows build tag) | Stub on non-Windows | Package-level atomic var; IPC `StatusResponse.EventsDropped` already wires it |
| Catalog freshness | `internal/catalog` (`state.go`) | — | `SourceState` has `Hash`, `Count`, `Degraded` per source |
| `beekeeper-self` client | `internal/catalog` (new source adapter) | — | Plugs into `MultiIndex` exactly like OSV/Socket adapters |
| `beekeeper-self` startup check | `cmd/` (startup hook) or `internal/check` init | — | Must run before any tool-call evaluation; also on every `catalogs sync` |
| Self-quarantine state | `~/.beekeeper/state.json` | — | `WatchState.Sources` map already holds per-source state; add a `SelfQuarantine` marker |
| Threat-model docs | `docs/THREAT-MODEL.md` | — | Static markdown; no Go code |

---

## Standard Stack

### Core (no new runtime dependencies required)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | stdlib | Policy file parsing, JSON schema validation | Already used throughout |
| `os` / `path/filepath` | stdlib | Policy file loading from `policies/` dir | Already used throughout |
| `sync/atomic` | stdlib | Read `windows.EventsLost` on non-Windows via cross-compile stub | Already used |
| `github.com/spf13/cobra` | existing | New `policy` and `diag` subcommands | Already in `go.mod` |

No new external dependencies are required for Phase 9. All new packages (`internal/policyloader`, potential `internal/selfcatalog`) use only stdlib and existing internal packages.

**Version verification:** No new packages added — no version check needed. [VERIFIED: codebase inspection]

---

## Architecture Patterns

### System Architecture Diagram

```
Startup / catalogs sync
        │
        ▼
  beekeeper-self fetch
        │
        ├─ Network error → cache hit? yes → use cached → continue
        │                              no  → FAIL CLOSED (block all checks)
        ├─ Signature invalid → FAIL CLOSED (block all checks, log reason)
        └─ Version match → SELF QUARANTINE (refuse to run, print warning + PRD§12.7 link)

Policy as Code
  Developer writes policies/*.json
        │
        ▼
  policy validate <file>   ──→  JSON schema check (I/O tier)
  policy test <file>        ──→  load file → build MultiCatalogLookup → policy.Evaluate → print decision
  policy list               ──→  scan ~/.beekeeper/policies/, count rules per file

Layered Config Merge (CODE-05)
  /etc/beekeeper/config.json      (system, optional)
        ↓ JSON merge (lower wins)
  ~/.beekeeper/config.json        (user)
        ↓ JSON merge
  <cwd>/.beekeeper/config.json    (project, optional)
        ↓ env-var overlay
  BEEKEEPER_* env vars
        ↓ flag overlay
  CLI flags
        │
        ▼
  config.Config (final merged value)

beekeeper diag (CODE-06)
        ├─ hook_latency_p95/p99   ← internal/check.GlobalHookTracker.P95()/.P99()
        ├─ sidecar_latency_p95    ← internal/llamafirewall.GlobalLatencyTracker.P95()
        ├─ catalog_freshness      ← catalog.LoadState(stateFile).Sources[name]
        │                              (last sync time derived from SourceState.Hash staleness)
        └─ etw_events_lost        ← windows.EventsLost (atomic read; 0 on non-Windows via stub)
```

### Recommended Project Structure

```
internal/
  config/
    config.go           # existing Load(path) — extend with LoadLayered(opts)
  catalog/
    selfcatalog.go      # beekeeper-self adapter: fetch, verify, match running version
    selfcatalog_test.go
  check/
    handler.go          # existing — add GlobalHookTracker *llamafirewall.LatencyTracker
    diag.go             # new: DiagReport struct + CollectDiag()
    diag_windows.go     # etw_events_lost: atomic.LoadUint64(&windows.EventsLost)
    diag_other.go       # etw_events_lost: return 0 (build tag !windows)
  policyloader/         # new package: I/O side of policy-as-code
    loader.go           # LoadPolicyFile, ListPolicyFiles, ValidateSchema
    loader_test.go
cmd/beekeeper/
  main.go               # add newPolicyCmd(), newDiagCmd()
docs/
  THREAT-MODEL.md       # new: Section 12 public writeup
```

### Pattern 1: Policy File Schema Design (CODE-01)

**What:** A declarative JSON file that maps onto the inputs of `policy.Evaluate` — `ToolCall`, `MultiCatalogLookup`, `CorroborationThresholds`, `AgentContext` — without introducing any code-execution surface. The schema expresses **restrictions** (what to block/warn) and **overrides** (allowlists, threshold adjustments) over the engine's built-in logic.

**When to use:** Every capability in the schema must be representable as a pure data predicate that the existing engine already evaluates, or as a parameter it already accepts. No new expression language, no URL references, no exec paths.

**Recommended schema:**

```json
{
  "schema_version": "1",
  "name": "my-project-policy",
  "description": "Project-level restrictions for my-repo",
  "rules": [
    {
      "id": "block-fresh-npm",
      "rule_type": "release_age",
      "ecosystems": ["npm"],
      "min_age_hours": 48,
      "action": "block"
    },
    {
      "id": "allow-react",
      "rule_type": "package_allowlist",
      "ecosystem": "npm",
      "packages": ["react", "react-dom"],
      "note": "Trusted first-party packages exempt from release-age policy"
    },
    {
      "id": "block-ssh-reads",
      "rule_type": "sensitive_path",
      "path_patterns": ["~/.ssh/**"],
      "action": "block"
    },
    {
      "id": "custom-lifecycle-allowlist",
      "rule_type": "lifecycle_script_allowlist",
      "packages": ["esbuild", "node-gyp"],
      "ecosystems": ["npm"]
    },
    {
      "id": "lower-corroboration-threshold",
      "rule_type": "corroboration_threshold",
      "ecosystem": "editor-extension",
      "warn_at": 1,
      "block_at": 1,
      "quarantine_at": 2
    }
  ]
}
```

**Key safety invariants:**
- No `"action": "exec"` or any field that triggers code execution [VERIFIED: engine is purely data-driven, CLAUDE.md §11.3]
- No `"url"` fields that Beekeeper fetches at evaluation time [VERIFIED: PRD §12.3]
- `rule_type` is an enum — unknown values produce a validation error, not silent acceptance
- `schema_version` is validated before loading; unknown versions reject the file

**How it composes with built-in policies (Claude's Discretion recommendation):** User policy files are **additive + override**, not replacement. Built-in rules (lifecycle deny-by-default, sensitive path defaults, release-age defaults, corroboration thresholds) remain active unless explicitly overridden by a policy file rule of the same `rule_type` + `ecosystem`. This is the least-surprise model: adding a policy file can only restrict or relax, never silently drop built-in protections unless an explicit allowlist rule appears.

**Rule count for `policy list`:** Count `len(file.rules)` per loaded file. Simple, deterministic.

**Source:** [VERIFIED: codebase — `internal/policy/types.go`, `engine.go`, `corroboration.go`, `path.go`, `lifecycle.go`, `release_age.go`; PRD §11.3, §12.3]

### Pattern 2: Policy Loader (I/O Side, Outside Pure Engine)

The pure engine (`internal/policy`) must not be touched. All I/O lives in `internal/policyloader`:

```go
// Source: internal/policyloader/loader.go

// PolicyFile is the typed in-memory representation of a loaded policies/*.json file.
// It is parsed OUTSIDE the pure engine and its rules are converted to engine inputs
// before Evaluate is called. No I/O or side effects in this struct.
type PolicyFile struct {
    SchemaVersion string        `json:"schema_version"`
    Name          string        `json:"name"`
    Description   string        `json:"description"`
    Rules         []PolicyRule  `json:"rules"`
}

// PolicyRule is one entry in PolicyFile.Rules.
type PolicyRule struct {
    ID          string   `json:"id"`
    RuleType    string   `json:"rule_type"` // "release_age" | "package_allowlist" | "sensitive_path" | "lifecycle_script_allowlist" | "corroboration_threshold"
    Ecosystems  []string `json:"ecosystems,omitempty"`
    Ecosystem   string   `json:"ecosystem,omitempty"`
    Packages    []string `json:"packages,omitempty"`
    PathPatterns []string `json:"path_patterns,omitempty"`
    MinAgeHours int      `json:"min_age_hours,omitempty"`
    Action      string   `json:"action,omitempty"`   // "block" | "warn" | "allow"
    WarnAt      int      `json:"warn_at,omitempty"`
    BlockAt     int      `json:"block_at,omitempty"`
    QuarantineAt int     `json:"quarantine_at,omitempty"`
    Note        string   `json:"note,omitempty"`
}

// ValidateSchema checks PolicyFile for structural correctness: known schema_version,
// known rule_type values, no URL fields, no exec fields. Returns all errors (not just first).
func ValidateSchema(pf PolicyFile) []error { ... }

// LoadPolicyFile reads path, parses, validates schema, and returns the PolicyFile.
// Returns errors with file + field context for `policy validate` output.
func LoadPolicyFile(path string) (PolicyFile, []error) { ... }

// ListPolicyFiles scans dir for *.json files and returns (path, ruleCount) pairs.
func ListPolicyFiles(dir string) ([]PolicyFileSummary, error) { ... }
```

[VERIFIED: pattern derived from existing `config.Load`, `catalog.ParseCatalogFile` patterns in codebase]

### Pattern 3: `policy.Evaluate` Signature (Unchanged)

The existing `policy.Evaluate` signature is:

```go
func Evaluate(tc ToolCall, idx MultiCatalogLookup, t CorroborationThresholds, ac AgentContext) Decision
```

For `policy test <file>`:
1. Load sample tool-call JSON from a second argument or stdin
2. Load and validate the policy file
3. Convert policy file rules to engine inputs (thresholds, allowlist overrides)
4. Create a `fakeMultiCatalogLookup` that returns no matches (dry-run: tests only the policy file's own rules, not live catalogs) — OR load real catalogs depending on UX choice
5. Call `policy.Evaluate(tc, idx, thresholds, ac)` and print the resulting `Decision`

**Recommendation:** `policy test <file> --tool-call <json-file>` where tool-call JSON can be a file path or `-` for stdin. Dry-run uses NO live catalog (empty lookup) to make results deterministic. The policy file's allowlist/threshold rules are the only effective rules in test mode. Document this clearly.

[VERIFIED: `internal/policy/engine.go` lines 49–176 — `Evaluate` is stable, pure, no I/O]

### Pattern 4: Layered Config Merge (CODE-05)

**Existing state (verified):**
- `config.Load(path string)` reads ONE file and returns `Config{}` [VERIFIED: `internal/config/config.go` lines 157–183]
- No project-layer support, no env-var overrides, no flag layer [VERIFIED: same file, Phase 1 comment on line 8]
- Missing from existing code: `/etc/beekeeper/config.json` (system), `<project>/.beekeeper/config.json` (project), `BEEKEEPER_*` env vars, CLI flag binding

**Pattern to add — `LoadLayered(opts LayerOpts) (Config, error)`:**

```go
// Source: internal/config/config.go (extension)

type LayerOpts struct {
    SystemPath  string            // /etc/beekeeper/config.json (empty = skip)
    UserPath    string            // ~/.beekeeper/config.json (required)
    ProjectPath string            // <cwd>/.beekeeper/config.json (empty = skip)
    Environ     []string          // os.Environ() or test override
    FlagOverrides map[string]string // flag name → string value (from cobra PersistentPreRunE)
}

// LoadLayered merges the five layers deterministically. Each later layer wins.
// Missing optional files (system, project) are silently skipped.
// Returns a validation error if the merged FailMode is invalid (same rule as Load).
func LoadLayered(opts LayerOpts) (Config, error) {
    cfg := Config{FailMode: FailModeClosed} // baseline defaults
    // Layer 1: system
    if opts.SystemPath != "" {
        sys, err := Load(opts.SystemPath); if err == nil { cfg = merge(cfg, sys) }
    }
    // Layer 2: user
    user, err := Load(opts.UserPath); if err != nil { return Config{}, err }
    cfg = merge(cfg, user)
    // Layer 3: project
    if opts.ProjectPath != "" {
        proj, err := Load(opts.ProjectPath); if err == nil { cfg = merge(cfg, proj) }
    }
    // Layer 4: env vars (BEEKEEPER_FAIL_MODE, BEEKEEPER_SOCKET_API_TOKEN, etc.)
    cfg = applyEnvVars(cfg, opts.Environ)
    // Layer 5: CLI flags
    cfg = applyFlagOverrides(cfg, opts.FlagOverrides)
    // Validate merged result
    return validate(cfg)
}

// merge applies src fields over dst where src fields are non-zero.
// Primitive fields: src wins if non-zero value.
// Slice fields (e.g. Watch.Directories): src appends/replaces per field semantics.
func merge(dst, src Config) Config { ... }
```

**`BEEKEEPER_*` env var mapping (recommended set):**

| Env var | Config field |
|---------|-------------|
| `BEEKEEPER_FAIL_MODE` | `Config.FailMode` |
| `BEEKEEPER_SOCKET_API_TOKEN` | `Config.Socket.APIToken` |
| `BEEKEEPER_LLAMAFIREWALL_ENABLED` | `Config.LlamaFirewall.Enabled` ("true"/"false") |
| `BEEKEEPER_AUDIT_SINKS` | `Config.Audit.Sinks` (comma-separated) |

More env vars can be added without a schema change; the mapping is in `applyEnvVars`.

**Project path discovery:** `<cwd>/.beekeeper/config.json`. Walk up the directory tree from `os.Getwd()` until a `.beekeeper/config.json` is found OR the filesystem root is reached. This matches how Git locates `.git/`. [ASSUMED — tree-walk is not pinned in PRD; PRD §9 says "project" but does not specify discovery algorithm. Safe default: search `os.Getwd()` + parents up to root.]

**CLI flag binding in `main.go`:** `PersistentPreRunE` on the root command resolves paths and builds `LayerOpts`, then stores the merged `Config` in the `cmd.Context()` for subcommands to retrieve. This avoids each subcommand calling `config.Load` independently. [VERIFIED: current pattern in `main.go` shows each subcommand calling `config.Load(configPath)` independently — centralize in Phase 9]

[VERIFIED: `internal/config/config.go` lines 1–246; PRD §9 lines 453–459]

### Pattern 5: `beekeeper diag` Assembly (CODE-06)

**Data sources (all verified in codebase):**

| Metric | Source | Access | Platform |
|--------|--------|--------|----------|
| Hook latency p95/p99 | New: `internal/check.GlobalHookTracker` (`*llamafirewall.LatencyTracker`) | `P95()` + new `P99()` method | All |
| Sidecar inference latency p95 | `internal/llamafirewall.GlobalLatencyTracker` | `P95()` | All |
| Catalog freshness per source | `catalog.LoadState(stateFile).Sources` (`SourceState.Hash`, `Degraded`) | `LoadState` | All |
| ETW EventsLost | `internal/sentry/windows.EventsLost` (`var uint64`) | `atomic.LoadUint64(&windows.EventsLost)` | Windows only |

**Hook latency gap:** `RunCheck` has a benchmark test but NO runtime `LatencyTracker` that accumulates p95/p99 over real production calls. Phase 9 must add a `GlobalHookTracker` in `internal/check` and call `Record(latencyMs)` at the end of `runCheck`. [VERIFIED: `internal/check/handler.go` — no LatencyTracker found]

**`LatencyTracker.P99()` gap:** The existing `LatencyTracker` in `internal/llamafirewall/latency.go` has `P95()` and `Mean()` but no `P99()`. Add `P99()` following the same sorted-buffer pattern as `P95()` but at the 99th percentile index. [VERIFIED: `internal/llamafirewall/latency.go` lines 36–54]

**Cross-platform ETW isolation pattern** (already established in Phase 7):

```go
// diag_windows.go (//go:build windows)
package check
import "sync/atomic"
import windows "github.com/mzansi-agentive/beekeeper/internal/sentry/windows"
func eventsLost() uint64 { return atomic.LoadUint64(&windows.EventsLost) }

// diag_other.go (//go:build !windows)
package check
func eventsLost() uint64 { return 0 } // ETW not available on this platform
```

[VERIFIED: same pattern used for `protect_windows.go` / `protect_darwin.go` / `protect_other.go` in `cmd/beekeeper/`; ETW import pattern verified in `internal/sentry/windows/etw.go`]

**`DiagReport` struct (recommended):**

```go
// Source: internal/check/diag.go
type DiagReport struct {
    HookLatencyP95MS  int64
    HookLatencyP99MS  int64
    SidecarLatencyP95MS int64
    CatalogSources    []CatalogSourceStatus
    ETWEventsLost     uint64 // 0 on non-Windows
}

type CatalogSourceStatus struct {
    Name     string
    Degraded bool
    Count    int
    Hash     string // last-known hash (proxy for last-sync identity)
}

func CollectDiag(stateFile string) DiagReport { ... }
```

**`diag` output format (recommendation — sections, not table):**

```
Hook Handler
  p95 latency:  12ms  (target <100ms)
  p99 latency:  18ms

LlamaFirewall Sidecar
  p95 latency:  67ms  (target <100ms)
  status:       active

Catalog Sources
  bumblebee     last sync: sha256:abc123  entries: 654   degraded: false
  osv           last sync: sha256:def456  entries: 1203  degraded: false
  socket        last sync: sha256:789abc  entries: 0     degraded: false
  beekeeper-self last sync: sha256:111bbb entries: 2    degraded: false

ETW Event Loss (Windows only)
  events lost:  0
```

### Pattern 6: `beekeeper-self` Catalog (CTLG-04, SFDF-06)

**Feed schema:** A static JSON catalog file following the existing `CatalogFile` / `Entry` schema (source: `internal/catalog/schema.go`). The `catalog_source` field is `"beekeeper-self"`. The `ecosystem` field is `"beekeeper"` (new value). The `package` field is `"beekeeper"`. The `versions` field lists known-compromised version strings. The `catalog_signature` holds the Ed25519/cosign signature.

```json
{
  "schema_version": "1",
  "entries": [
    {
      "id": "beekeeper-self-2026-001",
      "name": "Beekeeper v0.4.2 release pipeline compromise",
      "ecosystem": "beekeeper",
      "package": "beekeeper",
      "versions": ["v0.4.2"],
      "severity": "critical",
      "catalog_source": "beekeeper-self",
      "catalog_signature": "<cosign-bundle-base64>"
    }
  ]
}
```

**Client-side integration points:**

1. **On startup** (before any check): fetch `beekeeper-self` feed, verify signature, check if `version.Version` appears in any entry's versions list → if match, self-quarantine.
2. **On every `catalogs sync`**: same check after regular sync completes.
3. **`MultiIndex` integration**: add a `BeeKeeperSelf` adapter to `catalog.MultiIndex` so it participates in `LookupAll`. This enables it to flow through the normal `policy.Evaluate` path AND the startup check.

**Transport:** A single HTTPS GET to a configurable URL, defaulting to a documented official endpoint (e.g. `https://beekeeper-self.mzansi-agentive.io/beekeeper-self.json` or GitHub Releases asset of the separate repo). URL is configurable via `config.json` `self_catalog.url` field. [ASSUMED — specific CDN/hosting not pinned in PRD; PRD §12.6 says "separately hosted" without specifying. Use configurable URL with official default.]

**Separate signing key:** The `beekeeper-self` feed is verified against a DISTINCT public key embedded in the binary (not the same cosign identity used for release signing). This key is hardcoded as a Go constant (`[]byte` public key) in `internal/catalog/selfcatalog.go`. Compromise of the release pipeline key does NOT allow forging the self-catalog signature. [VERIFIED: PRD §12.6 "its own signing key"]

**The fail-closed vs. transient-network-failure tension — resolution:**

| Condition | Behaviour | Rationale |
|-----------|-----------|-----------|
| Fetch succeeds, signature valid, no version match | Continue normally | Normal operation |
| Fetch succeeds, signature valid, version matches | SELF QUARANTINE: exit non-zero, print warning + PRD §12.7 verification path | PRD §12.6 |
| Fetch succeeds, signature INVALID | FAIL CLOSED: refuse to run | Integrity failure; worse than network failure |
| Fetch fails (network error, timeout), cached copy exists + is fresh (< 24h old) | Use cached copy, continue | Transient network failure; don't brick on flaky connectivity |
| Fetch fails, cached copy exists + is STALE (> 24h old) | WARN prominently, continue | PRD §12.3 "degraded mode: read-and-notify" |
| Fetch fails, NO cached copy | WARN prominently, continue | First-run or cache-purged case; don't brick; but surface warning |

**Rationale:** The "fail closed on integrity failure but not on network failure" rule matches PRD §12.3 ("degraded mode under uncertainty: read-and-notify"). An absent or stale self-catalog is uncertainty, not a proven compromise. A bad signature IS a proven integrity failure. The distinction is important: bricking on every network failure would cause support burden and incentivize users to disable the check entirely.

**Self-quarantine state representation:** Add a `SelfQuarantine` bool + `SelfQuarantineVersion` string + `SelfQuarantineReason` string to `catalog.WatchState` (or a separate top-level key in `state.json`). When self-quarantine fires, write this state, and on every subsequent startup check for this flag BEFORE attempting the network fetch (allows offline self-quarantine to persist).

```go
// Source: internal/catalog/state.go (extension)
type WatchState struct {
    Sources          map[string]SourceState `json:"sources"`
    SelfQuarantine   *SelfQuarantineState   `json:"self_quarantine,omitempty"`
}

type SelfQuarantineState struct {
    Version  string `json:"version"`
    EntryID  string `json:"entry_id"`
    Reason   string `json:"reason"`
    FiredAt  string `json:"fired_at"` // RFC3339
}
```

[VERIFIED: `internal/catalog/state.go` — `WatchState` struct exists, `SelfQuarantine` field absent; extension is backward-compatible with `omitempty`]

**Running version at runtime:** `version.Version` variable (set via `-ldflags -X ...version.Version=v1.0.0`). [VERIFIED: `internal/version/version.go` — `var Version = "dev"`]

### Pattern 7: New Cobra Commands

**Existing command registration pattern** (verified in `cmd/beekeeper/main.go` lines 53–72):

```go
root.AddCommand(
    newVersionCmd(),
    // ... existing commands
)
```

**New commands to add:**

```go
root.AddCommand(
    newPolicyCmd(),   // groups: policy validate, policy test, policy list
    newDiagCmd(),     // standalone: beekeeper diag
)
```

**`newPolicyCmd()` structure:**

```go
func newPolicyCmd() *cobra.Command {
    policyCmd := &cobra.Command{Use: "policy", Short: "Manage and test declarative policy files"}
    policyCmd.AddCommand(
        newPolicyValidateCmd(),   // beekeeper policy validate <file>
        newPolicyTestCmd(),       // beekeeper policy test <file> [--tool-call <json>]
        newPolicyListCmd(),       // beekeeper policy list
    )
    return policyCmd
}
```

[VERIFIED: pattern from existing `newCatalogsCmd()`, `newQuarantineCmd()`, etc. in `main.go`]

### Anti-Patterns to Avoid

- **Putting any I/O in `internal/policy`:** The CLAUDE.md constraint is absolute. Adding file-reading, JSON parsing, or network calls to `engine.go` or `types.go` would break the hook handler, gateway, and Sentry — all three call this one implementation. [VERIFIED: CLAUDE.md "pure function library"]
- **Hardcoding the self-catalog URL as a constant:** Use a config field with a well-documented default so operators can point to a mirror.
- **Blocking startup on self-catalog network failure:** If the network is unavailable and no cache exists, warn and continue. Bricking on first install (no cache) is a severe UX regression.
- **Creating a new expression engine in the policy schema:** No eval, no Lua, no CEL. Pure data predicates only. [VERIFIED: PRD §11.3]
- **Calling `config.Load` independently in every subcommand after CODE-05:** Centralize in `PersistentPreRunE` on the root command; pass via `cobra.Command.Context()`.
- **Using Go build tag `//go:build !windows` for ETW stub without the matching `//go:build windows` file:** Ensure both files exist to avoid compile errors. [VERIFIED: existing pattern in `internal/ipc/` and `cmd/beekeeper/protect_*.go`]

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Crypto signature verification for self-catalog | Custom Ed25519 verify | `crypto/ed25519` (stdlib) or existing cosign bundle verify (already in codebase from Phase 1) | Edge cases in signature verification are exactly how attacks succeed |
| P95/P99 ring buffer | New implementation | `llamafirewall.LatencyTracker` (already exists) — just add `P99()` method | Ring buffer already tested; duplication creates drift |
| File-based atomic state writes | `os.WriteFile` with direct rename | Existing `writeFileAtomic` pattern in `internal/catalog/state.go` | Atomic rename prevents partial-write corruption of self-quarantine state |
| JSON schema validation | Full JSON Schema validator library | Simple struct-field validation in `policyloader.ValidateSchema` | Policy schema is small and stable; heavyweight validator adds ~2MB to binary |
| BEEKEEPER_* env var parsing | `os.LookupEnv` in each subcommand | Centralize in `config.LoadLayered` → `applyEnvVars` | Prevents per-subcommand inconsistency and missed env var handling |

---

## Runtime State Inventory

This phase involves a refactor of the config loader and addition of new state (self-quarantine marker). No renaming.

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | `~/.beekeeper/state.json` — `WatchState.Sources` map; new `SelfQuarantine` field added | Backward-compatible struct extension (`omitempty`); no migration |
| Live service config | No live services reconfigured by Phase 9 | None |
| OS-registered state | None relevant to Phase 9 | None |
| Secrets/env vars | `BEEKEEPER_SOCKET_API_TOKEN` (existing) — CODE-05 formally maps it to `BEEKEEPER_SOCKET_API_TOKEN` via env layer. New: `BEEKEEPER_SELF_CATALOG_URL` (optional override) | Document in config.go; no key rename |
| Build artifacts | None — Phase 9 adds new packages but does not rename existing ones | None |

---

## Common Pitfalls

### Pitfall 1: Loading Policy Files Inside `internal/policy`

**What goes wrong:** A developer adds a `LoadFromDir(dir string)` function to `internal/policy` for convenience. This introduces I/O into the pure engine. The gateway, hook handler, and Sentry daemon all call `internal/policy` — now they all transitively do filesystem I/O at evaluation time.

**Why it happens:** The natural place to put "load policy" seems to be the policy package.

**How to avoid:** Create `internal/policyloader` as a separate package. `internal/policy` imports nothing new. `internal/policyloader` imports `internal/policy` (one-way dependency). The I/O tier (cmd + check + gateway) imports `policyloader`, not `policy` directly for loading.

**Warning signs:** Any `os.` or `io.` import appearing in `internal/policy/*.go`.

### Pitfall 2: Self-Catalog Bricking on First Run

**What goes wrong:** `beekeeper-self` check requires a valid cached copy to continue. On a fresh install (no cache), OR after `rm -rf ~/.beekeeper/catalogs/`, Beekeeper refuses to run even though no compromise signal exists.

**Why it happens:** Trying to implement "fail closed" for both integrity failures AND network failures using the same logic.

**How to avoid:** The distinction is: (a) "signature invalid" = integrity failure = fail closed; (b) "network error, no cache" = uncertainty = warn and continue. Implement these as separate branches in the self-catalog check.

**Warning signs:** `if err != nil { os.Exit(1) }` in the fetch path without checking whether the error is a network error vs. a signature error.

### Pitfall 3: Missing `policies/` Directory Initialization

**What goes wrong:** `policy list` returns an error ("no such file or directory") instead of an empty list when the user has never created any policy files.

**Why it happens:** `os.ReadDir` on a non-existent directory returns an error.

**How to avoid:** Create `~/.beekeeper/policies/` during `beekeeper init` (alongside existing dir creation in `newInitCmd()`). `policy list` treats a missing `policies/` dir as empty (same pattern as `config.Load` treating a missing file as defaults).

### Pitfall 4: Policy Test Without Catalog Produces Misleading Results

**What goes wrong:** `policy test` loads live OSV/Socket/Bumblebee catalogs and the "allow" result from a policy test is actually because the package isn't in any catalog, not because the policy file allows it.

**Why it happens:** Naively wiring real catalogs into the test dry-run.

**How to avoid:** `policy test` uses an empty/stub `MultiCatalogLookup` by default (catalog-free evaluation). Add an optional `--with-catalogs` flag for users who want to test against live catalogs. Document the difference prominently.

### Pitfall 5: Layered Config `merge` Losing Zero-Value Fields

**What goes wrong:** User sets `fail_mode: "open"` in the user config. Project config exists but omits `fail_mode`. After merging, `fail_mode` is reset to the zero value `""` (which defaults back to `"closed"`).

**Why it happens:** Naive struct overlay where zero-value fields overwrite non-zero fields in lower layers.

**How to avoid:** The `merge` function must use a "src wins if non-zero" rule: `if src.FailMode != "" { dst.FailMode = src.FailMode }`. For bool fields (e.g., `LlamaFirewall.Enabled`), this requires a pointer-or-omitempty approach to distinguish "not set" from "set to false". Use `*bool` for bool config fields that must participate in the merge correctly, or use a sentinel empty-string convention.

**Warning signs:** Tests where a project config with only one field set unexpectedly resets all other fields to zero values.

### Pitfall 6: `beekeeper-self` Signing Key Embedded Insecurely

**What goes wrong:** The public key for verifying the self-catalog is a long comment string in a Go file, or worse, fetched at runtime from the same host as the catalog (allowing an attacker who compromises the host to substitute both).

**Why it happens:** Convenience.

**How to avoid:** Embed the public key as a Go `[]byte` constant derived from a well-documented key generation procedure. Key is in the binary at compile time, not fetched at runtime. [VERIFIED: PRD §12.6 "its own signing key"; same principle as pinned deps in go.sum]

---

## Code Examples

### Existing `policy.Evaluate` Call Pattern (for `policy test` reference)

```go
// Source: internal/check/handler.go lines 186–198 (verified)
multiIdx := catalog.NewMultiIndex(bbIdx, osvAdapter, socketAdapter)
ac := readAgentContext(stdinAgentID)
decision := policy.Evaluate(toolCall, multiIdx, policy.DefaultCorroborationThresholds(), ac)
```

For `policy test`, substitute an empty `MultiCatalogLookup` and the thresholds derived from the policy file:

```go
// policy test dry-run pattern (new in policyloader package)
type emptyLookup struct{}
func (emptyLookup) LookupAll(_, _ string) []policy.CatalogMatch { return nil }

decision := policy.Evaluate(tc, emptyLookup{}, thresholdsFromPolicyFile(pf), ac)
```

### Existing `LatencyTracker.P95()` (for `diag` reference)

```go
// Source: internal/llamafirewall/latency.go lines 36–54 (verified)
func (t *LatencyTracker) P95() int64 {
    t.mu.Lock(); defer t.mu.Unlock()
    if t.count == 0 { return 0 }
    n := 100
    if !t.filled { n = t.head }
    buf := make([]int64, n)
    copy(buf, t.p95buf[:n])
    sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })
    idx := int(float64(n) * 0.95)
    if idx >= n { idx = n - 1 }
    return buf[idx]
}
// Add P99() identically but with 0.99 percentile index.
```

### Existing `WatchState` + `SourceState` (for `diag` catalog freshness)

```go
// Source: internal/catalog/state.go lines 17–33 (verified)
type SourceState struct {
    Hash           string `json:"hash"`
    Count          int    `json:"count"`
    Degraded       bool   `json:"degraded"`
    DegradedReason string `json:"degraded_reason,omitempty"`
}
type WatchState struct {
    Sources map[string]SourceState `json:"sources"`
    // Phase 9: add SelfQuarantine *SelfQuarantineState `json:"self_quarantine,omitempty"`
}
```

### ETW EventsLost Read Pattern (already in daemon.go)

```go
// Source: internal/sentry/windows/daemon.go line 233 (verified)
dropped := atomic.LoadUint64(&EventsLost)
// EventsLost is the package-level var in internal/sentry/windows/etw.go line 16
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Single-layer `config.Load(path)` | 5-layer `LoadLayered` (system→user→project→env→flag) | Phase 9 (this phase) | Project .beekeeper/config.json overrides user config without env vars |
| No policy files; engine rules are hardcoded | Declarative JSON policy files in `policies/` | Phase 9 (this phase) | Developer can version-control and test policy changes |
| No `beekeeper-self` source | Self-quarantine feed live with separate key | Phase 9 (this phase) | Closes the self-compromise detection loop |
| `diag` placeholder comment in `llamafirewall status` output | Full `beekeeper diag` command | Phase 9 (this phase) | All health signals in one place; latency + freshness + ETW loss |

**Deprecated/outdated:**
- Per-subcommand `config.Load(configPath)` calls: replaced by single centralized load in `PersistentPreRunE`

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Project config discovery walks up from `os.Getwd()` to filesystem root (git-style) | Pattern 4: Layered Config | If PRD intends a different discovery rule (e.g., fixed `<project>/.beekeeper/`), the behavior diverges from user expectations |
| A2 | `beekeeper-self` public key is embedded as a Go `[]byte` constant at compile time | Pattern 6: beekeeper-self | If a different key distribution mechanism is used (e.g., TOFU on first fetch), the security model differs |
| A3 | `beekeeper-self` CDN/hosting URL defaults to an official documented endpoint; exact URL TBD by operator | Pattern 6: beekeeper-self | No risk to Phase 9 implementation; URL is a config field with a default string |
| A4 | `policy test` uses an empty catalog (no live lookup) by default | Pattern 1: Policy Schema | If users expect live-catalog testing by default, the UX may confuse (recommend `--with-catalogs` flag as optional) |

---

## Open Questions (RESOLVED)

All three open questions were resolved during planning; the decisions are implemented in the Phase 9 plans.

1. **Project config discovery algorithm** — RESOLVED: git-style upward walk from `os.Getwd()` to the filesystem root, stopping at the first `.beekeeper/config.json` found. Implemented in Plan 09-02 Task 1; documented in config comments.
   - What we know: PRD §9 says `<project>/.beekeeper/config.json` overrides user-level without env vars
   - What was unclear: Does "project" mean `$CWD/.beekeeper/config.json` exactly, or does it search parent directories (git-style)?

2. **Hook latency persistence across restarts** — RESOLVED: option (a) — append each latency sample to a small ring file under `~/.beekeeper/` after every `beekeeper check`; `diag` reads the rolling window and computes p95/p99. Implemented in Plan 09-04 Task 1.
   - What we know: `LatencyTracker` is in-memory; resets on binary restart. Hook handler is a subprocess (one invocation per tool call), so the tracker would always show 0 samples on cold start.
   - What was unclear: Should hook latency p95/p99 be persisted between `beekeeper check` invocations? (Considered: (a) persisted ring file vs (b) reporting the last `go test -bench BenchmarkCheck` result.)

3. **`beekeeper-self` self-quarantine — does it lock out `policy test` / `diag`?** — RESOLVED: self-quarantine blocks the enforcement paths (`check`, `gateway`, `sentry`, `watch`, `catalogs sync`) and allows the diagnostic paths (`version`, `diag`, `selftest`, `policy validate`) so the developer can investigate. Implemented in Plan 09-05 Task 2; the exception set is documented.
   - What we know: PRD §12.6 says Beekeeper "refuses to run" on a version match
   - What was unclear: Does this apply to ALL subcommands (including `beekeeper version`), or only enforcement-relevant ones?

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25+ | All compilation | ✓ | verified from existing working phases | — |
| `internal/llamafirewall` LatencyTracker | `beekeeper diag` sidecar latency | ✓ | Phase 6 complete | — |
| `internal/sentry/windows` EventsLost | `beekeeper diag` ETW section | ✓ (Windows) / stub (other) | Phase 7 complete | 0 on non-Windows |
| `internal/catalog` WatchState | `beekeeper diag` catalog freshness | ✓ | Phase 2 complete | — |
| `internal/version` Version variable | beekeeper-self version match | ✓ | Phase 1 complete | — |
| External HTTPS endpoint for beekeeper-self | CTLG-04/SFDF-06 | ✗ (not yet hosted) | — | Warn + continue (not blocking) |

**Missing dependencies with no fallback:** None — the external beekeeper-self endpoint is not yet hosted, but this is intentional: Phase 9 implements the client side and a placeholder/test feed; the live endpoint is an ops deliverable that can be the final gate for v1.0.0 tagging.

---

## Validation Architecture

> `workflow.nyquist_validation: true` in `.planning/config.json` — section included.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go standard test + testify (existing project pattern) |
| Config file | None — `go test ./...` |
| Quick run command | `go test ./internal/policyloader/... ./internal/config/... ./internal/catalog/... -run TestPolicy` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CODE-01 | Policy file loads and converts to engine inputs | unit | `go test ./internal/policyloader/... -run TestLoadPolicyFile` | ❌ Wave 0 |
| CODE-01 | Adversarial policy file (URL field, exec field) is rejected | unit | `go test ./internal/policyloader/... -run TestValidateSchema_RejectsExec` | ❌ Wave 0 |
| CODE-01 | Unknown `rule_type` fails validation | unit | `go test ./internal/policyloader/... -run TestValidateSchema_UnknownRuleType` | ❌ Wave 0 |
| CODE-02 | `policy test` dry-run produces expected Decision for a block rule | unit | `go test ./internal/policyloader/... -run TestPolicyTest_BlockRule` | ❌ Wave 0 |
| CODE-02 | `policy test` with allowlist overrides produces allow Decision | unit | `go test ./internal/policyloader/... -run TestPolicyTest_AllowlistOverride` | ❌ Wave 0 |
| CODE-03 | `policy validate` exits non-zero + prints field errors on invalid file | integration | `go test ./cmd/... -run TestPolicyValidateCmd_Invalid` | ❌ Wave 0 |
| CODE-03 | `policy validate` exits 0 on valid file | integration | `go test ./cmd/... -run TestPolicyValidateCmd_Valid` | ❌ Wave 0 |
| CODE-04 | `policy list` shows correct rule counts per file | unit | `go test ./internal/policyloader/... -run TestListPolicyFiles` | ❌ Wave 0 |
| CODE-04 | `policy list` returns empty list (not error) for missing policies dir | unit | `go test ./internal/policyloader/... -run TestListPolicyFiles_MissingDir` | ❌ Wave 0 |
| CODE-05 | User config overrides system config; project overrides user | unit | `go test ./internal/config/... -run TestLoadLayered_PrecedenceOrder` | ❌ Wave 0 |
| CODE-05 | `BEEKEEPER_FAIL_MODE=open` overrides JSON file `fail_mode: "closed"` | unit | `go test ./internal/config/... -run TestLoadLayered_EnvVarOverride` | ❌ Wave 0 |
| CODE-05 | Missing optional layers (system, project) are silently skipped | unit | `go test ./internal/config/... -run TestLoadLayered_MissingOptionalLayers` | ❌ Wave 0 |
| CODE-05 | Zero-value project field does NOT reset non-zero user field | unit | `go test ./internal/config/... -run TestMerge_ZeroValuePreservation` | ❌ Wave 0 |
| CODE-06 | `diag` outputs all four sections | integration | `go test ./cmd/... -run TestDiagCmd_Output` | ❌ Wave 0 |
| CODE-06 | Hook latency p95/p99 accumulated after simulated checks | unit | `go test ./internal/check/... -run TestGlobalHookTracker` | ❌ Wave 0 |
| CODE-06 | ETW EventsLost reports 0 on non-Windows | unit | `go test ./internal/check/... -run TestEventsLost_NonWindows` | ❌ Wave 0 |
| CTLG-04/SFDF-06 | Self-catalog version match triggers self-quarantine | unit | `go test ./internal/catalog/... -run TestSelfCatalog_VersionMatch` | ❌ Wave 0 |
| CTLG-04/SFDF-06 | Self-catalog signature invalid → fail closed | unit | `go test ./internal/catalog/... -run TestSelfCatalog_InvalidSignature` | ❌ Wave 0 |
| CTLG-04/SFDF-06 | Self-catalog network error + no cache → warn, continue | unit | `go test ./internal/catalog/... -run TestSelfCatalog_NetworkError_NoCache` | ❌ Wave 0 |
| CTLG-04/SFDF-06 | Self-catalog network error + fresh cache → use cache, continue | unit | `go test ./internal/catalog/... -run TestSelfCatalog_NetworkError_FreshCache` | ❌ Wave 0 |
| CTLG-04/SFDF-06 | Self-quarantine state persisted to state.json and read back | unit | `go test ./internal/catalog/... -run TestSelfQuarantineState_Persistence` | ❌ Wave 0 |

### Key Test Fixtures Needed (Wave 0)

**Policy file fixtures (in `internal/policyloader/testdata/`):**

| File | Purpose |
|------|---------|
| `valid_release_age.json` | Valid policy with `release_age` rule |
| `valid_allowlist.json` | Valid policy with `package_allowlist` rule |
| `invalid_url_field.json` | Adversarial: contains `"url"` field → must fail validation |
| `invalid_exec_action.json` | Adversarial: `"action": "exec"` → must fail validation |
| `invalid_unknown_rule_type.json` | Unknown `rule_type` value → must fail validation |
| `invalid_schema_version.json` | Unknown `schema_version` → must fail validation |

**beekeeper-self test fixtures:**

| Fixture | Purpose |
|---------|---------|
| `testdata/selfcatalog_match.json` | Feed that matches current `version.Version` (set to "test-v0.0.1" in test) |
| `testdata/selfcatalog_no_match.json` | Feed with no matching versions |
| `testdata/selfcatalog_invalid_sig.json` | Feed with a tampered signature |

**Layered config test matrix:**

| Test | system | user | project | env | flag | Expected FailMode |
|------|--------|------|---------|-----|------|-------------------|
| baseline | absent | closed | absent | none | none | closed |
| user override | absent | open | absent | none | none | open |
| project override | absent | open | closed | none | none | closed |
| env override | absent | open | closed | BEEKEEPER_FAIL_MODE=warn | none | warn |
| flag override | absent | open | closed | BEEKEEPER_FAIL_MODE=warn | fail_mode=open | open |

### Sampling Rate

- **Per task commit:** `go test ./internal/policyloader/... ./internal/config/... -v -count=1`
- **Per wave merge:** `go test ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/policyloader/` package — does not exist yet
- [ ] `internal/policyloader/testdata/` — policy fixture files listed above
- [ ] `internal/check/diag.go` + `diag_windows.go` + `diag_other.go` — do not exist yet
- [ ] `internal/catalog/selfcatalog.go` + `selfcatalog_test.go` — do not exist yet
- [ ] `internal/config/config_test.go` additions for `LoadLayered` tests
- [ ] `docs/THREAT-MODEL.md` — does not exist yet

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | — |
| V3 Session Management | No | — |
| V4 Access Control | No | — |
| V5 Input Validation | Yes | `policyloader.ValidateSchema` — reject unknown rule_types, reject URL fields, reject exec fields; bounded JSON decode (existing 1MB cap pattern) |
| V6 Cryptography | Yes (beekeeper-self signature) | `crypto/ed25519` stdlib — never hand-roll signature verification |
| V7 Error / Logging | Yes | Self-quarantine events must be written to audit log AND stderr before exit |
| V10 Malicious Code | Yes | Policy files must never enable code execution — ValidateSchema rejects any field that could carry execution intent |

### Known Threat Patterns for This Phase

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Adversarial policy file smuggles eval/exec via unknown field | Tampering | `ValidateSchema` enum-validates `rule_type`; unknown values = error, not skip |
| Attacker hosts a fake `beekeeper-self` feed at a user-controlled URL | Tampering / Spoofing | Separate hardcoded public key (not fetched at runtime); signature verification before applying |
| Compromised beekeeper-self CDN serves unsigned feed | Tampering | Signature required; invalid signature = fail closed, not warn |
| Transient DNS failure prevents self-catalog check → bricking | Denial of Service | Distinguish network error from integrity failure; warn-and-continue on network error |
| Config merge allows escalation: project config sets `fail_mode: "open"` | Tampering | This is by design (project can relax user config) — document it clearly in config comments |
| Layered config env var injection via CI env | Tampering | `BEEKEEPER_*` env vars only map to known fields; unknown env vars are ignored |

---

## Threat Model Documentation Plan

The `docs/THREAT-MODEL.md` document (Phase 9 success criterion 5) must cover:

**Required sections:**

1. **Beekeeper's own threat model** — What compromising Beekeeper gives an attacker (full developer privileges); six attack surfaces (ordered by risk)
2. **Build and release pipeline hardening** — Reproducible builds, Sigstore, SLSA Level 3, pinned deps, two-account release approval
3. **Catalog feed integrity: the 2FA principle** — Corroboration semantics as 2FA; signatures required; sanity bounds; degraded mode
4. **Coordinated false-positive poisoning attack surface** — An adversary who controls ≥2 catalog sources can generate false-positive block events that degrade developer experience, pressuring users to disable enforcement. No known fix without human-in-the-loop review. Documented explicitly as a known gap. [VERIFIED: PRD §12.3 "catalog DoS" attack type; Phase 9 success criterion 5 explicitly requires documenting this surface]
5. **The fanotify mmap gap on Linux** — fanotify intercepts file-open events; however, files already mapped into memory via mmap before Beekeeper is installed are not re-intercepted. An attacker who mmap-loads a malicious library at process start (before fanotify is active) evades Sentry file-access detection. Documented as a known gap in the v1.0.0 scope. [VERIFIED: CONTEXT.md success criterion 5 "fanotify mmap gap on Linux"]
6. **The `beekeeper-self` catalog** — How it works, what it defends against, the single-maintainer governance note (v1 is single-maintainer; intent to separate), the verification path (PRD §12.7)
7. **Verification path** — Step-by-step: `make verify-release VERSION=X.Y.Z`, cosign verify, SLSA provenance, SBOM inspection. Link `SECURITY.md` and `BUILDING.md`.
8. **Known gaps and explicit non-defenses** — Kernel rootkits, pre-existing malware, direct human malice, sophisticated prompt injections

**File location:** `docs/THREAT-MODEL.md` in the project root (publicly readable). [ASSUMED: docs/ directory is standard; CONTEXT.md says "publish as user-facing docs" without specifying exact path.]

---

## Sources

### Primary (HIGH confidence)
- `internal/config/config.go` — current `Config` struct, `Load()` function, missing CODE-05 layers confirmed by Phase 1 comment on line 8 [VERIFIED: codebase]
- `internal/policy/types.go` — `ToolCall`, `Decision`, `CatalogMatch`, `AgentContext`, `MultiCatalogLookup`, `CorroborationThresholds` — all confirmed [VERIFIED: codebase]
- `internal/policy/engine.go` — `Evaluate` signature confirmed pure, no I/O [VERIFIED: codebase]
- `internal/catalog/multi.go` — `MultiIndex` adapter pattern; how to add a 4th source [VERIFIED: codebase]
- `internal/catalog/state.go` — `WatchState`, `SourceState` — extension pattern confirmed backward-compatible [VERIFIED: codebase]
- `internal/llamafirewall/latency.go` — `LatencyTracker.P95()` and `Mean()` exist; `P99()` absent [VERIFIED: codebase]
- `internal/sentry/windows/etw.go` — `EventsLost uint64` package-level var; `atomic.AddUint64` write [VERIFIED: codebase]
- `internal/sentry/windows/daemon.go` — `atomic.LoadUint64(&EventsLost)` read pattern in IPC handler [VERIFIED: codebase]
- `internal/check/handler.go` — `RunCheck` confirmed no persistent `LatencyTracker`; hook latency tracking absent [VERIFIED: codebase]
- `internal/version/version.go` — `var Version = "dev"` for self-catalog version matching [VERIFIED: codebase]
- `cmd/beekeeper/main.go` — Cobra command registration pattern; per-subcommand `config.Load` calls confirmed [VERIFIED: codebase]
- `beekeeper-prd.md` §9, §10, §11.3, §12.3, §12.6, §12.7 — locked decisions and threat model requirements [CITED: beekeeper-prd.md]
- `.planning/phases/09-policy-as-code-self-defense-capstone/09-CONTEXT.md` — locked decisions confirmed [CITED: project planning]
- `.planning/config.json` — `nyquist_validation: true` confirmed [VERIFIED: codebase]

### Secondary (MEDIUM confidence)
- Pattern for JSON schema validation via explicit field enumeration (vs. full JSON Schema library): based on existing `ParseCatalogFile` + `ValidateSchemaVersion` patterns and the PRD "minimal non-stdlib dependencies" principle [VERIFIED: codebase pattern + PRD §14.1]

### Tertiary (LOW confidence)
- None — all claims are verified against the codebase or locked PRD decisions.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all packages are existing or stdlib; no new external deps
- Architecture: HIGH — verified against live codebase; all integration points confirmed
- Pitfalls: HIGH — pitfalls derived from concrete codebase issues (missing LatencyTracker, missing policies/ dir, zero-value merge bug class, etc.)

**Research date:** 2026-05-29
**Valid until:** Stable — locked decisions; no fast-moving dependencies
