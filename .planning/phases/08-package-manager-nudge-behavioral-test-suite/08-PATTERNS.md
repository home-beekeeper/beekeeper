# Phase 8: Package-Manager Nudge + Behavioral Test Suite - Pattern Map

**Mapped:** 2026-06-04
**Files analyzed:** 22 (new + modified, from 08-RESEARCH.md "Recommended Project Structure" + Validation Architecture file column)
**Analogs found:** 22 / 22 (every new/modified file has a verified live-codebase analog; net-new packages map to structural references)

> **Note:** There is NO CONTEXT.md for this phase (discuss-phase intentionally skipped). The file
> list below was extracted from `08-RESEARCH.md` (esp. its "Recommended Project Structure" tree,
> the "Files affected (Flag 4)" table, and the "Phase Requirements → Test Map" file column).
> Flag 2 / Flag 4 / Flag 5 are LOCKED decisions per the research — they are not open questions.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/pkgparse/pkgparse.go` (NEW) | utility (pure parser) | transform | `internal/policy/engine.go` (`installPrefixes`/`extractFromCommand`) | role+flow exact (extracts existing code) |
| `internal/pkgparse/pkgparse_test.go` (NEW) | test (table + purity) | transform | `internal/policy/release_age_test.go` (`TestReleaseAgeImportsArePure`) | exact |
| `internal/pkgparse/fuzz_test.go` (NEW) | test (fuzz) | transform | `internal/gateway/parser_fuzz_test.go` (`FuzzParseMessage`) | exact (template) |
| `internal/nudge/evaluate.go` (NEW) | service (PURE decision) | request-response | `internal/policy/release_age.go` (`EvaluateReleaseAge`) | exact (THE locked pattern) |
| `internal/nudge/detect.go` (NEW) | adapter (impure I/O) | file-I/O + event-driven (exec) | `internal/check/paths.go` (impure adapter) + `internal/shim/shim.go` (`osLookPath` injected var) | role-match |
| `internal/nudge/rewrite.go` (NEW) | utility (pure transform) | transform | `internal/policy/engine.go` (`splitVersion`/`normalize` pure helpers) | role+flow match |
| `internal/nudge/version.go` (NEW) | utility (pure compare) | transform | `internal/policy/release_age.go` (threshold compare) | role-match |
| `internal/nudge/reasons.go` (NEW) | model (closed enum) | — | `internal/policyloader/validate.go` (`legalRuleTypes`/`legalActions` enums) | role-match |
| `internal/nudge/config.go` (NEW) | config (defaults) | — | `internal/policy/release_age.go` (`DefaultReleaseAgeConfig`) | exact |
| `internal/nudge/scanners.go` (NEW) | adapter (impure file scan) | file-I/O | `internal/check/paths.go` (`extractBashCredentialPaths` string scanner) | flow-match |
| `internal/nudge/evaluate_test.go` (NEW) | test (table-driven) | request-response | `internal/policy/release_age_test.go` (table cases) | exact |
| `internal/nudge/detect_test.go` (NEW) | test (injected fn / fake clock) | event-driven | `internal/shim` `osLookPath` injection idiom | role-match |
| `internal/nudge/rewrite_test.go` (NEW) | test | transform | `internal/policy/release_age_test.go` | role-match |
| `internal/nudge/version_test.go` (NEW) | test | transform | `internal/policy/release_age_test.go` | role-match |
| `internal/nudge/scanners_fuzz_test.go` (NEW) | test (fuzz) | file-I/O | `internal/gateway/parser_fuzz_test.go` | exact (template) |
| `internal/check/handler.go` (EDIT) | controller (hook entry) | request-response | (self — Phase 7 SPATH block at lines 272-288) | exact (in-file precedent) |
| `internal/check/nudge_adapter.go` (NEW, optional) | adapter (impure glue) | request-response | `internal/check/paths.go` (impure adapter feeding pure fn) | exact |
| `internal/check/integration_test.go` (EDIT) | test (integration) | request-response | (self — `runCheckWithIndex` + `readLastAuditRecord`) | exact |
| `internal/check/e2e_test.go` (NEW) | test (live-binary E2E) | request-response | NO repo precedent — net-new; structural ref `integration_test.go` + research §Code Examples | net-new |
| `internal/policy/engine.go` (EDIT) | service (pure engine) | CRUD/transform | (self — `installPrefixes`/`extract`) | exact (Flag 4 swap to pkgparse) |
| `internal/policyloader/enforce.go` (EDIT) | service (overlay) | transform | (self — `installPrefixesOverlay`) | exact (Flag 4 swap to pkgparse) |
| `internal/gateway/policy.go` (EDIT) | controller (proxy mw) | request-response + pub-sub | (self — `applyPolicy`) + Cache home | exact |
| `internal/audit/types.go` (EDIT) | model (record schema) | — | (self — `AuditRecord` Phase 5/6 field-addition pattern) | exact |
| `internal/config/config.go` (EDIT) | config struct | — | (self — `Config` struct + `Load` defaulting) | exact |
| `internal/shim/shim.go` (EDIT) | adapter (npm wrapper) | request-response | (self — `osLookPath` injection) | exact |
| `cmd/beekeeper/nudge.go` (NEW) | route (thin Cobra CLI) | request-response | `cmd/beekeeper/policy.go` (`newPolicyCmd` group) + `main.go` audit query | exact |
| `cmd/beekeeper/nudge_test.go` (NEW) | test (CLI) | request-response | `cmd/beekeeper/policy_test.go` | exact |
| `docs/nudge.md` (NEW) | doc | — | (existing docs/ convention) | n/a |

---

## Pattern Assignments

### `internal/nudge/evaluate.go` (service, PURE decision) — THE locked pattern

**Analog:** `internal/policy/release_age.go` (verified, 109 lines, pure — imports only `fmt`)

**Core pure-decision pattern** (`release_age.go` lines 47-109): a function that takes a
caller-resolved input struct + a config struct and returns a `Decision` with NO `time.Now()`,
NO exec, NO os, NO io. `nudge.Evaluate(cmd pkgparse.ParsedCommand, state PMState, cfg Config) Decision`
mirrors this exactly. The doc comment on `EvaluateReleaseAge` is the model to copy:

```go
// EvaluateReleaseAge is pure: imports only "fmt" and "strings" (no time, net,
// os, io, sync, context).
func EvaluateReleaseAge(input ReleaseAgeInput, cfg ReleaseAgeConfig) Decision {
	// 1. Allowlist check — takes priority over everything including missing timestamp.
	for _, excluded := range cfg.Exclude { ... }
	// 2. Fail closed: missing publish timestamp.
	if input.TimestampMissing { return Decision{Allow:false, Level:"block", ...} }
	// 3. Resolve threshold: per-ecosystem override or global default.
	threshold := cfg.DefaultMinutes
	if cfg.PerEcosystemMinutes != nil { ... }
	// 4. Age check.
	if input.AgeMinutes < threshold { return Decision{Allow:false, Level:"block", ...} }
	// 5. Package is old enough.
	return Decision{Allow:true, Level:"allow", ...}
}
```

**Decision struct shape to copy** (`release_age.go` lines 64-72): every return is a struct literal
with `Allow bool`, `Level string` ("allow"/"warn"/"block"), `Reason string`, `RuleIDs []string`.
Nudge needs its own `Decision{Action, Reason (enum), Original, Rewritten}` shape PLUS a `Level`
field so `mergeDecisions` (see Shared Patterns) can rank it. Per research Pattern 2 / A1:
`Advise`→Level "warn" (exit 0), `Rewrite`→"warn" (exit 0), `Block`→"block", `Proceed`→"allow".

**Config-defaults pattern to copy** (`release_age.go` lines 36-45) — see `internal/nudge/config.go`:

```go
func DefaultReleaseAgeConfig() ReleaseAgeConfig {
	return ReleaseAgeConfig{DefaultMinutes: 1440, PerEcosystemMinutes: nil, Exclude: nil}
}
```
Note `DefaultMinutes: 1440` already matches Flag 5's corrected baseline — `nudge`'s
hardening-weakness comparison must use **1440**, not 60.

---

### `internal/nudge/evaluate_test.go` + the purity test (test, table-driven)

**Analog:** `internal/policy/release_age_test.go` lines 141-174 (`TestReleaseAgeImportsArePure`)

**Purity-enforcement test to copy VERBATIM** (rename file/forbidden-set as needed). This is
mandatory per CLAUDE.md and research Anti-Patterns — `TestNudgeEvaluateImportsArePure` must
AST-parse `evaluate.go` and fail if it imports any I/O package:

```go
func TestReleaseAgeImportsArePure(t *testing.T) {
	const srcPath = "release_age.go"
	src, _ := os.ReadFile(srcPath)
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, srcPath, src, parser.ImportsOnly)
	forbidden := map[string]bool{
		"os": true, "net": true, "net/http": true, "io": true,
		"sync": true, "time": true, "context": true,
	}
	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 { path = path[1 : len(path)-1] }
		if forbidden[path] {
			t.Errorf("release_age.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
```
Apply the **same** purity test to `internal/pkgparse/pkgparse.go` (`TestPkgparseImportsArePure`).
`pkgparse` must import only `strings` so `internal/policy` (currently imports only `fmt`) can import
it without breaking its own purity test.

---

### `internal/pkgparse/pkgparse.go` (utility, pure parser) — Flag 4 EXTRACTION

**Analog (extract from):** `internal/policy/engine.go` lines 31-47 + 324-376, AND
`internal/policyloader/enforce.go` lines 237-275 (the duplicate copy — comment literally says
"duplicated here to keep internal/policy untouched").

**Copy 1 — the prefix table to lift** (`engine.go` lines 34-47). Note: pkgparse MUST ADD
`pnpm`/`bun`/`yarn` rows mapped to ecosystem `"npm"` (closes F3 / SC1 — see Pitfall 7):

```go
var installPrefixes = []struct {
	prefix    string
	ecosystem string
}{
	{"npm install", "npm"},
	{"npm i ", "npm"},
	{"pip install", "pypi"},
	{"pip3 install", "pypi"},
	{"go get", "go"},
	{"gem install", "rubygems"},
	{"cargo add", "cargo"},
	{"cargo install", "cargo"},
	{"composer require", "packagist"},
	// Flag 4 / F3 additions — pnpm/bun/yarn install from the npm registry, so
	// Ecosystem MUST be "npm" for LookupAll("npm", pkg) to match (SC1):
	// {"pnpm add", "npm"}, {"pnpm install", "npm"}, {"pnpm i ", "npm"},
	// {"bun add", "npm"}, {"bun install", "npm"}, {"yarn add", "npm"}, ...
}
```

**Token/version/normalize helpers to lift** (`engine.go` lines 324-376) — `extractFromCommand`,
`firstPackageToken` (skips `-flag` tokens), `splitVersion` (uses `LastIndex("@")` for scoped
packages), `normalize` (lowercase + trim):

```go
func splitVersion(token string) (name, version string) {
	at := strings.LastIndex(token, "@")
	if at <= 0 { return token, "" } // no "@", or leading "@" only (scoped name, no version)
	return token[:at], token[at+1:]
}
func normalize(pkg string) string { return strings.ToLower(strings.TrimSpace(pkg)) }
```

**Copy 2 — the duplicate to delete** (`enforce.go` lines 237-275): `installPrefixesOverlay` is
the *identical* table; `extractEcoPackageFromCommand`/`firstNonFlagToken`/`stripVersionSuffix` are
the same helpers under different names. After extraction both `engine.go` AND `enforce.go` call
`pkgparse.Parse`. Research Flag 4 + A5: sequence this as Wave 0/1 and re-run `engine_test.go` +
`enforce_test.go` (the regression net) to prove byte-identical behavior BEFORE adding the new
pnpm/bun prefixes.

**`ParsedCommand` public surface** (from research Flag 4, fields: `Raw, Manager, Ecosystem, Verb,
Package, Version, IsInstall, IsExec, Sudo, Unpinned`) — `Manager` keeps "pnpm"/"bun" for the
nudge/audit view; `Ecosystem` is the catalog key ("npm").

---

### `internal/nudge/detect.go` (adapter, impure exec + file-I/O)

**Analog:** `internal/shim/shim.go` line 35 (injected-var idiom) + `internal/check/handler.go`
context-timeout discipline + `internal/check/paths.go` (impure-adapter-feeds-pure-fn structure).

**Injected-fn idiom for testable exec** (`shim.go` lines 32-35) — copy this so `detect_test.go`
can substitute a fake (criterion 12: slow fn → "not installed"; bun branch on dev box with no bun):

```go
// osLookPath is the exec.LookPath function used by findRealBinary. It is a
// package-level variable so tests can substitute a fake without a real binary
// in PATH.
var osLookPath = exec.LookPath
```
Nudge mirror (from research Code Examples):
```go
var pnpmVersionFn = func(ctx context.Context) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "pnpm", "--version").Output()
	if err != nil { return "", err } // timeout/not-found → caller treats as "not installed"
	return strings.TrimSpace(string(out)), nil
}
```

**2s-timeout discipline** — `handler.go` uses `context.WithTimeout` for every bounded sub-op
(lines 141-142 outer 5s; lines 188-189 a nested 3s net sub-context). `detect.go` uses the same
`context.WithTimeout(ctx, 2*time.Second)` per `exec.CommandContext`.

**Cache (gateway-only, Flag 2 Position B)** — the ONLY place `sync`/`time.Now()` live. Research
mandates `NewCache(d func(ctx, Config) PMState, ttl)` with an injectable detect-fn + fake clock so
the TTL test (criterion 11) calls `State` twice, asserts the underlying fn ran once, advances the
clock past TTL, asserts it ran again. NEVER constructed in the check hook.

**fail-OPEN-by-design note (NOT a fail-closed violation):** detection timeout/error → treat PM as
"not installed", proceed. Document this distinction explicitly (research Anti-Patterns + Project
Constraints) — it is the soft-nudge contract, distinct from the catalog/path fail-closed rule.

---

### `internal/nudge/scanners.go` + `scanners_fuzz_test.go` (impure file scan + fuzz)

**Analog (scanner structure):** `internal/check/paths.go` lines 218-267
(`extractBashCredentialPaths` — a hand-written, never-panic, returns-`nil`-on-no-match string
scanner). Hand scanners for `bunfig.toml` / `pnpm-workspace.yaml` follow this shape: return
`(value, ok)`, NEVER panic, default to safe value on parse error (criterion 13).

**Analog (fuzz gate):** `internal/gateway/parser_fuzz_test.go` (verified — the never-panic
release-gate template). Copy the `//go:build fuzz` tag, the RELEASE-GATE header comment, the
seed-corpus `f.Add(...)` pattern, and the never-panic contract:

