# Beekeeper Feature PRD: Package Manager Nudge

**Status:** Spec. Ready for build.
**Owner:** Mfanafuthi Mhlanga (bantuson).
**Module:** `beekeeper/internal/nudge/`
**Slots into:** Milestone **v1.2.0 "Runtime Behavioral Hardening"** as the F3 / package-manager-coverage phase. (The PRD was originally drafted against an aspirational "v0.3.0 / release-age + lifecycle" phasing; that framing is historical. In the live v1.x codebase this is net-new work for v1.2.0.)

> **Editor's note — codebase reality (2026-06-03):**
> - Integration seams in *this* repo are `internal/check/handler.go` (the `beekeeper check` hook handler), `internal/gateway/` (MCP proxy), and `internal/shim/shim.go` (PATH shims) — there is no `internal/hook/` package.
> - **Purity constraint (CLAUDE.md):** `internal/policy` must stay a pure function library (no I/O). Therefore `nudge.Evaluate` must be a **pure** decision over a caller-resolved `PMState` (detection I/O lives in `detect.go`/an adapter), exactly mirroring the existing `policy.EvaluateReleaseAge(ReleaseAgeInput, …)` pattern. Do not let the pure decision exec subprocesses.
> - Audit records flow through `internal/audit` (`FromDecision` + `RedactRecord`); a new `record_type: "nudge"` joins the existing `policy_decision` / `tool_result` / `llmf_alert` types.

---

## 1. Summary

When an agent invokes `npm install`, Beekeeper detects the call at the hook layer and steers the agent toward `pnpm` (>=11.0) or `bun` (>=1.3) if installed locally, because both ship structural supply-chain defenses (`minimumReleaseAge`, `blockExoticSubdeps`, lifecycle script allowlists, security scanner API) that npm does not. The nudge is **soft by default** (advise + proceed) and **hard via opt-in config** (rewrite the command). Beekeeper itself does not implement the defenses; it routes the agent's request to a package manager that already does.

This is defense-in-depth: Beekeeper's catalog matching is reactive (depends on threat intel freshness); pnpm/Bun strict defaults are proactive (block by structural policy regardless of intel). The two layers stack.

## 2. Scope

### 2.1 In scope
- Detection of npm install commands invoked by agents through hooks, MCP gateway, or shim layer.
- Version detection of locally installed pnpm and bun binaries.
- Two nudge modes: soft (default) and hard (opt-in).
- Compatibility check: Node.js >= 22 required for pnpm 11; surface this honestly.
- Bun nudge includes recommendation to install `@socketsecurity/bun-security-scanner`.
- Audit records for every nudge decision.
- Periodic major-version drift check for pnpm and bun.

### 2.2 Out of scope
- Beekeeper does not configure pnpm or bun settings on the user's machine. Users own their `pnpm-workspace.yaml` and `bunfig.toml`. Beekeeper detects and reports on configuration but does not edit it.
- No Yarn nudge in v1. Yarn Berry has `npmMinimalAge` but the install patterns are different enough to defer.
- No pip/cargo/gem/composer nudge in v1. JavaScript ecosystem first because that's where the threat data is most active.
- No installation of pnpm/bun by Beekeeper. We detect; the user installs.

## 3. Architecture

### 3.1 Module layout
```
beekeeper/internal/nudge/
├── nudge.go              # main entrypoint, decision logic
├── detect.go             # local binary + version detection
├── parse.go              # command parsing for npm/pnpm/bun/npx
├── rewrite.go            # hard-mode command rewriting
├── version.go            # semver checks, drift detection
├── nudge_test.go
├── detect_test.go
├── parse_test.go
├── rewrite_test.go
└── version_test.go
```

