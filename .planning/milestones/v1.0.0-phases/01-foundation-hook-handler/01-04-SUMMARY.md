# Phase 1 / Plan 04 — Pure Policy Engine — Summary

**Plan:** `01-PLAN-policy-engine.md`
**Executed:** 2026-05-26
**Status:** Complete — all acceptance criteria met. 10/10 tests pass; `go vet` and `go build ./...` clean.
**Commit:** `afd5f67` feat: Phase 1 Plan 04 — pure policy engine, Bumblebee single-source warn semantics (TDD)
**Approach:** TDD — tests written RED first, then GREEN implementation.

## What Was Built

The `internal/policy` package: a pure, side-effect-free policy-evaluation library
that the hook handler (Phase 1), MCP gateway (Phase 4), and Sentry correlation
(Phase 5+) all share. Phase 1 implements Bumblebee single-source catalog
matching only: a match yields a **warn** (Allow stays true — warn does not
block); unsigned matches are warn-only (CTLG-07). Corroboration-based block
escalation is deferred to Phase 2 (PLCY-01).

### Files created

- `internal/policy/types.go` — `ToolCall`, `CatalogMatch`, `Decision`, `CatalogLookup` interface
- `internal/policy/engine.go` — pure `Evaluate(tc ToolCall, idx CatalogLookup) Decision` + unexported `extract`/`versionMatches`/`splitVersion`/`normalize` helpers
- `internal/policy/engine_test.go` — 10 tests with a `fakeCatalog` implementing `CatalogLookup` (no mmap needed)
- `testdata/toolcalls/block_nx_console.json` — direct-shape Nx Console match fixture (reused by hook handler/selftest plans)
- `testdata/toolcalls/allow_express.json` — npm-command-shape clean fixture

## Key Interfaces Created (downstream plans consume these)

```go
// internal/policy/types.go
type ToolCall struct {
    AgentName string         `json:"agent_name"`
    ToolName  string         `json:"tool_name"`
    ToolInput map[string]any `json:"tool_input"`
}

type CatalogMatch struct {
    CatalogSource string // e.g. "bumblebee"
    EntryID       string
    Ecosystem     string
    Package       string
    Version       string // extracted tool-call version ("" if unspecified)
    Severity      string
    Signed        bool   // true iff entry carries a signature (CTLG-07)
}

type Decision struct {
    Allow          bool     // true => exit 0; false => block (non-zero)
    Level          string   // "allow" | "warn" | "block"
    Reason         string
    RuleIDs        []string // e.g. ["bumblebee-catalog-match"]
    CatalogMatches []CatalogMatch
}

// CatalogLookup is the minimal interface Evaluate depends on; the concrete
// *catalog.Index satisfies it. Keeps policy free of the mmap type for testability.
type CatalogLookup interface {
    Lookup(ecosystem, pkg string) (catalog.Entry, bool)
}

// internal/policy/engine.go
func Evaluate(tc ToolCall, idx CatalogLookup) Decision
```

### Decision semantics (Phase 1)

| Situation | Allow | Level | Reason | CatalogMatches |
|-----------|-------|-------|--------|----------------|
| Catalog hit (version in `Entry.Versions`, or entry version-less, or input version-less) | true | `warn` | `bumblebee catalog match: <id>` | 1 (RuleIDs `[bumblebee-catalog-match]`) |
| Unsigned catalog hit | true | `warn` | as above | 1, `Signed:false` (never escalates, CTLG-07) |
| Signed catalog hit | true | `warn` | as above | 1, `Signed:true` (no escalation in Phase 1) |
| Package extracted but no catalog entry, or version not covered | true | `allow` | `no catalog match` | 0 |
| No package extractable from input | true | `allow` | `no package identified` | 0 |

### Package extraction (`extract`)

- **Direct shape:** `tool_input{ecosystem, package, version}` taken verbatim (editor-extension corpus case).
- **Command shape:** `tool_input.command` install command; prefix → ecosystem table:
  `npm install`/`npm i ` → npm, `pip install`/`pip3 install` → pypi, `go get` → go,
  `gem install` → rubygems, `cargo add`/`cargo install` → cargo, `composer require` → packagist.