```go
//go:build fuzz

// RELEASE GATE: ... FuzzParseMessage must pass (seed corpus run) in CI before any release tag.
func FuzzParseMessage(f *testing.F) {
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call",...}`))
	f.Add([]byte(``))   // empty
	f.Add([]byte(`{}`)) // missing fields
	// ... adversarial seeds covering every bound ...
	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := ParseMessage(data) // must NEVER panic
		// every return: exactly one of (valid result) | (typed error with non-zero code)
	})
}
```
New targets: `FuzzBunfig`, `FuzzPnpmWorkspace` (scanners), and `FuzzParse` in
`internal/pkgparse/fuzz_test.go`. Contract for all three: "must NEVER panic on any input"
(BTEST-03 release gate).

---

### `internal/check/handler.go` (EDIT) + `nudge_adapter.go` (controller wiring)

**Analog:** the file's OWN Phase 7 SPATH block (`handler.go` lines 272-298) — this is the seam to
mirror. Add the nudge merge AFTER `ApplyPolicyOverlay` (lines 268-270) so a `package_allowlist`
allow cannot downgrade a nudge `Block` (CR-02 ordering), same as the path block:

```go
// SPATH block (existing — the precedent to mirror):
spathCfg := policy.DefaultSensitivePaths()
for _, rawPath := range extractPathTargets(toolCall) {
	resolved := canonicalizePath(rawPath)
	if resolved == "" { continue }
	pathDecision := policy.EvaluatePath(resolved, spathCfg)
	decision = mergeDecisions(decision, pathDecision)
}
// NUDGE block (new — same shape; adapter resolves PMState fresh, NO cache in check hook):
//   parsed, ok := pkgparse.Parse(cmd); if ok && parsed.IsInstall {
//     state := nudge.DetectState(ctx, nudgeCfg)   // fresh each call (Flag 2 Position B)
//     nudgeDecision := nudge.Evaluate(parsed, state, nudgeCfg)
//     decision = mergeDecisions(decision, toPolicyDecision(nudgeDecision))
//   }
```
Place the impure glue (`nudge.DetectState` call) in `nudge_adapter.go`, keeping `paths.go` as the
template: the impure adapter lives in `internal/check`, the pure decision in `internal/nudge`.
Detection runs ONLY when `parsed.IsInstall` (Pitfall 2 — non-install commands like `npm ls`/`npm run`
never trigger exec).

---

### `internal/check/integration_test.go` (EDIT) — BTEST-02

**Analog:** the file's OWN `runCheckWithIndex` (lines 33-92), `mapMultiIndex` (lines 23-31), and
`readLastAuditRecord` (lines 94-118). Add pnpm/bun install integration cases that assert BOTH the
exit code AND the audit NDJSON record. The `mapMultiIndex` fake keys by `"ecosystem::pkg"` — for
F3 a pnpm install of `evil-pkg` must be looked up under key `"npm::evil-pkg"` (ecosystem "npm"):

```go
type mapMultiIndex struct{ matchesByKey map[string][]policy.CatalogMatch }
func (f *mapMultiIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	return f.matchesByKey[ecosystem+"::"+pkg]
}
// readLastAuditRecord(t, auditPath) returns the last NDJSON audit.AuditRecord —
// use it to assert record_type, decision, and (new) nudge fields.
```

---

### `internal/check/e2e_test.go` (NEW) — BTEST-03 release gate (NET-NEW, no repo precedent)

**Structural reference (no exact analog):** `integration_test.go` (`readLastAuditRecord`) +
research §Code Examples. Build the binary with `go build -o bin ...cmd/beekeeper`, pipe stdin via
`exec.Command(bin, "check")`, assert exit code AND read the audit NDJSON record_type+decision.
`//go:build e2e` tag (mirror the fuzz file's build-tag + RELEASE-GATE header convention).

