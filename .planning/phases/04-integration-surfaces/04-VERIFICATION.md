---
phase: 04-integration-surfaces
verified: 2026-05-27T00:00:00Z
status: gaps_found
score: 6/10 must-haves verified
overrides_applied: 0
gaps:
  - truth: "beekeeper shim install creates wrapper scripts using exec; each shim invokes beekeeper check before calling the real binary"
    status: failed
    reason: "Shim scripts invoke 'beekeeper check --tool <name> --args <args>' but the check command has no --tool or --args flags wired in main.go. Every shim invocation will fail with 'unknown flag: --tool', meaning shims cannot actually intercept package manager calls."
    artifacts:
      - path: "internal/shim/shim_unix.go"
        issue: "Generates 'beekeeper check --tool \"<name>\" --args \"$@\"' but beekeeper check does not accept --tool/--args flags"
      - path: "internal/shim/shim_windows.go"
        issue: "Generates 'beekeeper check --tool \"<name>\" --args %%*' but beekeeper check does not accept --tool/--args flags"
      - path: "cmd/beekeeper/main.go"
        issue: "newCheckCmd() defines no --tool or --args flags; Cobra will reject them as unknown"
    missing:
      - "Wire --tool and --args flags into newCheckCmd() and pass them to RunCheck() OR revert to JSON stdin approach with proper escaping"
  - truth: "gateway proxy strips Authorization header before forwarding upstream (all paths — block, warn, allow, passthrough)"
    status: failed
    reason: "Authorization header stripping (CR-02) is applied only in forwardWithWarningInjection (warn path). The ReverseProxy allow path and non-tool-call passthrough path use httputil.ReverseProxy.Rewrite which only calls SetURL and SetXForwarded — no Authorization header is stripped. The Beekeeper gateway token leaks to the upstream MCP server on every allowed tool call and every passthrough request."
    artifacts:
      - path: "internal/gateway/proxy.go"
        issue: "ReverseProxy Rewrite at line 47-50 does not strip Authorization; only forwardWithWarningInjection (warn path) at line 244-249 strips it"
    missing:
      - "Add 'pr.Out.Header.Del(\"Authorization\")' inside the ReverseProxy Rewrite func, OR strip the header from r before calling h.reverseProxy.ServeHTTP"
  - truth: "evalCtx timeout is actually enforced (goroutine + select)"
    status: failed
    reason: "CR-01 fix was applied — applyPolicy runs in a goroutine and a select races chFull against evalCtx.Done(). However, the goroutine-based approach wraps a different panic recovery in the goroutine, and the outer defer recover() at the top of handleToolCall can no longer catch goroutine panics. If applyPolicy panics, the inner goroutine's recover fires and sends {panicked: true} on chFull, so the panic IS handled. This is VERIFIED for CR-01. Marking as verified."
    status: verified
  - truth: "shim scripts do not embed raw $* or %%* in shell pipes (injection-safe)"
    status: failed
    reason: "CR-03/CR-04 fixes changed shims to use 'beekeeper check --tool --args' flags instead of heredoc JSON with $* or %%*. However, since beekeeper check does not implement --tool/--args flags, this creates a non-functional shim. The shell injection vector from the original implementation was fixed at the cost of breaking shim functionality entirely."
    artifacts:
      - path: "internal/shim/shim_unix.go"
        issue: "Uses --tool/--args flags that are not implemented in beekeeper check"
      - path: "internal/shim/shim_windows.go"
        issue: "Uses --tool/--args flags that are not implemented in beekeeper check"
    missing:
      - "Implement --tool and --args flags in beekeeper check to accept pre-parsed tool name and arguments, then build the JSON internally using json.Marshal"
  - truth: "writeFileAtomic in hooks uses os.CreateTemp (not fixed-name temp)"
    status: verified
    reason: "hooks.go line 164: os.CreateTemp(dir, filepath.Base(path)+\".tmp-*\") with deferred Remove — CR-05 fix applied"
  - truth: "gateway writeAudit calls RedactRecord"
    status: verified
    reason: "proxy.go lines 339-342: DefaultRedactPatterns() called and RedactRecord applied — CR-06 fix applied"
  - truth: "Config.FailOpen is read in error paths"
    status: verified
    reason: "proxy.go lines 131-132, 168-169, 180-181: all three fail paths (outer panic, goroutine panic, timeout) check h.cfg.FailOpen — CR-07 fix applied"
---

# Phase 4: Integration Surfaces Verification Report

