# Phase 7: Sensitive-Path Runtime Enforcement — Research

**Researched:** 2026-06-03
**Domain:** Go wiring + path canonicalization — wiring an existing pure policy engine into the live `beekeeper check` pipeline and closing bypass vectors
**Confidence:** HIGH — all findings derived from direct codebase inspection with file:line citations; zero web-search dependencies needed for this wiring phase

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SPATH-01 | `beekeeper check` blocks Read/Write/Edit tool calls whose `file_path` targets a sensitive credential path by wiring `policy.EvaluatePath`/`DefaultSensitivePaths` into `runCheck` | Engine exists at `internal/policy/path.go:76`; wiring point identified at `handler.go:257` (after `policy.Evaluate`, before `ApplyPolicyOverlay`); new file `internal/check/paths.go` |
| SPATH-02 | Path targets canonicalized before evaluation — tilde expansion, `filepath.Abs`, `EvalSymlinks`, slash normalization | Canonicalization lives in `internal/check/paths.go` (impure caller); pure `EvaluatePath` receives pre-resolved string; `expandHome` pattern already in `internal/watch/watcher.go:121-132` |
| SPATH-03 | Credential access via shell-command targets (`cat`/`type`/`Get-Content`/`gc` inside Bash tool calls) detected and flagged | `isSensitivePath` in `internal/sentry/rules.go:53` is prior art; new `extractBashCredentialPaths` in `paths.go`; conservative prefix-allowlist approach |
| SPATH-04 | Default allowlist prevents false positives on `.env.example`/`.env.test`/`.env.schema`; policy-file `sensitive_path` rules merge by most-restrictive-wins with allowlist escape hatch | `DefaultSensitivePaths().AllowPatterns` is currently `nil`; three entries must be added; `ApplyPolicyOverlay` already handles `sensitive_path` rule type |
</phase_requirements>

---

## Summary

Phase 7 is a wiring phase, not a feature build. The full sensitive-path policy engine (`policy.EvaluatePath` + `DefaultSensitivePaths`) exists at `internal/policy/path.go` and is currently exercised only by its own unit tests. The wiring gap is that `runCheck` in `internal/check/handler.go` never calls `EvaluatePath` — it only calls `policy.Evaluate` (the catalog/corroboration engine) and `policyloader.ApplyPolicyOverlay` (declarative JSON-policy overlay). A `Read` tool call with `{"file_path":"~/.aws/credentials"}` passes through all three stages as allow because none of them inspect `file_path`.

Three sub-problems must be solved together: (1) a new `internal/check/paths.go` that extracts path targets from a `policy.ToolCall` and canonicalizes them (tilde expansion, `filepath.Abs`, `EvalSymlinks`, slash normalization — all impure I/O that must NOT enter the pure policy package); (2) insertion of the path-evaluation block in `runCheck` at exactly the right place in the existing pipeline; and (3) fixing a secondary gap in `policyloader/enforce.go:extractTargetPath` which reads only `tc.ToolInput["path"]`, missing the `"file_path"` key that Claude Code's Read/Write/Edit tools actually emit.

SPATH-04 (allowlist for safe `.env` lookalikes) requires adding three entries to `DefaultSensitivePaths().AllowPatterns` before the wiring goes live, because the current `.env.*` glob would block `.env.example`, `.env.test`, and `.env.schema` — files that appear in almost every project. Phase 6's `resolveCatalogHealthy` wiring into four consumers (check/gateway/watch/scan) via `internal/catalog/health.go` is the exact model this phase follows.

**Primary recommendation:** Build `internal/check/paths.go` first (pure extraction + impure canonicalization, fully unit-tested), then insert the wiring block in `handler.go`, then fix `enforce.go:extractTargetPath`, then fix `DefaultSensitivePaths` allowlist. The integration tests (`RunCheck` with raw stdin JSON asserting exit code + `decision:"block"` audit record) prove the pipeline end-to-end.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Path-sensitivity decision (is this a credential file?) | Pure policy library (`internal/policy`) | — | `EvaluatePath` already exists here; must stay pure |
| Path target extraction from ToolCall | Check handler (`internal/check/paths.go`) | Policyloader overlay (`enforce.go`) | Reads `map[string]any` — I/O-free extraction; correct home for this logic |
| Tilde expansion, `filepath.Abs`, `EvalSymlinks` | Check handler (`internal/check/paths.go`) | — | Filesystem I/O; forbidden in pure `internal/policy` |
| Bash command scanning for credential reads | Check handler (`internal/check/paths.go`) | — | Close to the tool-call decode point; follows existing sentry `isSensitivePath` pattern |
| Default allowlist (`.env.example` etc.) | Pure policy library (`internal/policy/path.go`) | — | `DefaultSensitivePaths().AllowPatterns` is the correct home — pure value, no I/O |
| Policy-file `sensitive_path` overlay rules | Policyloader (`internal/policyloader/enforce.go`) | — | Already wired; gap is `extractTargetPath` reading wrong key |
| Exit-code + audit record for path blocks | Check handler (`internal/check/handler.go`) | — | `finalizeWithAC` is the single chokepoint; path block feeds into existing `mergeDecisions` flow |

---

## Standard Stack

No new dependencies. This phase uses only Go stdlib and existing internal packages.

### Core (all existing)

| Package | Purpose | Status |
|---------|---------|--------|
| `internal/policy/path.go` | `EvaluatePath`, `DefaultSensitivePaths`, `SensitivePathConfig` | EXISTS — unwired |
| `internal/check/handler.go` | `runCheck` pipeline; insertion point identified | EXISTS — needs wiring |
| `internal/policyloader/enforce.go` | `extractTargetPath`, `ApplyPolicyOverlay`, `matchesSensitivePath` | EXISTS — has `file_path` key gap |
| `path/filepath` | `Abs`, `EvalSymlinks`, `ToSlash` | stdlib |
| `os` | `UserHomeDir`, `os.Getenv` (USERPROFILE fallback) | stdlib |
| `strings` | Path manipulation in pure functions | stdlib |

### New File

| File | Type | Purpose |
|------|------|---------|
| `internal/check/paths.go` | NEW (impure adapter) | `extractPathTargets`, `canonicalizePath`, `extractBashCredentialPaths` |

---

## Architecture Patterns

### System Architecture: runCheck Pipeline After Phase 7

```
stdin JSON
  |
  v
[decode hookInput]                         handler.go:153
  |
  v
[open mmap bbIdx]                          handler.go:177
  |
  v
[build MultiIndex]                         handler.go:194-211
  |
  v
[load policyFiles + derive thresholds]     handler.go:233-246
  |
  v
[policy.Evaluate]                          handler.go:257  (catalog/corroboration)
  |
  v
[extractPathTargets(toolCall)]             NEW -- paths.go  (SPATH-01/02/03)
  |   reads file_path, path, and Bash command targets
  |   canonicalizes each: tilde → Abs → EvalSymlinks → ToSlash
  v
[EvaluatePath(resolvedPath, cfg)]          pure call into policy.EvaluatePath
  |   per path; merge most-restrictive-wins
  v
[mergeDecisions(catalogDecision, pathDecision)]
  |   block > warn > allow
  v
[ApplyPolicyOverlay(policyFiles, ...)]     handler.go:273  (declarative JSON overlay)
  |   sensitive_path rules from policy files applied on top
  v
[finalizeWithAC]                           single chokepoint — exit code + audit record
```

### Recommended Project Structure

```
internal/
  check/
    handler.go          # runCheck: insert EvaluatePath block between Evaluate and ApplyOverlay
    paths.go            # NEW: extractPathTargets, canonicalizePath, extractBashCredentialPaths
    integration_test.go # add SPATH RunCheck tests here (existing file)
  policy/
    path.go             # EvaluatePath + DefaultSensitivePaths: add AllowPatterns for .env.example etc.
    path_test.go        # add allowlist + traversal tests
  policyloader/
    enforce.go          # fix extractTargetPath to also read "file_path" key
    enforce_test.go     # add file_path extraction test
```