**Open item to VERIFY before planning this task (research A2 / Open Q1):** the compiled binary must
honor an overridable state/audit dir for hermetic E2E. `newCheckCmd` resolves `auditPath` via
`platform.AuditDir()`; confirm a `BEEKEEPER_*`/HOME env override exists or add one (Wave 0 task).
bun E2E case must `t.Skip` when `exec.LookPath("bun")` fails (bun NOT installed on dev box / CI).

---

### `cmd/beekeeper/nudge.go` (NEW) — thin Cobra CLI (NUDGE-07 / SC5)

**Analog:** `cmd/beekeeper/policy.go` (verified — the grouped-subcommand template) + `main.go`
audit-query block (lines 886-936).

**Group-command template** (`policy.go` lines 27-45) — copy for `newNudgeCmd()` with
`status|check|audit` subcommands; register in `main.go` via `root.AddCommand(newNudgeCmd())`
(the `AddCommand` block at `main.go` line 62-67):

```go
func newPolicyCmd() *cobra.Command {
	policyCmd := &cobra.Command{Use: "policy", Short: "...", Long: `...`}
	policyCmd.AddCommand(newPolicyValidateCmd(), newPolicyTestCmd(), newPolicyListCmd())
	return policyCmd
}
```

**`nudge check "<cmd>"` dry-run output** — mirror `newPolicyTestCmd` output (`policy.go` lines
124-129): `decision: <level>` / `reason: <reason>` / `rules: <ids>`.

