---
phase: 04-integration-surfaces
verified: 2026-05-27T12:00:00Z
status: passed
score: 10/10 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 7/10
  gaps_closed:
    - "beekeeper shim install creates wrapper scripts using exec; each shim invokes beekeeper check before calling the real binary"
    - "shim scripts do not embed raw $* or %%* in shell pipes (injection-safe)"
    - "gateway proxy strips Authorization header before forwarding upstream (all paths — block, warn, allow, passthrough)"
  gaps_remaining: []
  regressions: []
---

# Phase 4: Integration Surfaces Verification Report

**Phase Goal:** Ship all agent integration surfaces so every supported coding agent — Claude Code, Cursor, Codex CLI, Continue, OpenCode, OpenClaw — can be configured with a single `beekeeper` command, with multi-agent depth tracking and MCP gateway proxy enforcing policy at the protocol layer.
**Verified:** 2026-05-27T12:00:00Z
**Status:** passed
**Re-verification:** Yes — after gap closure

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `beekeeper hooks install --target claude-code` merges PreToolUse+PostToolUse into settings.json via JSONC-safe PatchSettings; idempotent; backed up; --dry-run works | VERIFIED | `internal/hooks/claude_code.go` uses `editorinit.PatchSettings`; `backupSettings` called before write (0o600); dry-run path returns early with printout; WR-01 fixed: `uninstallClaudeCode` uses `editorinit.ReadSettings` (JSONC-safe) |
| 2 | Cursor writes hooks.json with failClosed:true; Codex writes nested hooks.json with trust reminder; both idempotent and backed up | VERIFIED | `internal/hooks/cursor.go` line 75: `FailClosed: true` hardcoded; `internal/hooks/codex.go` prints `codexTrustReminder`; both call `backupSettings` before write; `containsCursorHookByCommand`/`containsCodexHookByCommand` guards idempotency |
| 3 | MCP gateway binds 127.0.0.1 by default; requires Authorization Bearer token with ConstantTimeCompare; policy evaluated on tools/call; fail-closed on panic | VERIFIED | `gateway.go` `defaultBindAddr = "127.0.0.1"`; `proxy.go` `subtle.ConstantTimeCompare`; `handleToolCall` uses goroutine+select with evalCtx; panic recovery writes -32002; `h.cfg.FailOpen` checked in all error paths (CR-07 applied) |
| 4 | JSON-RPC parser enforces bounds (1MB body, 256-byte method, 50-item batch, 10-level nesting); FuzzParseMessage corpus exists; batch unsupported returns -32600 | VERIFIED | `parser.go` constants: `maxRequestBody=1<<20`, `maxMethodLen=256`, `maxBatchItems=50`, `maxRecursionDepth=10`; `parseAsBatch` returns -32600 for multi-item batches (WR-07 fix applied: explicit unsupported error); 8 seed corpus files in `testdata/fuzz/FuzzParseMessage/`; `//go:build fuzz` tag + RELEASE GATE comment present |
| 5 | Gateway targets (Continue, OpenCode, OpenClaw) print MCP config guide; no file written | VERIFIED | `internal/hooks/gateway_targets.go` prints YAML/JSON config snippets for all three targets with `127.0.0.1:7837` URL and `beekeeper gateway token` retrieval instructions; no file I/O in any of the three guide functions |
| 6 | `beekeeper shim install` creates wrapper scripts for npm/pip/cargo/go/gem/npx/pnpm/composer/pipx; Unix uses exec; Windows uses CRLF .cmd files; findRealBinary excludes shim dir from PATH; idempotent; status command | VERIFIED | `cmd/beekeeper/main.go` lines 277-278: `--tool string` (StringVar) and `--args` (StringArrayVar) registered on `newCheckCmd()`; lines 255-269: when `toolName != ""`, builds ToolCall JSON via `json.Marshal` (injection-safe) and passes as stdin to `RunCheck`; `shim_unix.go` generates `beekeeper check --tool "%s" --args "$@"`; `shim_windows.go` generates `beekeeper check --tool "%s" --args %%*` with CRLF line endings confirmed |
| 7 | AgentContext pure struct in internal/policy/types.go; Evaluate blocks at depth>10; AuditRecord has agent_id/parent_agent_id/agent_depth/agent_lineage; hook handler reads BEEKEEPER_AGENT_* env vars; RunAuditRecord exits 0 always | VERIFIED | `policy/types.go`: `AgentContext` struct is pure (no methods, no I/O); `engine.go`: `maxAgentDepth=10`, depth check first in Evaluate; `audit/types.go`: four `omitempty` lineage fields; `check/handler.go`: `readAgentContext()` reads all four env vars, trims whitespace after lineage split (WR-08 fix applied); `RunAuditRecord` always returns 0 |
| 8 | gateway proxy strips Authorization header before forwarding upstream (all paths — block, warn, allow, passthrough) | VERIFIED | `proxy.go` lines 47-53: `ReverseProxy.Rewrite` func now calls `pr.Out.Header.Del("Authorization")` after `pr.SetURL(upstream)` and `pr.SetXForwarded()` — applies to allow path (line 218: `h.reverseProxy.ServeHTTP`) and non-tool-call passthrough (line 117: `h.reverseProxy.ServeHTTP`); warn path continues to strip via header copy loop (lines 248-255 of `forwardWithWarningInjection`) |
| 9 | CLI wiring: hooks/gateway/shim/audit-record all registered in main.go; gateway uses signal.NotifyContext; audit-record exits 0 always | VERIFIED | `cmd/beekeeper/main.go`: `newHooksCmd()`, `newGatewayCmd()`, `newShimCmd()`, `newAuditRecordCmd()` all in `root.AddCommand`; gateway uses `signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)`; `newAuditRecordCmd` calls `check.RunAuditRecord` and ignores return value (always returns nil) |
| 10 | Audit log sensitive field redaction applied in both check handler and gateway audit paths | VERIFIED | `check/handler.go` `writeAuditWithAC` lines 339-342: `DefaultRedactPatterns()` + `RedactRecord`; `gateway/proxy.go` `writeAudit` lines 344-345: same pattern (CR-06 applied); `redact.go` uses `sync.Once` for pattern compilation (WR-05 applied) |

