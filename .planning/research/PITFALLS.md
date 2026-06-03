# Pitfalls Research

**Domain:** Agent runtime safety harness — v1.2.0 "Runtime Behavioral Hardening" (PLCY-05 sensitive-path wiring, NUDGE package-manager nudge, PLCY-07 corroboration hardening, BTEST behavioral tests)
**Researched:** 2026-06-03
**Confidence:** HIGH — pitfalls derived from live codebase inspection (handler.go, policy/path.go, policy/corroboration.go, catalog/verify.go, catalog/sanity.go), the NUDGE PRD spec (§11 edge cases, §12 self-defense), PROJECT.md milestone findings (F1/F2/F3 runtime-validation gaps), and CLAUDE.md constraints.

---

## Critical Pitfalls

### Pitfall 1: NUDGE Detection on the One-Shot `beekeeper check` Hot Path — The 60-Second Cache Trap

**What goes wrong:**

The NUDGE PRD §4 describes a "60-second detection cache" and criterion §10.11 says "Detection cache prevents re-running `pnpm --version` more than once per 60 seconds in the **same session**." This makes sense in a long-running daemon or shim process. It is **completely meaningless** in `beekeeper check`, which is a one-shot process: every hook invocation forks a new OS process, lives for the duration of one tool call, and exits. There is no in-process cache to hit. The first time `Evaluate` is called in a fresh process, `DetectPnpm()` execs `pnpm --version`. That exec happens every single hook invocation — because there is no surviving process to hold the cache.

With the exec budget unrestricted, every `beekeeper check` invocation on the hook hot path runs `pnpm --version`, `bun --version`, and `node --version` — three subprocess execs — before the policy engine can decide anything. Each `exec.Command` on Windows with the 2-second timeout specified in PRD §6.1 can take 30-80ms in practice (process startup + PATH lookup + binary load), even on a warm machine. Three execs × 50ms = 150ms overhead per tool call, before catalog lookup or policy evaluation runs. This demolishes the sub-100ms target established in CLAUDE.md.

The PRD is self-consistent within a shim or gateway context (long-lived processes). The critical omission is that `internal/check/handler.go` (the `beekeeper check` one-shot process) is listed as the **first** integration point in PRD §3.3, and the cache does not help there at all.

**Why it happens:**

The PRD was drafted with a session-oriented mental model. The NUDGE feature is conceptually session-scoped ("detect once, nudge during the session"). The hook handler, however, is invocation-scoped. The mismatch is invisible at spec time because both paths call `nudge.Evaluate` through the same interface — the difference only manifests under real workload profiling.

**How to avoid:**

Pick exactly one mitigation path and commit to it in the NUDGE phase plan before writing `detect.go`:

1. **File-based detection cache** (recommended for check): Write detected PM state to `~/.beekeeper/state/nudge-detect.json` with a `last_checked` timestamp. On the next `beekeeper check` invocation, if `last_checked` is within 60 seconds (or a configurable window), load the cached state rather than exec-ing version commands. The file must be written atomically (write to `.tmp` + rename) and handled gracefully on read error (fall through to live detection, no crash). This is the only approach that survives the one-shot process model. 60s TTL means a developer who changes pnpm version mid-session gets a stale detection for up to 60 seconds — acceptable trade-off vs. per-invocation exec overhead.

2. **Skip detection in `check`; only nudge in gateway/shim**: The NUDGE PRD lists three integration points (check, gateway, shim). Detection in `beekeeper check` adds latency on every hook (file Read, Bash, Edit). Detection in the MCP gateway and shim adds latency once per long-lived session, where the 60-second in-process cache actually helps. A simpler v1.2.0 scope: wire nudge only into the gateway and shim, not into check. The check path gets a no-op nudge stub that always returns `Proceed`. Defer check-path nudge to v1.3.0 when the latency budget is re-evaluated.

3. **Lazy/async detection with PROCEED-on-miss**: Detection runs asynchronously; the first invocation returns `Proceed` immediately while detection executes in background and stores the result to the file cache. This adds implementation complexity and leaves a window where `npm install` is not nudged (the first call in a session). Not recommended as the primary strategy.

**The file-based cache is the correct default.** Build it first. Make TTL configurable in the nudge config block. Document that the in-process 60-second cache described in the PRD applies only to gateway/shim; the check path uses the file cache.

**Warning signs:**