### Pattern 1: Caller-Resolved I/O, Pure Decision

The existing `EvaluatePath` docstring at `path.go:18-21` explicitly states:

```go
// Path normalization (resolving "~" to the home directory, converting OS
// separators) is the CALLER's responsibility. EvaluatePath receives an
// already-resolved string and matches verbatim.
```

This is the established pattern across the codebase: `policy.EvaluateReleaseAge(ReleaseAgeInput, cfg)` (release_age.go), `policy.Evaluate(ToolCall, MultiCatalogLookup, ...)` (engine.go:64), all follow the same shape. The new `internal/check/paths.go` is the I/O adapter. It performs all filesystem operations, then passes a clean absolute forward-slash path to the pure `EvaluatePath`.

### Pattern 2: expandHome (prior art in `internal/watch/watcher.go:121-132`)

```go
// internal/watch/watcher.go:121-132 — existing implementation to copy
func expandHome(dir string) string {
    if len(dir) == 0 || dir[0] != '~' {
        return dir
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return dir  // fail-safe: return unchanged on error
    }
    return filepath.Join(home, dir[1:])
}
```

`paths.go` should copy this exact implementation (not import it — `internal/watch` depends on `fsnotify`; we don't want that dependency in `internal/check`). The fail-safe on `UserHomeDir` error is important: if home cannot be resolved, return the unresolved path — `EvaluatePath` will not match it (safe direction since it does not canonicalize to a credential path).

### Pattern 3: Phase 6 resolveCatalogHealthy Wiring Model

Phase 6 wired `resolveCatalogHealthy` into four consumers by creating `internal/catalog/health.go` (the single shared implementation) and thin delegating functions in each caller package (`internal/check/sanity.go:11-13`). This phase follows the same model: the pure engine (`policy.EvaluatePath`) is already in `internal/policy`; the adapter (`extractPathTargets` + `canonicalizePath`) lives in `internal/check/paths.go`.

The ROADMAP's Phase 7 success criteria explicitly scope this to `beekeeper check` only — the gateway, watch, and scan consumers are NOT in scope for SPATH-01–04. The Phase 6 model wired into all four consumers; this phase wires into check only. The planner must NOT follow Phase 6's four-consumer pattern for SPATH.

### Pattern 4: mergeDecisions (most-restrictive-wins)

```go
// internal/check/paths.go (new helper)
// mergeDecisions returns the most restrictive of base and overlay.
// block(2) > warn(1) > allow(0).
func mergeDecisions(base, overlay policy.Decision) policy.Decision {
    rank := map[string]int{"allow": 0, "warn": 1, "block": 2}
    if rank[overlay.Level] > rank[base.Level] {
        return overlay
    }
    return base
}
```

This is consistent with `policyloader/enforce.go:139-165` which uses the same rank logic internally. Do not reuse `ApplyPolicyOverlay` for the path merge — it is designed for JSON policy-file rules, not for merging two programmatic `Decision` values.

### Pattern 5: Integration Test Shape (from Phase 6 plan 06-03)

Phase 6 plan 06-03 added three `RunCheck` integration tests in `handler_test.go`. The pattern (lines 659-817):

```go
func TestRunCheckAiFigureBlocks(t *testing.T) {
    dir := t.TempDir()
    idxPath := buildCriticalTestIndex(t, dir)
    cacheDir := filepath.Join(dir, "catalogs")
    os.MkdirAll(cacheDir, 0700)

    stdin := strings.NewReader(`{"agent_name":"test-agent","tool_name":"Bash","tool_input":{"command":"npm install ai-figure-test@1.0.0"}}`)
    res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), cacheDir)

    if res.Decision.Level != "block" { t.Errorf(...) }
    if res.ExitCode != exitBlock { t.Errorf(...) }
    if res.Decision.Allow { t.Error(...) }
}
```

SPATH tests follow this exact shape. The `auditPathIn(t)` helper + `readLastAuditRecord(t, auditPath)` (integration_test.go:76-99) are available for asserting `decision:"block"` in the NDJSON record.

### Anti-Patterns to Avoid

- **Putting tilde/Abs/EvalSymlinks in `internal/policy/path.go`:** Violates the pure-library contract. The `TestPathImportsArePure` test in `path_test.go:198-230` will catch this — it explicitly forbids `"os"` as an import.
- **Using `extractTargetPath` from `enforce.go` inside the handler:** That function reads only `tc.ToolInput["path"]`; Read/Write/Edit use `"file_path"`. The fix goes in `enforce.go` (add `"file_path"` fallback), not by importing enforce from the handler.
- **Blocking on `EvalSymlinks` error when the target file does not exist:** `filepath.EvalSymlinks` returns an error when the path does not exist. The correct behavior is to fall back to `filepath.Abs` (without symlink resolution) so that a non-existent credential path like `~/.aws/credentials` (before first AWS use) is still evaluated. Fail-closed means: if `EvalSymlinks` errors, use the `Abs`-only result.
- **Matching USERPROFILE Windows env-var expansion in the tilde step:** On Windows, `~` is not a shell concept natively; Claude Code resolves `~` itself before embedding in `file_path`. However, `os.UserHomeDir()` on Windows returns `%USERPROFILE%` correctly. The canonicalization must handle `C:\Users\foo\.aws\credentials` (backslash path from Windows agent). The existing `normalizeSlashes` helper in `path.go:166-168` handles this at the pattern-matching level.
- **Scanning the entire Bash command for every tool call:** Only scan `Bash` tool calls (`tc.ToolName == "Bash"`) and only when they have a `"command"` key. Parsing the command for credential-read patterns on every `Bash` call is cheap, but do not parse non-Bash tools.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead |
|---------|-------------|-------------|
| Sensitive-path decision | Custom blocklist loop in handler | `policy.EvaluatePath(resolvedPath, cfg)` — already exists |
| Most-restrictive-wins merge | Custom if-chain in handler | `mergeDecisions` helper (5 lines, mirrors enforce.go logic) |
| Home directory resolution | `os.Getenv("HOME")` direct read | `os.UserHomeDir()` — handles `USERPROFILE` on Windows correctly |
| Policy-file allowlist (`.env.example`) | Policy file rule per project | `DefaultSensitivePaths().AllowPatterns` — built-in default for safe lookalikes |
| Shell tokenizer for Bash commands | Full POSIX parser | Conservative prefix-matching with a short allowlist of read-command verbs (`cat `, `type `, `Get-Content `, `gc `) — mirrors `extractFromCommand` in engine.go |

**Key insight:** The engine exists. The only work is (a) extracting the right keys from `ToolInput`, (b) canonicalizing paths in the impure adapter, and (c) calling the already-tested pure function. Any deviation from this structure introduces complexity without adding safety.

---

## Research Question Answers

### Q1: Engine Surface

**`policy.EvaluatePath` signature** (`path.go:76`):
```go
func EvaluatePath(resolvedPath string, cfg SensitivePathConfig) Decision
```

Returns a `policy.Decision` (same struct as `policy.Evaluate`) with:
- `Allow: false`, `Level: "block"`, `Reason: "sensitive path blocked: " + pattern`, `RuleIDs: []string{"sensitive-path-policy"}` on a block
- `Allow: true`, `Level: "allow"` on allow

**`DefaultSensitivePaths()` contents** (`path.go:34-56`):
```go
BlockPatterns: []string{
    "/.ssh/", "/.aws/", "/.gnupg/",
    "/.config/Claude/", "/.config/op/", "/.config/gh/",
    "/.netrc", "/.npmrc", "/.pypirc", "/.cargo/credentials.toml",
    ".env", ".env.local", ".env.*",
},
AllowPatterns: nil,  // <-- GAP: must add .env.example, .env.test, .env.schema
```

**Coverage gaps for SPATH-01 requirements:**

| Required path | Covered? | Notes |
|--------------|----------|-------|
| `~/.ssh/*` | YES — `"/.ssh/"` fragment match | |
| `~/.aws/*` | YES — `"/.aws/"` fragment match | |
| `~/.gnupg/*` | YES — `"/.gnupg/"` fragment match | |
| `~/.npmrc` | YES — `".npmrc"` basename match | |
| `~/.pypirc` | YES — `".pypirc"` basename match | |
| `~/.cargo/credentials*` | PARTIAL — `".cargo/credentials.toml"` is a fragment match; covers `credentials.toml` but not `credentials` (bare file). Add `"/.cargo/credentials"` fragment or rely on the `credentials.toml` match. Recommendation: add `"/.cargo/credentials"` as a second entry since the bare `credentials` file (no extension) is the pre-2022 format. |
| `.env`/`.env.*` | YES — basename match + glob | |
| MCP host-config files | PARTIAL — `"/.config/Claude/"` covers `~/.config/Claude/claude_desktop_config.json`. The `path.go:22-23` comment says "MCP config files enumerated by Bumblebee are appended to BlockPatterns by the wiring layer at Plan 08 time." For Phase 7, `"/.config/Claude/"` covers the Claude Desktop case. Cursor/Windsurf MCP paths (`~/.cursor/mcp.json`, `~/.windsurf/mcp.json`) are NOT in DefaultSensitivePaths today. |

**Recommendation:** For Phase 7 scope (SPATH-01), add `"/.cursor/"` and `"/.windsurf/"` to `BlockPatterns` to cover Cursor and Windsurf MCP config directories, since they are primary dev tools per this codebase (Phase 4 wired them). Add `"/.cargo/credentials"` (bare). Add `.env.example`, `.env.test`, `.env.schema` to `AllowPatterns`.

**Matching behavior:**
- Patterns containing `/` or `\` → `strings.Contains` fragment match (`path.go:115-121`)
- Patterns without separator → basename match against last segment (`path.go:123-133`)
- `.env.*` → glob simulation: `strings.HasPrefix(seg, ".env.")` (`path.go:127-129`)
- AllowPatterns → checked first; `isAllowedPath` allows exact or path-boundary prefix (`path.go:141-152`)

**Allowlist concept (SPATH-04):** YES — `SensitivePathConfig.AllowPatterns []string` exists at `path.go:9-27`. `EvaluatePath` checks allowlist FIRST (step 1 at line 78-87) before blocklist. Currently `nil` in defaults. Adding `.env.example`, `.env.test`, `.env.schema` here fixes the false-positive gap before wiring goes live.

### Q2: Wiring Point

**`runCheck` pipeline** (handler.go:101-279):

```
handler.go:153  — json.Decode into hookInput (ToolCall)
handler.go:177  — open mmap bbIdx
handler.go:194  — build OSV adapter
handler.go:208  — build Socket adapter (if token configured)
handler.go:213  — catalog.NewMultiIndex
handler.go:233  — LoadPolicyDir (policyFiles)
handler.go:246  — ThresholdsFromPolicyFiles(policyFiles)
handler.go:251  — resolveCatalogHealthy → thresholds.CatalogHealthy
handler.go:257  — policy.Evaluate(toolCall, multiIdx, thresholds, ac)   ← INSERT AFTER HERE
handler.go:273  — ApplyPolicyOverlay(policyFiles, toolCall, decision)    ← INSERT BEFORE HERE
handler.go:278  — finalizeWithAC(decision, ...)
```

**Insertion point:** After line 257 (`decision := policy.Evaluate(...)`) and before line 273 (`ApplyPolicyOverlay`). The path-evaluation merge must run BEFORE the overlay so that:
1. A path-block from `EvaluatePath` can be escalated further (but not downgraded) by the overlay
2. A JSON-policy-file `sensitive_path` allow rule (the escape hatch) applies on top

**Proposed insertion:**
```go
// handler.go — after "decision := policy.Evaluate(...)" (line 257)

// SPATH-01/02/03: sensitive-path evaluation.
// extractPathTargets reads file_path, path, and Bash command targets;
// canonicalizePath resolves tilde, Abs, EvalSymlinks, and slash normalization.
// EvaluatePath is pure and receives only already-resolved strings.
spathCfg := policy.DefaultSensitivePaths()
// Merge policy-file sensitive_path block patterns (if any) into config:
// handled by ApplyPolicyOverlay below — DefaultSensitivePaths is the baseline.
for _, rawPath := range extractPathTargets(toolCall) {
    resolved := canonicalizePath(rawPath)
    if resolved == "" {
        continue
    }
    pathDecision := policy.EvaluatePath(resolved, spathCfg)
    decision = mergeDecisions(decision, pathDecision)
}
```

**Tool-call input struct:** `policy.ToolCall` at `types.go:19-23`:
```go
type ToolCall struct {
    AgentName string         `json:"agent_name"`
    ToolName  string         `json:"tool_name"`
    ToolInput map[string]any `json:"tool_input"`
}
```

Path keys in `ToolInput` by tool:

| Tool | ToolInput key | Value |
|------|--------------|-------|
| Read | `"file_path"` | string |
| Write | `"file_path"` | string |
| Edit | `"file_path"` | string |
| MultiEdit | `"file_path"` | string |
| Bash | `"command"` | string (may contain `cat <path>`) |
| Legacy overlay path | `"path"` | string (keep for compat) |

`extractTargetPath` in `policyloader/enforce.go:279-287` currently reads ONLY `tc.ToolInput["path"]`. Claude Code's Read/Write/Edit use `"file_path"`. This is a confirmed gap. **Fix required in enforce.go:** add `"file_path"` as primary key with `"path"` as fallback.

**Phase 6 wiring model:** Phase 6 wired `resolveCatalogHealthy` into check/gateway/watch/scan via `internal/catalog/health.go`. For SPATH, the scope is `beekeeper check` only per success criteria SC1-SC4 in the ROADMAP. Gateway, watch, and scan are NOT in Phase 7 scope (they are Phase 8+ or deferred per the REQUIREMENTS.md `sensitive_path` traceability table which maps SPATH-01–04 to Phase 7 only).

### Q3: Canonicalization (SPATH-02)

**Where canonicalization lives:** `internal/check/paths.go` (new file). The pure `internal/policy` package must not acquire filesystem I/O. `TestPathImportsArePure` at `path_test.go:198-230` enforces this — it fails if `path.go` imports `"os"`, `"io"`, `"context"`, `"time"`, `"net"`, `"net/http"`, or `"sync"`. `filepath.Abs` and `filepath.EvalSymlinks` touch the filesystem; they belong in the impure adapter.

**Canonicalization sequence:**
```go
// internal/check/paths.go
func canonicalizePath(raw string) string {
    // Step 1: tilde expansion
    p := expandHome(raw)

    // Step 2: filepath.Abs resolves relative paths (../traversal) to absolute.
    // Abs does not require the file to exist.
    abs, err := filepath.Abs(p)
    if err != nil {
        // Abs fails only on very degenerate inputs; return Abs-less result.
        abs = p
    }

    // Step 3: EvalSymlinks resolves symlinks.
    // IMPORTANT: EvalSymlinks errors when the target does not exist.
    // Fail-safe: on error, use the Abs result (not the raw unresolved path).
    // This means a non-existent but valid credential path like ~/.aws/credentials
    // (before first AWS use) is still evaluated — it will match "/.aws/" via Abs.
    resolved, err := filepath.EvalSymlinks(abs)
    if err != nil {
        resolved = abs // fall back to Abs-resolved path
    }

    // Step 4: normalize to forward slashes for cross-platform pattern matching.
    // On Windows, filepath.ToSlash converts backslash to forward slash.
    // EvaluatePath's matchesBlockPattern already handles backslash via normalizeSlashes,
    // but normalizing here makes test assertions platform-independent.
    return filepath.ToSlash(resolved)
}
```

**Windows specifics:**
- `os.UserHomeDir()` on Windows returns `C:\Users\username` (from `USERPROFILE` env). [VERIFIED: Go stdlib docs]
- `filepath.Abs` on Windows produces backslash paths (e.g. `C:\Users\u\.aws\credentials`).
- `filepath.ToSlash` converts to `C:/Users/u/.aws/credentials`.
- `EvaluatePath` in `path.go:119-121` handles backslash paths via `normalizeSlashes` even without step 4, but normalizing in the adapter is cleaner.
- `%USERPROFILE%\.ssh\id_rsa` in a Bash `type` command: the handler reads it as the raw string. Tilde expansion does not apply (it's `%USERPROFILE%`, not `~`). For `type` command parsing (SPATH-03), extract the path token and then let `canonicalizePath` run it through `filepath.Abs` which resolves relative to CWD. Env-var expansion (`%USERPROFILE%`) is explicitly out of scope for SPATH-03 Phase 7 — see SPATH-03 section below.
- Windows drive letters: `filepath.Abs` handles them correctly. `C:path` (no backslash) is rare from agent output; `C:\path` is the common form and is handled correctly.

**`EvalSymlinks` when file does not exist:** Returns `syscall.ENOENT` (or platform equivalent). The fail-safe `resolved = abs` preserves the Abs-resolved path, which still contains the credential path substring (e.g. `/.aws/credentials`), so `EvaluatePath` still blocks it. This is the correct fail-closed behavior.

### Q4: Shell Extraction (SPATH-03)

**Prior art:** `isSensitivePath` in `internal/sentry/rules.go:53-61`:
```go
func isSensitivePath(path string) bool {
    normalised := filepath.ToSlash(path)
    for _, s := range defaultSensitivePaths {
        if strings.Contains(normalised, s) {
            return true
        }
    }
    return false
}
```
This function is in `internal/sentry` (the OS-level sentry package). It cannot be imported from `internal/check` without creating a package dependency. The logic (ToSlash + contains loop) is simple enough to replicate in `internal/check/paths.go` — and the canonical check is done via `policy.EvaluatePath` anyway; `isSensitivePath` is used only to decide whether to apply the rule, not to produce a Decision.

**Phase 5 `filepath.ToSlash` fix:** Phase 5 plan 05-01 fixed a Windows backslash bug in `isSensitivePath` by adding `filepath.ToSlash`. This is already reflected in the current `rules.go:54`. The new `extractBashCredentialPaths` must apply the same ToSlash normalization.

**Approach for SPATH-03:** Conservative verb-prefix extraction — do not tokenize the full Bash command with a shell parser (too complex, introduces scope creep). Instead:

```go
// Read-command verbs that precede a path argument.
// Conservative list: add new verbs only when evidence of real bypass exists.
var bashReadVerbs = []string{
    "cat ",          // Unix
    "head ",         // Unix
    "tail ",         // Unix
    "less ",         // Unix
    "more ",         // Unix
    "type ",         // Windows CMD (note: "type" may appear in content, so require space+path)
    "Get-Content ",  // PowerShell
    "gc ",           // PowerShell alias
}

// extractBashCredentialPaths extracts path tokens from a Bash command that
// appear after a known read-command verb. Returns paths for canonicalization.
// Conservative: only returns paths that contain a plausible credential-path
// substring after initial extraction — avoids false positives on "cat poem.txt".
func extractBashCredentialPaths(cmd string) []string {
    var paths []string
    for _, verb := range bashReadVerbs {
        idx := strings.Index(cmd, verb)
        if idx == -1 {
            continue
        }
        rest := strings.TrimSpace(cmd[idx+len(verb):])
        token := firstShellToken(rest) // first non-flag, non-redirect token
        if token != "" {
            paths = append(paths, token)
        }
    }
    return paths
}
```

After extraction, each token is run through `canonicalizePath` and then `policy.EvaluatePath`. The credential-path check is done by `EvaluatePath` itself (not by a second contains loop).

**Bypass surface (in-scope vs deferred):**

| Bypass | In scope Phase 7 | Notes |
|--------|-----------------|-------|
| `cat ~/.aws/credentials` | YES | Direct verb+tilde — handled |
| `type %USERPROFILE%\.ssh\id_rsa` | PARTIAL — `type ` verb is parsed; `%USERPROFILE%` env-var expansion is deferred | `filepath.Abs` won't expand env vars; the raw token `%USERPROFILE%\.ssh\id_rsa` will NOT match `/.ssh/` after ToSlash. Documented as deferred: detecting `%USERPROFILE%` expansion requires `os.ExpandEnv` which introduces complexity and a new attack surface. The agent on Windows typically uses tilde via the tool, not `type`. |
| `cat "~/.aws/credentials"` (quoted) | YES — `firstShellToken` strips leading/trailing quotes | |
| `cat ~/.aws/credentials ~/.npmrc` (multiple paths) | YES — loop re-runs for each verb occurrence | |
| `zsh -c "cat ~/.aws/credentials"` (nested shell) | DEFERRED — not in SPATH-03 scope | |
| Base64-encoded commands | DEFERRED | |
| Command chaining: `ls && cat ~/.aws/credentials` | PARTIAL — `cat ` verb will be found anywhere in the string | |
| Here-string: `cat <<EOF` | DEFERRED | |

The success criterion SC2 in the ROADMAP is: `cat ~/.ssh/id_rsa` and `type %USERPROFILE%\.ssh\id_rsa`. The tilde form is fully covered. The `%USERPROFILE%` form requires env-var expansion; document this as a known limitation of Phase 7 (deferred to Phase 8+) and ensure the test for SC2 passes with the tilde form.

**Purity:** `extractBashCredentialPaths` is I/O-free — it operates only on the `string` extracted from `tc.ToolInput["command"]`. Canonicalization of the extracted tokens happens in `canonicalizePath` in the same `paths.go` file.

### Q5: Allowlist + Policy-File Merge (SPATH-04)

**Existing config/policy-file layering:**

`policyloader/loader.go` defines `PolicyRule` with `RuleType: "sensitive_path"` and `PathPatterns []string` (`loader.go:43`). `ValidateSchema` in `validate.go:14` has `"sensitive_path"` in `legalRuleTypes`. `ApplyPolicyOverlay` in `enforce.go:101-128` already processes `sensitive_path` rules with `matchesSensitivePath`.

**Where `sensitive_path` rules belong in schema:** Already in schema. No schema change needed for SPATH-04. A user policy file with `"rule_type":"sensitive_path"` and `"action":"allow"` functions as an escape hatch today (the overlay handles it at enforce.go:108-114, defaulting to `"block"` when action is empty).

**Most-restrictive-wins merge with allowlist escape hatch:**

The merge in Phase 7 is two-level:
1. `EvaluatePath(resolvedPath, DefaultSensitivePaths())` — hardcoded engine block. A JSON-policy-file `sensitive_path` allow rule in `ApplyPolicyOverlay` can override this because the allow rule is checked in `enforce.go:96-99` (package_allowlist) — but `sensitive_path` rules in the overlay only have `"block"` and `"warn"` actions (enforce.go:108-114), not `"allow"`. **Gap:** The overlay does not support `action:"allow"` for sensitive_path rules today.

   However, a user who wants to allow `.aws/credentials` can use a `package_allowlist` "allow" rule or rely on the `policy.SensitivePathConfig.AllowPatterns` field if they can configure it. For Phase 7, the escape hatch is via `DefaultSensitivePaths()` AllowPatterns populated at startup from config — not from policy files. This is acceptable for Phase 7 scope; policy-file-level allow for sensitive_path is a future enhancement.

   Actually, re-reading enforce.go:108-114 more carefully: the `sensitive_path` case only handles `"block"` (empty treated as block) and `"warn"` — there is no `"allow"` branch for sensitive_path overlay rules. The allowlist escape hatch described in SPATH-04 operates through `DefaultSensitivePaths().AllowPatterns` (the engine-level allowlist), not through the JSON policy overlay. This is consistent with the design.

2. `ApplyPolicyOverlay` on top of the merged decision — adds user-defined `sensitive_path` block/warn rules from JSON policy files.

**Most-restrictive-wins implementation:** `mergeDecisions` (described above) takes the highest-rank decision. The overlay's `ApplyPolicyOverlay` independently applies most-restrictive-wins against its own rules. The sequence (engine first, overlay second) means:
- Engine blocks: only a JSON policy-file `package_allowlist` allow rule (escape hatch for packages, not paths) can override it via ApplyPolicyOverlay step 2
- Engine allows: JSON policy-file `sensitive_path` block rule can escalate to block

**SPATH-04 allowlist additions to `DefaultSensitivePaths()`:**
```go
AllowPatterns: []string{
    ".env.example",
    ".env.test",
    ".env.schema",
},
```
These are basename patterns (no `/`). `isAllowedPath` at `path.go:141-152` does exact match for basename-only patterns. `.env.example` will match `lastSegment("/home/u/project/.env.example") == ".env.example"`.

Wait — `isAllowedPath` checks `resolvedPath == allow` (exact full-path match) or prefix-with-separator match. It does NOT do basename matching. But the allowlist is checked against the full `resolvedPath`, and `.env.example` (a basename) will only exactly-match if `resolvedPath == ".env.example"` — which will never happen for an absolute path.

**This is a design gap.** The `isAllowedPath` function (`path.go:141-152`) does path-prefix matching, not basename matching. To allow `.env.example` via `AllowPatterns`, the pattern must use the same basename matching that `matchesBlockPattern` uses — but `isAllowedPath` uses a different matching logic than `matchesBlockPattern`.

**Recommended fix:** Extend `isAllowedPath` OR add a separate `isAllowedBasename` helper that checks the last segment, mirroring `matchesBlockPattern`'s basename-pattern logic. The allowlist check should support basename patterns (without `/`) using the same `lastSegment` logic:

```go
// Extended isAllowedPath logic for basename patterns:
// If allow pattern contains no "/" or "\", match against lastSegment.
// This parallels matchesBlockPattern's basename logic.
func isAllowedPath(resolvedPath, allow string) bool {
    // Existing prefix/exact logic for path patterns:
    if strings.Contains(allow, "/") || strings.Contains(allow, "\\") {
        if resolvedPath == allow { return true }
        if strings.HasSuffix(allow, "/") || strings.HasSuffix(allow, "\\") {
            return strings.HasPrefix(resolvedPath, allow)
        }
        return strings.HasPrefix(resolvedPath, allow+"/") ||
            strings.HasPrefix(resolvedPath, allow+"\\")
    }
    // Basename pattern: match against last segment (mirrors matchesBlockPattern).
    seg := lastSegment(resolvedPath)
    if strings.HasSuffix(allow, ".*") {
        prefix := allow[:len(allow)-1]
        return strings.HasPrefix(seg, prefix)
    }
    return seg == allow
}
```

This fix stays inside `internal/policy/path.go` and remains pure (no I/O). `TestEvaluatePathAllowlistOverride` in `path_test.go:145-157` tests the existing behavior; new tests must cover the basename allowlist case.

**Policy-file merge for most-restrictive-wins:** When a user adds a `sensitive_path` rule with `action:"block"` for a custom path in their `~/.beekeeper/policies/my.json`, `ApplyPolicyOverlay` picks it up via `matchesSensitivePath` (enforce.go:101-128). The merge is: if JSON policy says block, block wins over the engine's allow. This already works without any Phase 7 changes.

### Q6: Test Strategy

**`RunCheck` integration test shape** (from Phase 6 plan 06-03, `handler_test.go:659-817`):

```go
// SPATH-01/02: credential file path block
func TestRunCheckCredentialFileBlocks(t *testing.T) {
    dir := t.TempDir()
    idxPath := buildTestIndex(t, dir) // uses existing buildTestIndex helper
    cacheDir := filepath.Join(dir, "catalogs")
    os.MkdirAll(cacheDir, 0700)

    stdin := strings.NewReader(`{"agent_name":"test-agent","tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}`)
    res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t), cacheDir)

    if res.Decision.Level != "block" { t.Errorf(...) }
    if res.ExitCode != exitBlock { t.Errorf(...) }
    if res.Decision.Allow { t.Error(...) }

    // Assert audit record has decision:"block" (SPATH success criterion SC4)
    rec := readLastAuditRecord(t, auditPath)
    if rec.Decision != "block" { t.Errorf(...) }
    if !containsRuleID(rec.RuleIDs, "sensitive-path-policy") { t.Errorf(...) }
}
```

**Existing helpers available in `handler_test.go`:**
- `buildTestIndex(t, dir) string` — creates a real mmap index (line 22)
- `closedConfig() config.Config` — returns `config.Config{FailMode: config.FailModeClosed}` (line 41)
- `auditPathIn(t) string` — returns `t.TempDir()/audit/beekeeper.ndjson` (line 43)

**Existing helpers available in `integration_test.go`:**
- `readLastAuditRecord(t, auditPath) audit.AuditRecord` — reads + parses the last NDJSON line (line 76-99)
- `mapMultiIndex` — fake multi-source catalog for hermetic tests (line 20-30)

**`runCheckWithIndex` in `integration_test.go:36-73`** can be used to test path evaluation without any catalog; the path block should fire even when the catalog index returns no matches (credential-file block is independent of catalog matching).

**Required tests for SPATH-01–04:**

| Test | Location | Input stdin | Expected outcome |
|------|----------|-------------|-----------------|
| `TestRunCheckCredentialFileBlocks` | `handler_test.go` | `Read` + `file_path:"~/.aws/credentials"` | exit 1, decision block, `rule_ids` contains `sensitive-path-policy` |
| `TestRunCheckTraversalBlocks` | `handler_test.go` | `Read` + `file_path:"../../.aws/credentials"` | exit 1, decision block |
| `TestRunCheckWindowsCredentialBlocks` | `handler_test.go` (Windows guard or cross-platform) | `Read` + `file_path:"C:\\Users\\u\\.aws\\credentials"` | exit 1 |
| `TestRunCheckBashCatCredentialBlocks` | `handler_test.go` | `Bash` + `command:"cat ~/.ssh/id_rsa"` | exit 1, decision block (SPATH-03) |
| `TestRunCheckEnvExampleAllowed` | `handler_test.go` | `Read` + `file_path:"/home/u/project/.env.example"` | exit 0, decision allow (SPATH-04) |
| `TestRunCheckEnvTestAllowed` | `handler_test.go` | `Read` + `file_path:"/home/u/project/.env.test"` | exit 0, decision allow |
| `TestRunCheckEnvSchemaAllowed` | `handler_test.go` | `Read` + `file_path:"/home/u/project/.env.schema"` | exit 0, decision allow |
| `TestRunCheckEnvProductionBlocked` | `handler_test.go` | `Read` + `file_path:"/home/u/project/.env.production"` | exit 1 (`.env.*` glob) |

**Pure unit tests (in `internal/policy/path_test.go`):**
- Basename allowlist matching: `EvaluatePath("/project/.env.example", cfg)` → allow (add after fixing `isAllowedPath`)
- Traversal: `EvaluatePath("/home/u/../../root/.ssh/id_rsa", ...)` is not needed here because `filepath.Abs` canonicalization happens in the caller, not in the pure engine. But test that `EvaluatePath` blocks `/.ssh/` even in unusual path shapes after Abs.

**Pure unit tests (in `internal/check/paths_test.go` — new file):**
- `extractPathTargets` reads `"file_path"` key
- `extractPathTargets` reads `"path"` key (fallback compat)
- `extractPathTargets` returns nil for non-file tools
- `canonicalizePath` expands `~` to absolute path
- `canonicalizePath` resolves `../../.aws` via `filepath.Abs`
- `extractBashCredentialPaths` extracts path from `cat ~/.aws/credentials`
- `extractBashCredentialPaths` extracts path from `Get-Content ~/.npmrc`

### Q7: Validation Architecture (Nyquist)

See Section: Validation Architecture below.

---

## Common Pitfalls

### Pitfall 1: `.env.example` False Positive (from `.planning/research/SUMMARY.md:76`)

**What goes wrong:** `DefaultSensitivePaths()` has `AllowPatterns: nil`. The `.env.*` glob in BlockPatterns matches `.env.example`, `.env.test`, `.env.schema`. After SPATH wiring goes live, reading any of these files blocks — a regression on every project that reads its own `.env` documentation.

**Why it happens:** The engine was built for correctness (blocking all `.env.*` variants) before the allowlist was populated.

**How to avoid:** Add `.env.example`, `.env.test`, `.env.schema` to `DefaultSensitivePaths().AllowPatterns` AND fix `isAllowedPath` to handle basename patterns. Do this in the FIRST task, before any integration wiring — so the unit tests start green.

**Warning signs:** `TestEvaluatePath` passing while a new test for `.env.example` fails.

### Pitfall 2: `isAllowedPath` Does Not Do Basename Matching

**What goes wrong:** Adding `.env.example` to `AllowPatterns` has no effect because `isAllowedPath` only does full-path or path-prefix matching, not basename matching. Tests appear to pass (the unit test may not be written yet), but `.env.example` is still blocked at runtime.

**Why it happens:** `isAllowedPath` (`path.go:141-152`) predates the allowlist need for basename patterns.

**How to avoid:** Extend `isAllowedPath` to handle patterns without path separators using the same `lastSegment` logic as `matchesBlockPattern`. Add `TestEvaluatePathBasenameAllowlist` to `path_test.go`.

### Pitfall 3: `EvalSymlinks` Errors on Non-Existent Paths

**What goes wrong:** `~/.aws/credentials` is the most important path to block, but many users have never configured AWS. `filepath.EvalSymlinks("~/.aws/credentials")` returns an error (file not found). The naive implementation returns `""` or the raw path, causing `EvaluatePath` to receive a non-credential string and allow the read.

**Why it happens:** `EvalSymlinks` is documented to require the file to exist.

**How to avoid:** Fall back to `filepath.Abs` result (without symlink resolution) when `EvalSymlinks` errors. `filepath.Abs` does not require the file to exist. The `/.aws/` fragment still appears in the Abs-resolved path — `EvaluatePath` still blocks it.

**Warning signs:** `TestRunCheckCredentialFileBlocks` fails on a machine where `~/.aws/credentials` does not exist.

### Pitfall 4: `extractTargetPath` in enforce.go Only Reads `"path"` Key

**What goes wrong:** The `ApplyPolicyOverlay` sensitive_path evaluation (`enforce.go:101-128`) calls `extractTargetPath` which only returns `tc.ToolInput["path"]`. Claude Code Read/Write/Edit tools use `"file_path"`. Policy-file sensitive_path overlay rules never fire for these tools.

**Why it happens:** Historical gap — the overlay was written before `file_path` was confirmed as the Claude Code key.

**How to avoid:** Fix `extractTargetPath` to try `"file_path"` first, fall back to `"path"`, return `""` if neither. This is a 3-line fix in `enforce.go`.

### Pitfall 5: Testing Only the Pure Function, Not the Wired Pipeline

**What goes wrong:** All new tests pass at the `policy.EvaluatePath` level. But `RunCheck` still returns exit 0 for `~/.aws/credentials` because the handler never calls `EvaluatePath`. This was the exact failure mode for F2 in the live validation run.

**Why it happens:** Unit tests mock the engine boundary, making the wiring gap invisible.

**How to avoid:** SPATH-04 success criterion SC4 explicitly requires `RunCheck` integration tests that drive the full stdin-to-exit-code path. These must be the primary success signal, not unit tests of `EvaluatePath` in isolation.

**Warning signs:** Unit tests green, but manual `echo '{"tool_name":"Read","tool_input":{"file_path":"~/.aws/credentials"}}' | beekeeper check` returns exit 0.

### Pitfall 6: `%USERPROFILE%` in Bash `type` Command Not Expanded

**What goes wrong:** The ROADMAP success criterion SC2 includes `type %USERPROFILE%\.ssh\id_rsa`. The `%USERPROFILE%` env-var expansion is NOT handled by `canonicalizePath` (only tilde and `filepath.Abs` are performed). The path token `%USERPROFILE%\.ssh\id_rsa` does not contain `\.ssh\` after `filepath.ToSlash` because `%USERPROFILE%` is not expanded.

**Why it happens:** Env-var expansion is complex (which vars, which OS, etc.) and introduces a new attack surface.

**How to avoid:** Document this as a Phase 7 known limitation. The success criterion for SC2 can be satisfied by `cat ~/.ssh/id_rsa` (tilde form, which is fully in scope). `type %USERPROFILE%` detection is explicitly deferred. Add a comment in `extractBashCredentialPaths` noting the limitation.

---

## Code Examples

### Example 1: EvaluatePath call in handler.go (insertion point)

```go
// internal/check/handler.go — after line 257 (decision := policy.Evaluate(...))
// Source: internal/policy/path.go:76 (EvaluatePath signature)
//         internal/check/paths.go (extractPathTargets — new file)