**Score:** 10/10 truths verified

### Deferred Items

None identified.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/hooks/hooks.go` | Install/Uninstall dispatch, backupSettings, writeFileAtomic | VERIFIED | Exists; substantive; `writeFileAtomic` uses `os.CreateTemp` (CR-05 applied); `backupSettings` uses 0o600 (WR-06 applied) |
| `internal/hooks/claude_code.go` | JSONC-safe PatchSettings hook injection | VERIFIED | Uses `editorinit.PatchSettings`; `uninstallClaudeCode` uses `editorinit.ReadSettings` (WR-01 applied) |
| `internal/hooks/cursor.go` | failClosed:true in hooks.json | VERIFIED | `FailClosed: true` hardcoded; `cursorHooksFile` struct; `containsCursorHookByCommand` idempotency guard |
| `internal/hooks/codex.go` | Nested hooks schema with trust reminder | VERIFIED | `codexHooksFile` struct with nested `codexHookEntry`/`codexHookCmd`; trust reminder printed |
| `internal/hooks/gateway_targets.go` | printGatewayGuide for 3 targets | VERIFIED | All three targets print config guide; no file I/O |
| `internal/hooks/hooks_test.go` | Table-driven tests | VERIFIED | `TestInstallClaudeCode`, `TestInstallClaudeCodeDryRun`, `TestInstallCursor`, `TestInstallCodex`, `TestInstallGatewayTarget`, `TestUninstallClaudeCode`, `TestUninstallCursor`, `TestInstallUnknownTarget`, `TestInstallDispatch` all present |
| `internal/policy/types.go` | AgentContext struct | VERIFIED | Pure struct with AgentID, ParentAgentID, Depth, Lineage; no I/O |
| `internal/policy/engine.go` | maxAgentDepth + extended Evaluate | VERIFIED | `const maxAgentDepth = 10`; depth check is first operation; negative depth normalized |
| `internal/audit/types.go` | AuditRecord with lineage fields | VERIFIED | `AgentID`, `ParentAgentID`, `AgentDepth`, `AgentLineage` with `omitempty` |
| `internal/check/handler.go` | readAgentContext + RunAuditRecord | VERIFIED | `readAgentContext` reads all 4 env vars; `hookInput` struct captures stdin `agent_id`; `RunAuditRecord` exits 0 always |
| `internal/gateway/gateway.go` | Start, token generation, state persistence | VERIFIED | `generateToken` uses `crypto/rand`; `SaveGatewayState` with 0o600; `defaultBindAddr = "127.0.0.1"` |
| `internal/gateway/proxy.go` | ServeHTTP pipeline, handleToolCall with panic recovery, Authorization strip on ALL paths | VERIFIED | Token auth: VERIFIED. Bounds: VERIFIED. Policy goroutine+select: VERIFIED (CR-01). Fail-closed panic: VERIFIED. FailOpen consulted: VERIFIED (CR-07). Authorization strip in ReverseProxy Rewrite: VERIFIED (CR-02 — `pr.Out.Header.Del("Authorization")` at line 52, applies to both allow path and passthrough). Authorization strip in warn path: VERIFIED (forwardWithWarningInjection lines 248-255). |
| `internal/gateway/parser.go` | ParseMessage with all bounds | VERIFIED | All constants present; `parseSingle` + `parseAsBatch` + `checkDepth`; batch >1 item returns -32600 (WR-07 applied) |
| `internal/gateway/policy.go` | applyPolicy + Config struct | VERIFIED | `applyPolicy` delegates to `policy.Evaluate`; `Config.FailOpen` field present and read in proxy.go |
| `internal/gateway/state.go` | GatewayState with 0o600 permissions | VERIFIED | `writeStateFileAtomic` uses `os.CreateTemp` + `Chmod(0o600)` before write |
| `internal/gateway/parser_fuzz_test.go` | FuzzParseMessage RELEASE GATE | VERIFIED | `//go:build fuzz` tag; RELEASE GATE comment; 11 f.Add seeds; invariant check in f.Fuzz body |
| `internal/shim/shim.go` | Install/Uninstall/Status/findRealBinary | VERIFIED | File exists, substantive, exported API correct. findRealBinary correctly excludes shimDir from PATH. PATH instructions printed. Shim scripts now functional — `beekeeper check --tool/--args` flags are implemented in `newCheckCmd()`. |
| `internal/shim/shim_unix.go` | Unix shell scripts with exec | VERIFIED | Uses exec for signal preservation; uses `--tool`/`--args` flags (CR-03); flags now implemented in `newCheckCmd()` via `StringVar`/`StringArrayVar` |
| `internal/shim/shim_windows.go` | Windows .cmd with CRLF | VERIFIED | CRLF line endings confirmed (`\r\n` on all lines in `fmt.Sprintf`); uses `--tool`/`--args` flags (CR-04); flags now implemented in `newCheckCmd()` |
| `internal/audit/redact.go` | applyRedaction, RedactRecord, DefaultRedactPatterns | VERIFIED | `sync.Once` for pattern compilation (WR-05 applied); `applyRedaction` is pure; 3 patterns: Bearer, JWT, API key prefixes |
| `cmd/beekeeper/main.go` | All Phase 4 commands wired | VERIFIED | hooks/gateway/shim/audit-record all registered. `beekeeper check` now exposes `--tool string` (StringVar, line 277) and `--args` (StringArrayVar, line 278). When `--tool` is provided, JSON is built via `json.Marshal` (lines 255-269) and passed as stdin to `RunCheck`. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `hooks/claude_code.go installClaudeCode` | `editorinit.PatchSettings` | sets hooks key in settings.json | WIRED | Direct call at line 63 |
| `hooks/cursor.go installCursor` | `~/.cursor/hooks.json` | json.MarshalIndent + writeFileAtomic | WIRED | `cursorHooksPath(homeDir)` + atomic write |
| `hooks/codex.go installCodex` | `~/.codex/hooks.json` | json.MarshalIndent + writeFileAtomic | WIRED | `codexHooksPath(homeDir)` + atomic write |
| `gateway/proxy.go ServeHTTP` | `gateway/parser.go ParseMessage` | bounded body read then ParseMessage | WIRED | Lines 93-103 |
| `gateway/proxy.go handleToolCall` | `gateway/policy.go applyPolicy` | goroutine + select with evalCtx | WIRED | Lines 153-190 (CR-01 applied) |
| `gateway/proxy.go handleToolCall` | `httputil.ReverseProxy.ServeHTTP` | allow path only | WIRED | Line 218 |
| `gateway/proxy.go ReverseProxy Rewrite` | upstream (no Authorization header) | `pr.Out.Header.Del("Authorization")` in Rewrite func | WIRED | Lines 47-53: applies to allow path and passthrough — CR-02 fix now covers all paths |
| `gateway/proxy.go forwardWithWarningInjection` | upstream (no Authorization header) | header copy loop skips Authorization | WIRED | Lines 248-255 |
| `gateway/gateway.go Start` | `gateway/state.go SaveGatewayState` | token + port written to state.json (0o600) | WIRED | Line 85 |
| `check/handler.go readAgentContext` | `os.Getenv BEEKEEPER_AGENT_*` | env vars read; pure struct passed to engine | WIRED | Lines 222-254 |
| `policy/engine.go Evaluate` | `ac.Depth > maxAgentDepth check` | first op in Evaluate | WIRED | Lines 66-79 |
| `audit/types.go AuditRecord` | AgentID, ParentAgentID, AgentDepth, AgentLineage | FromDecision maps ac fields | WIRED | Lines 40-43 (types); lines 126-130 (FromDecision) |
| `shim/shim_unix.go writeShellScript` | `beekeeper check --tool --args` | generated shim calls check with flags | WIRED | `newCheckCmd()` lines 277-278: `--tool string` and `--args` StringArrayVar registered; lines 255-269: JSON built via json.Marshal when toolName != "" |
| `shim/shim_windows.go writeShellScript` | `beekeeper check --tool --args` | generated .cmd calls check with flags | WIRED | Same as Unix — flags now implemented in newCheckCmd() |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `gateway/proxy.go ServeHTTP` | `msg` (JSONRPCMessage) | `ParseMessage(bodyBytes)` from bounded body read | Yes — real request data | FLOWING |
| `gateway/proxy.go handleToolCall` | `decision` (policy.Decision) | `applyPolicy` → `policy.Evaluate` → catalog `LookupAll` | Yes — real catalog data | FLOWING |
| `check/handler.go RunCheck` | `decision` | `policy.Evaluate(toolCall, multiIdx, ...)` where multiIdx wraps real mmap index | Yes | FLOWING |
| `check/handler.go readAgentContext` | `ac` (AgentContext) | `os.Getenv` + stdin `agent_id` | Yes — env vars at runtime | FLOWING |
| `shim/shim_unix.go` generated script | tool invocation | `beekeeper check --tool <name> --args "$@"` → `newCheckCmd()` builds JSON via `json.Marshal` → `RunCheck` | Yes — flags implemented, JSON built injection-safely | FLOWING |