- `beekeeper check` latency benchmark showing 100ms+ on a machine with pnpm installed — the exec overhead is present
- PRD criterion §10.11 passing in a test that creates a long-lived `detect` object in process (mocks the check invocation model, doesn't test it)
- No `nudge-detect.json` state file in the design — means file cache was not implemented
- `beekeeper diag` p99 regression appearing exactly after NUDGE phase is merged

**Phase to address:**

NUDGE phase (v1.2.0). Must be resolved in the NUDGE detection design, not deferred. Gate the NUDGE phase plan on confirming which mitigation path is chosen before any `detect.go` implementation begins.

---

### Pitfall 2: PLCY-07 Corroboration Poisoning — Treating Bundled Catalog as Signed-Equivalent Creates a New Single Point of Compromise

**What goes wrong:**

PLCY-07's goal is to make a critical-severity catalog match block even when the bumblebee catalog is unsigned (`Signed:false`), because today a critical match from bumblebee + OSV gets `CorroborationCount:1` (only OSV is signed) and only warns. The tempting implementation is: "if severity == critical and the bundled bumblebee catalog matched, treat it as signed-equivalent for corroboration purposes."

This is the self-defense trap CLAUDE.md identifies as a Phase 2 non-negotiable: **corroboration sanity bounds + catalog signature verification**. If you treat the bundled bumblebee catalog as signed-equivalent without cryptographic verification, you have just created a scenario where a single poisoned entry in the local bumblebee `threat_intel/` directory can single-handedly block any package as "critical". The bumblebee catalog is fetched from an unauthenticated GitHub raw endpoint (unless catalog signature verification is implemented). An attacker who can perform a MITM, a compromised CDN cache, or a supply-chain compromise of the bumblebee repo itself can inject a critical-severity entry for any package — say, `react` or `typescript` — and every Beekeeper user who then runs `beekeeper catalogs sync` will have that entry load as "signed-equivalent", causing every React install to be blocked with no second-source corroboration required.

This is strictly worse than the current state. Today a poisoned bumblebee entry + no OSV corroboration = warn only. After the "treat bundled as signed-equivalent" shortcut, a poisoned bumblebee entry + no OSV corroboration = block. You have traded "critical CVEs only warn" for "a single poisoned catalog can block anything."

The existing `catalog/verify.go` has `VerifySignatureWithKey(entry, pubKey)` already built. The existing `catalog/sanity.go` has delta sanity bounds already built. The corroboration model in `policy/corroboration.go` correctly requires `Signed:true` for escalation. The infrastructure exists; the question is whether PLCY-07 uses it correctly.

**Why it happens:**

The symptom (critical CVEs warn-only) is real and needs fixing. The fast path — escalate severity-tagged matches regardless of signature — solves the symptom without addressing the root cause (bumblebee entries are unsigned because there is no trusted Ed25519 key configured for the bundled catalog). Severity-based escalation without signature verification inverts the trust model: the catalog's own claim of "critical" is now sufficient to block, which is exactly the attack surface the corroboration model was designed to prevent.

**How to avoid:**

There are two correct paths — choose one:

1. **Sign the bundled bumblebee catalog slice** (correct, higher effort): Generate an Ed25519 keypair for Beekeeper's bundled catalog. During `beekeeper catalogs sync`, verify each downloaded bumblebee entry against the public key (using the existing `VerifySignatureWithKey`). Entries that pass verification are loaded with `Signed:true`. Critical-severity signed entries then naturally reach the corroboration block threshold (1 signed source at `WarnAt:1, BlockAt:1` for critical, configurable via policy file). The bundled public key is pinned in the Beekeeper binary (not downloaded at runtime — downloaded public keys are trivially MITMed). This is the long-term correct architecture.

2. **Per-severity corroboration threshold policy** (correct, lower effort): Keep `Signed:false` for bumblebee entries. Add a configurable per-severity corroboration override to `CorroborationThresholds`: `CriticalBlockAt: 1` means "for critical severity, block at 1 source regardless of signing." Crucially: this threshold applies only when the sanity check passes (catalog delta is within bounds, catalog signature verification is not actively failing), and the threshold is set conservatively: `CriticalBlockAt:1` with unsigned is still gated on the sanity bounds system treating the catalog as non-degraded. Document explicitly: if the bumblebee catalog fails sanity bounds (sudden delta spike), it reverts to unsigned warn-only regardless of severity, so catalog poisoning that injects many critical entries at once triggers the sanity system and fails closed. The existing `catalog/sanity.go` `BlockDeltaEntries:10000` and `AlertDeltaEntries:1000` thresholds are the backstop.

**Option 2 is the right scope for v1.2.0.** Document path 1 as the v1.3.0 or v2.0.0 follow-on (catalog signing infrastructure). Include in the policy file schema:

```json
{
  "corroboration_threshold": {
    "warn_at": 1,
    "block_at": 2,
    "quarantine_at": 3,
    "critical_block_at": 1
  }
}
```

Gate `critical_block_at: 1` on the source not being in degraded mode (sanity bounds not exceeded). Test this explicitly: inject a single bumblebee entry with `severity: "critical"` and zero OSV corroboration — should block. Then inject 1001 new critical entries at once (triggers alert sanity bound) — should revert to warn-only regardless of severity.

**Warning signs:**

- PLCY-07 implementation adds `if severity == "critical" { forceSigned = true }` or similar shortcut in `corroborate()` without sanity-bound gating
- No test covering the case: "critical severity match + catalog sanity bound exceeded → still warn only"
- No test covering: "catalog with 1000 new critical entries → degraded mode, not block storm"
- `critical_block_at` threshold is not configurable (hardcoded 1) — means it cannot be dialed back if false positives occur

**Phase to address:**

PLCY-07 phase (v1.2.0). The sanity-bound gating is non-negotiable before the severity-escalation threshold is live. Both must ship together.

---

### Pitfall 3: PLCY-05 Path Canonicalization — The `~` Unresolved and Windows Path Mismatch Gaps

**What goes wrong:**

`EvaluatePath` in `policy/path.go` is a pure function that receives an "already-resolved string" — normalization is explicitly the caller's responsibility (see the doc comment: "Path normalization (resolving `~` to the home directory, converting OS separators) is the CALLER's responsibility"). The existing engine is correct and well-tested in isolation. The pitfall is in the wiring: when the hook handler calls `EvaluatePath` with a `file_path` from agent tool call JSON, the `file_path` may arrive in any of these forms:

- `~/.aws/credentials` — tilde not expanded (most common in Claude Code tool calls)
- `../../.env` — relative path with traversal
- `.env` — relative, no traversal, but the cwd context is unknown
- `C:\Users\user\.aws\credentials` — Windows backslash (the primary dev machine)
- `C:/Users/user/.aws/credentials` — Windows forward-slash variant
- `//server/share/.aws/credentials` — UNC path
- `//?/C:/Users/user/.aws/credentials` — Win32 extended-length path prefix

The existing `normalizeSlashes()` in `path.go` handles backslash→forward-slash conversion for fragment matching. But if the caller passes `~/.aws/credentials` unresolved, `strings.Contains(path, "/.aws/")` will match (the `~` prefix is irrelevant for the fragment). So the tilde case accidentally works for fragment patterns — but it will NOT work for allow patterns: if the user has an allowlist entry like `/home/user/projects/.env.test`, the tilde in the incoming path won't match `/home/user/projects/` as a prefix.

The deeper problem is relative paths with `..` traversal: `../../.aws/credentials` does NOT contain `/.aws/` as a substring — it contains `.aws/credentials`. The fragment pattern `/.aws/` requires the leading slash. An agent reading `../../.aws/credentials` from within a project directory bypasses the block check entirely.

On Windows, UNC paths (`\\server\share\`) and extended-length prefixes (`\\?\`) are additional cases that `normalizeSlashes()` does not handle.

**Why it happens:**

Path normalization is handled at the caller level by design (keeping `EvaluatePath` pure). The pitfall is that the wiring in `internal/check/handler.go` may not call `filepath.Abs()` + `os.UserHomeDir()` before invoking `EvaluatePath`. If the handler just extracts `file_path` from the tool call JSON and passes it directly, all relative and tilde paths are under-normalized.

**How to avoid:**

In the PLCY-05 wiring layer (not in `EvaluatePath` itself, which should remain pure):

1. **Expand `~`**: Replace leading `~` with `os.UserHomeDir()` result before path evaluation. On Windows, `os.UserHomeDir()` returns the correct `C:\Users\user` path. Handle `~/` and `~\` variants.

2. **Resolve to absolute**: Call `filepath.Abs(expandedPath)` to resolve relative paths against the process working directory. For tool calls that include a `cwd` field (Claude Code provides this in some contexts), use that cwd, not `os.Getwd()`.

3. **Normalize separators**: After `filepath.Abs()`, call `filepath.ToSlash()` so the resolved path is forward-slash canonical before pattern matching. This is already what `normalizeSlashes()` does, but it needs to run on the fully-resolved path, not just the fragment patterns.

4. **Handle UNC paths**: On Windows, `filepath.Abs()` preserves UNC paths but the leading `\\` needs special handling. Normalize `\\server\share\` to `/server/share/` or treat UNC as out-of-scope and log a warning.

5. **Test the wiring, not just `EvaluatePath`**: Add integration tests in `check/integration_test.go` that feed raw tilde-prefixed and relative `file_path` values through `RunCheck` (stdin→decision) and assert they trigger the sensitive-path block. This is the gap the milestone's F2 finding surfaced — the engine was correct but the wiring was absent.

The `EvaluatePath` function itself needs no changes. The wiring adapter that resolves paths before calling it is the PLCY-05 deliverable.

**Warning signs:**

- PLCY-05 tests only unit-test `EvaluatePath` with already-resolved paths (no integration test feeding `~/.aws/credentials` through `RunCheck`)
- Handler code extracting `tool_call.FilePath` and passing it directly to `EvaluatePath` without `filepath.Abs()` + tilde expansion
- Windows CI showing pass on sensitive-path tests but the tests only use Unix-style paths with forward slashes
- `.env` (relative, no directory component) not being matched by the `basename pattern` branch — needs a test specifically for the no-separator case

**Phase to address:**

PLCY-05 phase (v1.2.0). The path normalization adapter must be written and tested as part of the wiring, not treated as a caller assumption.

---

### Pitfall 4: PLCY-05 False Negatives — Indirect Credential Access Bypasses `file_path` Inspection

**What goes wrong:**

PLCY-05 evaluates the `file_path` parameter of `Read`, `Write`, and `Edit` tool calls, plus command targets for `cat`/`type`/`Get-Content` in `Bash` tool calls. This covers direct file access. It does not cover:

- **Environment variable indirection**: An agent runs `echo $AWS_ACCESS_KEY_ID` or `env | grep AWS` — no file path is inspected, but credential values are now in the tool output.
- **Shell expansion in Bash commands**: `cat ~/.aws/credentials` — the command string contains the credential path but it is embedded in a Bash command, not a standalone `file_path` parameter. PLCY-05 must parse Bash command strings, not just structured JSON fields.
- **Python/Node one-liners**: `python -c "import configparser; c = configparser.ConfigParser(); c.read('~/.aws/credentials'); print(dict(c['default']))"` — the credential path appears only as a string literal inside an exec'd script. Command parsing at the PLCY-05 level cannot see inside exec'd scripts.
- **`type` on Windows**: Windows `type` is the equivalent of `cat`. If the command parser for Bash tool calls only checks for `cat`, `Get-Content`, and `type` but misses `more`, `findstr`, or PowerShell `Get-Content` aliases (`gc`, `cat` on PSAlias), credential files can be read without triggering the check.
- **Symlinks**: An agent creates a symlink from a non-sensitive path to `~/.aws/credentials` and then reads the symlink target. The `file_path` is the symlink location, not the credential file. `filepath.Abs()` does NOT resolve symlinks (use `filepath.EvalSymlinks()` for that).

The existing `policy/path.go` `EvaluatePath` handles what it is given correctly. The issue is what the PLCY-05 wiring layer extracts from the tool call and passes to `EvaluatePath`.

**Why it happens:**

A static `file_path` check is the 80% solution. It catches the most common agent patterns: explicit Read tool calls on credential files. The remaining 20% requires either deeper command parsing or output scanning — both are significantly more complex. The pitfall is shipping PLCY-05 as "credential protection is done" when it covers only direct structured tool calls, not Bash-based access patterns.

**How to avoid:**

- Explicitly scope PLCY-05 in its requirements: "covers `file_path` in Read/Write/Edit tool calls AND direct `cat`/`type`/`Get-Content`/PowerShell `Get-Content` (`gc`) patterns in Bash tool call command strings. Does not cover env-var indirection or exec'd script access." Document the uncovered cases in `docs/THREAT-MODEL.md` as known limitations.
- In the PLCY-05 command parser for Bash tool calls, add patterns for Windows PowerShell variants: `Get-Content`, `gc`, `cat` (PSAlias), `type`, `more`. Test each on a Windows CI matrix.
- For symlink resolution: call `filepath.EvalSymlinks(resolvedPath)` and re-evaluate the resolved target against the blocklist. If `EvalSymlinks` fails (path does not exist yet, for a Write), evaluate the pre-symlink path only. Log symlink resolution failures as warnings, not errors.
- The output scanning path (catching env-var exfiltration through tool output) belongs to the LLMF/exfil detection scope, not PLCY-05. Do not conflate the two.

**Warning signs:**

- PLCY-05 tests only cover `Read` tool calls with explicit `file_path` fields; no tests for `Bash` tool calls containing `cat ~/.aws/credentials`
- No Windows-specific tests for `type`, `Get-Content`, `gc` patterns
- Symlink tests absent from the PLCY-05 behavioral test suite
- Documentation claims "credential file access is blocked" without caveat for indirect access patterns

**Phase to address:**

PLCY-05 phase (v1.2.0). Scope the command-string parsing explicitly in the phase plan. Add Windows-specific path and command patterns to the behavioral test suite.

---

### Pitfall 5: PLCY-05 False Positives — Blocking `.env.example` and Other Legitimately-Named Non-Credential Files

**What goes wrong:**

The default `SensitivePathConfig` in `policy/path.go` includes the pattern `.env.*` which matches "any basename with prefix `.env.`" via glob simulation. This correctly blocks `.env.production`, `.env.local`. It also matches:

- `.env.example` — a committed, non-secret file that documents required environment variables. Agents legitimately read and write `.env.example` constantly.
- `.env.test` — often committed and non-sensitive in test suites.
- `.env.schema` — JSON schema for environment variable validation tools.
- `.envrc` — direnv configuration, sensitive if it contains credentials but often just `export PATH=...`.

Blocking access to `.env.example` on an `npm install` post-setup hook will confuse agents and developers. If the default blocklist causes false positives on the first day of use, developers will add broad allowlist entries like `"/*.env.*"` that then allow the actual `.env` files too.

The `.env` exact-match pattern in the blocklist is correct — `.env` files almost always contain secrets. The `.env.*` glob is overaggressive for certain common names.

**Why it happens:**

The glob `.env.*` was written to catch `.env.production` and `.env.staging` without enumerating every suffix. The failure mode is not visible in the policy engine unit tests, which test blocking, not the agent's workflow. It becomes visible only in live use when an agent tries to scaffold a new project and reads `.env.example` to populate the new `.env` file.

**How to avoid:**

- Add an allowlist default for `.env.example` in `DefaultSensitivePaths()` alongside the `.env.*` block pattern. The allowlist is checked first (existing behavior in `EvaluatePath`), so `.env.example` gets allowlisted before the `.env.*` block fires.
- Also consider adding `.env.test` and `.env.schema` to the default allowlist. These are low-risk and commonly non-sensitive.
- Add a test to the PLCY-05 behavioral suite: `EvaluatePath(".env.example", DefaultSensitivePaths())` must return allow, not block.
- Document the default blocklist behavior in `docs/nudge.md` or a new `docs/policies.md` so users understand what they are getting out of the box and how to add project-specific allowlist entries via `.beekeeper.json`.

**Warning signs:**

- No test for `.env.example` being allowed while `.env` is blocked
- `DefaultSensitivePaths()` has no `AllowPatterns` at all (the existing code has `AllowPatterns: nil`)
- First live use of PLCY-05 blocks agent from reading `.env.example` in a new project scaffold

**Phase to address:**

PLCY-05 phase (v1.2.0). Add the default allowlist entries to `DefaultSensitivePaths()` before the PLCY-05 wiring is live.

---

### Pitfall 6: NUDGE Hard-Mode Command Rewriting Breaks Agent Output Parsing

**What goes wrong:**

In hard mode, NUDGE rewrites `npm install foo` to `pnpm add foo`. The agent issued an npm command expecting npm output. It then parses that output. npm and pnpm output differ:

- npm install success: `added 1 package in 0.5s` plus a JSON lockfile update note
- pnpm add success: `Packages: +1 / Progress: resolved 1, reused 0, downloaded 1, added 1, done`

An agent that issues `npm install foo` and then checks the output for `"added 1 package"` to confirm success will silently conclude the install failed. The agent may retry with the same command, creating an install loop. Or the agent may take an error-handling branch designed for npm failures when the install actually succeeded.

The same class of problem applies to error messages, exit code conventions (pnpm and npm handle some error cases differently), and the shape of `--json` output when agents request machine-readable npm output.

**Why it happens:**

Hard-mode rewriting is transparent to the agent at the decision layer but not at the output layer. The agent generated the original command assuming a specific output format. Rewriting the command changes the program that runs. This is a known footgun in transparent proxy architectures.

**How to avoid:**

- Soft mode (advise + proceed with original command) is the safe default. PRD §5.1 correctly defaults to `mode: "soft"`. Do not make hard mode the default; do not enable hard mode in any default config shipped with Beekeeper.
- Document hard mode explicitly as requiring the user to verify that their agent is output-agnostic with respect to npm vs pnpm output. Include this caveat in `docs/nudge.md`.
- In the audit record for a rewrite decision, log `original_command` and `rewritten_command` both. This enables forensic reconstruction of which rewritten commands may have produced unexpected agent behavior.
- For the `--json` flag specifically: if `npm install --json` is rewritten to `pnpm add --json`, test that pnpm's JSON output is structurally compatible with what the agent expected. If it is not (field name differences, nesting changes), this is a reason to NOT rewrite commands with `--json` flags — add a rule: "if the npm command contains `--json`, do not hard-rewrite; soft-advise only."
- Add a behavioral test: issue `npm install foo --json` in hard mode, capture the rewritten command, verify it does NOT strip `--json` without also verifying pnpm's `--json` output is compatible.

**Warning signs:**

- Hard mode enabled by default in any config template
- No test verifying that hard-rewritten commands produce output the agent can parse
- Agent emit logs showing `npm install` retry loops after NUDGE is enabled in hard mode
- No `original_command` field in the audit record for rewrite decisions

**Phase to address:**

NUDGE phase (v1.2.0). The rewrite test (behavioral compatibility with agent output parsing) must be in the NUDGE acceptance criteria, not deferred to a follow-on.

---

### Pitfall 7: NUDGE Monorepo Dual-Lockfile Ambiguity and Docker-Exec PM Detection

**What goes wrong:**

Two specific edge cases from PRD §11 have implementation-level failure modes beyond what the spec describes:

**Dual-lockfile:** PRD §11 says "treat as pnpm project; the npm lockfile is likely stale or a CI artifact." The `detect.go` implementation must find `pnpm-lock.yaml` in the project root to make this determination. But `detect.go` needs a `projectRoot` path — and the NUDGE detect layer runs from the hook handler, where the current working directory context is the cwd of the Beekeeper process, not the agent's project root. The agent's project root is available in tool call context (Claude Code provides `cwd` in the hook stdin JSON for some tool types) but is not reliably present for all hook types. If `detect.go` defaults to `os.Getwd()` as the project root, it may check for `pnpm-lock.yaml` in the Beekeeper installation directory, not the agent's project.

**Docker-exec:** PRD §11 says "Decision is logged with `context: "docker-exec"` and proceeds since the container's PM may be different." Detecting "inside a Docker exec" from the hook handler is non-trivial: the hook runs on the host, but the npm command may be running inside a container. The container's `npm` is not the host's `npm`, and the container's pnpm/bun state is invisible to the host's `exec.LookPath`. The implementation must not attempt host pnpm detection for commands that the hook context identifies as container-scoped — doing so would rewrite a container-targeted `npm install` to use the host's pnpm, which is not installed inside the container, causing the install to fail silently.

**Why it happens:**

Both pitfalls stem from the detect layer running in host context while the commanded tool may be in container context, or from using the wrong working directory for lockfile detection. They are invisible in pure-unit testing (which mocks filesystem state) and only appear in realistic integration scenarios.

**How to avoid:**

For dual-lockfile detection:
- Extract the project root from the hook input's `cwd` field (Claude Code provides this in `HookInput.WorkingDirectory` or equivalent) rather than from `os.Getwd()`. If the hook input does not provide a cwd, skip lockfile-based detection and rely only on binary detection.
- Test with a synthetic monorepo fixture (tmpdir with both `pnpm-lock.yaml` and `package-lock.json`) — not with mocked return values.

For docker-exec:
- Parse the Bash command string for `docker exec` or `docker run` prefixes before dispatching to the npm command parser. If detected, set `Decision.Action = Proceed` with a structured reason `"docker-exec-host-context-mismatch"` and log the audit record — do not detect host PM state or rewrite.
- Add a test: a Bash tool call containing `docker exec mycontainer npm install foo` must return `Proceed` with the docker reason code, not `Advise` or `Rewrite`.

**Warning signs:**

- Detect tests only use `t.TempDir()` as project root, not a mock of hook-input cwd extraction
- No test for `docker exec` prefix in the command parser
- No test verifying `pnpm-lock.yaml` is found in the `cwd`-provided root, not the process working directory

**Phase to address:**

NUDGE phase (v1.2.0). Add docker-exec test and lockfile-root-extraction to the NUDGE acceptance criteria alongside PRD §10.

---

### Pitfall 8: PLCY-07 False Positives from Single-Source Critical Escalation — Severity Inflation

**What goes wrong:**

Lowering `CriticalBlockAt` to 1 means a single catalog source can block a package. This is justified when the source is highly reliable and the severity classification is accurate. The risk is that catalog maintainers (including bumblebee's) have historically experienced **severity inflation**: packages are initially tagged `critical` during incident response (threat is active, time pressure) and later downgraded when the scope is better understood, or when the CVE is assigned a lower CVSS by NVD.

If `CriticalBlockAt:1` is live and a package is mis-tagged `critical` in bumblebee, every developer who tries to install that package is blocked with no warning-only grace period. The only recovery path is: (1) update the catalog entry (requires upstream change to bumblebee), (2) add the package to the allowlist, or (3) reduce `CriticalBlockAt` back to 2. None of these are fast for a developer mid-sprint.

A real-world example of the failure mode: `ua-parser-js` was initially flagged `critical` during the October 2021 compromise. Later analysis confirmed the window of exposure was narrow and many consumers were unaffected. A blanket block of all versions would have broken Node.js projects that didn't use the compromised version range.

**Why it happens:**

Severity is assigned during incident response under time pressure. Catalog maintainers prioritize false negatives (missing a real threat) over false positives (blocking a legitimate package). The user-visible asymmetry: a missed threat results in developer embarrassment later; a blocked legitimate package results in an angry developer right now who disables Beekeeper.

**How to avoid:**

- `CriticalBlockAt:1` should ONLY apply to packages where the catalog entry includes a populated `Versions` list (specific affected version ranges), not to entries with `Versions: []` or `Versions: ["*"]` (all versions). An entry claiming all versions of a major package are critical is a strong signal of mis-tagging. For all-version critical entries with a single source, still require 2-source corroboration.
- Add a per-entry allow-override escape hatch to the policy file: `"package_allowlist": [{"ecosystem": "npm", "name": "foo", "until": "2026-06-10", "reason": "..."}]`. This already exists in the policyloader; make sure it works for corroboration-triggered blocks, not just rule-based blocks.
- In the block message surfaced to the agent, include the catalog source, the severity, and the affected versions. A developer who sees "beekeeper blocked foo@1.2.3 (critical, bumblebee, affects 1.2.2-1.2.4)" can make an informed decision. A developer who sees "blocked by policy" cannot.
- Add a behavioral test: a catalog entry with `severity:"critical"`, `versions:["*"]`, single unsigned source → `CriticalBlockAt:1` config → should still produce warn, not block. The all-versions guard must be enforced.

**Warning signs:**

- `CriticalBlockAt:1` applies equally to version-specific and all-version entries
- Block messages do not include the version range that triggered the match
- No per-package allowlist-until-date mechanism in the policy schema
- No community-facing issue template for "this package was incorrectly blocked"

**Phase to address:**

PLCY-07 phase (v1.2.0). Include the all-versions guard and the block-message enrichment as acceptance criteria.

---

### Pitfall 9: Behavioral Tests That Mock Too Much — The Gap That Caused This Milestone

**What goes wrong:**

The three gaps (F1/F2/F3) that define this milestone were found by **live testing with the agent as the test subject**, not by the unit test suite. This is the defining testing pitfall of the milestone: unit tests that mock `runPollenFn`-style interfaces validate that the code correctly handles the mock's return values — they do not validate that the code correctly handles the real system's output.

The specific failure mode for this milestone:

- `EvaluatePath` has extensive unit tests that pass pre-normalized paths. The wiring in `handler.go` was absent. The unit tests could not catch the gap because they never called through `RunCheck`.
- The corroboration test for critical CVEs passes the policy engine in isolation. The live system has `Signed:false` from the bumblebee loader — a fact that a mocked `MultiCatalogLookup` may not faithfully represent.
- The NUDGE cache test (PRD §10.11) passes in a long-lived test process. The one-shot `beekeeper check` process invalidates the cache assumption.

**Why it happens:**

Unit tests are fast and deterministic. The natural tendency is to maximize unit test coverage and treat integration tests as expensive edge cases. For a security enforcement tool, this hierarchy is inverted: the unit tests prove correctness of components; the integration tests prove that the components are connected to the real system correctly.

**How to avoid:**

The BTEST phase is the explicit cross-cutting answer. For this milestone specifically:

- **Every PLCY-05 acceptance criterion** must have a corresponding `check/integration_test.go` test that calls `RunCheck` with real (or realistic) stdin JSON. Not a mock. Not a unit call to `EvaluatePath`. A full stdin→exit-code pipeline.
- **Every PLCY-07 criterion** must have a test that constructs a real `catalog.MultiIndex` (with the bumblebee `Signed:false` property preserved, not a mock that returns `Signed:true`) and asserts the correct block/warn decision.
- **Every NUDGE criterion** that involves detection must have a test that verifies the behavior under a one-shot process model (start a test binary, exec it, check file-cache state).
- **E2E battery**: run the live Beekeeper binary against synthetic tool call JSON, assert exit code. The PRD for the v1.2.0 milestone explicitly calls for "check-handler integration tests (stdin→decision)" and "live-binary E2E battery (catalog-backed)". These are not optional — they are the verification that the three F-gaps are actually closed.

Fuzz testing (already in `policy/fuzz_test.go` and `catalog/fuzz_test.go`) covers edge-case input variation but does not verify wiring. Keep fuzz tests; add integration tests.

**Warning signs:**

- BTEST phase plan contains only unit tests and fuzz tests, no live-binary E2E tests
- PLCY-05 tests import `policy` directly rather than calling through `check.RunCheck`
- PLCY-07 tests mock `MultiCatalogLookup` with `Signed:true` rather than using the actual bumblebee catalog loader with `Signed:false` entries
- No test file that imports `os/exec` to invoke the built binary and assert exit code

**Phase to address:**

BTEST phase (v1.2.0) and embedded in each of PLCY-05, NUDGE, PLCY-07 phases. The E2E battery is a release gate for v1.2.0.

---

### Pitfall 10: NUDGE `bunfig.toml` Parse Failures and Corepack-Shimmed pnpm

**What goes wrong:**

Two implementation-level pitfalls from the NUDGE spec that the test criteria cover but the implementation can still get wrong:

**`bunfig.toml` parse failure (PRD §10.13):** If `bunfig.toml` exists but contains a TOML syntax error (common during developer editing), `BunScannerOK` defaults to `false` and a warning is logged. The pitfall is that a TOML parse error in Go with a naive `toml.Unmarshal` call may panic on some malformed inputs if the TOML library used does not handle all error cases. The detection must wrap the parse in a recover-or-error handler, not just check the error return. Use `github.com/BurntSushi/toml` which has well-tested error handling, not a minimal TOML parser.

A second `bunfig.toml` pitfall: the spec says look for it in "project root and `~/.bunfig.toml`." The project root is subject to the same cwd ambiguity as the lockfile detection in Pitfall 7. Use the hook-input cwd, not `os.Getwd()`.

**Corepack-shimmed pnpm (PRD §11):** The spec says "both detected by `exec.LookPath`. Corepack-shimmed pnpm responds to `--version` identically. No special handling needed." This is **conditionally true** on macOS and Linux. On Windows, Corepack installs shims as `.cmd` files (e.g., `pnpm.cmd`). `exec.LookPath("pnpm")` on Windows finds `pnpm.cmd` if `.CMD` is in `PATHEXT`, which it is by default. The `pnpm --version` exec through the cmd shim works but adds a cmd.exe process in between — increasing the exec latency by 20-40ms on Windows (cmd.exe startup). Under the 2-second timeout, this is fine, but it contributes to the overall detection overhead identified in Pitfall 1.

The real corepack trap: if corepack is enabled but the project's `package.json` specifies a different pnpm version via `"packageManager": "pnpm@10.x"`, corepack will download pnpm 10 on first run, causing the 2-second timeout to expire while a network download is in progress. The version returned would be pnpm 10, not pnpm 11, causing `PnpmHardened:false` even though the user has pnpm 11 system-installed.

**How to avoid:**

- TOML parse: use `BurntSushi/toml` with explicit error checking; wrap in a named recover function; test with a deliberately malformed `bunfig.toml` fixture.
- Corepack version ambiguity: after getting the version from `pnpm --version`, check if it satisfies the `versionFloors.pnpm` floor. If corepack returned an old version (< 11), log this as a warning: "pnpm detected via corepack may be using a project-pinned version older than the security floor." Do not try to detect corepack directly — that adds complexity. The version check is the right guard.
- Test: fixture a `package.json` with `"packageManager": "pnpm@10.5.0"` and verify that NUDGE correctly identifies `PnpmHardened:false`.

**Warning signs:**

- `bunfig.toml` parse uses a minimal library without error recovery
- No fixture test with malformed `bunfig.toml` content
- No test for corepack-pinned pnpm version below the security floor
- Detection runs with a 2-second timeout on Windows without accounting for cmd.exe shim startup overhead

**Phase to address:**

NUDGE phase (v1.2.0). Add corepack and TOML error tests to the NUDGE acceptance criteria.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| In-process 60-second nudge cache only (no file cache) | Simpler detect.go implementation | Cache is a no-op in one-shot `beekeeper check` invocations; exec overhead on every hook call; p99 regression | Never acceptable for the check integration path |
| `critical_block_at:1` applies to `versions:["*"]` entries | No extra logic needed | A single upstream mis-tagging of a major package blocks all installs of it across all Beekeeper users | Never acceptable; the all-versions guard is mandatory |
| Treat bundled bumblebee as signed-equivalent without Ed25519 verification | Unblocks PLCY-07 immediately | Poisoned bundled catalog can single-handedly block any package | Never acceptable; must use sanity-bound gating or catalog signing |
| PLCY-05 wiring passes raw `file_path` to `EvaluatePath` without normalization | Less code in handler | Tilde, relative paths, and `..` traversal bypass the blocklist silently | Never acceptable; normalization is documented as caller responsibility |
| BTEST only contains unit and fuzz tests, no E2E binary invocations | Faster CI | Does not verify wiring gaps — precisely the class of bug that caused this milestone | Never acceptable; E2E tests are a v1.2.0 release gate |
| NUDGE hard mode enabled as default config | Users immediately get the stronger protection | Breaks agent workflows that parse npm output; increases support burden | Never acceptable; soft mode default is a security product UX non-negotiable |
| Skip PLCY-05 Windows path tests (forward-slash only) | CI passes without Windows-specific fixtures | Windows-primary dev box (the dogfood environment) has backslash paths; the most used environment is untested | Never acceptable; Windows CI matrix exists for exactly this reason |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| PLCY-05 wiring in `handler.go` | Extracting `file_path` from tool call JSON and passing directly to `EvaluatePath` | Expand tilde, call `filepath.Abs()`, call `filepath.EvalSymlinks()`, normalize slashes, then call `EvaluatePath` |
| NUDGE detect in `check` path | Using in-process sync.Once or time.Now()-based cache | Write detection result to `~/.beekeeper/state/nudge-detect.json` with TTL; read it on next invocation |
| PLCY-07 per-severity threshold | Adding `forceSigned = true` shortcut in `corroborate()` | Add `CriticalBlockAt` field to `CorroborationThresholds`; gate on sanity bounds not exceeded; test with degraded-mode override |
| NUDGE bun scanner detection | Only looking in project root `bunfig.toml` | Also check `~/.bunfig.toml`; use hook-input cwd as project root, not `os.Getwd()` |
| NUDGE docker-exec detection | Parsing `npm install foo` inside `docker exec ... npm install foo` as a nudge candidate | Detect `docker exec` / `docker run` prefix first; return Proceed with reason code, skip PM detection |
| PLCY-05 command parsing for Bash tool calls | Only checking `cat` prefix for credential-file access | Also check `type` (Windows cmd), `Get-Content` (PowerShell), `gc` (PSAlias), `more` (cross-platform) |
| BTEST integration tests | Calling policy engine functions directly with pre-normalized inputs | Feed raw tool call JSON through `RunCheck`; assert on exit code and audit record |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| NUDGE: 3× subprocess exec on every `beekeeper check` invocation | `beekeeper diag` p99 spikes 100ms+ after NUDGE phase merge; agent feels slow | File-based detection cache with 60s TTL in `~/.beekeeper/state/nudge-detect.json` | Every hook invocation on a machine with pnpm/bun installed |
| PLCY-05: `filepath.EvalSymlinks()` on every Bash command's file targets | Extra stat/readlink syscall per path candidate in every Bash tool call | Only call `EvalSymlinks` for paths that survive the initial blocklist substring check (fail fast on the cheap check first) | Every Bash tool call that reads a file |
| PLCY-07: `CriticalBlockAt:1` with network-fetched OSV corroboration | Extra OSV HTTP call on critical severity match, adding latency | Critical-severity escalation for bundled bumblebee should not require OSV confirmation — the whole point is to block without needing a second source | Every critical-severity npm install |
| NUDGE: `pnpm view pnpm version` for weekly drift check on check path | Weekly drift check runs during `beekeeper check` if timer check is done in-process | Drift check belongs in the `beekeeper catalogs sync` daemon or a scheduled task, never in the one-shot check path | First check invocation after the 168h drift interval elapses |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| PLCY-07: severity-based corroboration escalation without sanity-bound gating | Poisoned catalog entry with `severity:"critical"` blocks any package without second-source corroboration | Gate `CriticalBlockAt:1` on catalog sanity result: if catalog is in `Alert` or `Block` state from `CheckSanity`, revert to unsigned warn-only regardless of severity |
| NUDGE: recommending `@socketsecurity/bun-security-scanner` without disclosure | User may not realize Beekeeper is adding a third-party npm package to their supply chain trust | PRD §12 requires explicit disclosure text in the recommendation message: "published by Socket Inc." — enforce this as a test against the message string |
| PLCY-05: not re-evaluating resolved symlink target | Agent creates symlink from `/tmp/safe` to `~/.aws/credentials`; reads `/tmp/safe`; PLCY-05 sees a `/tmp/safe` path and allows | Call `filepath.EvalSymlinks()` before blocklist check; evaluate both the original path and the resolved target |
| PLCY-07: `critical_block_at` threshold stored in user-editable policy file without validation | User sets `critical_block_at: 0` (which disables all critical blocking) without understanding the implication | Validate `CriticalBlockAt >= 1` in `validateCorroborationThresholds()`; treat `CriticalBlockAt: 0` as unset (use default of 2, not 0) |
| NUDGE: hard-mode rewriting changes signed vs unsigned npm artifact | pnpm's lockfile format and integrity fields differ from npm's; rewriting may bypass npm's lockfile integrity checks | Document that hard mode changes the lockfile format; users must regenerate lockfiles when switching; do not present hard-mode rewriting as transparent |
| PLCY-05: normalization converts `../../.env` to a path outside the project | Absolute resolution of `../../.env` from a deep project subdirectory reaches actual `~/.env` | After `filepath.Abs()`, check that the resolved path is under the allowed project root OR evaluate against the blocklist (either hit is correct behavior — blocking is the right outcome here) |

---

## "Looks Done But Isn't" Checklist

- [ ] **PLCY-05 tilde expansion:** `RunCheck` with `file_path: "~/.aws/credentials"` exits 1 (block) — verify with an integration test, not just a unit test of `EvaluatePath`
- [ ] **PLCY-05 `.env.example` allowed:** `EvaluatePath(".env.example", DefaultSensitivePaths())` returns allow — verify default `AllowPatterns` includes this
- [ ] **PLCY-05 `..` traversal caught:** `RunCheck` with `file_path: "../../.aws/credentials"` exits 1 — requires `filepath.Abs()` in wiring
- [ ] **PLCY-05 Windows backslash:** `RunCheck` with `file_path: "C:\\Users\\user\\.aws\\credentials"` exits 1 on Windows CI matrix
- [ ] **PLCY-05 PowerShell `Get-Content`:** Bash tool call with `Get-Content ~/.aws/credentials` exits 1 — not just `cat` coverage
- [ ] **NUDGE file cache:** After a `beekeeper check` invocation, `~/.beekeeper/state/nudge-detect.json` exists and contains the detected PM state
- [ ] **NUDGE one-shot cache hit:** A second `beekeeper check` invocation within 60s does NOT exec `pnpm --version` again — verify via file modification timestamp, not in-process mock
- [ ] **NUDGE docker-exec non-nudge:** Bash tool call containing `docker exec ... npm install foo` returns Proceed with correct reason code — not Advise or Rewrite
- [ ] **NUDGE hard-mode soft-default:** Default config has `mode: "soft"` — verify by loading default config and asserting mode
- [ ] **PLCY-07 sanity-bound gate:** `CriticalBlockAt:1` + degraded catalog (>1000 new entries) → warn-only, not block — E2E test with injected large catalog delta
- [ ] **PLCY-07 all-versions guard:** Single-source `critical` + `versions:["*"]` → warn-only even with `CriticalBlockAt:1`
- [ ] **PLCY-07 block message:** Block decision includes catalog source name, severity, and affected version range in `Reason` field
- [ ] **BTEST E2E battery:** A test file execs the compiled Beekeeper binary with synthetic tool call JSON on stdin and asserts exit code — not just `go test ./...`
- [ ] **BTEST catalog-backed E2E:** At least one E2E test uses a real (or real-format) bumblebee catalog slice with `Signed:false` entries to verify corroboration behavior matches production

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| NUDGE detection exec overhead discovered post-merge (p99 regression) | MEDIUM | Implement file-based detection cache; add latency regression gate to CI; one-commit fix but requires re-verification of one-shot behavior |
| PLCY-07 false positive blocks a widely-used package | LOW-MEDIUM | Add to `package_allowlist` in policy file (immediate relief); upstream catalog correction (longer-term); `CriticalBlockAt` → 2 as emergency rollback |
| PLCY-07 catalog poisoning via single critical entry | MEDIUM | Emergency: set `CriticalBlockAt` back to 2 in shipped config; publish updated Beekeeper with sanity-bound gate reinforced; requires users to update |
| PLCY-05 `.env.example` false positive discovered in production | LOW | Add to default `AllowPatterns`; release patch; users can add local allowlist immediately as workaround |
| NUDGE hard-mode rewrite breaks agent output parsing | MEDIUM | Disable hard mode (config change); add `--json` guard to hard-mode rewrite logic; re-enable after testing |
| BTEST gaps discovered during v1.2.0 milestone audit | HIGH | Cannot close milestone without E2E coverage; add integration tests before closing; may delay release |
| PLCY-05 `..` traversal bypass discovered post-release | HIGH | SECURITY.md disclosure; patch release with `filepath.Abs()` in handler wiring; advise users to update immediately |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| NUDGE detection exec overhead (one-shot cache trap) | NUDGE phase plan — must resolve mitigation path before implementation | `beekeeper diag` p99 does not regress after NUDGE merge; file cache exists on disk after first invocation |
| PLCY-07 corroboration poisoning (signed-equivalent shortcut) | PLCY-07 phase — sanity-bound gating required before `CriticalBlockAt` goes live | E2E: 1001 new critical entries → degraded mode, not block storm; behavioral test: critical + sanity alert → warn |
| PLCY-05 path normalization gaps (tilde, `..`, Windows) | PLCY-05 phase — normalization adapter in handler wiring | Integration tests: tilde/relative/backslash paths through `RunCheck` all exit 1 as expected |
| PLCY-05 false negatives (indirect access via Bash) | PLCY-05 phase — command parser for Bash tool calls | Tests covering `cat`, `type`, `Get-Content`, `gc` patterns in Bash tool call command strings |
| PLCY-05 false positives (`.env.example`) | PLCY-05 phase — update `DefaultSensitivePaths()` allow list | Unit test: `.env.example` → allow; `.env.production` → block |
| NUDGE hard-mode agent output breakage | NUDGE phase — rewrite compatibility notes; no hard-mode default | Config assert: default mode is soft; `--json` flag guard in rewrite logic |
| NUDGE docker-exec PM detection mismatch | NUDGE phase — docker-exec prefix detection in command parser | Test: `docker exec ... npm install` → Proceed with docker reason code |
| PLCY-07 false positives from severity inflation | PLCY-07 phase — all-versions guard; block message enrichment | Test: `versions:["*"]` critical + single source → warn even with `CriticalBlockAt:1` |
| Behavioral tests mocking too much | BTEST phase — live-binary E2E battery requirement | BTEST phase plan includes exec-based E2E tests as acceptance criterion; milestone audit checks for these tests |
| NUDGE TOML parse errors and corepack version trap | NUDGE phase — error handling in `bunfig.toml` parse; corepack version check | Fixture tests with malformed TOML; fixture `package.json` with pnpm@10 corepack pin |

---

## Sources

- Live codebase: `internal/policy/path.go` — caller-normalization contract documented in EvaluatePath doc comment; `DefaultSensitivePaths()` has `AllowPatterns: nil`
- Live codebase: `internal/policy/corroboration.go` — `Signed:false` entries go to unsigned warn-only path; `CriticalBlockAt` field does not yet exist
- Live codebase: `internal/catalog/verify.go` — `VerifySignatureWithKey` exists and is correct; bundled catalog uses `VerifySignature` (presence-only)
- Live codebase: `internal/catalog/sanity.go` — `CheckSanity` is pure, correct, and already enforces delta bounds
- Live codebase: `internal/check/handler.go` — path normalization is absent before `policy.Evaluate`; nudge integration point does not yet exist
- PROJECT.md — F1/F2/F3 runtime-validation findings that define this milestone; "runtime-validation findings exposed by live testing, not unit tests"
- NUDGE PRD §4 — 60-second detection cache defined for in-process context; §3.3 lists `internal/check` as first integration point without noting the one-shot contradiction
- NUDGE PRD §6.1 — 2-second hard timeout on all detection commands; all detection commands exec subprocesses
- NUDGE PRD §10.11 — "Detection cache prevents re-running `pnpm --version` more than once per 60 seconds in the same session" — the "session" assumption is broken in one-shot check
- NUDGE PRD §11 — docker-exec and monorepo edge cases; §12 — supply-chain trust disclosure requirement for Socket scanner recommendation
- CLAUDE.md — "Corroboration sanity bounds + catalog signature verification" listed as Phase 2 self-defense non-negotiables; "Fail closed by default" as architecture constraint; adversarial corpus + fuzz + OS-specific build tag requirements

---
*Pitfalls research for: Beekeeper v1.2.0 "Runtime Behavioral Hardening" — PLCY-05, NUDGE, PLCY-07, BTEST*
*Researched: 2026-06-03*