**`nudge audit --since=` filter** — reuse `audit.Query` + `audit.QueryOpts{Since}` exactly as
`main.go` audit query does (lines 912-928); add a `record_type:"nudge"` filter:

```go
opts := audit.QueryOpts{ ... }
if qSince != "" {
	if dur, err := time.ParseDuration(qSince); err == nil { opts.Since = time.Now().Add(-dur) }
	else if ts, err := time.Parse(time.RFC3339, qSince); err == nil { opts.Since = ts }
	else { return fmt.Errorf("--since %q: expected duration (e.g. 24h) or RFC3339 timestamp", qSince) }
}
return audit.Query(cmd.Context(), f, opts, cmd.OutOrStdout())
```

**Path resolution** — `platform.StateDir()` / `platform.AuditDir()` as `newPolicyListCmd` does
(`policy.go` lines 152-156).

**`nudge_test.go`** — mirror `cmd/beekeeper/policy_test.go` structure.

---

### `internal/audit/types.go` (EDIT) — new record types + fields (NUDGE-06)

**Analog:** the file's OWN Phase 5/6 field-addition pattern (`types.go` lines 44-61): grouped,
commented (`// Phase N additions`), `omitempty`-tagged new fields on `AuditRecord`. Add nudge
fields the same way:

```go
// Phase 8 additions (NUDGE-06): package-manager nudge provenance.
OriginalCommand  string `json:"original_command,omitempty"`
RewrittenCommand string `json:"rewritten_command,omitempty"`
ReasonCode       string `json:"reason_code,omitempty"`
PMState          string `json:"pm_state,omitempty"`
```
New record types `record_type:"nudge"` and `record_type:"version_drift"` join the existing
`"policy_decision"`/`"tool_result"`/`"llmf_alert"` set. The override-RecordType idiom is in
`handler.go` (`rec.RecordType = "tool_result"` at line 483) and `writeLLMFAlertRecord` (line 634)
— construct an `audit.AuditRecord{RecordType: "nudge", ...}` literal or set the field after
`FromDecision`. Keep `FromDecision` a pure mapping (caller supplies recordID + timestamp).

