# Beekeeper install posture and policy enforcement

Status: supersedes the package-manager nudge PRD. The nudge ("steer npm installs toward pnpm or Bun") is retired. Its premise, that npm runs lifecycle scripts by default and safer managers do not, is removed by npm v12 (estimated July 2026), which blocks install scripts, git dependencies, and remote URL dependencies by default and brings npm in line with pnpm, Yarn, and Bun. The surviving value of the nudge (release-age enforcement, an opinion about what a safe install looks like) is repositioned here as a tool-agnostic enforcement layer.

## The reframe in one line

Beekeeper does not manage your package manager's configuration. Beekeeper is the consistent place your install-safety policy lives and is enforced, at the surfaces Beekeeper already occupies, regardless of which package manager or version is underneath.

This matters because the ecosystem's install-safety settings are now numerous, incoherent across tools, and changing fast. npm has `allowScripts`, `--allow-git`, `--allow-remote`. pnpm has `minimumReleaseAge`, `blockExoticSubdeps`, `trustLockfile`. Bun has its security scanner API. pip, cargo, gem each differ again, and each changes its defaults on its own schedule. A user cannot reason about their real install posture across all of these, and a team on mixed package-manager versions has no consistent guarantee. Beekeeper becomes the one place that delivers the same guarantee no matter what is underneath, within the honest boundaries stated below.

## What this is not

This is not a configuration manager. Beekeeper does not, by default, edit `.npmrc`, `package.json`, `pnpm-workspace.yaml`, or any other package-manager config file. Owning those files would make Beekeeper responsible for every package manager's evolving config schema, would risk breaking user builds, and would create a new high-value attack surface (a security tool that writes package-manager config). Beekeeper observes and enforces a property at its own layer. Config mutation, if offered at all, is opt-in, reversible, audited, and never silent.

## Enforcement surfaces and their boundaries

This is the most important section. Install posture can only be enforced where an install is actually observable, and Beekeeper has exactly three surfaces, each with different reach. Stating these boundaries honestly is mandatory; the harness-coverage table already sets this standard for the rest of the product and install posture must hold to it.

### Surface 1: the pre-exec hook (prevention, primary)

When an agent runs an install as a tool call (for example a Bash `npm install`), a Tier-1 harness hook sees it before it executes. This is the primary enforcement point and where the old nudge lived, so install posture inherits the same attachment. The hook parses the install, evaluates it against the posture (version too fresh, pulls from git or remote URL, will run unapproved scripts), and blocks, warns, or quarantines before the install runs. This is genuine pre-execution prevention: the bad install never happens.

Boundaries:
- Works only for installs invoked as a tool call on a harness that has a working pre-exec hook (Tier-1).
- Tier-2 harnesses inherit their documented caveats (for example Hermes is fail-open, so a block rests on stdout JSON only). Install posture is exactly as reliable as the harness's hook, no more.
- Tier-3 harnesses (Kilo, Trae, Continue, OpenClaw) invoke native Bash directly with no pre-exec hook. An install run through their native shell is invisible to this surface. This is a documented harness gap, not a Beekeeper bug, and install posture inherits it.
- An install a human types directly in a plain terminal is not an agent tool call and is not seen by this surface.

### Surface 2: the Sentry daemon (observation and audit, not prevention)

Sentry is detection-only, privileged, and opt-in. It watches process, file, and network behavior at the machine level, so it can see an install process spawn regardless of which harness invoked it, or whether an agent was involved at all, including a human-run install. Its role in install posture is observation and audit, not prevention.

Boundaries:
- Detection-only by design. Sentry records that an install happened and whether it matched posture, writes the audit record, and feeds the corpus. It does not block. Making the privileged daemon an enforcer would change its risk profile and contradict its stated detection-only role.
- Reactive, not pre-exec. By the time Sentry sees the install process, the install may already be underway. It is the right place to observe and record machine-wide install activity, not to prevent it.
- This is the surface that gives install posture machine-wide visibility into installs (agent or human), as detection feeding the audit log and corpus, clearly labeled as detection not prevention.

### Surface 3: the MCP gateway (not a general install surface)

