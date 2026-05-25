---
phase: 01-foundation-hook-handler
plan: 04
type: tdd
wave: 3
depends_on: [02]
files_modified:
  - internal/policy/types.go
  - internal/policy/engine.go
  - internal/policy/engine_test.go
  - testdata/toolcalls/block_nx_console.json
  - testdata/toolcalls/allow_express.json
autonomous: true
requirements: [CTLG-07]
must_haves:
  truths:
    - "Evaluate is a pure function: given a tool call and a catalog index it returns a Decision with no I/O, goroutines, or side effects"
    - "A tool call matching a catalog entry produces a warn decision (single source → warn in Phase 1)"
    - "An unsigned catalog match produces warn-only regardless of severity (CTLG-07)"
    - "A tool call with no catalog match produces an allow decision"
    - "The ecosystem + package + version are extracted from the tool call input"
  artifacts:
    - path: "internal/policy/types.go"
      provides: "ToolCall, Decision, Reason, CatalogMatch types"
      exports: ["ToolCall", "Decision", "Reason"]
    - path: "internal/policy/engine.go"
      provides: "Pure Evaluate function"
      exports: ["Evaluate"]
  key_links:
    - from: "internal/policy/engine.go"
      to: "internal/catalog.Index"
      via: "Lookup called with extracted ecosystem+package"
      pattern: "Lookup"
---

<objective>
Implement `internal/policy` as a pure function library: `Evaluate(toolCall, index) Decision`. In Phase 1 the only rule is Bumblebee single-source catalog matching, which produces a warn decision (corroboration-based block enforcement is Phase 2, PLCY-01, out of scope). Unsigned catalog matches are warn-only per CTLG-07.

Purpose: This package is the single policy implementation that the hook handler (Phase 1), the MCP gateway (Phase 4), and Sentry correlation (Phase 5+) all call — so it MUST stay pure (no I/O, no goroutines, no side effects) to prevent policy drift. Getting this contract right now is load-bearing for the whole project.
Output: `internal/policy` package with `ToolCall`/`Decision`/`Reason` types and a pure `Evaluate` function, fully TDD'd against fixtures.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/phases/01-foundation-hook-handler/01-CONTEXT.md
@.planning/phases/01-foundation-hook-handler/01-RESEARCH.md
@CLAUDE.md
@.planning/phases/01-foundation-hook-handler/01-02-SUMMARY.md

<interfaces>
<!-- From plan 02 (already built): -->
```go
// internal/catalog
type Entry struct {
    ID, Name, Ecosystem, Package string
    Versions []string
    Severity, SourceURL, CatalogSignature, CatalogSource string
}
type Index struct { /* mmap-backed */ }
func (idx *Index) Lookup(ecosystem, pkg string) (Entry, bool)
func VerifySignature(e Entry) bool
```

<!-- Contracts this plan CREATES — the hook handler (plan 05) and audit writer (plan 06) consume these: -->
```go
// internal/policy/types.go
type ToolCall struct {
    AgentName string                 `json:"agent_name"`
    ToolName  string                 `json:"tool_name"`
    ToolInput map[string]any         `json:"tool_input"`
}
type CatalogMatch struct {
    CatalogSource string // "bumblebee"
    EntryID       string
    Ecosystem     string
    Package       string
    Version       string
    Severity      string
    Signed        bool
}
type Decision struct {
    Allow         bool           // true => exit 0; false => block exit non-zero
    Level         string         // "allow" | "warn" | "block"
    Reason        string         // human-readable structured reason
    RuleIDs       []string       // e.g. ["bumblebee-catalog-match"]
    CatalogMatches []CatalogMatch
}
// Evaluate is PURE: no I/O, no goroutines, no globals, no time.Now side effects.
func Evaluate(tc ToolCall, idx CatalogLookup) Decision
// CatalogLookup is the minimal interface Evaluate needs (so policy doesn't import a concrete mmap type for testability).
type CatalogLookup interface {
    Lookup(ecosystem, pkg string) (catalog.Entry, bool)
}
```
</interfaces>
</context>