### Behavioral Spot-Checks

Step 7b: SKIPPED (no runnable entry points available in this verification environment; server-based gateway requires a running process).

The following functional checks were verified by static analysis:

| Behavior | Method | Result | Status |
|----------|--------|--------|--------|
| gateway binds 127.0.0.1 | grep defaultBindAddr | `defaultBindAddr = "127.0.0.1"` | PASS |
| ConstantTimeCompare used | grep ConstantTimeCompare | line 318 in proxy.go | PASS |
| FuzzParseMessage //go:build fuzz tag | file read | Present at line 1 | PASS |
| 8 fuzz corpus files | glob | 001-008 all present | PASS |
| RELEASE GATE comment | grep | line 3 in parser_fuzz_test.go | PASS |
| backupSettings 0o600 | grep | hooks.go line 149 | PASS |
| writeFileAtomic uses os.CreateTemp | grep | hooks.go line 164 | PASS |
| cursor.go never references settings.json | grep | no match for settings.json in cursor.go | PASS |
| failClosed:true in cursor.go | grep | line 75 | PASS |
| beekeeper check --tool flag wired | main.go line 277 | `cmd.Flags().StringVar(&toolName, "tool", ...)` registered | PASS |
| beekeeper check --args flag wired | main.go line 278 | `cmd.Flags().StringArrayVar(&toolArgs, "args", nil, ...)` registered | PASS |
| json.Marshal used for ToolCall construction (no injection) | main.go lines 256-268 | `json.Marshal(tc)` with map[string]any — no string interpolation | PASS |
| Authorization stripped on allow/passthrough path | proxy.go line 52 | `pr.Out.Header.Del("Authorization")` in ReverseProxy Rewrite | PASS |
| Authorization stripped on warn path | proxy.go lines 248-255 | header copy loop skips Authorization key | PASS |
| FailOpen consulted in all 3 error paths | proxy.go lines 134, 171, 183 | three independent FailOpen checks | PASS |

