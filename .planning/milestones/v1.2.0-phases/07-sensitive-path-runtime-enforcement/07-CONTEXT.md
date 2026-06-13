# Phase 7: Sensitive-Path Runtime Enforcement - Context

**Gathered:** 2026-06-03
**Status:** Ready for planning
**Source:** Resolved open questions from 07-RESEARCH.md (discuss-phase skipped; Research → Plan path chosen)

<domain>
## Phase Boundary

Wire the **already-built, pure** `policy.EvaluatePath` / `policy.DefaultSensitivePaths` engine
(`internal/policy/path.go`, currently referenced ONLY by its own test) into the live
`beekeeper check` pipeline (`internal/check/handler.go:runCheck`), and close the bypass vectors
(traversal, tilde, Windows env-var forms, shell-command reads, `.env` lookalike false-positives).

This is a **wiring + hardening** phase, not a greenfield feature. The decision engine exists;
the work is the impure adapter (`internal/check/paths.go`), the insertion point in `runCheck`,
a secondary `extractTargetPath` key fix in `policyloader/enforce.go`, an `isAllowedPath`
basename-matching fix in `path.go`, and `RunCheck` integration tests proving the wiring is live.

Requirements in scope: **SPATH-01, SPATH-02, SPATH-03, SPATH-04**.
</domain>

<decisions>
## Implementation Decisions

### Shell env-var expansion (SC2 scope)
- **D-01**: The impure canonicalization adapter (`internal/check/paths.go`) MUST expand Windows
  env-var path forms — at minimum `%USERPROFILE%` and `%HOMEPATH%` — before pattern matching,
  so that ROADMAP success criterion **SC2** (`type %USERPROFILE%\.ssh\id_rsa` inside a `Bash`
  tool call) is **fully** detected and blocked, not just the tilde form. Expansion is for
  **matching only** (resolving the string so `EvaluatePath` can match `/.ssh/`); the command is
  never executed. Use a targeted `%VAR%` → `os.Getenv(VAR)` replacement (NOT `os.ExpandEnv`,
  which only handles `$VAR`/`${VAR}`). On unresolved/empty env var, fail-closed: keep the raw
  token so a real credential substring still matches where possible, never silently allow.
  *(Rationale: Windows is the primary dev machine; deferring a literally-stated success criterion
  on the primary platform risks a verifier-flagged gap. Overrides 07-RESEARCH.md Q4/Pitfall-6
  "defer" recommendation by maintainer decision.)*

### MCP host-config block coverage
- **D-02**: `DefaultSensitivePaths()` adds `/.cursor/` and `/.windsurf/` to `BlockPatterns` in
  THIS phase (Claude Desktop is already covered via `/.config/Claude/`). SPATH-01 explicitly
  lists "MCP host-config files" as in-scope. The stale `internal/policy/path.go:22-23` comment
  that says MCP dirs are appended "at Plan 08 time" refers to old v1.0.0 planning and MUST be
  updated to reference v1.2.0 Phase 7. Also add `/.cargo/credentials` (bare, pre-2022 format)
  alongside the existing `/.cargo/credentials.toml`.

### Consumer scope
- **D-03**: SPATH wiring lands in **`beekeeper check` (`runCheck`) ONLY** this phase. The
  gateway, watch, and scan consumers are explicitly **out of scope** (deferred). This is per
  REQUIREMENTS.md traceability (SPATH-01–04 map to Phase 7 only) and all four success criteria
  (SC1–SC4 reference only `beekeeper check`). Do NOT replicate Phase 6's four-consumer
  `resolveCatalogHealthy` fan-out for SPATH — the pure engine is shared, but the wiring is
  check-only.