---

### `internal/config/config.go` (EDIT) — `NudgeConfig` (NUDGE-08, layered config)

**Analog:** the file's OWN `Config` struct (lines 105-142) + `Load` defaulting (lines 178-204).

**Field-addition pattern** (lines 119-141): add `Nudge NudgeConfig json:"nudge,omitempty"` with a
grouped comment, exactly like `Watch`/`Audit`/`LlamaFirewall`/`SelfCatalog`.

**Defaulting pattern to mirror** (lines 192-194) — a missing `nudge` key must resolve to documented
defaults, just as `FailMode == ""` → `FailModeClosed`:

```go
if cfg.FailMode == "" { cfg.FailMode = FailModeClosed }
```
Research §Runtime State: default `enabled:true` (PRD §5.1) but a *missing* block resolves via the
loader to documented defaults; project `.beekeeper.json` `nudge.enabled:false` disables it (layered
merge wins). Validate bounds fail-closed (see Shared Patterns — `validate.go`).

---

### `internal/policy/engine.go` / `internal/policyloader/enforce.go` (EDIT) — Flag 4 consumers

Both replace their in-file copies with `pkgparse.Parse`. `engine.go`'s `extract(tc.ToolInput)`
(line 81) and `extractFromCommand` (line 244-ish call site) route through pkgparse; `enforce.go`'s
`extractEcoPackageFromCommand` (line 231 call site) too. Adding pnpm/bun/yarn → ecosystem "npm" in
pkgparse automatically gives F3 catalog matching to BOTH paths (Pitfall 7 / SC1).