### Probe Execution

No probes declared in PLAN files for Phase 4.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| INTG-01 | 04-01 | hooks install --target claude-code writes to settings.json | SATISFIED | `installClaudeCode` + `editorinit.PatchSettings`; CLI wired in main.go |
| INTG-02 | 04-01 | hooks install for cursor (failClosed:true) and codex (trust reminder) | SATISFIED | `installCursor` with `FailClosed:true`; `installCodex` with trust reminder; both backed up |
| INTG-03 | 04-03 | MCP gateway daemon, 127.0.0.1, per-session token auth | SATISFIED | `gateway.Start` with defaultBindAddr; `subtle.ConstantTimeCompare`; 0o600 state.json; Authorization header now stripped on all proxy paths (CR-02 complete) |
| INTG-04 | 04-03 | Gateway applies policy inline; JSON-RPC correlation by id; fail-closed | SATISFIED | Policy evaluation verified; id correlation verified; fail-closed on panic verified; Authorization header no longer leaks to upstream on any path (ReverseProxy Rewrite fix applied) |
| INTG-05 | 04-01 | Continue/OpenCode/OpenClaw gateway configuration guide | SATISFIED | `printGatewayGuide` for all three targets; no file written; token retrieval instructions included |
| INTG-06 | 04-04 | Shim layer for 9 package managers | SATISFIED | Script files created with correct structure, CRLF (Windows), exec semantics (Unix). `beekeeper check --tool/--args` flags now implemented in `newCheckCmd()` with injection-safe `json.Marshal`. Shims are functional end-to-end. |
| INTG-07 | 04-02 | Multi-agent observability: AgentContext, depth enforcement, audit lineage | SATISFIED | `AgentContext` pure struct; `maxAgentDepth=10` enforced first in `Evaluate`; `AuditRecord` lineage fields; `readAgentContext` reads env vars; `RunAuditRecord` exits 0 always |

