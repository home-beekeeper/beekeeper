# Install Posture

Beekeeper enforces a default install posture at the agent pre-exec hook. Three
rules evaluate every package install an agent runs:

- **Release age** - a package whose first publish was less than 24 hours ago warns.
- **Lifecycle scripts** - an install that would run pre or post-install scripts warns.
- **Git or remote-URL dependencies** - a dependency pulled from a git ref or an
  arbitrary URL instead of a registry warns.

All three warn by default and fail soft: an unknown answer (a registry timeout, a
missing publish timestamp) warns rather than blocks, so a slow or offline check
never stops an agent. The posture is tool agnostic. The same checks apply at the
hook regardless of which package manager the agent reaches for.

> npm v12 blocks install lifecycle scripts by default, so the old steer-to-pnpm-or-Bun
> nudge that earlier Beekeeper versions shipped is no longer the headline win. Install
> posture is the tool-agnostic successor: it applies the same structural checks to every
> package manager, at the hook. The nudge was removed in v1.1.0.

## Enforcement boundary (read this)

Install posture is enforced pre-execution at the agent hook for hooked harnesses
that support it, inheriting each harness tier's caveats. For harnesses with no
pre-exec hook, and for installs a person runs directly in a terminal, it is
observed and audited by the Sentry layer rather than prevented, unless the
experimental package-manager shim is installed. The MCP gateway is not a general
install surface. The shim extends pre-exec enforcement to installs run through the
shimmed PATH, but it is limited: it can be bypassed by calling a tool by its
absolute path, and it requires adding the shim directory to your PATH. It is not a
headline v1.0 guarantee.

This is the canonical statement (`posture.BoundaryStatement` in
`internal/posture/boundary.go`). It appears wherever install posture is described,
to the same honesty standard as the harness-coverage matrix.

## The three rules

| Rule | Key | Default | Fires when |
|---|---|---|---|
| Release age | `release-age` | warn | The package's first publish is younger than 24 hours. Unknown publish time warns (fail soft). |
| Lifecycle scripts | `lifecycle` | warn | The install would run a pre or post-install lifecycle script. |
| Git / remote-URL dependency | `git-remote` | warn | A dependency resolves from a git ref or arbitrary URL instead of a registry. |

A definite violation of a rule that has been opted up to `block` (see below) is
blocked. The unknown / fail-soft path always warns regardless, so a registry
outage never blocks an install even under `block`.

## The `beekeeper posture` view (read-only)

```sh
beekeeper posture            # side-by-side: each package manager's posture vs Beekeeper's enforced posture
beekeeper posture --full     # print the full enforcement-boundary statement
```

`beekeeper posture` is strictly read-only and machine-wide. For each detected
package manager (npm, pnpm, bun) it reads the manager's own version and config
(`.npmrc`, `pnpm-workspace.yaml`, `bunfig.toml`) and shows it next to the posture
Beekeeper enforces at the hook, naming the gaps Beekeeper covers that the package
manager does not. It never writes config. The same comparison appears as a
read-only card in the `beekeeper dashboard` TUI.

## Scoped overrides

A posture warn (or block) offers a graduated response: allow this install once,
allow a package always with a recorded reason, or raise a rule to block.

```sh
beekeeper posture allow <package> --once               # allow the next matching install, then warn again (all rules)
beekeeper posture allow <package> --always --reason "…" # standing exception (reason recorded)
    --rule release-age|lifecycle|git-remote            # with --always: scope to one rule (omit = all rules; not supported with --once)
    --ecosystem npm                                    # scope to one ecosystem (empty = any)

beekeeper posture enforce <rule> --block               # opt a rule UP from warn to block
beekeeper posture enforce <rule> --warn                # lower it back to the default warn
```

An override is **posture-scoped**. `posture allow` silences a posture warn for the
named package but never downgrades a catalog or corroboration malware block for the
same package: it appends to the posture allow list, not the general
`package_allowlist`. Per-rule opt-up (`posture enforce --block`) flows tighten-only
from untrusted layers (a project or env layer may raise a rule to block but never
lower it; the unknown path stays fail-soft warn).

Each override writes a distinct `posture_override` audit record with a
`posture_override_action` of `allow_once`, `allow_always`, `enforce_block`, or
`enforce_warn`.

## Audit records

- **`posture_override`** - written by `beekeeper posture allow` / `enforce`. Carries
  the package, ecosystem, rule, recorded reason, and `posture_override_action`.
- **`sentry_install_observed`** - written by the Sentry layer (SENTRY-009) when a
  monitored descendant spawns a package-manager install, including installs a person
  runs directly in a terminal. Detection-only process attribution; Sentry never
  evaluates posture or blocks (see the threat model).

The `nudge` record type that earlier versions produced is no longer emitted as of
v1.1.0. The deprecated `nudge_*` audit fields are retained unpopulated for corpus
schema compatibility only.

## Roadmap (not shipped)

Config mutation from the posture view, a per-ecosystem and per-project policy
matrix, and the package-manager shim as a first-class machine-wide enforcement
surface are roadmap, not v1.1.0 guarantees.
