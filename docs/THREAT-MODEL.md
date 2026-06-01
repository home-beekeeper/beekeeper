# Beekeeper Threat Model

**Version:** v1.0.0  
**Date:** 2026-05-29  
**Status:** Published (Phase 9 capstone — final release gate)

This document describes the security properties Beekeeper provides, the
attack surfaces it exposes, the known gaps in its defenses, and the
verification path an operator uses to confirm binary integrity. It is written
for a technical audience: security researchers, operators, and developers who
want to understand what they are trusting when they add Beekeeper to their
CI pipeline or developer workstation.

---

## Table of Contents

1. [Beekeeper's Own Threat Model](#1-beekeepers-own-threat-model)
2. [Build and Release Pipeline Hardening](#2-build-and-release-pipeline-hardening)
3. [Catalog Feed Integrity: The 2FA Principle](#3-catalog-feed-integrity-the-2fa-principle)
4. [Coordinated False-Positive Poisoning Attack Surface](#4-coordinated-false-positive-poisoning-attack-surface)
5. [The fanotify mmap Gap on Linux](#5-the-fanotify-mmap-gap-on-linux)
6. [The beekeeper-self Catalog](#6-the-beekeeper-self-catalog)
7. [Verification Path](#7-verification-path)
8. [Known Gaps and Explicit Non-Defenses](#8-known-gaps-and-explicit-non-defenses)
9. [Declarative Policy Overlay: Escape Hatch and Known Limitations](#9-declarative-policy-overlay-escape-hatch-and-known-limitations)

---

## 1. Beekeeper's Own Threat Model

### What Beekeeper Is Defending Against

Beekeeper is a real-time safety harness that intercepts autonomous coding
agent tool calls before they execute. It defends against:

- **Malicious package installation** — an agent that installs a compromised
  npm, PyPI, or extension package as part of legitimate-looking work.
- **Sensitive path access** — an agent that reads `~/.ssh/`, `~/.aws/`, or
  other credential paths outside the project working directory.
- **Lifecycle script abuse** — a package whose `postinstall` script executes
  arbitrary code.
- **Supply-chain corroboration evasion** — a single attacker-controlled catalog
  that flags a legitimate package as malicious (or misses a truly malicious
  one).
- **Prompt injection** — adversarial content in the developer's codebase that
  hijacks the agent's next action.

### What Compromising Beekeeper Gives an Attacker

Beekeeper runs with the developer's full filesystem and network privileges. A
compromised Beekeeper binary is equivalent to compromising the developer's
account. The attacker gains:

- Read access to all files the developer can read (credentials, SSH keys,
  tokens, source code).
- Write access to all files the developer can write (source code,
  configuration, CI scripts).
- Network access to all endpoints the developer's machine can reach
  (internal services, cloud APIs, package registries).
- The ability to silently suppress block decisions, allowing the agent to
  install malicious packages or exfiltrate data undetected.

This threat is not hypothetical — it is the reason Beekeeper implements the
`beekeeper-self` self-quarantine catalog (see Section 6) and the reproducible
build + Sigstore pipeline (see Section 2).

### Attack Surface Summary (ordered by risk)

| Surface | Risk | Mitigation |
|---------|------|------------|
| Beekeeper binary supply chain | Critical | Reproducible builds, Sigstore, SLSA Level 3, `beekeeper-self` feed |
| Catalog feeds (bumblebee, OSV, Socket) | High | Corroboration semantics (see Section 3), sanity bounds, signatures |
| Policy file injection | Medium | `policyloader.ValidateSchema` rejects unknown fields and `exec` actions |
| Policy allowlist override (escape hatch) | Medium | `package_allowlist` allow rules can override catalog blocks — see Section 9 |
| Config injection via env/project | Low | Documented, operator controls trusted project directories |
| Audit log tampering | Low | Owner-only permissions (0600), append-only writes, OTLP/syslog fan-out |
| IPC named pipe / Unix socket | Low | OS-level owner-only permissions, no auth bypass path |

---

## 2. Build and Release Pipeline Hardening

### Reproducible Builds

Every Beekeeper binary is built with deterministic flags:

```
-trimpath -buildvcs=false -mod=readonly
```

`-trimpath` removes all local filesystem paths from the binary so two builds
of the same source produce byte-identical output regardless of the builder's
machine. `-buildvcs=false` prevents the VCS state from affecting the build.
`-mod=readonly` ensures no dependency changes happen at build time.

Verify locally:

```bash
make verify-release VERSION=X.Y.Z
```

### Keyless Signing (Sigstore/cosign v3)

Every released binary is keylessly signed via Sigstore/cosign v3 using GitHub
Actions OIDC. No long-lived private signing key exists to be stolen. The
signing identity is tied to the GitHub Actions workflow:

```
https://github.com/bantuson/beekeeper/.github/workflows/release.yml
```

Verify with cosign:

```bash
cosign verify \
  --certificate-identity=https://github.com/bantuson/beekeeper/.github/workflows/release.yml@refs/heads/main \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  beekeeper
```

### SLSA Level 3 Provenance

Release provenance is generated via `slsa-github-generator@v2.1.0` (full
semver tag required — pinning to `@v2` is insufficient for SLSA Level 3).
SLSA Level 3 guarantees:

- Build was triggered by a source commit the attacker cannot forge without
  write access to the repository.
- The build environment was ephemeral and not modified by the build script.
- The provenance is signed by the SLSA generator, not by the project itself.

Verify SLSA provenance:

```bash
slsa-verifier verify-artifact beekeeper \
  --provenance-path beekeeper.intoto.jsonl \
  --source-uri github.com/bantuson/beekeeper
```

### Pinned Dependencies

All Go dependencies are pinned via `go.mod` + `go.sum` with `go mod verify`
enforced in CI. Dependency updates are proposed by Renovate and require human
review before merge. Indirect dependencies are covered by `go.sum` checksums.

### CycloneDX SBOM

A CycloneDX Software Bill of Materials is published with every release. It
lists all direct and transitive Go dependencies with their checksums. Use the
SBOM to verify no unexpected dependencies were introduced:

```bash
# Compare SBOM dependency list against expected
cat beekeeper.cyclonedx.json | jq '.components[].name'
```

---

## 3. Catalog Feed Integrity: The 2FA Principle

Beekeeper's threat-intel feed design treats corroboration as a second factor
for block decisions. This is the core defense against a single attacker-
controlled catalog source.

### Corroboration Semantics

By default (configurable via `corroboration_threshold` policy rules):

| Source count flagging a package | Action |
|----------------------------------|--------|
| 1 source | Warn — investigate but do not block |
| 2 sources | Block — enforce |
| 3+ sources | Block + quarantine |

An adversary who controls a single catalog source (bumblebee, OSV, or Socket)
can generate warning events for legitimate packages, but cannot escalate to a
block or quarantine decision without controlling a second independent source.
This is analogous to two-factor authentication: one compromised factor is
not sufficient.

### Signature Verification

Every catalog feed is verified against an embedded public key before the feed
entries are applied. A feed with an invalid or absent signature is treated as
an integrity failure and causes the corresponding source to enter degraded
mode. Degraded-mode matches count at most 0.5 toward the corroboration count,
meaning even a successfully-fetched but integrity-failed feed cannot drive a
block decision on its own.

### Sanity Bounds

Catalog syncs apply configurable sanity bounds on entry-count deltas. A sync
that reports a sudden drop of >50% or an implausibly large spike in entries
triggers a sanity alert or hard-block before the new index is applied. This
prevents a feed compromise that simply wipes all entries (allowing all
packages) or floods the index with false positives.

### Degraded Mode

When a catalog source cannot be fetched or fails signature verification,
Beekeeper continues operating using the last-known good cached state. The
`beekeeper diag` command shows per-source degraded status so operators can
investigate without service interruption.

---

## 4. Coordinated False-Positive Poisoning Attack Surface

**This is a known, documented gap in Beekeeper's defenses.**

### The Attack

An adversary who controls two or more catalog sources (the corroboration
threshold for a block decision) can manufacture false-positive block events
for any legitimate package. For example:

1. Attacker compromises or creates two catalog sources (e.g., bumblebee +
   Socket.dev API account).
2. Attacker adds legitimate popular packages (e.g., `react`, `lodash`) to
   both sources' threat-intel feeds as "malicious."
3. Beekeeper blocks every agent action that installs these packages.
4. The developer, frustrated by constant false blocks, disables Beekeeper
   enforcement (`fail_mode: open`) or removes the hooks.
5. With enforcement disabled, the attacker can now install genuinely malicious
   packages undetected.

This is a **denial of service / coercion attack** — the attacker does not need
to install malicious code directly; they degrade the developer's trust in
Beekeeper's signal quality until enforcement is disabled.

### Why There Is No Complete Fix at This Layer

Full defense against this attack requires human-in-the-loop review of catalog
changes. Automated systems with a fixed corroboration threshold cannot
distinguish "two independent sources both discovered this package is malicious"
from "two colluding or compromised sources manufactured this block event."

**Known mitigations (partial):**

- Sanity bounds limit how rapidly any single feed can change, reducing the
  blast radius of a sudden poisoning event.
- The audit log records every block decision with its source provenance, so
  operators can investigate patterns.
- Degraded mode and corroboration counts are surfaced to operators in `beekeeper diag`.
- Future work: human-in-the-loop review workflow, source reputation weighting,
  and independent source audits (post-v1.0.0).

**For v1.0.0:** This gap is accepted and documented. Users who operate in
adversarial environments where catalog source compromise is a realistic threat
should configure a low `corroboration_threshold` (warn-only) and manually
review catalog state changes.

---

## 5. The fanotify mmap Gap on Linux

**This is a known, documented gap in Beekeeper's Linux Sentry defenses.**

### How fanotify Works

The Linux Sentry uses `fanotify` to intercept `FAN_OPEN_PERM` events — kernel
notifications for file-open operations on watched paths. When a process opens
a file, Beekeeper's Sentry can evaluate the access before the kernel grants
the open permission.

### The Gap

`fanotify` only intercepts **new file-open operations** after the watch is
established. Files that were already mapped into a process's address space via
`mmap(2)` **before** fanotify was activated are NOT re-intercepted.

Specifically:

- A library that was `mmap`-loaded at process startup (before Beekeeper Sentry
  was installed or before the fanotify watch was placed on the library path)
  will continue executing without triggering any Sentry alert.
- An attacker with code-execution access who pre-loads a malicious shared
  library via `LD_PRELOAD` before Beekeeper's Sentry starts can evade all
  file-access-based detection.

### Scope

This gap is limited to the Sentry's file-access detection path. It does NOT
affect:

- The hook handler (`beekeeper check`) which intercepts agent tool calls at
  the protocol layer, not the kernel file-access layer.
- The catalog corroboration check, which evaluates packages by name/version.
- The MCP gateway, which intercepts JSON-RPC at the network layer.
- Quarantine decisions based on catalog matches.

### Mitigation

No complete in-process mitigation exists for this gap in v1.0.0 scope. The
Sentry is designed as a behavioral correlation layer, not a kernel rootkit
prevention system.

**Partial mitigations:**

- Install Beekeeper Sentry before the agent process starts, not after.
- Use immutable read-only bind-mounts for sensitive library paths on production
  systems (defense-in-depth at the OS layer).
- The Sentry's network-level correlation (ETW on Windows, process-tree
  tracking) provides independent detection that does not rely on fanotify.

This gap is accepted for v1.0.0 scope and will be revisited when
`FAN_MARK_FILESYSTEM` and `FAN_MARK_INODE` semantics for mmap'd regions
improve in future kernel versions.

---

## 6. The beekeeper-self Catalog

The `beekeeper-self` catalog is Beekeeper's self-defense against its own
supply-chain compromise. It is the answer to the question: "What if Beekeeper
itself is the malicious package?"

### How It Works

1. **Startup check**: Before any enforcement command runs (`beekeeper check`,
   `beekeeper gateway`, `beekeeper sentry`, `beekeeper watch`), Beekeeper
   fetches the `beekeeper-self` feed and checks whether the running binary's
   version string appears in the compromised-version list.

2. **Sync check**: After every `beekeeper catalogs sync`, the same check runs
   so that a newly-published compromise entry is acted on immediately.

3. **On a version match**: Beekeeper self-quarantines — it writes an audit
   record, prints a prominent warning to stderr with the full verification
   path, and refuses to run enforcement commands. Diagnostic commands
   (`beekeeper version`, `beekeeper diag`, `beekeeper selftest`,
   `beekeeper policy validate`) remain runnable so the developer can
   investigate.

4. **Separate key, separate host**: The `beekeeper-self` feed is hosted
   separately from the main repository and verified against a distinct Ed25519
   public key embedded in the binary at compile time. Compromising the release
   pipeline key does NOT allow forging a `beekeeper-self` feed signature. Both
   systems must be compromised independently.

5. **Fail-closed vs. transient network failure**: Beekeeper distinguishes
   between "signature invalid" (integrity failure → fail closed) and "network
   unreachable" (transient → warn and continue). An absent or stale cache is
   uncertainty, not a proven compromise. Only a bad signature triggers fail-
   closed behavior.

### Feed Schema

The `beekeeper-self` feed is a static signed JSON file:

```json
{
  "schema_version": "1",
  "entries": [
    {
      "id": "beekeeper-self-2026-001",
      "name": "Beekeeper v0.4.2 release pipeline compromise",
      "ecosystem": "beekeeper",
      "package": "beekeeper",
      "versions": ["v0.4.2"],
      "severity": "critical",
      "catalog_source": "beekeeper-self"
    }
  ],
  "catalog_signature": "<base64-ed25519-signature-over-entries-json>"
}
```

The signature is an Ed25519 signature over `json.Marshal(entries)` — the
canonical JSON encoding of the entries array — not over the full feed JSON
(which would be circular). The public key is embedded in the binary.

### Governance Honesty Note (v1.0.0)

**For v1.0.0, the `beekeeper-self` feed is maintained by a single maintainer
(Mzansi Agentive Pty Ltd — Mfanafuthi Mhlanga).** This means that an attacker
who compromises the maintainer's GitHub account and the feed hosting has full
control over the self-quarantine signal.

This is a single point of trust. It is documented here explicitly because
pretending it does not exist would be dishonest.

**Intent for post-v1.0.0:** Separate the `beekeeper-self` feed maintainer set
from the main repository maintainers, require multi-party approval for feed
entries (M-of-N signing), and publish a key ceremony transcript. This reduces
the single-maintainer risk to a multi-party compromise requirement.

For v1.0.0, users who cannot accept this single-maintainer governance model
can point to a self-hosted `beekeeper-self` feed by setting:

```json
{
  "self_catalog": {
    "url": "https://your-mirror.example.com/beekeeper-self.json",
    "pub_key": "<your-base64-ed25519-public-key>"
  }
}
```

in `~/.beekeeper/config.json` (or the project `.beekeeper/config.json`).

---

## 7. Verification Path

Use this path to verify a Beekeeper binary before installing it, or to
investigate a self-quarantine event.

### Step 1: Verify the build hash

```bash
make verify-release VERSION=X.Y.Z
```

This command downloads the released binary for `VERSION`, rebuilds it from the
corresponding source tag, and compares the output hash. A mismatch indicates
the released binary was not produced from the tagged source.

### Step 2: Verify the Sigstore/cosign signature

```bash
cosign verify \
  --certificate-identity=https://github.com/bantuson/beekeeper/.github/workflows/release.yml@refs/heads/main \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  beekeeper
```

A successful `cosign verify` confirms the binary was produced by the GitHub
Actions release workflow and was signed by a GitHub OIDC token, not a manually
held key.

### Step 3: Verify SLSA provenance

```bash
slsa-verifier verify-artifact beekeeper \
  --provenance-path beekeeper.intoto.jsonl \
  --source-uri github.com/bantuson/beekeeper
```

SLSA Level 3 provenance confirms the build environment was ephemeral,
the build was triggered by a signed commit, and the provenance itself was
produced by the SLSA generator (not the project's own workflow).

### Step 4: Inspect the SBOM

```bash
# List all direct and transitive dependencies
cat beekeeper.cyclonedx.json | jq '.components[] | "\(.name)@\(.version)"'
```

Compare against the expected dependency list for the release tag. Unexpected
entries indicate a dependency was added between the tagged source and the
released binary.

### Step 5: Report and recover

If verification fails at any step:

1. **Do not run the binary on any system with sensitive credentials.**
2. File a private security advisory via the process in [SECURITY.md](../SECURITY.md).
3. While waiting for a response, roll back to the last known-good version by
   re-running steps 1–4 on the previous release.

### During a self-quarantine event

If `beekeeper` refuses to run with a self-quarantine message, run the
verification steps above on the installed binary. If the binary passes all
verification steps but the self-quarantine fires, the `beekeeper-self` feed
entry may be incorrect — file a private advisory to report the false positive.

Diagnostic commands remain usable during self-quarantine:

```bash
beekeeper version          # confirm installed version
beekeeper diag             # inspect catalog and latency state
beekeeper selftest         # run adversarial corpus against the engine
beekeeper policy validate  # check policy file schemas
```

For the full disclosure and bug-report process, see [SECURITY.md](../SECURITY.md).
For build verification details, see [BUILDING.md](../BUILDING.md) (if present).

---

## 8. Known Gaps and Explicit Non-Defenses

### What Beekeeper Does NOT Defend Against

The following threats are explicitly out of scope for v1.0.0. Documenting them
here prevents false confidence.

#### Kernel Rootkits and Pre-Boot Attacks

An adversary with kernel-level access (ring-0) can intercept or modify any
process's memory, file I/O, or network operations before Beekeeper sees them.
Kernel rootkits can also patch `fanotify` event delivery, suppress audit log
writes, or modify the Beekeeper binary in memory. Beekeeper provides no
defense against an already-compromised kernel.

**Mitigation at a higher layer:** Secure Boot, measured boot (TPM), and OS
integrity verification tools (e.g., `aide`, `dm-verity`) provide defense at
the kernel layer. These are OS-level concerns outside Beekeeper's scope.

#### Pre-Existing Malware on the Developer Machine

If malicious code is already present on the developer's machine when Beekeeper
is installed, that malware can observe Beekeeper's binary, modify it, or
intercept its decisions. Beekeeper is designed to detect and block malicious
agent behavior; it is not designed to remediate an already-compromised host.

#### Direct Human Malice

If the developer themselves (or someone with their credentials) wants to
install a malicious package, Beekeeper's enforcement can be trivially bypassed
by disabling hooks, setting `fail_mode: open`, or running package managers
directly. Beekeeper is an agent safety harness, not a policy enforcement
system for humans.

#### Sophisticated Prompt Injection Beyond LlamaFirewall's Detection Capability

LlamaFirewall's CodeShield and AlignmentCheck models detect common prompt
injection patterns. Novel, highly-tailored prompt injection attacks that
specifically craft payloads to evade the current model versions may succeed
undetected. Model detection quality improves with each LlamaFirewall release
but is inherently bounded by training data and adversarial capability.

#### Zero-Day Agent Tool-Call Semantics

Beekeeper intercepts tool calls by tool name and argument structure. A new
agent tool or tool-call format that Beekeeper has not been updated to parse
correctly may pass through undetected. The hook handler fails closed (blocks)
on malformed or oversized input, but a new tool with a novel JSON structure
may produce an "allow" decision by default until Beekeeper's schema is updated.

#### Catalog Source Unavailability (Sustained)

If all catalog sources are simultaneously unreachable for more than 24 hours,
Beekeeper will continue in degraded mode (read from stale cache with
prominent warnings). A sustained multi-source outage does not cause Beekeeper
to block all agent actions, but it reduces the quality of threat intelligence.
This is an explicit availability vs. security tradeoff: bricking all agents
for a multi-day outage was not acceptable for v1.0.0.

---

---

## 9. Declarative Policy Overlay: Escape Hatch and Known Limitations

### Package Allowlist Override Escape Hatch (T-09-31)

Beekeeper supports version-controlled policy files (`~/.beekeeper/policies/*.json`)
with `package_allowlist` rules. A `package_allowlist` rule whose `action` is `"allow"`
can **override a catalog-corroborated block or warn decision** for the exact listed
package.

**This is intentional behavior.** The purpose is to let operators make explicit,
version-controlled trust decisions — for example, to allowlist an internal package
that a fresh catalog source incorrectly flags, or to express that a particular
"flagged" package is a known false-positive in the operator's environment.

**Why this is a documented escape hatch:**

- A `package_allowlist` allow rule for a package silences all enforcement against
  that package, including catalog-corroborated blocks (2+ signed sources agreeing).
- A malicious or misconfigured policy file placed in `~/.beekeeper/policies/` can
  selectively disable enforcement for targeted packages.
- The override is recorded in the decision `Reason` field in every audit record, so
  it is forensically visible: any allow decision that cites a policy allowlist rule
  is immediately distinguishable from a genuine catalog-clear decision.

**Mitigations:**

- Policy files are validated by `policyloader.ValidateSchema` which rejects unknown
  fields, `exec` action values, and unknown rule types (T-09-30).
- The `~/.beekeeper/policies/` directory is governed by the same owner-only (0600)
  permission convention as the config file. An attacker who can write to this
  directory already has the developer's full filesystem access.
- Every allowlist-override allow decision includes `"policy overlay: rule ... allowlists
  this package (user-trust override — recorded for audit)"` in the `Reason` field.
  Operators monitoring audit logs can detect unexpected allowlist overrides.

**Recommendation:** Treat `~/.beekeeper/policies/` as part of your security-relevant
configuration. Place policy files in version-controlled directories and review
`allow`-action rules carefully before deploying.

### Overlay Limitations: `release_age` and `lifecycle_script_allowlist` Not Enforced

The declarative policy overlay (`ApplyPolicyOverlay` in `internal/policyloader/enforce.go`)
enforces `package_allowlist` and `sensitive_path` rules from policy files against live
tool calls. However, **two rule types are NOT enforced by the overlay in v1:**

- `release_age` rules (minimum package age before install is permitted)
- `lifecycle_script_allowlist` rules (allowlist of lifecycle scripts permitted to run)

**Why:** These rule types require package publication age and lifecycle-script metadata
that is NOT present in a pure `policy.ToolCall`. The overlay evaluates only data
available in the tool call itself (package name/ecosystem, target path). Release age
requires a catalog or registry API lookup; lifecycle-script data is extracted during
catalog sync.

**Enforcement path in v1:** The engine's built-in release-age and lifecycle-script
policies (configured via catalog entries and `internal/config`) remain the enforcement
path for these rule types. Declarative policy files can declare `release_age` rules
for documentation and `policy test` dry-run context, but they do not affect live
`beekeeper check` decisions in v1.

**Future work:** A v2 overlay enhancement could pass catalog-side metadata (package
publish timestamp, lifecycle-script hash) into the overlay for these rule types.
Until then, `release_age` and `lifecycle_script_allowlist` in policy files are
informational only and must not be relied upon for enforcement.

---

*This document is published as part of the Beekeeper v1.0.0 release and will
be updated when new threats are identified or mitigations change. For the
vulnerability disclosure process, see [SECURITY.md](../SECURITY.md).*