**Phase Goal:** Ship all agent integration surfaces so every supported coding agent — Claude Code, Cursor, Codex CLI, Continue, OpenCode, OpenClaw — can be configured with a single `beekeeper` command, with multi-agent depth tracking and MCP gateway proxy enforcing policy at the protocol layer.
**Verified:** 2026-05-27T00:00:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `beekeeper hooks install --target claude-code` merges PreToolUse+PostToolUse into settings.json via JSONC-safe PatchSettings; idempotent; backed up; --dry-run works | VERIFIED | `internal/hooks/claude_code.go` uses `editorinit.PatchSettings`; `backupSettings` called before write (0o600); dry-run path returns early with printout; WR-01 fixed: `uninstallClaudeCode` uses `editorinit.ReadSettings` (JSONC-safe) |
| 2 | Cursor writes hooks.json with failClosed:true; Codex writes nested hooks.json with trust reminder; both idempotent and backed up | VERIFIED | `internal/hooks/cursor.go` line 75: `FailClosed: true` hardcoded; `internal/hooks/codex.go` prints `codexTrustReminder`; both call `backupSettings` before write; `containsCursorHookByCommand`/`containsCodexHookByCommand` guards idempotency |
| 3 | MCP gateway binds 127.0.0.1 by default; requires Authorization Bearer token with ConstantTimeCompare; policy evaluated on tools/call; fail-closed on panic | VERIFIED | `gateway.go` `defaultBindAddr = "127.0.0.1"`; `proxy.go` `subtle.ConstantTimeCompare`; `handleToolCall` uses goroutine+select with evalCtx; panic recovery writes -32002; `h.cfg.FailOpen` checked in all error paths (CR-07 applied) |
| 4 | JSON-RPC parser enforces bounds (1MB body, 256-byte method, 50-item batch, 10-level nesting); FuzzParseMessage corpus exists; batch unsupported returns -32600 | VERIFIED | `parser.go` constants: `maxRequestBody=1<<20`, `maxMethodLen=256`, `maxBatchItems=50`, `maxRecursionDepth=10`; `parseAsBatch` returns -32600 for multi-item batches (WR-07 fix applied: explicit unsupported error); 8 seed corpus files in `testdata/fuzz/FuzzParseMessage/`; `//go:build fuzz` tag + RELEASE GATE comment present |
| 5 | Gateway targets (Continue, OpenCode, OpenClaw) print MCP config guide; no file written | VERIFIED | `internal/hooks/gateway_targets.go` prints YAML/JSON config snippets for all three targets with `127.0.0.1:7837` URL and `beekeeper gateway token` retrieval instructions; no file I/O in any of the three guide functions |
| 6 | `beekeeper shim install` creates wrapper scripts for npm/pip/cargo/go/gem/npx/pnpm/composer/pipx; Unix uses exec; Windows uses CRLF .cmd files; findRealBinary excludes shim dir from PATH; idempotent; status command | FAILED | See gap below — scripts invoke `beekeeper check --tool --args` but those flags are not wired in `newCheckCmd()`. Shim files are physically created but non-functional; every invocation fails with "unknown flag: --tool" |
| 7 | AgentContext pure struct in internal/policy/types.go; Evaluate blocks at depth>10; AuditRecord has agent_id/parent_agent_id/agent_depth/agent_lineage; hook handler reads BEEKEEPER_AGENT_* env vars; RunAuditRecord exits 0 always | VERIFIED | `policy/types.go`: `AgentContext` struct is pure (no methods, no I/O); `engine.go`: `maxAgentDepth=10`, depth check first in Evaluate; `audit/types.go`: four `omitempty` lineage fields; `check/handler.go`: `readAgentContext()` reads all four env vars, trims whitespace after lineage split (WR-08 fix applied); `RunAuditRecord` always returns 0 |
| 8 | gateway proxy strips Authorization header before forwarding upstream | FAILED | CR-02 fix was applied to warn path only (`forwardWithWarningInjection` lines 244-249). Allow path (`h.reverseProxy.ServeHTTP`) and non-tool-call passthrough use `httputil.ReverseProxy` whose `Rewrite` func only calls `pr.SetURL` and `pr.SetXForwarded()`. Beekeeper gateway token leaks to upstream on every allowed tool call and passthrough request. |
| 9 | CLI wiring: hooks/gateway/shim/audit-record all registered in main.go; gateway uses signal.NotifyContext; audit-record exits 0 always | VERIFIED | `cmd/beekeeper/main.go`: `newHooksCmd()`, `newGatewayCmd()`, `newShimCmd()`, `newAuditRecordCmd()` all in `root.AddCommand`; gateway uses `signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)`; `newAuditRecordCmd` calls `check.RunAuditRecord` and ignores return value (always returns nil) |
| 10 | Audit log sensitive field redaction applied in both check handler and gateway audit paths | VERIFIED | `check/handler.go` `writeAuditWithAC` lines 339-342: `DefaultRedactPatterns()` + `RedactRecord`; `gateway/proxy.go` `writeAudit` lines 339-342: same pattern (CR-06 applied); `redact.go` uses `sync.Once` for pattern compilation (WR-05 applied) |