### `internal/gateway/policy.go` (EDIT) — gateway wiring + the 60s Cache home

**Analog:** the file's OWN `applyPolicy` (lines 82-131) — already mirrors `handler.go` (overlay
then merge). Add the nudge merge here too, but this is the ONLY place the `nudge.Cache` (60s TTL)
is constructed (Flag 2). Long-lived `Start` loop calls `applyPolicy` per request → cache hits.

### `internal/shim/shim.go` (EDIT) — npm shim calls `nudge.Evaluate`

**Analog:** the file's OWN `osLookPath` injected-var idiom (line 35) + `DefaultTools` list (line
28). Shim calls `pkgparse.Parse` → `nudge.Evaluate` before proxying; short-lived shim behaves like
the check hook (fresh detect, no cache).

---

## Shared Patterns

### Most-restrictive-wins merge
**Source:** `internal/check/paths.go` lines 306-318 (`mergeDecisions`)
**Apply to:** `handler.go` (runCheck), `gateway/policy.go` (applyPolicy), `shim.go` — every place
nudge decisions merge into the catalog/overlay decision.
```go
// mergeDecisions returns the most restrictive of base and overlay.
// Rank: block(2) > warn(1) > allow(0).
func mergeDecisions(base, overlay policy.Decision) policy.Decision {
	rank := map[string]int{"allow": 0, "warn": 1, "block": 2}
	if rank[overlay.Level] > rank[base.Level] { return overlay }
	return base
}
```
**Ordering rule (CR-02):** run nudge merge AFTER `ApplyPolicyOverlay` so a `package_allowlist`
allow cannot downgrade a nudge `Block`. Map nudge `Advise`/`Rewrite`→Level "warn" (exit 0),
`Block`→"block", `Proceed`→"allow" (research A1).