### 3.2 Public API
```go
package nudge

// Decision is what the policy engine returns for a parsed install command.
type Decision struct {
    Action      Action   // Proceed | Advise | Rewrite | Block
    Reason      string   // structured reason code, see §6
    Original    string   // original command as invoked
    Rewritten   string   // populated when Action == Rewrite
    Detected    PMState  // what we found on disk
    AuditFields map[string]any
}

type Action int
const (
    Proceed Action = iota
    Advise          // soft nudge: show message, run original
    Rewrite         // hard nudge: replace command with pnpm/bun equivalent
    Block           // npm install attempted with no fallback and policy forbids
)

type PMState struct {
    NpmInstalled  bool
    NpmVersion    string
    PnpmInstalled bool
    PnpmVersion   string         // empty if not installed
    PnpmHardened  bool           // version >= 11.0
    BunInstalled  bool
    BunVersion    string
    BunScannerOK  bool           // @socketsecurity/bun-security-scanner present
    NodeVersion   string         // for pnpm 11 compatibility check
}

// Evaluate is called by the hook handler with the parsed command.
// NOTE (purity): in this codebase Evaluate must take the resolved PMState as
// an argument (detection done by detect.go/an adapter) so the decision stays
// pure — see the editor's note at the top.
func Evaluate(cmd ParsedCommand, cfg Config) Decision
```

### 3.3 Integration points
- **Hook handler** (`internal/check/`): when a `Bash` tool call contains an npm install pattern, the handler calls `nudge.Evaluate` and acts on the `Decision`.
- **MCP gateway** (`internal/gateway/`): same path for MCP tool calls that wrap shell commands.
- **Shim layer** (`internal/shim/`): the `npm` shim calls `nudge.Evaluate` before proxying.
- **Audit log**: `Decision.AuditFields` written as an NDJSON record with `record_type: "nudge"`.

## 4. Decision flow
```
parsed npm install command
    │
    ▼
detect local PM state ─────────────────► cache for 60s
    │
    ▼
is pnpm installed and >=11.0?
    │
    ├─ YES → is config.mode == "hard"?
    │           ├─ YES → Action=Rewrite, cmd → pnpm equivalent
    │           └─ NO  → Action=Advise, message + proceed with npm
    │
    ├─ NO, but bun installed and >=1.3?
    │           ├─ scanner installed?
    │           │       ├─ YES → same hard/soft branch as above
    │           │       └─ NO  → Advise, recommend installing scanner
    │           │                proceed
    │
    └─ NO  → Action=Proceed, log "no hardened pm available"
              (unless config.requireHardened == true → Action=Block)
```
The 60-second detection cache prevents `nudge.Evaluate` from re-running `pnpm --version` on every tool call in a session.

## 5. Configuration

Lives in `beekeeper/config.json` under the `nudge` key:
```json
{
  "nudge": {
    "enabled": true,
    "mode": "soft",
    "requireHardened": false,
    "preferred": "pnpm",
    "checkSocketScanner": true,
    "majorDriftCheck": {
      "enabled": true,
      "interval": "168h"
    },
    "versionFloors": {
      "pnpm": "11.0.0",
      "bun": "1.3.0",
      "node": "22.0.0"
    }
  }
}
```

### 5.1 Defaults
- `enabled: true`: nudge is on out of the box.
- `mode: "soft"`: advise but don't rewrite. Respects agent agency.
- `requireHardened: false`: npm calls proceed even if no hardened PM is available.
- `preferred: "pnpm"`: when both pnpm and bun are installed, pick pnpm. pnpm's defaults are turned on automatically; Bun requires the Socket scanner package which adds a configuration step.
- `checkSocketScanner: true`: for Bun, verify `@socketsecurity/bun-security-scanner` is present before treating Bun as hardened.
- `majorDriftCheck.interval: 168h`: weekly check for pnpm 12 / bun 2 availability.

### 5.2 Configuration MUST/SHOULD
- Beekeeper MUST NOT silently change `mode` between sessions.
- Beekeeper MUST log every config change to the audit trail.
- If `mode == "hard"` and `requireHardened == true` and no hardened PM is installed, Beekeeper MUST block npm install commands with a structured reason pointing to the install guidance.

## 6. Detection rules

### 6.1 Binary detection
```go
func DetectPnpm() (installed bool, version string, err error) {
    // exec.LookPath("pnpm")
    // if found: exec "pnpm --version", parse semver
}
func DetectBun() (installed bool, version string, err error) {
    // exec.LookPath("bun")
    // if found: exec "bun --version", parse semver
}
func DetectNode() (version string, err error) {
    // exec "node --version", parse semver
}
```
All detection commands MUST have a 2-second hard timeout. A timeout returns `(false, "", ErrDetectionTimeout)` and is treated as "not installed."