**Score:** 7/10 truths verified (2 blockers: INTG-06 shim flag wiring broken; gateway Authorization header leaks on allow/passthrough paths)

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
| `internal/gateway/proxy.go` | ServeHTTP pipeline, handleToolCall with panic recovery | PARTIAL | Token auth: VERIFIED. Bounds: VERIFIED. Policy goroutine+select: VERIFIED (CR-01 applied). Fail-closed panic: VERIFIED. FailOpen consulted: VERIFIED (CR-07 applied). Authorization strip on warn path: VERIFIED (CR-02). **Authorization strip on allow/passthrough path: MISSING** — ReverseProxy Rewrite does not strip the header |
| `internal/gateway/parser.go` | ParseMessage with all bounds | VERIFIED | All constants present; `parseSingle` + `parseAsBatch` + `checkDepth`; batch >1 item returns -32600 (WR-07 applied) |
| `internal/gateway/policy.go` | applyPolicy + Config struct | VERIFIED | `applyPolicy` delegates to `policy.Evaluate`; `Config.FailOpen` field present and read in proxy.go |
| `internal/gateway/state.go` | GatewayState with 0o600 permissions | VERIFIED | `writeStateFileAtomic` uses `os.CreateTemp` + `Chmod(0o600)` before write |
| `internal/gateway/parser_fuzz_test.go` | FuzzParseMessage RELEASE GATE | VERIFIED | `//go:build fuzz` tag; RELEASE GATE comment; 11 f.Add seeds; invariant check in f.Fuzz body |
| `internal/shim/shim.go` | Install/Uninstall/Status/findRealBinary | PARTIAL | File exists, substantive, exported API correct. findRealBinary correctly excludes shimDir from PATH. PATH instructions printed. **Shims non-functional** because they invoke `beekeeper check --tool --args` which are unimplemented flags. |
| `internal/shim/shim_unix.go` | Unix shell scripts with exec | PARTIAL | Uses exec for signal preservation; uses `--tool`/`--args` flags. CR-03 injection fix applied but creates a broken shim since flags are not wired. |
| `internal/shim/shim_windows.go` | Windows .cmd with CRLF | PARTIAL | CRLF line endings confirmed; CR-04 injection fix applied; same broken-flags issue. |
| `internal/audit/redact.go` | applyRedaction, RedactRecord, DefaultRedactPatterns | VERIFIED | `sync.Once` for pattern compilation (WR-05 applied); `applyRedaction` is pure; 3 patterns: Bearer, JWT, API key prefixes |
| `cmd/beekeeper/main.go` | All Phase 4 commands wired | PARTIAL | hooks/gateway/shim/audit-record all registered. `beekeeper check` does NOT expose `--tool`/`--args` flags needed by shims. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `hooks/claude_code.go installClaudeCode` | `editorinit.PatchSettings` | sets hooks key in settings.json | WIRED | Direct call at line 63 |
| `hooks/cursor.go installCursor` | `~/.cursor/hooks.json` | json.MarshalIndent + writeFileAtomic | WIRED | `cursorHooksPath(homeDir)` + atomic write |
| `hooks/codex.go installCodex` | `~/.codex/hooks.json` | json.MarshalIndent + writeFileAtomic | WIRED | `codexHooksPath(homeDir)` + atomic write |
| `gateway/proxy.go ServeHTTP` | `gateway/parser.go ParseMessage` | bounded body read then ParseMessage | WIRED | Lines 93-103 |
| `gateway/proxy.go handleToolCall` | `gateway/policy.go applyPolicy` | goroutine + select with evalCtx | WIRED | Lines 153-190 (CR-01 applied) |
| `gateway/proxy.go handleToolCall` | `httputil.ReverseProxy.ServeHTTP` | allow path only | WIRED | Line 215 |
| `gateway/proxy.go forwardWithWarningInjection` | upstream (no Authorization header) | Authorization stripped for warn path | WIRED | Lines 244-249 |
| `gateway/proxy.go ServeHTTP (allow/passthrough)` | upstream (Authorization header NOT stripped) | ReverseProxy Rewrite has no Del(Authorization) | NOT_WIRED | Lines 47-50: Rewrite only calls SetURL and SetXForwarded |
| `gateway/gateway.go Start` | `gateway/state.go SaveGatewayState` | token + port written to state.json (0o600) | WIRED | Line 85 |
| `check/handler.go readAgentContext` | `os.Getenv BEEKEEPER_AGENT_*` | env vars read; pure struct passed to engine | WIRED | Lines 222-254 |
| `policy/engine.go Evaluate` | `ac.Depth > maxAgentDepth check` | first op in Evaluate | WIRED | Lines 66-79 |
| `audit/types.go AuditRecord` | AgentID, ParentAgentID, AgentDepth, AgentLineage | FromDecision maps ac fields | WIRED | Lines 40-43 (types); lines 126-130 (FromDecision) |
| `shim/shim_unix.go writeShellScript` | `beekeeper check --tool --args` | generated shim calls check with flags | NOT_WIRED | `beekeeper check` has no `--tool`/`--args` flags; shims will fail at runtime |
| `shim/shim_windows.go writeShellScript` | `beekeeper check --tool --args` | generated .cmd calls check with flags | NOT_WIRED | Same as Unix — flags unimplemented |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|-------------------|--------|
| `gateway/proxy.go ServeHTTP` | `msg` (JSONRPCMessage) | `ParseMessage(bodyBytes)` from bounded body read | Yes — real request data | FLOWING |
| `gateway/proxy.go handleToolCall` | `decision` (policy.Decision) | `applyPolicy` → `policy.Evaluate` → catalog `LookupAll` | Yes — real catalog data | FLOWING |
| `check/handler.go RunCheck` | `decision` | `policy.Evaluate(toolCall, multiIdx, ...)` where multiIdx wraps real mmap index | Yes | FLOWING |
| `check/handler.go readAgentContext` | `ac` (AgentContext) | `os.Getenv` + stdin `agent_id` | Yes — env vars at runtime | FLOWING |
| `shim/shim_unix.go` generated script | tool invocation | `beekeeper check --tool ... --args ...` | No — flags not implemented | DISCONNECTED |