### Pure-library import enforcement
**Source:** `internal/policy/release_age_test.go` lines 141-174 (`TestReleaseAgeImportsArePure`)
**Apply to:** `internal/nudge/evaluate.go` (`TestNudgeEvaluateImportsArePure`) and
`internal/pkgparse/pkgparse.go` (`TestPkgparseImportsArePure`). AST-parse imports, fail on any of
`{os, net, net/http, io, sync, time, context}`. (Excerpt under evaluate_test.go above.)

### Injected-function for testable I/O
**Source:** `internal/shim/shim.go` line 35 (`var osLookPath = exec.LookPath`)
**Apply to:** `detect.go` (`pnpmVersionFn`/`bunVersionFn`/`nodeVersionFn`) and the `Cache` clock.
Lets `detect_test.go` inject a slow/erroring fn (criterion 12) and a fake clock (criterion 11)
without a real `bun` binary (absent on dev box).

### Fail-closed config-bounds validation
**Source:** `internal/policyloader/validate.go` lines 39-77 (`ValidateSchema`) — collects ALL
errors, rejects out-of-range values at load time.
**Apply to:** the new `nudge` config block bounds (mirror `validateCorroborationThresholds` /
CORR-02 discipline). A typo or out-of-range value must be rejected at load, never silently degrade.
```go
if r.CriticalBlockAt < 1 {
	errs = append(errs, fmt.Errorf("rule[%d] %q: critical_block_at (%d) must be >= 1", i, r.ID, r.CriticalBlockAt))
}
```

### Closed-enum pattern (reason codes)
**Source:** `internal/policyloader/validate.go` lines 14-28 (`legalRuleTypes`/`legalActions` maps)
**Apply to:** `internal/nudge/reasons.go` — the closed reason-code enum (PRD §9). A `map[string]bool`
or typed-string-const set so unknown reasons are caught, not silently passed.

### Audit record-type override + write
**Source:** `internal/check/handler.go` line 483 (`rec.RecordType = "tool_result"`) and
`writeLLMFAlertRecord` lines 627-648 (construct `audit.AuditRecord{RecordType: "...", ...}` literal,
write best-effort, never let a write failure change the decision).
**Apply to:** writing `record_type:"nudge"` and `record_type:"version_drift"` records.

### Fuzz release-gate template
**Source:** `internal/gateway/parser_fuzz_test.go` (`//go:build fuzz` + RELEASE-GATE header +
seed corpus + never-panic contract).
**Apply to:** `internal/nudge/scanners_fuzz_test.go` (`FuzzBunfig`, `FuzzPnpmWorkspace`) and
`internal/pkgparse/fuzz_test.go` (`FuzzParse`). (Excerpt under scanners.go above.)

---

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/check/e2e_test.go` | test (live-binary E2E) | request-response | No compiled-binary E2E harness exists anywhere in the repo. Net-new (BTEST-03 release gate). Structural reference only: `integration_test.go` (`readLastAuditRecord`) + the `//go:build`-tag + RELEASE-GATE header convention from `parser_fuzz_test.go`, plus research §Code Examples. **Blocker to resolve first:** verify/add an overridable state-audit dir on the binary (research A2). |

> Net-new packages `internal/pkgparse/` and `internal/nudge/` have no *package* analog but every
> *file* in them maps to a verified structural reference (tabled above) — they are not "no analog".

---

## Metadata

**Analog search scope:** `internal/policy`, `internal/check`, `internal/policyloader`,
`internal/gateway`, `internal/audit`, `internal/config`, `internal/shim`, `cmd/beekeeper`
**Files scanned (read or grepped):** `release_age.go`, `release_age_test.go`, `paths.go`,
`handler.go`, `engine.go`, `enforce.go`, `validate.go`, `parser_fuzz_test.go`, `types.go`,
`integration_test.go`, `policy.go`, `main.go` (grep), `config.go`, `gateway/policy.go`,
`shim/shim.go` (grep), `audit/query.go` (grep)
**Locked decisions honored:** Flag 2 (Position B — cache gateway-only), Flag 4 (extract pkgparse),
Flag 5 (1440 baseline + Node 24 recommended). `internal/policy` purity, fail-closed (with the
documented detection-fail-OPEN exception), thin Cobra wiring, no new third-party deps.
**Pattern extraction date:** 2026-06-04