The gateway scans MCP server responses for prompt injection. Package installs are shell commands, not MCP traffic, so the gateway is not a general install-enforcement point. The only exception: if an MCP server exposes an install-type tool, that call routes through the gateway and can be checked there. Do not imply the gateway enforces install posture broadly; it does not. Its job is response scanning.

### Surface 4 (roadmap, not v1.0): the package-manager shim

True pre-exec prevention of installs a human runs directly in a terminal, independent of any agent, requires a different mechanism: a shim that intercepts `npm`/`pnpm`/`pip`/etc. on the user's PATH, checks posture, then passes through to the real binary. This is the capability that would let Beekeeper claim it enforces posture on every install, even ones the user types themselves, making install posture truly machine-level rather than agent-scoped.

It is deliberately deferred:
- It modifies the user's shell environment (PATH or aliases), can be bypassed by invoking the real binary by absolute path, and must track each package manager's CLI surface.
- It is a new component Beekeeper installs into the user's environment, which for a security tool is a real trust and attack-surface consideration.
- It is the heaviest option and closest in spirit to the config-manager trap this PRD avoids.

The shim is a labeled roadmap item (v1.x or v2). It is the honest path to machine-wide pre-exec enforcement, and shipping it in v1.0 would expand scope and trust surface significantly. v1.0 does not include it.

### The boundary statement (for the website and docs)

Install posture is enforced pre-execution at the agent hook for harnesses that support it. For Tier-3 harnesses and direct human-run installs it is observed and audited by the Sentry layer rather than prevented. The package-manager shim that would extend pre-exec enforcement to all installs, including those a user types directly, is on the roadmap. This sentence, or one like it, must appear wherever install posture is described, to the same honesty standard as the harness-coverage table.

## The model: opinionated default, visible, overridable, never silent

The governing principle is the one Beekeeper already applies to corroboration thresholds and quarantine: ship a sensible secure default, show it plainly, let the user tune or override it explicitly and auditable, never weaken it silently or irreversibly. Three layers, shipped in order of priority.

### Layer 1: the shipped default posture (v1.0, on by default)

Beekeeper ships an opinionated default install posture so it protects on install with zero configuration, honoring the "protected in 60 seconds" promise. Secure but not maximal: a too-strict default breaks legitimate installs on day one and the first experience becomes "it broke my build," the worst outcome. The default should feel like a smart colleague looking over your shoulder, not a bureaucrat.

Default posture (all values are defaults, all tunable later), enforced at the hook surface, observed at Sentry:
- **Release age.** Installs of a package version published less than 24 hours ago are warned. The strongest surviving piece of the old nudge: it catches a freshly published malicious version before any catalog has flagged it. 24 hours, not 14 days, to catch the attack window without blocking normal "I want this morning's release" workflows.
- **Lifecycle scripts.** Default to the same soft posture npm v12 chose: an unapproved install script is warned, not hard-failed. Aligned with where the ecosystem landed, not stricter for no reason. Raisable to block per ecosystem.
- **Git and remote URL dependencies.** Flagged and warned on first encounter, not hard-blocked, with the reason surfaced. Legitimate in many projects; the default makes them visible and deliberate, not forbidden.
- **Corroboration and sensitive paths.** Unchanged. Install posture sits alongside these existing policies in the same policy engine.

Install-posture rules are new inputs to the existing policy engine that already runs on every tool call. No new enforcement mechanism is built; they reuse the existing warn/block/quarantine machinery at the hook, and the existing audit path at Sentry.

### Layer 2: posture visibility in the TUI (v1.0, read-only, machine-wide)

A `beekeeper posture` view (TUI and CLI) reads, read-only, each detected package manager's actual current config and shows it side by side with Beekeeper's enforced posture, naming the gaps Beekeeper covers. Because it reads config files, this view is machine-wide regardless of agents, which is the piece that genuinely delivers the "see your whole machine's exposure" value without overpromising enforcement.

Example the view makes legible:
- npm v11 detected. Scripts run by default. Git deps allowed. Beekeeper enforces: scripts warned, release-age 24h, git deps flagged. Beekeeper is covering 3 gaps your npm version does not.
- pnpm v11 detected. minimumReleaseAge honored. Beekeeper enforces: aligned, no gap.

Pure visibility, zero mutation. Reads config files, never writes them. Ships alongside the default posture as the first increment, because together a default that protects plus a view that shows what it protects against deliver most of the value with little surface area.