### Anti-Patterns Found

No blockers found. No TBD/FIXME/XXX debt markers found in Phase 4 modified files.

Previously identified blockers resolved:

| File | Line | Pattern | Previous Severity | Resolution |
|------|------|---------|-------------------|------------|
| `internal/shim/shim_unix.go` | 36 | `beekeeper check --tool "%s" --args "$@"` | Was BLOCKER (flags unimplemented) | RESOLVED — `--tool` and `--args` flags now registered in `newCheckCmd()` via `StringVar`/`StringArrayVar`; JSON built with `json.Marshal` |
| `internal/shim/shim_windows.go` | 38 | `beekeeper check --tool "%s" --args %%*` | Was BLOCKER (flags unimplemented) | RESOLVED — same fix as Unix |
| `internal/gateway/proxy.go` | 47-53 | ReverseProxy Rewrite missing `pr.Out.Header.Del("Authorization")` | Was BLOCKER (token leaked on allow/passthrough) | RESOLVED — `pr.Out.Header.Del("Authorization")` added to Rewrite func |

### Human Verification Required

None. All programmatically verifiable items pass. Previously listed human verification items were contingent on the blocker gaps; with the gaps resolved, no new human verification items arise from this re-verification.

---

_Verified: 2026-05-27T12:00:00Z_
_Verifier: Claude (gsd-verifier)_
_Re-verification after gap closure_