- First non-flag token after the prefix is the package; trailing `@version` split off via **last** `@` (scoped npm `@scope/pkg@1.0.0` safe).
- Package names lowercased + trimmed (`normalize`) to match index key normalization.
- No explicit `@version` → version `""`, which still matches (defense-favoring).

## Purity Contract — TestEngineImportsArePure

**Result: PASS.** The test reads `engine.go`, parses its import block with
`go/parser` (`ImportsOnly`), and asserts none of `os`, `net`, `net/http`, `io`,
`sync`, `time`, `context` are imported. `engine.go` imports only `strings` and
`internal/catalog` (for `catalog.Entry` and `catalog.VerifySignature`). `Evaluate`
does O(1) map reads + one `Lookup` + a linear scan of `Entry.Versions` — no I/O,
no goroutines, no globals mutated, no wall-clock access.

## Acceptance Criteria Met

- [x] `go test ./internal/policy/... -count=1` exits 0 (10/10 pass)
- [x] TestEngineImportsArePure passes — engine.go imports none of os/net/http/io/sync/time/context
- [x] TestCatalogMatchProducesWarn: nrwl.angular-console@18.95.0 → Level `warn`, Allow true, one CatalogMatch
- [x] TestUnsignedCatalogIsWarnOnly: unsigned entry → Level `warn`, CatalogMatches[0].Signed false, Allow true
- [x] TestSignedCatalogStillWarnInPhase1: signed entry → Level `warn`, Signed true, Allow true
- [x] TestNoMatchAllows: express@4.18.2 → Level `allow`, Allow true, empty CatalogMatches
- [x] TestRemediatedVersionAllows: nrwl.angular-console@18.100.0 (not in Versions) → Level `allow`
- [x] TestNpmInstallCommandExtraction: `npm install some-pkg@1.0.0` parsed to npm/some-pkg/1.0.0
- [x] TestNoPackageIdentifiedAllows: empty ToolInput → Level `allow`, Reason `no package identified`
- [x] `go vet ./internal/policy/...` exits 0
- [x] `go build ./...` exits 0

Tests beyond the listed set (still in scope, exercising specified behavior):
`TestVersionlessCommandStillWarns` (version-less command is defense-favoring),
`TestPackageNameNormalizedBeforeLookup` (mixed-case/padded input normalized).

## Requirements Satisfied

- **CTLG-07** — Unsigned catalog matches are warn-only and never set `Allow=false`
  (`VerifySignature` drives `CatalogMatch.Signed`; Level stays `warn` regardless).

## Threat Mitigations Implemented

- **T-04-01 (extraction evasion)** — `extract` normalizes (lowercase/trim) and
  supports both direct and command shapes; unknown shapes → allow with reason
  `no package identified` (documented Phase 1 limitation; expands in Phase 2).
- **T-04-02 (unsigned-entry forced block as DoS)** — unsigned matches never
  block; proven by TestUnsignedCatalogIsWarnOnly.
- **T-04-03 (hidden I/O / policy drift)** — TestEngineImportsArePure enforces the
  pure-library import allowlist at the AST level.
- **T-04-04 (pathological input DoS)** — accepted; `Evaluate` is O(1) map reads +
  one lookup; stdin/time caps live in the hook handler (Plan 05).

## Deviations from the Plan

1. **`Reason` field, not a `Reason` type.** The plan's `must_haves.exports` lists
   `Reason`, but the interface block defines `Reason string` as a field of
   `Decision`. Implemented as the field per the interface block (the authoritative
   contract). No separate `Reason` type exists.
2. **Added two extra tests** (`TestVersionlessCommandStillWarns`,
   `TestPackageNameNormalizedBeforeLookup`) covering behavior the plan's
   `<behavior>` section specifies (version-less defense-favoring match; package
   normalization). No scope expansion.
3. **`splitVersion` uses the last `@`** to correctly handle scoped npm packages
   (`@scope/pkg@1.0.0`) — a robustness detail within the specified command-shape
   extraction.

No scope beyond Plan 04 was implemented: no audit writer, no hook handler, no
config — only `internal/policy` and `testdata/toolcalls`.