### Behavioral Spot-Checks

Step 7b: SKIPPED (no runnable entry points available in this verification environment; server-based gateway requires a running process).

The following functional checks were verified by static analysis:

| Behavior | Method | Result | Status |
|----------|--------|--------|--------|
| gateway binds 127.0.0.1 | grep defaultBindAddr | `defaultBindAddr = "127.0.0.1"` | PASS |
| ConstantTimeCompare used | grep ConstantTimeCompare | line 315 in proxy.go | PASS |
| FuzzParseMessage //go:build fuzz tag | file read | Present at line 1 | PASS |
| 8 fuzz corpus files | glob | 001-008 all present | PASS |
| RELEASE GATE comment | grep | line 3 in parser_fuzz_test.go | PASS |
| backupSettings 0o600 | grep | hooks.go line 149 | PASS |
| writeFileAtomic uses os.CreateTemp | grep | hooks.go line 164 | PASS |
| cursor.go never references settings.json | grep | no match for settings.json in cursor.go | PASS |
| failClosed:true in cursor.go | grep | line 75 | PASS |
| beekeeper check --tool flag wired | grep main.go | No --tool flag in newCheckCmd | FAIL |
| Authorization stripped on allow path | grep ReverseProxy Rewrite | No Del(Authorization) in Rewrite | FAIL |

### Probe Execution