### Claude's Discretion
The following are research-recommended implementation details, NOT locked decisions — the
planner/executor may refine them, but they capture the intended shape (see 07-RESEARCH.md for
file:line citations and code excerpts):
- Canonicalization order in `paths.go`: tilde/env-var expand → `filepath.Abs` → `filepath.EvalSymlinks`
  (fall back to the `Abs` result on error, since credential files often don't exist yet) → `filepath.ToSlash`.
  All filesystem/env I/O stays in `internal/check`, never in pure `internal/policy` (`TestPathImportsArePure` enforces this).
- `extractTargetPath` in `policyloader/enforce.go` must read `"file_path"` (primary) with `"path"` fallback.
- `isAllowedPath` in `path.go` must be extended to match basename patterns (no separator) against
  the last path segment, mirroring `matchesBlockPattern`; otherwise the `AllowPatterns` additions
  (`.env.example`, `.env.test`, `.env.schema`) have no effect.
- Path/Decision merge uses a local `mergeDecisions` (block > warn > allow), inserted after
  `policy.Evaluate` and before `ApplyPolicyOverlay` in `runCheck`.
- Bash shell extraction is conservative verb-prefix matching (`cat `/`head `/`tail `/`less `/`more `/
  `type `/`Get-Content `/`gc `), not a full shell tokenizer; deeper bypasses (nested shells,
  base64, here-strings) are explicitly deferred.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 7 research + requirements
- `.planning/phases/07-sensitive-path-runtime-enforcement/07-RESEARCH.md` — full wiring map, insertion point (handler.go:257), code excerpts, pitfalls, Validation Architecture
- `.planning/REQUIREMENTS.md` — SPATH-01–04 exact requirement text + traceability
- `.planning/ROADMAP.md` — "### Phase 7: Sensitive-Path Runtime Enforcement" goal + SC1–SC4

### Engine + wiring (the files this phase changes)
- `internal/policy/path.go` — `EvaluatePath`, `DefaultSensitivePaths`, `isAllowedPath`, `matchesBlockPattern` (pure; no I/O)
- `internal/policy/path_test.go` — existing tests incl. `TestPathImportsArePure` (forbids `os`/`io`/etc. imports)
- `internal/check/handler.go` — `runCheck` pipeline; insertion point after `policy.Evaluate`, before `ApplyPolicyOverlay`; `finalizeWithAC` chokepoint
- `internal/check/handler_test.go` + `internal/check/integration_test.go` — `buildTestIndex`, `closedConfig`, `auditPathIn`, `readLastAuditRecord`, Phase 6 RunCheck test pattern (the integration-test template)
- `internal/policyloader/enforce.go` — `extractTargetPath` (`"path"`-only gap), `ApplyPolicyOverlay`, `matchesSensitivePath`

### Prior-art patterns to copy (not import)
- `internal/watch/watcher.go:121-132` — `expandHome` tilde-expansion pattern
- `internal/sentry/rules.go:53-61` — `isSensitivePath` + `filepath.ToSlash` Windows fix (Phase 5 05-01)

### Project constraints
- `./CLAUDE.md` — `internal/policy` pure-library rule; fail-closed by default; Windows primary dev machine
</canonical_refs>

<specifics>
## Specific Ideas

- New file: `internal/check/paths.go` (impure adapter) + `internal/check/paths_test.go`.
- `DefaultSensitivePaths().AllowPatterns` goes from `nil` → `[".env.example", ".env.test", ".env.schema"]`.
- Integration tests must assert BOTH `res.ExitCode == exitBlock` AND a `decision:"block"` NDJSON
  audit record (SC4) — proving live wiring, not just isolated `EvaluatePath` correctness (Pitfall 5 / F2).
- The path block must be independent of catalog matching: a credential read blocks even when the
  catalog index returns no match (`runCheckWithIndex` test path).
</specifics>

<deferred>
## Deferred Ideas

- Gateway / watch / scan SPATH wiring (D-03) — future phase.
- Deeper shell bypasses: nested shells (`zsh -c "..."`), base64-encoded commands, here-strings — out of scope for SPATH-03.
- Policy-file `sensitive_path` `action:"allow"` overlay branch (engine-level `AllowPatterns` is the Phase 7 escape hatch) — future enhancement.
</deferred>

---

*Phase: 07-sensitive-path-runtime-enforcement*
*Context gathered: 2026-06-03 — resolved open questions from research (Research → Plan path)*
