# Phase 9: Policy as Code + Self-Defense Capstone - Context

**Gathered:** 2026-05-29
**Status:** Ready for planning
**Source:** PRD Express Path (beekeeper-prd.md)

<domain>
## Phase Boundary

Phase 9 is the v1.0.0 capstone. It makes Beekeeper's policy **version-controllable, testable, and layered**, and closes the self-defense loop so Beekeeper can detect its own compromise. After this phase, the developer is meant to trust Beekeeper on real production work.

**This phase delivers:**
1. Declarative JSON policy files loaded from `policies/`, separate from config — version-controllable, with `beekeeper policy validate`, `beekeeper policy test`, and `beekeeper policy list`.
2. A full layered config merge: system → user → project → `BEEKEEPER_*` env vars → CLI flags.
3. `beekeeper diag` — a single human-readable health output (hook latency p95/p99, sidecar inference latency, catalog freshness per source, ETW `EventsLost`).
4. The `beekeeper-self` catalog: a separately hosted, separately signed self-quarantine feed listing known-compromised Beekeeper releases by version + signature hash; checked on every startup and every catalog sync, firing self-quarantine on a match.
5. Public, complete threat-model documentation (Section 12 self-defense writeup), including the known coordinated false-positive poisoning attack surface and the fanotify mmap gap.

**This phase does NOT deliver** (out of Phase 9 scope — see Deferred):
- Distributed mode / team-shared catalogs (PRD lists under v1.0.0 core but no Phase 9 REQ-ID maps to it).
- The independent external security review, bug-bounty/VDP publication (process gates for tagging v1.0.0, not code).
- Any change to the corroboration thresholds or weighting (v1 ships unweighted equal-vote; revisit post-1.0).

**Builds on (brownfield):** `internal/policy` (pure engine + `types.go`), `internal/config` (`config.go` layered loader), `internal/catalog` (`sync.go`, `verify.go`, `state.go`, `sanity.go`), `internal/llamafirewall` (sidecar latency tracker), `internal/sentry/windows` (ETW `EventsLost`), `cmd/beekeeper/main.go` (Cobra wiring).

</domain>

<decisions>
## Implementation Decisions

Every item below is a **locked decision** sourced from the PRD. Schema/format details not pinned by the PRD are listed under "Claude's Discretion."

### CODE-01 — Declarative JSON policy files (`policies/`)
- Policy files are **declarative JSON, not code** — an adversary must not be able to smuggle execution through a policy file (PRD §11.3). No URL fetching, no module loading, no eval.
- Policy files live in `policies/` (under `~/.beekeeper/policies/` per project file-structure conventions) and are **loaded by the policy engine**, distinct from `config.json` which governs how Beekeeper itself runs (PRD §9, §461).
- Policies are **version-controllable** — plain JSON files a developer can commit to a repo.
- `internal/policy` must remain a **pure function library** (CLAUDE.md constraint): no I/O in the engine. Policy file loading/parsing happens outside the pure engine and is passed in.

### CODE-02 — `beekeeper policy test <file>`
- Dry-run a policy file against a sample tool-call JSON and report the decision the engine would produce (allow / warn / block / quarantine), with the structured reason.
- Reuses the existing pure `policy.Evaluate` path — `test` is an evaluation harness, not a second engine.

### CODE-03 — `beekeeper policy validate <file>`
- Validate a policy file against its schema; report schema errors with file/field context. Exit non-zero on invalid.

### CODE-04 — `beekeeper policy list`
- List loaded policy files with **rule counts** per file.

### CODE-05 — Layered config merge
- Merge order, lowest → highest precedence (PRD §9, §453):
  1. `/etc/beekeeper/config.json` (system, optional)
  2. `~/.beekeeper/config.json` (user)
  3. `<project>/.beekeeper/config.json` (project, when present)
  4. `BEEKEEPER_*` environment variables
  5. CLI flags
- A project-level `.beekeeper/config.json` **overrides user-level config without requiring environment variables** (Phase 9 success criterion 2).
- Config is JSON; policy files stay separate from config.

### CODE-06 — `beekeeper diag`
- Single human-readable output combining: hook latency **p95/p99**, sidecar (LlamaFirewall) inference latency, catalog freshness **per source**, and ETW `EventsLost` count.
- Reuses existing latency tracking (LlamaFirewall `LatencyTracker` P95 ring buffer from Phase 6) and the Windows ETW `EventsLost` counter (Phase 7).

### CTLG-04 / SFDF-06 — `beekeeper-self` catalog
- A dedicated upstream feed listing **known-compromised Beekeeper releases by version + signature hash** (PRD §12.6, §620).
- Consulted on **every startup and during every catalog sync** (PRD §620).
- On a `beekeeper-self` match for the running version: Beekeeper **self-quarantines** — refuses to run, surfaces the warning prominently, and points the user to the verification path for a known-good version (PRD §622).
- Hosted **separately from the main repo**, with its **own signing key** and **own access control** — the intent is that compromising both surfaces requires defeating two independent security postures (PRD §626, SFDF-06).
- Same corroboration/static-JSON discipline as other catalog sources: signature-verified before applying, no remote-URL fetching from catalog data (PRD §12.3).

