# Feature Research

**Domain:** Agent runtime safety harness / developer workstation security — Milestone v1.2.0 "Runtime Behavioral Hardening"
**Researched:** 2026-06-03
**Confidence:** HIGH (command mappings verified against pnpm/bun official docs; severity conventions verified against OSV schema spec + pnpm 11 release notes; sensitive-path blocklist grounded in Claude Code issue tracker and AI agent security literature)

> **Scope note:** This file covers the three v1.2.0 features: PLCY-05 (sensitive-path enforcement wiring), NUDGE (package-manager nudge), and PLCY-07 (corroboration hardening). The v1.0.0 milestone feature landscape is preserved at the bottom of this file. The downstream consumer for this research is `rewrite.go` authors and roadmap planners deriving REQ-IDs and success criteria.

---

## Part I: v1.2.0 Feature Research

### 1. npm → pnpm/bun Command Equivalence Mapping

**Confidence:** HIGH. Verified directly against pnpm `pnpm add` docs (pnpm.io/cli/add), bun `bun add` docs (bun.sh/docs/cli/add), and bun `bunx` docs (bun.sh/docs/cli/bunx). Sources agree on all flag mappings below.

#### 1.1 Install-with-packages (`npm install <pkg>` / `npm i <pkg>` / `npm add <pkg>`)

| npm command | pnpm equivalent | bun equivalent | Notes |
|-------------|-----------------|----------------|-------|
| `npm install <pkg>` | `pnpm add <pkg>` | `bun add <pkg>` | Production dep; both default to production |
| `npm i <pkg>` | `pnpm add <pkg>` | `bun add <pkg>` | `npm i` is a first-class alias |
| `npm add <pkg>` | `pnpm add <pkg>` | `bun add <pkg>` | `npm add` is a first-class alias |
| `npm install --save-dev <pkg>` | `pnpm add --save-dev <pkg>` | `bun add --dev <pkg>` | pnpm: `--save-dev` or `-D`; bun: `--dev`, `-d`, `-D`, `--development` |
| `npm install -D <pkg>` | `pnpm add -D <pkg>` | `bun add -D <pkg>` | Short flag; `-D` is valid for all three |
| `npm install --save-optional <pkg>` | `pnpm add --save-optional <pkg>` | `bun add --optional <pkg>` | pnpm: `--save-optional` or `-O`; bun: `--optional` |
| `npm install -O <pkg>` | `pnpm add -O <pkg>` | `bun add --optional <pkg>` | bun has no `-O` short form; expand to `--optional` |
| `npm install --global <pkg>` | `pnpm add --global <pkg>` | `bun add --global <pkg>` | Both support `--global` and `-g` |
| `npm install -g <pkg>` | `pnpm add -g <pkg>` | `bun add -g <pkg>` | Short flag identical |
| `npm install --save-exact <pkg>` | `pnpm add --save-exact <pkg>` | `bun add --exact <pkg>` | pnpm: `--save-exact` or `-E`; bun: `--exact` or `-E` |
| `npm install -E <pkg>` | `pnpm add -E <pkg>` | `bun add -E <pkg>` | Short flag identical |
| `npm install <pkg>@1.2.3` | `pnpm add <pkg>@1.2.3` | `bun add <pkg>@1.2.3` | Version spec syntax identical across all three |
| `npm install <pkg>@latest` | `pnpm add <pkg>@latest` | `bun add <pkg>@latest` | Tag spec syntax identical; this is the risky unpin case (see §3) |
| `npm install <pkg>@^1.2.0` | `pnpm add <pkg>@^1.2.0` | `bun add <pkg>@^1.2.0` | Range spec syntax identical; also a risky case |

**Implementation note for `rewrite.go`:** Flag preservation is straightforward for the flags above. The only flag that requires transformation is `--save-optional`/`-O` (pnpm keeps it as-is; bun must expand to `--optional`). All other flags either pass through identically or have the same short form.