### Layer 3: policy tuning and scoped override

The scoped override exists at v1.0, using machinery Beekeeper already has. When a posture rule blocks an install (at the hook surface), the TUI incident card and the CLI offer graduated, scoped, audited responses, not a binary on/off:
- Allow this one, once (a recorded one-time exception).
- Allow this always, and record why (a scoped, audited standing exception for this package or pattern).
- Block (the default action stands).

Every override is an audit-log entry: a recorded, scoped, deliberate trust decision, never a silent weakening. The explicit-trust model the whole product is built on, applied to install posture. The user is never dead-ended on legitimate work; they are asked to make the exception deliberate and recorded.

Deep per-rule policy editing is deferred to v1.x: full customization (release-age window, warn/block/quarantine per rule per ecosystem, per-project overrides) lives in the existing policy editor extended with install-posture rules. Not required for launch, because the shipped default plus the scoped override already protect people and handle exceptions. Do not let "configure everything" balloon launch scope.

### Layer 4 (out of scope for v1.0): config mutation

Beekeeper could, as an opt-in action, generate the recommended package-manager config that closes a posture gap and offer to apply it. If ever built: opt-in, reversible (restore manifest like quarantine), audited, dry-run by default, never automatic. The default and recommended posture is that Beekeeper enforces at its own surfaces and leaves the user's config files alone.

## Why this is strictly better than the nudge

- **Tool-agnostic.** Enforces a property (no unapproved scripts, minimum release age, no unflagged remote sources) rather than tracking each tool's config schema, across npm, pnpm, Bun, pip, cargo, gem.
- **Survives version drift.** npm v12 ships, v13 changes the flags, pnpm renames an option, and Beekeeper does not care, because it enforces at its own surfaces rather than depending on the package manager's config. Beekeeper becomes the stable layer over a churning ecosystem. The value of one consistent policy rises as the underlying tools keep changing their defaults incoherently. That is a moat.
- **On-thesis.** "Detection cannot keep pace, so move to containment and explicit trust" is Beekeeper's philosophy and is exactly why npm v12 exists. Install-posture enforcement applies that philosophy to the install-configuration surface npm just vacated.
- **Folds in the surviving good part of the nudge.** Release-age enforcement, the strongest piece of the old feature, becomes a first-class install policy rather than a side effect of recommending pnpm.

## Acceptance criteria

1. Beekeeper ships a default install posture (release-age 24h warn, lifecycle-script warn, git/remote-URL flag) enforced at the pre-exec hook via the existing warn/block/quarantine engine, on by default, no configuration required.
2. The enforcement boundaries are documented exactly: prevention at the hook for hooked (Tier-1) harnesses, inheriting all tier caveats; observation and audit at Sentry for anything it sees including human-run installs, labeled detection not prevention; not a general MCP-gateway function; the package-manager shim for machine-wide human-install prevention is a labeled roadmap item.
3. Release-age enforcement catches a package version published under the configured window and surfaces the reason. Default 24h, semver-aware, not pinned to any package manager's implementation.
4. `beekeeper posture` (TUI and CLI) reads each detected package manager's config read-only and displays it side by side with Beekeeper's enforced posture, naming the gaps. Machine-wide. No config file is written.
5. A posture-rule block at the hook offers graduated scoped responses (allow once, allow always with recorded reason, block), each written to the audit log as a scoped trust decision.
6. No package-manager config file is modified by default or without an explicit, reversible, audited, opt-in action. Config mutation is out of scope for v1.0. The shim is out of scope for v1.0.
7. Posture rules are inputs to the existing policy engine, not a parallel mechanism. Deep per-rule per-ecosystem editing is deferred to v1.x and marked as such.
8. The boundary statement appears wherever install posture is described in product and docs, to the same honesty standard as the harness-coverage table.

## Migration note

Remove the nudge feature and its "steer to pnpm/Bun" copy from the product and docs. Replace the home page "Agent safety" nudge bullet with the install-posture framing. Preserve the release-age logic from the nudge implementation and reposition it as the release-age posture rule. The package-manager preference steering itself is gone; do not carry it forward.

Style: no em dashes, sentence case, match repo conventions.