### Threat-model documentation (Phase 9 success criterion 5)
- Publish the complete Section 12 self-defense threat model as user-facing docs, **explicitly including**:
  - the coordinated false-positive poisoning attack surface against corroboration semantics, and
  - the fanotify mmap gap on Linux.
- Document the verification path (PRD §12.7) and the `beekeeper-self` governance honesty note (single maintainer for v1, intent to separate — PRD §17).

### Claude's Discretion
- **Policy file JSON schema design.** The PRD mandates declarative JSON separate from config but does not pin the exact schema. The schema must map cleanly onto the existing pure `policy` engine inputs/outputs (`types.go`). Research should survey prior art (OPA/Rego conceptually, simple JSON rule DSLs) but the implementation stays pure-data, no expression evaluation that could enable code execution.
- **Where loaded policy files compose with built-in policies.** How user `policies/*.json` layer over the engine's built-in policy set (override vs. additive) is a design choice to settle in planning.
- **`beekeeper-self` transport/host details.** The client-side check + self-quarantine logic is in scope; the actual hosting location/CDN is partly ops. Use a configurable separate base URL + separate trusted signing key, defaulting to a documented official endpoint.
- **`diag` output formatting** (table vs. sections) — human-readable is the only constraint.
- **Self-quarantine state representation** (e.g., a marker in `~/.beekeeper/state.json` / quarantine dir) — implementation detail.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase requirements & scope
- `beekeeper-prd.md` §9 (Configuration / layered merge) — CODE-05
- `beekeeper-prd.md` §10 (CLI surface) — `policy test/validate/list`, `diag` command shapes
- `beekeeper-prd.md` §11 (Threat model) + §12 (Self-defense and supply chain integrity) — CTLG-04, SFDF-06, threat-model docs
- `beekeeper-prd.md` §12.6 (recursive principle / `beekeeper-self`) — the self-quarantine contract
- `beekeeper-prd.md` §15 "v1.0.0" — capstone deliverable list
- `beekeeper-prd.md` §17 (Open questions) — `beekeeper-self` governance honesty note
- `.planning/ROADMAP.md` Phase 9 section — goal + 5 success criteria
- `.planning/REQUIREMENTS.md` — CODE-01..06, SFDF-06, CTLG-04 exact text

### Existing code to extend / respect (brownfield)
- `internal/config/config.go` — current config loader; CODE-05 extends merge order and adds project + env-var + flag layers
- `internal/policy/types.go`, `internal/policy/engine.go` — pure engine; policy files must map onto these inputs without breaking purity (CLAUDE.md: no I/O in `internal/policy`)
- `internal/catalog/sync.go`, `verify.go`, `state.go`, `sanity.go`, `multi.go` — catalog sync + signature verification; `beekeeper-self` is a new source plugged into this pipeline
- `internal/llamafirewall/` — `LatencyTracker` P95 ring buffer (Phase 6) reused by `diag`
- `internal/sentry/windows/` — ETW `EventsLost` counter (Phase 7) surfaced by `diag`
- `cmd/beekeeper/main.go` — Cobra command wiring for new `policy` and `diag` subcommands

### Project rules
- `CLAUDE.md` — Architecture Constraints (pure policy library, fail-closed, single static binary, reproducible builds), Self-Defense Non-Negotiables (Phase 9: `beekeeper-self` catalog live)

</canonical_refs>

<specifics>
## Specific Ideas

- Minimal example config and merge order are spelled out in PRD §9 (lines 463–477).
- CLI command shapes are exact in PRD §10 (lines 506–516).
- `beekeeper-self` self-quarantine narrative is PRD §12.6 (lines 618–626) — "refuses to run, surfaces the warning prominently, points the user to the verification path."
- Verification path (`make verify-release`, Sigstore, SLSA provenance, SBOM) is PRD §12.7 — the threat-model docs must link this.
- Catalog feed integrity rules (signatures required, no remote URL fetching, sanity bounds, degraded read-and-notify) are PRD §12.3 and apply to `beekeeper-self` like any source.

</specifics>

<deferred>
## Deferred Ideas

- **Distributed mode / team-shared catalogs** — listed under v1.0.0 core in PRD §15.5 but has no Phase 9 REQ-ID; not in scope for this phase.
- **Independent external security review** before tagging v1.0.0 (PRD §15.5, §17) — a release-process gate, not code deliverable.
- **Bug bounty / VDP scope publication** (PRD §15.5) — docs/process, can follow the threat-model writeup.
- **Weighted corroboration** (e.g., Bumblebee = 1.5 sources) — PRD §17 explicitly defers; v1 ships unweighted equal-vote.
- **`beekeeper-self` separate maintainer set** — PRD §17: documented as single-maintainer for v1.0.0 with intent to separate as the project grows.
- **macOS notarization / Windows trusted-publisher code signing** — PRD §17 accepts Sigstore-only verification path for v1.

</deferred>

---

*Phase: 09-policy-as-code-self-defense-capstone*
*Context gathered: 2026-05-29 via PRD Express Path*