**Flag preservation implementation guidance:**
- Strip `npm install`, `npm i`, or `npm add` as the command verb. Replace with target PM verb.
- For pnpm: pass all remaining flags and package args through unchanged (pnpm flag names are very close to npm's).
- For bun: handle these specific transforms:
  - `--save-dev` → `--dev` (or keep `-D` unchanged)
  - `--save-optional` → `--optional`
  - `--save-exact` → `--exact`
  - `-O` → `--optional`
  - `-o` → `--optional`
  - All other flags that pnpm/bun do not support: drop and log to audit as `flag_dropped`.

#### 1.2 No-argument install (`npm install` / `npm i`)

`npm install` with no arguments installs all dependencies from the existing `package.json`, restoring `node_modules` from the lockfile. This is the "lockfile install" pattern used in CI and after git pull.

| npm command | pnpm equivalent | bun equivalent | Notes |
|-------------|-----------------|----------------|-------|
| `npm install` | `pnpm install` | `bun install` | Both restore from lockfile |
| `npm i` | `pnpm install` | `bun install` | `npm i` maps to `pnpm install` / `bun install` not `pnpm add` / `bun add` |
| `npm install --production` | `pnpm install --prod` | `bun install --production` | Production-only; pnpm uses `--prod`, bun uses `--production` |
| `npm install --frozen-lockfile` | `pnpm install --frozen-lockfile` | `bun install --frozen-lockfile` | CI-mode lockfile enforcement; flag identical |
| `npm ci` | `pnpm install --frozen-lockfile` | `bun ci` | `bun ci` is a first-class alias for `bun install --frozen-lockfile` |

**UX treatment for no-argument form:** The NUDGE-PRD specifies "still nudge candidate, but a softer message because the lockfile already pins versions." This is correct. The no-argument form does not add new packages; it hydrates from a lockfile that was presumably pinned when written. The nudge message should differ:
- **With packages:** "Agent is installing `chalk@latest` via npm. pnpm with `minimumReleaseAge` enabled provides supply-chain protection. Consider: `pnpm add chalk@latest`"
- **Without packages (lockfile restore):** "Agent is running `npm install` to restore dependencies. If you migrate this project to pnpm, supply-chain defaults apply on every future install. No action required for this run."

Reason code for no-argument form: `npm-lockfile-restore` (distinct from `npm-install-package`).

#### 1.3 `npx <package>` → pnpm dlx / bun x

`npx` fetches and executes a package binary without permanently installing it. Both pnpm and bun have direct equivalents.

| npx command | pnpm equivalent | bun equivalent | Notes |
|-------------|-----------------|----------------|-------|
| `npx <pkg>` | `pnpm dlx <pkg>` | `bun x <pkg>` or `bunx <pkg>` | Both auto-fetch from registry if not cached |
| `npx <pkg>@version` | `pnpm dlx <pkg>@version` | `bun x <pkg>@version` | Version pinning syntax identical |
| `npx -y <pkg>` | `pnpm dlx <pkg>` | `bun x <pkg>` | `-y` (skip prompt) is the default for pnpm dlx and bun x |
| `npx --package=<pkg> <cmd>` | `pnpm dlx --package=<pkg> <cmd>` | `bunx --package <pkg> <cmd>` | When binary name differs from package name |
| `npx create-<x> [args]` | `pnpm dlx create-<x> [args]` | `bun x create-<x> [args]` or `bun create <x> [args]` | Scaffolding pattern |

**pnpm dlx security note (HIGH confidence):** pnpm dlx inherits pnpm 11's `minimumReleaseAge` and `trustPolicy` settings (confirmed by pnpm.io/cli/dlx). This means `pnpm dlx some-scaffolder` refuses to execute packages published less than 24 hours ago by default. `npx` has no such gate. This is the concrete security argument for the rewrite.

**bun x security note:** `bunx` (alias for `bun x`) auto-installs and caches the package globally. It inherits bun's lifecycle script policy (no scripts by default unless in `trustedDependencies`). This is the security gain over npx.

**pnpm dlx alias:** pnpm also accepts `pnpx` and `pnx` as aliases for `pnpm dlx`. `rewrite.go` should emit `pnpm dlx` (canonical form), not the aliases.

#### 1.4 Complete rewrite.go decision table

The following is the complete parser → rewriter decision table. Implement each row as a case in the command rewriter.

```
Input pattern                         → pnpm rewrite                    → bun rewrite
─────────────────────────────────────────────────────────────────────────────────────
npm install                           → pnpm install                    → bun install
npm i                                 → pnpm install                    → bun install
npm install <flags> <pkgs>            → pnpm add <mapped-flags> <pkgs>  → bun add <mapped-flags> <pkgs>
npm i <flags> <pkgs>                  → pnpm add <mapped-flags> <pkgs>  → bun add <mapped-flags> <pkgs>
npm add <flags> <pkgs>                → pnpm add <mapped-flags> <pkgs>  → bun add <mapped-flags> <pkgs>
npx <flags> <pkg>[@ver] [args]        → pnpm dlx <pkg>[@ver] [args]    → bun x <pkg>[@ver] [args]
```

Flag mapping (applies to the `<mapped-flags>` column):

```
npm flag            → pnpm flag         → bun flag
────────────────────────────────────────────────────
--save-dev          → --save-dev        → --dev
-D                  → -D                → -D
-d                  → -d                → -d
--development       → (drop)            → --development
--save-optional     → --save-optional   → --optional
-O                  → -O                → --optional
-o                  → -o                → --optional
--save-exact        → --save-exact      → --exact
-E                  → -E                → -E
--global            → --global          → --global
-g                  → -g                → -g
--production        → --prod            → --production
-P (install only)   → --prod            → --production
--frozen-lockfile   → --frozen-lockfile → --frozen-lockfile
--ignore-scripts    → --ignore-scripts  → --ignore-scripts
--workspace         → --workspace       → (no equiv — drop, log)
--save-peer         → --save-peer       → --peer
```

Flags not in this table: preserve for pnpm (likely recognized), drop for bun with `flag_dropped` audit entry.

---

### 2. Version Pinning Conventions and the Risky Spec Cases

**Confidence:** HIGH. Grounded in the 2026 Axios compromise postmortem (Microsoft Security Blog, April 2026), the Mini Shai-Hulud/TeamPCP wave, and the arxiv paper "Pinning Is Futile" (2502.06662) which documents npm ecosystem pinning failure modes at scale.

#### 2.1 Why exact pinning matters for supply-chain safety

The core risk: npm's version resolution algorithm resolves ranges at install time, not at lockfile creation time. When `package.json` contains `"axios": "^1.14.0"` and an attacker publishes `axios@1.14.1` with a backdoor, every developer and CI run that does `npm install` (without a lockfile, or with a stale lockfile) installs the backdoor automatically.

The 2026 Axios compromise is the canonical example. The attacker hijacked the lead maintainer's npm account and published two poisoned versions across both the 1.x and legacy 0.x release branches within 39 minutes. Caret ranges (`^1.14.0`) silently pulled the next minor/patch. The same blast-radius strategy appeared in the Mini Shai-Hulud campaign: publishing across two major version lines simultaneously maximizes coverage across `~9.1.x`, `~9.2.x`, `^9`, `^12`, `~12.0` pinners.

**Version spec risk ranking (for NUDGE to surface):**

| Spec pattern | Risk level | Why |
|---|---|---|
| `pkg@latest` | CRITICAL | Resolves to whatever is newest at install time. No pinning whatsoever. One poisoned publish → immediate exposure. |
| `pkg` (no version) | CRITICAL | npm defaults to `@latest`. Identical risk to explicit `@latest`. |
| `pkg@^major.minor.patch` | HIGH | Allows any minor or patch bump. The Axios and Mini Shai-Hulud attacks exploited this. |
| `pkg@~major.minor.patch` | MEDIUM | Allows patch bumps only. Narrower blast radius, but patch-bump attacks exist (e.g., node-ipc 10.1.2 protestware). |
| `pkg@major.minor.patch` | LOW | Exact pinning. Attacker must overwrite the specific version or attack the lockfile. |
| `pkg@major.minor.patch` + lockfile committed + `npm ci` | LOWEST | npm ci refuses to install if package.json and lockfile disagree. Integrity verification on every install. |

**NUDGE should flag:** `@latest`, bare package name (no `@version`), and caret ranges (`^`). Tilde ranges are a medium concern — do not block on tilde alone, but include in the advisory message.

#### 2.2 What the nudge message should say for pinning

The message should be:
1. Specific about what risk was detected (not generic security lecture)
2. Actionable (exact command to fix)
3. Non-blocking by default (soft-advise mode)

**Good nudge message pattern (for unpinned spec):**
```
[NUDGE] npm install chalk@latest — version @latest is unresolved at install time.
Supply-chain risk: any package published between now and your next install can be
resolved automatically. Consider pinning:
  pnpm add chalk@5.4.0      (resolve current @latest, then pin exact version)
  bun add chalk@5.4.0 -E    (--exact flag enforces no-range in package.json)
Proceeding with original npm command.
```

**Good nudge message pattern for hardened PM not installed:**
```
[NUDGE] npm install detected. pnpm 11+ provides structural supply-chain protection
(minimumReleaseAge=24h, lifecycle script allowlisting). pnpm is not installed.
To enable protection: install pnpm v11+ (https://pnpm.io/installation), then
re-run: pnpm add chalk@latest
Proceeding with npm.
```

The message must never say "blocked" when the action is `Advise` — it proceeds. The word "blocked" in a soft-advise message trains agents to interpret nudges as failures.

#### 2.3 OSV/CVSS severity alignment for PLCY-07

OSV schema severity field (verified against ossf.github.io/osv-schema):

```json
"severity": [
  { "type": "CVSS_V3", "score": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" },
  { "type": "CVSS_V4", "score": "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N" }
]
```

OSV does not define a textual mapping from CVSS score to critical/high/medium/low. Individual databases map in their `database_specific` fields. The industry standard CVSS → severity mapping (FIRST.org, NVD, used by GitHub Advisory Database, Bumblebee):

| CVSS v3.x score | Severity label |
|-----------------|----------------|
| 9.0 – 10.0 | Critical |
| 7.0 – 8.9 | High |
| 4.0 – 6.9 | Medium |
| 0.1 – 3.9 | Low |

Bumblebee catalog entries use a direct `"severity": "critical"` / `"high"` / `"medium"` / `"low"` string field (not the CVSS vector). The OSV-sourced advisories carry CVSS vectors; Beekeeper's catalog normalizer must parse the CVSS score and map to the Bumblebee severity enum.

**For PLCY-07 purposes:** a `severity: "critical"` catalog match from ANY catalog source (including Bumblebee with `Signed: false`) must escalate to block. The reasoning: the existing `BlockAt: 2` threshold was designed for the case where we cannot trust any single source. But a `severity: "critical"` designation is a qualitatively different signal — it means the catalog author has assessed the package as actively malicious with maximum impact. In the Shai-Hulud case, the OSV advisory `MAL-2026-4126` carried CVSS 9.8 (Critical, Remote, No Auth). Blocking on a single critical advisory from a trusted-ish source is the industry norm.

---

### 3. Soft-Advise vs Hard-Rewrite UX Norms

**Confidence:** MEDIUM-HIGH. Verified against npq behavior (github.com/lirantal/npq), Socket safe-npm FAQ, and Claude Code hooks documentation.

#### 3.1 The spectrum of interception tools

**npq (lirantal/npq):** Two-tier enforcement:
- **Errors → Block**: Hard block, install halts until user explicitly proceeds. Used for high-confidence bad signals.
- **Warnings → 15-second countdown with abort option**: Auto-continue after timeout. Respects developer time while ensuring visibility.
- `--disable-auto-continue` flag for strict environments.

The 15-second countdown is a key UX insight: it creates a forced pause (visibility) without requiring manual override for every package in a bulk install.

**Socket safe-npm:** Intercepts pre-install via PATH wrapping. For confirmed malware (Socket's human-reviewed set): blocks hard. For AI-flagged but unconfirmed: warns and prompts. The distinction between "confirmed malware (block)" and "suspicious signal (warn + prompt)" is the right mental model for Beekeeper too.

**Claude Code hooks (exit code semantics, verified against code.claude.com/docs/en/agent-sdk/hooks):**
- Exit 0: allow, agent proceeds
- Exit 2: block, agent receives structured error message and cannot retry the same tool call
- Non-zero exit other than 2: block (fail-closed)
- Stdout from hook handler becomes the message shown to the agent/user

There is **no native "rewrite" exit code** in Claude Code hooks. The hook can block (exit 2) and put the rewritten command in the output, which the agent can then re-issue. This means "hard rewrite" in Beekeeper's model is implemented as: block original command + emit rewritten command in output, trusting the agent to re-issue. This is the practical constraint that makes "Rewrite" a distinct action type even though hooks only have Allow/Block.

**For the NUDGE module, the UX implications are:**

| NUDGE action | Hook exit code | Stdout content | Agent behavior |
|---|---|---|---|
| Proceed | 0 | (empty or info-only) | Runs original command |
| Advise | 0 | Advisory message | Runs original command; agent sees message but proceeds |
| Rewrite | 2 (block) | Rewritten command text | Agent cannot run original; expected to run rewritten command |
| Block | 2 (block) | Explanation + install guidance | Agent cannot run npm install at all |

**Advise is the least intrusive.** It respects agent agency completely: the agent runs what it requested, and the human operator sees the advisory in the audit log and TUI. This is the correct default for a tool used in autonomous agent sessions where interruptions are costly.

**Rewrite requires trust.** Rewriting the command means Beekeeper is making a functional change to what the agent does. The agent asked for `npm install chalk`, Beekeeper runs `pnpm add chalk`. The semantics should be equivalent, but edge cases exist (different lockfile format, different `node_modules` layout for bun isolated installs vs npm hoisted). Hard-rewrite must be opt-in.

#### 3.2 Agent-specific UX considerations

In autonomous agent sessions, advisory messages in hook output are visible to the agent's context. A well-formed advisory message can itself guide the agent: "Consider re-issuing as `pnpm add chalk@5.4.0`" functions as a suggestion the agent can act on voluntarily. This is softer than a forced rewrite and more appropriate when the agent has a task to complete.

**Anti-pattern to avoid:** Do not emit a wall of security text on every `npm install`. In a session where an agent installs 50 packages, 50 advisory messages create noise that trains developers to disable the nudge. The advisory message must be concise (3-5 lines max) and appear once per session per package manager (after first advisory, subsequent npm installs in the same session get abbreviated: `[NUDGE] npm detected (advised). See first advisory. Proceeding.`).

---

### 4. Sensitive-Path Detection Norms

**Confidence:** HIGH. Blocklist derived from: the Claude Code issue #46741 community request (credential paths explicitly named); the AI agent security guardrails article (dev.to/maxkrivich); the MCP config path research (mcpplaygroundonline.com); and cross-referenced with pnpm supply-chain security documentation which names the specific files malware targets.

#### 4.1 Canonical sensitive-path blocklist

These paths are credential-bearing across the industry consensus. Every path below appears in at least two independent sources.

**Cloud credentials:**
- `~/.aws/credentials` — AWS access key ID + secret
- `~/.aws/config` — AWS profile configuration (may contain credential references)
- `~/.azure/` — Azure CLI credentials (token cache)
- `~/.config/gcloud/` — Google Cloud authentication tokens
- `~/.config/gcloud/application_default_credentials.json` — ADC credentials

**SSH:**
- `~/.ssh/` (entire directory) — Private keys, authorized_keys, known_hosts
- `~/.ssh/id_rsa`, `~/.ssh/id_ed25519`, `~/.ssh/id_ecdsa` — Private key files
- `~/.ssh/config` — SSH configuration (may reveal targets)

**GPG:**
- `~/.gnupg/` — GPG private keys, trustdb

**Container / Kubernetes:**
- `~/.kube/config` — Kubernetes cluster credentials, bearer tokens
- `~/.docker/config.json` — Docker registry auth tokens

**Version control / source hosting:**
- `~/.git-credentials` — git credential store (plaintext tokens)
- `~/.config/gh/` — GitHub CLI auth tokens (via `gh auth login`)
- `~/.config/hub` — Hub CLI credentials

**Package registries:**
- `~/.npmrc` — npm auth token (`//registry.npmjs.org/:_authToken=...`)
- `~/.pypirc` — PyPI upload credentials
- `~/.netrc` — Multi-service credential file (username/password pairs)

**Terraform / secrets managers:**
- `~/.terraform.d/` — Terraform credentials file with token
- `~/.vault-token` — HashiCorp Vault token

**Project-local (relative path patterns):**
- `.env` — Root .env file (most common credential store in dev projects)
- `.env.*` — `.env.local`, `.env.production`, `.env.test`, etc.
- `*.pem` — PEM-encoded certificates/private keys
- `*.key` — Private key files (broad pattern)
- `**/secrets/**` — Any file under a `secrets/` directory

**MCP config files (agent-specific, HIGH priority for Beekeeper's use case):**
- `~/.claude.json` — Claude Code MCP server configuration (may contain tokens, server URLs)
- `~/Library/Application Support/Claude/claude_desktop_config.json` — macOS
- `%APPDATA%\Claude\claude_desktop_config.json` — Windows
- `~/.config/Claude/claude_desktop_config.json` — Linux
- `~/.cursor/mcp.json` — Cursor MCP config
- `.cursor/mcp.json` — Project-level Cursor MCP config
- `~/.codeium/windsurf/mcp_config.json` — Windsurf MCP config
- `~/.mcp.json` — Generic MCP config

#### 4.2 Platform-specific path normalization

The blocklist is written in Unix notation. For Windows, Beekeeper's sensitive-path engine must expand:
- `~` → `%USERPROFILE%` (e.g., `C:\Users\<user>`)
- `~/.aws/` → `C:\Users\<user>\.aws\` (AWS CLI on Windows uses the same path)
- `~/Library/...` → not applicable on Windows (skip macOS-specific paths)
- `%APPDATA%\Claude\...` → add Windows-specific MCP paths

The existing `EvaluatePath`/`DefaultSensitivePaths` engine already does this normalization per the CLAUDE.md architecture. The research confirms the blocklist is correct; the wiring gap is that the engine output is not being used in the live check handler.

#### 4.3 Allowlist patterns

The blocklist must have an allowlist escape hatch for legitimate agent workflows:

**Legitimate accesses that should NOT be blocked:**
- Reading `~/.kube/config` by a DevOps tool explicitly configured to manage clusters
- Reading `~/.npmrc` by a task that is publishing a package (npm publish workflow)
- Reading `.env` to validate env var presence (not to exfiltrate content)

**Allowlist mechanism:** The policy overlay (`~/.beekeeper/policies/`) can override the default blocklist per path and per agent tool. An allowlist entry should require:
- Specific path (not a broad glob)
- Optional: specific tool (Read/Write/Edit/Bash)
- Optional: specific reason (stored in audit record)

**Warn vs block conventions:**

| Scenario | Recommended action | Rationale |
|---|---|---|
| Agent reads `~/.ssh/id_rsa` | Block | Private key has no legitimate read use case for an agent. Exfiltration risk is existential. |
| Agent reads `~/.aws/credentials` | Block | Same. The malware pattern from the Shai-Hulud postmortem is exactly this. |
| Agent reads `.env` | Warn (first time), then block if no allowlist | `.env` has legitimate uses (validate env vars exist). First access warns; subsequent reads without allowlist escalate to block. |
| Agent writes to `~/.npmrc` | Block | Writing to npmrc changes package registry auth — almost certainly not a legitimate agent task. |
| Agent writes to `~/.ssh/authorized_keys` | Block | Backdoor installation. |
| Agent reads MCP config files | Warn | MCP configs may be legitimate for tooling; warn and audit but do not block by default. |

Default enforcement: Block for SSH/cloud credentials/GPG/git-credentials/netrc/vault-token. Warn for `.env`/MCP configs/package registry configs. This matches the layered severity model in the existing sensitive-path engine.

---

### 5. Severity → Enforcement Mapping Conventions (PLCY-07)

**Confidence:** HIGH. Based on CVSS v3.1/v4.0 specification (FIRST.org), OSV schema spec (ossf.github.io/osv-schema), and Bumblebee catalog schema. Corroborated by industry practice from Socket (human-reviewed malware → hard block) and npq (errors → block, warnings → countdown).

#### 5.1 CVSS severity definitions (authoritative)

CVSS v3.x score ranges (FIRST.org, NVD, GitHub Advisory Database standard):

| Score range | Label | Enforcement implication |
|---|---|---|
| 9.0 – 10.0 | **Critical** | Auto-block on single high-confidence source |
| 7.0 – 8.9 | **High** | Block at corroboration threshold (2+ sources) |
| 4.0 – 6.9 | **Medium** | Warn at 1 source; block at 2+ sources |
| 0.1 – 3.9 | **Low** | Audit only; no enforcement default |

The 9.0+ Critical threshold is where OSV `MAL-*` advisories for actively exploited malware consistently land. `MAL-2026-4126` (Shai-Hulud worm, the specific incident that surfaced the PLCY-07 gap) is CVSS 9.8.

#### 5.2 The PLCY-07 problem and fix

**Current state:** Bumblebee catalog entries have `Signed: false` because they are unsigned (the Bumblebee project does not sign individual entries). Beekeeper's corroboration engine treats unsigned sources as weight 1 out of 2 required to block. Result: a single Bumblebee match for a critical-severity package produces `CorroborationCount: 1 < BlockAt: 2` → WARN, not BLOCK. This allowed `npm install ai-figure` (Shai-Hulud, CVSS 9.8) to pass with a warning.

**Fix options and recommendation:**

Option A — Per-severity escalation (recommended):
- When `severity == "critical"` in the catalog match, set `BlockAt = 1` regardless of source trust.
- Rationale: The catalog author has made a high-confidence determination. Critical malware should block on a single catalog source. The `Signed` flag addresses impersonation risk, not catalog accuracy risk.
- Sanity bounds required: add a `MaxCriticalBlockPerSyncCycle` limit (e.g., 20 new critical blocks per sync) to detect a compromised catalog pushing mass false positives.
- False-positive rigor: document in `docs/threat-model.md` that critical-severity escalation is the tradeoff, and that the sanity bound is the backstop.

Option B — Treat bundled catalog as signed-equivalent:
- Flip the internal `Signed` flag for the bundled Bumblebee catalog entries.
- Simpler but changes the trust model more broadly (affects high/medium too).
- Not recommended: creates the impression that all Bumblebee entries are "trusted" when the goal is per-severity escalation specifically.

Option C — OSV as second source for critical advisories:
- When severity is critical and Bumblebee matches, auto-query OSV for the same package+version. If OSV also has a record, escalate to block.
- Adds network call on the hot path. Violates the sub-100ms target for `beekeeper check`. Not recommended for the sync hot path; could work as a background corroboration enrichment.

**Recommended implementation (Option A + sanity bounds):**

```go
// In internal/policy/policy.go (pseudo-code, keep pure)
type CorroborationConfig struct {
    WarnAt                  int  // default: 1
    BlockAt                 int  // default: 2
    QuarantineAt            int  // default: 3
    CriticalBlockAtSingle   bool // default: true (PLCY-07 fix)
    CriticalSanityBound     int  // default: 20 new critical blocks per sync cycle
}

func Corroborate(matches []CatalogMatch, cfg CorroborationConfig) Decision {
    if cfg.CriticalBlockAtSingle {
        for _, m := range matches {
            if m.Severity == "critical" {
                return Decision{Action: Block, Reason: "critical-single-source"}
            }
        }
    }
    // existing corroboration logic...
}
```

#### 5.3 What "critical" means in OSV/Bumblebee context

For the npm/PyPI/cargo ecosystems, `severity: "critical"` in the Bumblebee catalog means actively exploited malware, not theoretical vulnerability. Bumblebee follows the OSV `MAL-*` advisory prefix specifically for malware (as opposed to `GHSA-*` for vulnerabilities). The distinction:
- `MAL-*`: Malicious package (backdoor, infostealer, cryptominer). CVSS 9.0+ common. Block immediately.
- `GHSA-*` Critical: Critical CVE in a legitimate package (RCE, auth bypass). Block at 2-source corroboration (the package may have patched versions).
- `GHSA-*` High: Block at 2-source corroboration per existing policy.

The PLCY-07 fix should target `MAL-*` or `severity: "critical"` specifically, not all high-severity advisories. This keeps the false-positive surface narrow.

---

## Part II: v1.2.0 Feature Landscape

### Table Stakes (Users Expect These for v1.2.0)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Sensitive-path engine wired into live check handler | Engine exists and tests pass. Users who read CLAUDE.md see it listed as a requirement. The gap (returns ALLOW for credential reads) is a bug, not a missing feature. | LOW | Pure wiring work: call `EvaluatePath()`/`DefaultSensitivePaths()` from `internal/check/handler.go`. Allowlist + policy-overlay merge already specced. |
| `npm install` → pnpm/bun nudge (soft-advise mode) | Any tool that claims supply-chain defense and does not intercept `npm install` is visibly incomplete post-2026 Axios/Shai-Hulud. Developers who read pnpm 11 release notes expect this capability. | MEDIUM | `internal/nudge/` new package. Decision is pure function over detected PMState. I/O in `detect.go`. |
| Critical-severity catalog match blocks (not warns) | After the Shai-Hulud postmortem made it into agent community Discords, the expectation is set: known malware = block, not warn. Any tool that warns on CVSS 9.8 packages is broken by definition. | LOW | Policy engine change: per-severity escalation in `internal/policy/policy.go`. Sanity bound required. |
| Behavioral test suite (BTEST) | Without tests exercising these new behaviors end-to-end, the fixes are not trustworthy. Integration tests that replay the exact check inputs that exposed F1/F2/F3 are the proof. | MEDIUM | Table-driven pure-policy tests + stdin→decision integration tests + live-binary E2E battery. |
| Audit records for nudge decisions | The audit log is already established as the source of truth. A new NUDGE action with no audit record is a gap in the forensic trail. | LOW | New `record_type: "nudge"` with the schema from NUDGE-PRD §9. Fits into existing `internal/audit/` without structural change. |

### Differentiators (v1.2.0 Specific)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Hard-rewrite mode for npm→pnpm/bun (opt-in) | No existing tool rewrites agent package manager commands to hardened alternatives. Socket safe-npm wraps the install but does not change the package manager. pnpm's own `minimumReleaseAge` only applies when pnpm is already used. Beekeeper hard-rewrite is the only mechanism that bridges this gap for `npm`-using agents. | MEDIUM | Requires rewrite.go flag-mapping table (see §1.4). Opt-in via `nudge.mode: "hard"`. Block-then-emit-rewrite pattern for Claude Code hook compatibility. |
| `beekeeper nudge status` / `nudge check` CLI | Operators need to verify their setup without running a real install. No competing tool provides a dry-run PM nudge inspector. | LOW | New subcommand group. Reads config and detected PMState. Human-readable output. |
| Node.js 22 compatibility gate for pnpm 11 | Surfacing the pnpm 11 / Node 22 dependency honestly and early avoids silent failures in mixed-version environments. No other tool does this check. | LOW | `detect.go` checks node version; `nudge_node_incompatible_with_pnpm_11` reason code. |
| Weekly major-version drift check for pnpm/bun | Alerts when pnpm 12 or bun 2 releases, so Beekeeper can validate new defaults and update floors before users hit behavior changes silently. | LOW | Background check via version metadata fetch. Audit record + TUI badge. |

### Anti-Features (Explicitly Out of Scope for v1.2.0)

| Feature | Why Requested | Why Problematic / Out of Scope | Better Approach |
|---------|---------------|--------------------------------|-----------------|
| Yarn Berry nudge | Yarn Berry has `npmMinimalAge`. Developers using Yarn might want the same nudge. | "Yarn Berry has different enough install patterns" (NUDGE-PRD §2.2). The command surface (`yarn add` vs `yarn dlx` vs `yarn exec`) is sufficiently different that getting it wrong has a higher risk than deferring. | v1.3.0 consideration after pnpm/bun nudge is validated. |
| pip/cargo/gem/composer nudge | JavaScript is not the only ecosystem attacked. Supply-chain attacks in PyPI and cargo are documented. | The Nudge feature is JavaScript-first because that is where the 2026 threat data is most active (Shai-Hulud, TeamPCP, Axios). Multi-ecosystem nudge requires per-ecosystem hardened-PM research that does not exist in clean form yet. | v1.3.0 if agent-triggered PyPI/cargo attacks are documented. |
| Auto-install of pnpm/bun | Reducing friction to zero. If pnpm is better, just install it. | Beekeeper installing additional tools on the user's machine without explicit consent is an overreach that mirrors the attack pattern it defends against. | Document install path in nudge advisory message; do not execute. |
| Beekeeper editing `pnpm-workspace.yaml` or `bunfig.toml` | Auto-configure the hardened PM on the user's behalf. | Same overreach concern. Users own their PM configuration. Beekeeper detects and reports configuration weaknesses but never edits PM config. | Surface config weakness in `beekeeper nudge status`. |
| Weighted corroboration for critical advisories | "Bumblebee should count as 1.5 votes if the severity is critical." | Option A (per-severity escalation to block at 1 source) is simpler, auditable, and achieves the same goal without weighted math. Weighted systems are harder to reason about and audit. | Flat per-severity escalation (PLCY-07 fix). |
| Blocking on `@latest` in soft-advise mode | `@latest` is risky; block it by default. | Blocking `npm install react@latest` in soft-advise mode contradicts the feature's own design principle (soft = advise + proceed). Hard blocking on version spec alone, without a catalog hit, generates false positives on every legitimately maintained package that publishes regularly. | Flag as risky in the advisory message. Block only when `requireHardened: true` AND no hardened PM is installed, not based on version spec alone. |

---

## Feature Dependencies

```
[PLCY-05 sensitive-path wiring]
    └──requires──> [EvaluatePath() in internal/policy] (already built; pure function)
    └──requires──> [DefaultSensitivePaths() blocklist] (already built)
    └──wires-into──> [internal/check/handler.go] (the gap being closed)
    └──wires-into──> [internal/gateway/] (MCP tool calls that access files)
    └──enables──> [BTEST: F2 integration tests]

[NUDGE package-manager nudge]
    └──requires──> [internal/nudge/detect.go] (binary detection, new)
    └──requires──> [internal/nudge/parse.go] (command parsing, new)
    └──requires──> [internal/nudge/rewrite.go] (flag mapping, new)
    └──requires──> [internal/nudge/nudge.go] (pure Evaluate(), new)
    └──wires-into──> [internal/check/handler.go] (hook handler)
    └──wires-into──> [internal/gateway/] (MCP proxy)
    └──wires-into──> [internal/shim/shim.go] (npm PATH shim)
    └──writes-to──> [internal/audit/] (record_type: "nudge")
    └──enables──> [beekeeper nudge status|check|audit CLI]
    └──enables──> [BTEST: F3 integration tests]
    └──conflicts-with──> [pnpm/bun not installed] (degrades to Proceed gracefully)

[PLCY-07 corroboration hardening]
    └──requires──> [internal/policy/policy.go] (CorroborationConfig, pure function)
    └──requires──> [catalog severity field parsing] (Bumblebee "severity": "critical")
    └──requires──> [sanity bound: MaxCriticalBlockPerSyncCycle]
    └──enables──> [BTEST: F1 integration tests]
    └──documents-in──> [docs/threat-model.md] (critical escalation rationale)

[BTEST behavioral test suite]
    └──requires──> [PLCY-05, NUDGE, PLCY-07 implementations] (what to test)
    └──requires──> [fixture: ai-figure catalog entry] (F1 regression)
    └──requires──> [fixture: ~/.aws/credentials read attempt] (F2 regression)
    └──requires──> [fixture: npm install ai-figure command] (F3 regression)
    └──enhances──> [CI release gate] (fuzz + behavioral tests)
```

### Dependency Notes

- **NUDGE purity constraint (CLAUDE.md):** `internal/policy` must remain a pure function library. Therefore `nudge.Evaluate()` takes a caller-resolved `PMState` as argument; detection I/O lives in `detect.go`. This mirrors the existing `policy.EvaluateReleaseAge(ReleaseAgeInput, …)` pattern. Do not let `Evaluate()` exec subprocesses.
- **PLCY-05 is pure wiring:** The engine already exists and its tests pass. The scope is `handler.go` + `gateway/` call sites. Risk is low; the main complexity is ensuring the allowlist policy overlay is correctly merged before the call.
- **PLCY-07 requires sanity bounds:** The critical escalation is a trust expansion. The sanity bound (`MaxCriticalBlockPerSyncCycle`) is the backstop against a compromised catalog pushing mass critical entries. This must be implemented alongside the escalation, not deferred.
- **BTEST is cross-cutting:** Every phase in v1.2.0 must include the behavioral test that proves the gap is closed. BTEST is not a separate phase; it is a required deliverable within each of F1/F2/F3.

---

## MVP Definition for v1.2.0

### Ship with v1.2.0 (all three gaps closed, all tested)

- [ ] PLCY-05: `EvaluatePath()` called from `internal/check/handler.go` for `file_path` fields and `cat`/`type`/`Get-Content` command targets; fail-closed; allowlist + policy-overlay merged. F2 behavioral test passes.
- [ ] NUDGE: `internal/nudge/` package complete with detect/parse/rewrite/evaluate; soft-advise default; wired into check + gateway + shim; `record_type: "nudge"` audit records; `beekeeper nudge status|check|audit` CLI. F3 acceptance criteria from NUDGE-PRD §10 (all 17 tests) pass.
- [ ] PLCY-07: Per-severity escalation (`severity: "critical"` → block at 1 source); sanity bound on new critical blocks per sync cycle; documented in threat model. F1 behavioral test (`npm install ai-figure` → BLOCK, not WARN) passes.
- [ ] BTEST: Table-driven pure-policy unit tests + stdin→decision integration tests + live-binary E2E battery covering all three gaps.

### Defer to v1.3.0

- [ ] Hard-rewrite mode (`nudge.mode: "hard"`) — soft-advise must be validated in production before enabling command rewrites. Ship after v1.2.0 has run against real agent sessions.
- [ ] Yarn Berry nudge — defer pending pattern research.
- [ ] Pip/cargo/gem nudge — defer pending threat data.
- [ ] OSV as second source for auto-corroboration of critical advisories (Option C) — defer; adds network latency on hot path. Evaluate after PLCY-07 Option A is in production.

---

## Feature Prioritization Matrix (v1.2.0)

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| PLCY-05 sensitive-path wiring | HIGH | LOW | P1 |
| PLCY-07 critical-severity block | HIGH | LOW | P1 |
| NUDGE detect + parse + evaluate (soft mode) | HIGH | MEDIUM | P1 |
| BTEST per-feature behavioral tests | HIGH | MEDIUM | P1 |
| NUDGE audit records | HIGH | LOW | P1 |
| NUDGE hard-rewrite mode (opt-in) | MEDIUM | MEDIUM | P2 |
| `beekeeper nudge status|check|audit` CLI | MEDIUM | LOW | P2 |
| Node 22 compatibility gate surfacing | MEDIUM | LOW | P2 |
| Weekly major-version drift check | LOW | LOW | P2 |
| Yarn/pip/cargo nudge | LOW | HIGH | P3 |
| OSV auto-corroboration for critical | MEDIUM | HIGH | P3 |

---

## Sources

**pnpm command reference (verified June 2026):**
- pnpm add: https://pnpm.io/cli/add
- pnpm install: https://pnpm.io/cli/install
- pnpm dlx: https://pnpm.io/cli/dlx
- pnpm 11.0 release notes: https://pnpm.io/blog/releases/11.0
- pnpm supply chain security: https://pnpm.io/supply-chain-security
- pnpm 11 security analysis (Socket): https://socket.dev/blog/pnpm-11-adds-new-supply-chain-protection-defaults

**bun command reference (verified June 2026):**
- bun add: https://bun.sh/docs/cli/add
- bun install: https://bun.sh/docs/cli/install
- bunx: https://bun.sh/docs/cli/bunx

**Version pinning and supply chain incidents:**
- 2026 Axios supply chain compromise (Microsoft Security Blog): https://www.microsoft.com/en-us/security/blog/2026/04/01/mitigating-the-axios-npm-supply-chain-compromise/
- Mini Shai-Hulud / TeamPCP analysis: https://safeheron.com/blog/npm-supply-chain-news-lessons-from-attacks-2026/
- Microsoft typosquatting campaign (May 2026): https://www.microsoft.com/en-us/security/blog/2026/05/28/typosquatted-npm-packages-used-steal-cloud-ci-cd-secrets/
- Dependency pinning for npm: https://exploitr.com/articles/dependency-pinning-npm-supply-chain-attacks/
- "Pinning Is Futile" paper: https://arxiv.org/pdf/2502.06662

**Sensitive path blocklist sources:**
- Claude Code issue #46741 (credential paths community request): https://github.com/anthropics/claude-code/issues/46741
- AI coding agent security guardrails: https://dev.to/maxkrivich/ai-coding-agent-security-practical-guardrails-for-claude-code-copilot-and-codex-och
- MCP config file paths guide: https://mcpplaygroundonline.com/blog/complete-guide-mcp-config-files-claude-desktop-cursor-lovable

**Severity and enforcement conventions:**
- OSV schema specification: https://ossf.github.io/osv-schema/
- CVSS v4.0 specification (FIRST.org): https://www.first.org/cvss/specification-document
- npq behavior reference: https://github.com/lirantal/npq
- Socket safe-npm FAQ: https://docs.socket.dev/docs/safe-npm-faq
- Claude Code hooks documentation: https://code.claude.com/docs/en/agent-sdk/hooks

---
*Feature research for: Beekeeper v1.2.0 Runtime Behavioral Hardening (PLCY-05, NUDGE, PLCY-07)*
*Researched: 2026-06-03*