// SPATH-01/02/03: evaluate sensitive-path policy for file-path and Bash targets.
spathCfg := policy.DefaultSensitivePaths()
for _, rawPath := range extractPathTargets(toolCall) {
    resolved := canonicalizePath(rawPath)
    if resolved == "" {
        continue
    }
    pathDecision := policy.EvaluatePath(resolved, spathCfg)
    decision = mergeDecisions(decision, pathDecision)
}
```

### Example 2: extractPathTargets (new in paths.go)

```go
// internal/check/paths.go
// Source: tool-call input key analysis (.planning/research/ARCHITECTURE.md:109-116)
func extractPathTargets(tc policy.ToolCall) []string {
    if tc.ToolInput == nil {
        return nil
    }
    var paths []string

    // file_path: Read/Write/Edit/MultiEdit tools (Claude Code)
    if p, ok := tc.ToolInput["file_path"].(string); ok && p != "" {
        paths = append(paths, p)
    }

    // path: legacy key used by policyloader overlay; keep for compat
    if p, ok := tc.ToolInput["path"].(string); ok && p != "" {
        paths = append(paths, p)
    }

    // Bash command: scan for read-verb + credential-path patterns
    if tc.ToolName == "Bash" {
        if cmd, ok := tc.ToolInput["command"].(string); ok && cmd != "" {
            paths = append(paths, extractBashCredentialPaths(cmd)...)
        }
    }

    return paths
}
```

### Example 3: canonicalizePath (new in paths.go)

```go
// internal/check/paths.go
// Source: internal/watch/watcher.go:121-132 (expandHome pattern)
//         filepath.Abs + EvalSymlinks stdlib docs
func canonicalizePath(raw string) string {
    p := expandHome(raw) // tilde expansion
    abs, err := filepath.Abs(p) // resolves .., relative paths
    if err != nil {
        abs = p
    }
    resolved, err := filepath.EvalSymlinks(abs) // resolves symlinks
    if err != nil {
        resolved = abs // IMPORTANT: fall back to Abs result, not raw
    }
    return filepath.ToSlash(resolved) // normalize to forward slashes
}
```

### Example 4: Fix to extractTargetPath in enforce.go

```go
// internal/policyloader/enforce.go — replace lines 279-287
// Source: .planning/research/ARCHITECTURE.md:107-116 (confirmed gap)
func extractTargetPath(tc policy.ToolCall) string {
    if tc.ToolInput == nil {
        return ""
    }
    // file_path: primary key for Read/Write/Edit/MultiEdit (Claude Code)
    if p, ok := tc.ToolInput["file_path"].(string); ok && p != "" {
        return p
    }
    // path: legacy key; keep for backward compatibility
    if p, ok := tc.ToolInput["path"].(string); ok {
        return p
    }
    return ""
}
```

### Example 5: DefaultSensitivePaths allowlist additions

```go
// internal/policy/path.go — modify DefaultSensitivePaths()
// Add AllowPatterns for safe .env lookalikes (SPATH-04)
func DefaultSensitivePaths() SensitivePathConfig {
    return SensitivePathConfig{
        BlockPatterns: []string{
            "/.ssh/", "/.aws/", "/.gnupg/",
            "/.config/Claude/", "/.config/op/", "/.config/gh/",
            "/.cursor/",         // Cursor MCP config (gap, Phase 7 addition)
            "/.windsurf/",       // Windsurf MCP config (gap, Phase 7 addition)
            "/.netrc", "/.npmrc", "/.pypirc",
            "/.cargo/credentials.toml",
            "/.cargo/credentials", // bare (pre-2022 format, Phase 7 addition)
            ".env", ".env.local", ".env.*",
        },
        AllowPatterns: []string{
            ".env.example", // safe lookalike — not a secrets file
            ".env.test",    // safe lookalike
            ".env.schema",  // safe lookalike
        },
    }
}
```

Note: the `isAllowedPath` function must be extended to match basename patterns (no `/`) against `lastSegment(resolvedPath)`, mirroring `matchesBlockPattern`. This is a prerequisite for the AllowPatterns additions to have any effect.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No path evaluation in RunCheck | `EvaluatePath` wired into runCheck pipeline | Phase 7 (this phase) | `~/.aws/credentials` now blocks |
| `extractTargetPath` reads only `"path"` key | Reads `"file_path"` (primary) + `"path"` (fallback) | Phase 7 fix | Policy-file overlay fires for Read/Write/Edit tools |
| `AllowPatterns: nil` | `.env.example`/`.env.test`/`.env.schema` in AllowPatterns | Phase 7 addition | No false positive on fixture `.env` files |
| `isAllowedPath` only prefix-matches | Extended to support basename patterns | Phase 7 fix | AllowPatterns actually work for basename-only patterns |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Claude Code Read/Write/Edit tools emit `"file_path"` as the ToolInput key; agents don't vary this | Q2 Wiring Point, Q6 Tests | If some agents use `"path"`, the fallback in `extractPathTargets` catches it. Low risk. |
| A2 | `%USERPROFILE%` in Windows Bash `type` commands is not in-scope for SPATH-03 Phase 7 | Q4 Shell Extraction | If the success criterion SC2 requires `%USERPROFILE%` detection to pass, test will fail. Mitigated by documenting the limitation and satisfying SC2 via tilde form. |
| A3 | Cursor and Windsurf MCP config files are at `~/.cursor/` and `~/.windsurf/` respectively | Q1 Engine Surface | Confirmed in Phase 4 v1.1.0 work (WEXT-03). LOW confidence for exact paths on all platforms — but the fragment match means any path under `/.cursor/` is caught. |

**Claims tagged `[ASSUMED]`:** None beyond the above. All code citations are `[VERIFIED]` from direct file inspection.

---

## Open Questions

1. **SPATH-03 `%USERPROFILE%` expansion scope**
   - What we know: `%USERPROFILE%\.ssh\id_rsa` in a `type` command does not resolve via `filepath.Abs` alone
   - What's unclear: Is the ROADMAP SC2 assertion `type %USERPROFILE%\.ssh\id_rsa` a must-pass test or a documented limitation?
   - Recommendation: Satisfy SC2 with `cat ~/.ssh/id_rsa` (fully covered). Document `%USERPROFILE%` as deferred in the Phase 7 plan. Check with the user if SC2 literally requires the `type %USERPROFILE%` form.

2. **`.cursor/` and `.windsurf/` in DefaultSensitivePaths**
   - What we know: Phase 4 confirmed these dirs contain MCP config; `path.go:22` comment says "MCP config files enumerated by Bumblebee are appended to BlockPatterns at Plan 08 time"
   - What's unclear: Should Phase 7 add `/.cursor/` and `/.windsurf/` to BlockPatterns now (getting ahead of the plan 08 comment), or follow the comment and defer to Phase 8?
   - Recommendation: Add them now since they are confirmed MCP config dirs and Phase 7 is the wiring phase. The plan 08 comment referred to the old v1.0.0 planning; v1.2.0 Phase 7 is the appropriate home.

3. **Gateway/watch/scan consumers for SPATH**
   - What we know: Phase 6 wired CORR-02 into all four consumers; SPATH REQUIREMENTS.md maps only to Phase 7 (check only)
   - What's unclear: Does the planner expect SPATH wiring in gateway, watch, scan for Phase 7, or is check-only correct?
   - Recommendation: Check-only per REQUIREMENTS.md traceability. Gateway SPATH wiring should be a Phase 8+ item when nudge wiring also hits the gateway.

---

## Environment Availability

Step 2.6: SKIPPED — this phase has no external dependencies. All work is internal Go code changes and stdlib usage. No new CLI tools, databases, services, or package managers required.

---

## Validation Architecture

`workflow.nyquist_validation: true` in `.planning/config.json`.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing (`go test`) |
| Config file | None — standard Go test discovery |
| Quick run command | `go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -run TestEvaluatePath\|TestRunCheckCredential\|TestRunCheckTraversal\|TestRunCheckBash\|TestRunCheckEnv -count=1` |
| Full suite command | `go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -count=1 -timeout=60s` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SPATH-01 | `RunCheck` blocks `Read` tool call with `file_path:"~/.aws/credentials"` | integration | `go test ./internal/check/... -run TestRunCheckCredentialFileBlocks -count=1` | Wave 0 |
| SPATH-01 | `EvaluatePath` blocks `/.ssh/id_rsa` path | unit | `go test ./internal/policy/... -run TestEvaluatePath/ssh_key_blocked -count=1` | EXISTS (`path_test.go`) |
| SPATH-02 | `RunCheck` blocks traversal `file_path:"../../.aws/credentials"` | integration | `go test ./internal/check/... -run TestRunCheckTraversalBlocks -count=1` | Wave 0 |
| SPATH-02 | `canonicalizePath` resolves `~` and `..` | unit | `go test ./internal/check/... -run TestCanonicalizePath -count=1` | Wave 0 |
| SPATH-03 | `RunCheck` blocks `Bash` + `cat ~/.ssh/id_rsa` | integration | `go test ./internal/check/... -run TestRunCheckBashCatCredentialBlocks -count=1` | Wave 0 |
| SPATH-03 | `extractBashCredentialPaths` extracts path from `cat` verb | unit | `go test ./internal/check/... -run TestExtractBashCredentialPaths -count=1` | Wave 0 |
| SPATH-04 | `RunCheck` allows `.env.example` | integration | `go test ./internal/check/... -run TestRunCheckEnvExampleAllowed -count=1` | Wave 0 |
| SPATH-04 | `EvaluatePath` allows `.env.example` with updated allowlist | unit | `go test ./internal/policy/... -run TestEvaluatePathBasenameAllowlist -count=1` | Wave 0 |
| SPATH-04 | `policyloader` overlay fires for `file_path` key | unit | `go test ./internal/policyloader/... -run TestExtractTargetPathFilePath -count=1` | Wave 0 |

### Observable Signals Per Success Criterion

| SC | Observable signal | How to assert |
|----|-----------------|---------------|
| SC1: `~/.aws/credentials` blocks, `../../.aws/credentials` blocks | `res.ExitCode == exitBlock && res.Decision.Level == "block"` | `RunCheck` integration test |
| SC2: `Bash` + `cat ~/.ssh/id_rsa` detected | Same as SC1 plus `rec.RuleIDs` contains `"sensitive-path-policy"` | `RunCheck` + `readLastAuditRecord` |
| SC3: `.env.example` NOT blocked | `res.ExitCode == exitAllow && res.Decision.Level == "allow"` | `RunCheck` integration test |
| SC4: exit code + `decision:"block"` audit record | `res.ExitCode == exitBlock && readLastAuditRecord(t, auditPath).Decision == "block"` | `RunCheck` + `readLastAuditRecord` |

### Sampling Rate

- **Per task commit:** `go test ./internal/policy/... ./internal/check/... ./internal/policyloader/... -count=1 -timeout=60s`
- **Per wave merge:** Full suite above
- **Phase gate:** Full suite green before `/gsd-verify-work 7`

### Wave 0 Gaps

- [ ] `internal/check/paths_test.go` — `TestExtractPathTargets`, `TestCanonicalizePath`, `TestExtractBashCredentialPaths`, `TestMergeDecisions`
- [ ] `internal/check/handler_test.go` additions: `TestRunCheckCredentialFileBlocks`, `TestRunCheckTraversalBlocks`, `TestRunCheckBashCatCredentialBlocks`, `TestRunCheckEnvExampleAllowed`, `TestRunCheckEnvTestAllowed`, `TestRunCheckEnvSchemaAllowed`, `TestRunCheckEnvProductionBlocked`
- [ ] `internal/policy/path_test.go` additions: `TestEvaluatePathBasenameAllowlist`, `TestEvaluatePathCursorMCPBlocked`, `TestEvaluatePathWindsurfMCPBlocked`
- [ ] `internal/policyloader/enforce_test.go` additions: `TestExtractTargetPathFilePath`

*(Existing `internal/policy/path_test.go`, `internal/check/handler_test.go`, `internal/check/integration_test.go` exist — additions only)*

---

## Security Domain

`security_enforcement` not explicitly set in config.json (absent = enabled).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | YES | `policy.EvaluatePath` + `DefaultSensitivePaths` — block unauthorized credential reads |
| V5 Input Validation | YES | `canonicalizePath` (tilde/Abs/EvalSymlinks/ToSlash) closes path-traversal bypass |
| V6 Cryptography | no | — |

### Known Threat Patterns for This Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Path traversal (`../../.aws/credentials`) | Spoofing/Information Disclosure | `filepath.Abs` before `EvaluatePath` (SPATH-02) |
| Tilde bypass (`~/.aws/credentials` not expanded) | Spoofing/Information Disclosure | `os.UserHomeDir()` + `filepath.Join` (expandHome) |
| Symlink dereference (symlink pointing to credential file) | Information Disclosure | `filepath.EvalSymlinks` with fallback to Abs |
| Shell command wrapping (`cat ~/.ssh/id_rsa` in Bash tool) | Information Disclosure | `extractBashCredentialPaths` + `canonicalizePath` + `EvaluatePath` (SPATH-03) |
| Windows backslash bypass (`C:\Users\u\.aws\credentials`) | Spoofing | `filepath.ToSlash` normalization; `matchesBlockPattern` already handles backslash via `normalizeSlashes` |
| `.env.example` false positive (over-blocking) | Denial of Service (availability) | AllowPatterns baseline in `DefaultSensitivePaths` (SPATH-04) |
| Policy-file allowlist injection (attacker writes a policy file to allow credential reads) | Tampering | Documented T-09-31; allowlist override is recorded in audit reason field — forensically visible |

---

## Project Constraints (from CLAUDE.md)

| Directive | Impact on Phase 7 |
|-----------|------------------|
| `internal/policy` must be a pure function library — no I/O, no goroutines, no side effects | Tilde expansion, `filepath.Abs`, `EvalSymlinks` live in `internal/check/paths.go`, NOT in `path.go`. `TestPathImportsArePure` enforces this. |
| Fail closed by default | `canonicalizePath` on error falls back to Abs (not raw); `EvalSymlinks` error falls back to Abs (not empty string). An empty or unresolvable path returns `""` which `extractPathTargets` caller skips — safe direction. |
| Windows is primary dev machine | `os.UserHomeDir()` handles Windows USERPROFILE. `filepath.ToSlash` applied after canonicalization. `extractBashCredentialPaths` handles `type ` and `Get-Content ` verbs. |
| No CGO in core | All code is pure Go stdlib. No new dependencies. |
| Single binary, `internal/` for all business logic | `paths.go` goes in `internal/check/`, not `cmd/`. |
| Hook handler loads catalog via mmap — never cold-load JSON per invocation | Unchanged — this phase does not touch the catalog loading path. |
| `internal/policy` called synchronously — one implementation, three consumers | `EvaluatePath` is already synchronous and pure. Phase 7 adds it as a new wiring point in check only; gateway/watch/scan are Phase 8+ scope. |

---

## Sources

### Primary (HIGH confidence — direct codebase inspection)

- `internal/policy/path.go` — `EvaluatePath` signature, `DefaultSensitivePaths`, `matchesBlockPattern`, `isAllowedPath`
- `internal/policy/path_test.go` — test coverage gaps, `TestPathImportsArePure` forbidden imports
- `internal/policy/types.go` — `ToolCall`, `Decision`, `SensitivePathConfig`
- `internal/check/handler.go` — `runCheck` pipeline, insertion point at line 257, `finalizeWithAC`, `runCheckWithIndex`
- `internal/check/integration_test.go` — `runCheckWithIndex`, `readLastAuditRecord`, `mapMultiIndex`
- `internal/check/handler_test.go` — `buildTestIndex`, `closedConfig`, `auditPathIn`, Phase 6 RunCheck integration tests (lines 659-817)
- `internal/check/sanity.go` — `resolveCatalogHealthy` delegation pattern (Phase 6 wiring model)
- `internal/policyloader/enforce.go` — `extractTargetPath` (reads only `"path"`, gap confirmed), `ApplyPolicyOverlay`, `matchesSensitivePath`
- `internal/policyloader/loader.go` — `PolicyRule` with `RuleType:"sensitive_path"`, `PathPatterns []string`
- `internal/policyloader/validate.go` — `legalRuleTypes` includes `"sensitive_path"`
- `internal/sentry/rules.go` — `isSensitivePath`, `defaultSensitivePaths`, `filepath.ToSlash` Windows fix (prior art)
- `internal/watch/watcher.go` — `expandHome` pattern (prior art for tilde expansion)
- `.planning/research/ARCHITECTURE.md` — Phase 7 wiring plan, component inventory, file key analysis
- `.planning/research/SUMMARY.md` — PLCY-05 plan summary, `.env.example` false-positive gap

### Secondary (MEDIUM confidence)

- `.planning/REQUIREMENTS.md` — SPATH-01–04 exact requirement text
- `.planning/ROADMAP.md` — Phase 7 goal, success criteria SC1–SC4
- `.planning/STATE.md` — Phase 6 decisions, accumulated context

---

## Metadata

**Confidence breakdown:**
- Engine surface: HIGH — read actual source files
- Wiring point: HIGH — traced runCheck line by line
- Canonicalization approach: HIGH — stdlib behavior verified, expandHome pattern from codebase
- Shell extraction approach: HIGH — prior art in sentry rules.go; conservative scope
- Allowlist gap: HIGH — `isAllowedPath` read directly, gap confirmed
- Test strategy: HIGH — existing test patterns read directly from integration_test.go and handler_test.go

**Research date:** 2026-06-03
**Valid until:** Indefinite — all findings are from the current codebase at commit eb20fd3; valid until code changes