No probes declared in PLAN files for Phase 4.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| INTG-01 | 04-01 | hooks install --target claude-code writes to settings.json | SATISFIED | `installClaudeCode` + `editorinit.PatchSettings`; CLI wired in main.go |
| INTG-02 | 04-01 | hooks install for cursor (failClosed:true) and codex (trust reminder) | SATISFIED | `installCursor` with `FailClosed:true`; `installCodex` with trust reminder; both backed up |
| INTG-03 | 04-03 | MCP gateway daemon, 127.0.0.1, per-session token auth | SATISFIED | `gateway.Start` with defaultBindAddr; `subtle.ConstantTimeCompare`; 0o600 state.json |
| INTG-04 | 04-03 | Gateway applies policy inline; JSON-RPC correlation by id; fail-closed | PARTIAL | Policy evaluation verified; id correlation verified; fail-closed on panic verified; **Authorization header leaks to upstream on allow path** (security gap) |
| INTG-05 | 04-01 | Continue/OpenCode/OpenClaw gateway configuration guide | SATISFIED | `printGatewayGuide` for all three targets; no file written; token retrieval instructions included |
| INTG-06 | 04-04 | Shim layer for 9 package managers | BLOCKED | Script files created with correct structure, CRLF, exec semantics. **Fatal defect: `beekeeper check --tool --args` flags not implemented.** Every shim invocation fails. |
| INTG-07 | 04-02 | Multi-agent observability: AgentContext, depth enforcement, audit lineage | SATISFIED | `AgentContext` pure struct; `maxAgentDepth=10` enforced first in `Evaluate`; `AuditRecord` lineage fields; `readAgentContext` reads env vars; `RunAuditRecord` exits 0 always |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/shim/shim_unix.go` | 36 | `beekeeper check --tool "%s" --args "$@"` — flags not wired | BLOCKER | Every Unix shim invocation fails with "unknown flag: --tool"; INTG-06 non-functional |
| `internal/shim/shim_windows.go` | 38 | `beekeeper check --tool "%s" --args %%*` — flags not wired | BLOCKER | Every Windows shim invocation fails; INTG-06 non-functional |
| `internal/gateway/proxy.go` | 47-50 | ReverseProxy Rewrite missing `pr.Out.Header.Del("Authorization")` | BLOCKER | Gateway token forwarded to upstream on every allowed tool call and non-tool-call passthrough (partial CR-02 fix — warn path only) |

No TBD/FIXME/XXX debt markers found in Phase 4 modified files.

### Human Verification Required

#### 1. Shim runtime behavior on a live system

**Test:** Run `beekeeper shim install`, then invoke `npm install some-package` (assuming npm is in PATH).
**Expected:** `beekeeper check` is invoked first, then npm runs if allowed.
**Why human:** Requires a live system with npm, a real shim dir in PATH, and actually running the shim. The static analysis shows the shim will fail because `--tool`/`--args` flags are not wired — this gap is already classified as FAILED and documented above. Human test would confirm the exact failure message.

#### 2. Gateway Authorization header behavior on allow path

**Test:** Start `beekeeper gateway --upstream http://localhost:3000`; send a valid tools/call request that results in an `allow` decision; inspect the request received by the upstream MCP server.
**Expected:** The upstream should NOT receive the `Authorization: Bearer <gateway-token>` header.
**Why human:** Requires a running gateway, a mock upstream that logs headers, and an allow-path tool call. The static analysis confirms the gap (ReverseProxy Rewrite does not strip the header) but runtime confirmation shows the token value actually reaching the upstream.

### Gaps Summary

Two blockers prevent full Phase 4 goal achievement:

**Blocker 1 — INTG-06 shim scripts invoke unimplemented flags (critical)**

The post-review fix for CR-03/CR-04 (shell injection via `$*`/`%%*`) replaced the heredoc JSON approach with `beekeeper check --tool <name> --args <args>`. This is a sound architectural fix, but `beekeeper check` was never updated to accept `--tool` or `--args` flags. The shim scripts are created on disk (the files exist with correct structure, CRLF, exec usage, findRealBinary PATH exclusion) but invoke a CLI interface that does not exist. Every invocation of a shimmed binary will fail with an error from Cobra ("unknown flag: --tool"), producing a non-zero exit that fails closed — meaning all package manager calls are silently blocked. The complete INTG-06 requirement (shim layer) is blocked.

**Fix required:** Either (a) add `--tool string` and `--args ...string` flags to `newCheckCmd()` and have `RunCheck` accept them as an alternative to JSON stdin; or (b) revert the shim template to use JSON stdin and apply proper shell escaping (using printf + `json.Marshal`-safe encoding).

**Blocker 2 — Gateway token forwarded to upstream on allow path (security)**

The CR-02 fix strips the `Authorization` header from the warn path (manual HTTP client in `forwardWithWarningInjection`). However, the allow path and non-tool-call passthrough path use `httputil.ReverseProxy` whose `Rewrite` function only sets the upstream URL and X-Forwarded headers. No `Authorization` header deletion is performed. The Beekeeper gateway token — a 256-bit secret generated per session — is forwarded to the upstream MCP server on every allowed tool call. This contradicts INTG-03's per-session token auth model and can expose the token in upstream server logs.

**Fix required:** In the `ReverseProxy` Rewrite function, add `pr.Out.Header.Del("Authorization")` after `pr.SetURL(upstream)`.

---

_Verified: 2026-05-27T00:00:00Z_
_Verifier: Claude (gsd-verifier)_