### 6.2 Socket scanner detection (Bun only)
The Socket Bun scanner is an npm package configured in `bunfig.toml`. Detection:
1. Look for `bunfig.toml` in the project root and `~/.bunfig.toml`.
2. Parse for `[install.security]` section with `scanner = "@socketsecurity/bun-security-scanner"`.
3. If found in either location, `BunScannerOK = true`.

If `bunfig.toml` parsing fails, default `BunScannerOK = false` and log a warning.

### 6.3 pnpm hardening verification
pnpm 11.0+ has security defaults on. Beekeeper verifies these are not overridden:
1. Look for `pnpm-workspace.yaml` in project root.
2. Check `minimumReleaseAge`: if explicitly set to a value less than 60, treat as a configuration weakness and log a warning. Do not block.
3. Check `blockExoticSubdeps`: if explicitly set to false, log a warning.
4. The defaults being on is the baseline; users can opt out but Beekeeper records the choice.

### 6.4 Command parsing
Patterns Beekeeper recognizes as "npm install":
- `npm install [...]`
- `npm i [...]`
- `npm add [...]`
- `npx [...]` (treated as install + execute; same rules)
- Same set prefixed by `sudo` (still parsed, but Beekeeper does not nudge sudo calls: they're a separate threat surface logged but not redirected)

Patterns explicitly NOT nudged:
- `npm ls`, `npm run`, `npm test`, `npm publish`, `npm view`, `npm whoami`: non-install operations.
- `npm install` with no arguments (project-level install from existing lockfile): still nudge candidate, but a softer message because the lockfile already pins versions.

## 7. Version compatibility matrix

| Package Manager | Floor | Reason | Recommended |
|---|---|---|---|
| pnpm | 11.0.0 | `minimumReleaseAge` and `blockExoticSubdeps` on by default | latest 11.x |
| Bun | 1.3.0 | Security Scanner API stable | latest 1.x |
| Node.js (for pnpm 11) | 22.0.0 | pnpm 11 requires Node 22+; ESM-only | latest 22.x LTS |

### 7.1 Major drift handling
The weekly `majorDriftCheck` runs `pnpm view pnpm version` and `bun upgrade --check` (or equivalent metadata fetch). If a new major is detected:
- Log an audit record with `record_type: "version_drift"`, severity `info`.
- Surface in TUI as a status badge in the System Health panel.
- Do NOT auto-update floors. New majors require explicit Beekeeper review (testing against new behavior, updating tests, releasing a new Beekeeper version).

## 8. CLI surface
Add to existing CLI:
```
beekeeper nudge status              # show current PM state and config
beekeeper nudge check <command>     # dry-run: parse a command, show what Beekeeper would do
beekeeper nudge audit [--since=...] # query nudge decisions from audit log
```
Existing config subcommands (`beekeeper policy edit`, `beekeeper config set`) handle nudge configuration like any other policy.

## 9. Audit record schema
Every nudge decision emits one NDJSON record:
```json
{
  "schema_version": "0.1.0",
  "record_type": "nudge",
  "record_id": "uuid-v7",
  "ts": "2026-06-03T14:22:07Z",
  "scanner_name": "beekeeper",
  "scanner_version": "0.3.0",
  "agent_name": "claude-code",
  "tool_name": "Bash",
  "original_command": "npm install chalk@5.4.0",
  "decision": "advise|proceed|rewrite|block",
  "reason_code": "pnpm-available-soft|bun-available-no-scanner|no-hardened-pm|node-incompatible|...",
  "rewritten_command": "pnpm add chalk@5.4.0",
  "pm_state": {
    "npm_version": "10.9.0",
    "pnpm_version": "11.3.0",
    "pnpm_hardened": true,
    "bun_version": "",
    "bun_scanner_ok": false,
    "node_version": "22.5.0"
  },
  "audit_fields": {}
}
```
Reason codes are a closed enum maintained in `internal/nudge/reasons.go`. New reason codes require updating tests.

## 10. Test acceptance criteria
Each numbered criterion is a test that MUST pass before merge.

1. `nudge.Evaluate` returns `Advise` when pnpm >= 11.0 is installed, mode is soft, and command is `npm install foo`.
2. `nudge.Evaluate` returns `Rewrite` with `pnpm add foo` when same setup with mode `hard`.
3. `nudge.Evaluate` returns `Proceed` when no hardened PM is installed and `requireHardened: false`.
4. `nudge.Evaluate` returns `Block` when no hardened PM is installed and `requireHardened: true`.
5. `nudge.Evaluate` returns `Advise` with reason `bun-available-no-scanner` when Bun >= 1.3 is installed but Socket scanner is absent.
6. `nudge.Evaluate` returns `Advise` when pnpm 11 is installed but Node.js < 22 is the active runtime, with reason `node-incompatible-with-pnpm-11`.
7. `npm ls`, `npm run start`, `npm publish` do NOT trigger nudge evaluation (parsed as non-install).
8. `npm install` with no args triggers nudge with a softer reason code than `npm install foo`.
9. `npx some-package` is parsed as install-and-execute and triggers nudge.
10. `sudo npm install foo` is parsed and logged but NOT rewritten (sudo is a separate concern).
11. Detection cache prevents re-running `pnpm --version` more than once per 60 seconds in the same session.
12. Detection timeout (2s) returns gracefully and treats the PM as not installed.
13. `bunfig.toml` parsing failure does not crash the nudge module; `BunScannerOK = false` is the safe fallback.
14. Audit record contains all fields specified in §9 schema, with correct `reason_code` from the closed enum.
15. Weekly drift check correctly identifies a hypothetical "pnpm 12.0.0" as a major drift and logs the audit record.
16. When pnpm `minimumReleaseAge` is explicitly set to 0 in `pnpm-workspace.yaml`, the warning is logged but `pnpm_hardened` is still true (defaults can be overridden, that's the user's choice).
17. Configuration changes via `beekeeper config set nudge.mode hard` are logged to the audit trail.

## 11. Edge cases
- **Monorepo with multiple package managers.** Project root has both `pnpm-lock.yaml` and `package-lock.json`. Treat as pnpm project; the npm lockfile is likely stale or a CI artifact.
- **Workspaces with `engines` constraint.** `package.json` declares `"engines": {"npm": ">=10"}`. Beekeeper respects the user's intent: log the engine constraint but still nudge to pnpm because the security trade-off favors the nudge.
- **Project explicitly excludes nudge.** A `.beekeeper.json` in the project root sets `nudge.enabled: false`. Beekeeper respects this without warning; project-level config wins over user-level config wins over system config.
- **Agent runs `npm install` inside a Docker exec.** Hook catches it. The PM state detected is the host's, not the container's. Decision is logged with `context: "docker-exec"` and proceeds since the container's PM may be different and Beekeeper can't see it.
- **pnpm installed via Corepack vs direct install.** Both detected by `exec.LookPath`. Corepack-shimmed pnpm responds to `--version` identically. No special handling needed.
- **Bun installed but `bunfig.toml` doesn't exist.** Treat `BunScannerOK = false`. Recommend creating `bunfig.toml` with the scanner config; Beekeeper does not create it.
- **Both pnpm and Bun installed, both hardened.** Use `config.preferred`. Default is pnpm.

## 12. Self-defense considerations
This feature adds two effective dependencies to Beekeeper's supply chain trust footprint:
- **pnpm**: when Beekeeper rewrites `npm install` to `pnpm add`, the user's machine ends up trusting pnpm's binary integrity. Beekeeper does not install pnpm; users own that decision. Beekeeper detects what's installed and uses it.
- **`@socketsecurity/bun-security-scanner`**: when Beekeeper recommends Bun, it effectively recommends installing this npm package. The recommendation surfaces Socket as a downstream trust dependency. The recommendation MUST be explicit in the message text: "Bun's Security Scanner API requires a scanner package. We recommend @socketsecurity/bun-security-scanner, which is an npm package published by Socket Inc."

Neither dependency is bundled into Beekeeper itself. Both are tools the user already has or chooses to install. The trust footprint expansion is real but bounded: Beekeeper does not become a distribution channel for either.

Audit log records every nudge so forensic review can trace which agent commands led to which package manager invocations.

## 13. Documentation requirements
- `docs/nudge.md` explains the feature, the version floors, the soft vs hard distinction, and the Node 22 caveat for pnpm 11.
- Default `config.json` ships with the nudge block populated with defaults and inline comments explaining each field.
- `beekeeper nudge status` output must be human-readable, not just NDJSON, so an operator can quickly verify their setup.

## 14. Open questions
None. This feature is fully specified. Proceed to implementation.

---
*End of feature PRD.*