<feature>
  <name>Pure Bumblebee catalog-match policy evaluation</name>
  <files>internal/policy/types.go, internal/policy/engine.go, internal/policy/engine_test.go</files>
  <behavior>
    Phase 1 evaluation rules (warn-only, single-source):
    - A tool call whose extracted (ecosystem, package) matches a catalog Entry whose Versions list contains the extracted version → Decision{Allow: true, Level: "warn", RuleIDs: ["bumblebee-catalog-match"], CatalogMatches: [one match]}. NOTE Phase 1: single source → warn, and warn does NOT block (Allow stays true) because catalog-driven blocking requires corroboration (PLCY-01, Phase 2). The warn is surfaced in the reason and audit record.
    - A catalog match where the entry is unsigned (VerifySignature false) → still warn (never escalates), Level "warn", Signed:false recorded (CTLG-07).
    - A catalog match where the entry IS signed → still warn in Phase 1 (no corroboration yet), Signed:true recorded. (Block escalation is Phase 2.)
    - No catalog match → Decision{Allow: true, Level: "allow", Reason: "no catalog match"}.
    - A tool call from which no ecosystem/package can be extracted (e.g. a non-package tool) → Decision{Allow: true, Level: "allow", Reason: "no package identified"} (no false block; sensitive-path/lifecycle rules are Phase 2).

    Extraction (ecosystem/package/version from ToolInput):
    - Support the install-command shape used by the adversarial corpus: a tool_input with a "command" string like "npm install <pkg>@<version>" maps to ecosystem "npm"; "pip install" → "pypi"; "go get" → "go"; "gem install" → "rubygems"; "cargo add"/"cargo install" → "cargo"; "composer require" → "packagist".
    - Support a direct shape: tool_input keys {"ecosystem","package","version"} taken verbatim (covers the editor-extension corpus case ecosystem "editor-extension", package "nrwl.angular-console", version "18.95.0").
    - Package names are lowercased and trimmed before lookup to match the index key normalization (catalog plan lowercases on build).
    - When a command lists a package without an explicit @version, version is "" and a version-less catalog match (entry matches package, version unspecified) still produces a warn (defense-favoring).

    Purity:
    - Evaluate takes only its arguments and returns a Decision; it must not import os, net, io, time (except no time use), sync, or any package that performs I/O or spawns goroutines.
  </behavior>
  <implementation>
    RED first: write internal/policy/engine_test.go with these tests, each asserting on Decision fields:
    - TestCatalogMatchProducesWarn (editor-extension nrwl.angular-console@18.95.0 → Level "warn", one CatalogMatch, Allow true)
    - TestUnsignedCatalogIsWarnOnly (matched entry with empty signature → Level "warn", CatalogMatches[0].Signed false, Allow true — never block)
    - TestSignedCatalogStillWarnInPhase1 (matched entry with signature → Level "warn", Signed true, Allow true)
    - TestNoMatchAllows (express@4.18.2 → Level "allow", Allow true, empty CatalogMatches)
    - TestRemediatedVersionAllows (nrwl.angular-console@18.100.0 not in entry Versions → allow)
    - TestNpmInstallCommandExtraction ("npm install nrwl..." style command parsed to ecosystem npm)
    - TestNoPackageIdentifiedAllows (a tool call with no package info → allow, reason "no package identified")
    - TestEngineImportsArePure (a test that reads engine.go via os.ReadFile in the TEST file only and asserts the engine source does not import "os", "net/http", "io", "sync", "time", or "context" — enforces the purity contract)
    Use a fake CatalogLookup implementing the CatalogLookup interface returning canned entries — no real mmap needed in unit tests.

    Then GREEN: implement types.go (the structs above) and engine.go (Evaluate + an unexported extract(toolInput) (ecosystem, pkg, version string, ok bool) helper). engine.go imports only: strings, and internal/catalog (for the Entry type and VerifySignature). It must NOT import os/net/io/sync/time/context.

    Create testdata/toolcalls/block_nx_console.json (direct shape: agent_name, tool_name, tool_input{ecosystem,package,version} for the Nx Console match) and testdata/toolcalls/allow_express.json (npm install express@4.18.2 command shape). These fixtures are reused by the hook handler and selftest plans.

    REFACTOR if extraction grows unwieldy: keep extract() table-driven (command prefix → ecosystem map).
  </implementation>
</feature>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| tool call JSON → policy | The decoded tool call is untrusted attacker-influenced input |
| catalog entry → decision | Catalog signedness governs whether a match can ever escalate |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-04-01 | Spoofing | Crafted tool_input evading package extraction to dodge a catalog match | mitigate | extract() normalizes (lowercase/trim) and supports both command and direct shapes; unknown shapes → allow but are logged; corroboration/sensitive-path coverage expands in Phase 2 (documented limitation) |
| T-04-02 | Tampering | Unsigned catalog entry used to force a block (false positive as DoS) | mitigate | CTLG-07: unsigned matches are warn-only and never set Allow=false; test TestUnsignedCatalogIsWarnOnly proves it |
| T-04-03 | Elevation of Privilege | Hidden I/O or goroutine in policy breaking the pure-library contract (policy drift across consumers) | mitigate | TestEngineImportsArePure asserts engine.go imports none of os/net/io/sync/time/context; CLAUDE.md locked constraint |
| T-04-04 | Denial of Service | Pathological ToolInput causing excessive work | accept | Evaluate does O(1) map reads + one index Lookup; stdin size + timeout caps live in the hook handler (plan 05) |
</threat_model>

<verification>
- `go test ./internal/policy/... -count=1` exits 0
- TestEngineImportsArePure passes (purity contract enforced)
- `go vet ./internal/policy/...` exits 0
</verification>

<success_criteria>
- A pure `Evaluate` function with no I/O/goroutines/side effects (CLAUDE.md locked constraint)
- Bumblebee single-source matching producing warn decisions; unsigned matches warn-only (CTLG-07)
- Tool-call package extraction for the adversarial corpus shapes
- Reusable tool-call fixtures for downstream plans
</success_criteria>

<output>
After completion, create `.planning/phases/01-foundation-hook-handler/01-04-SUMMARY.md`
</output>
