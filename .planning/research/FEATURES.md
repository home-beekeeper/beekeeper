# Feature Research

**Domain:** Adjudicated incident corpus with ground-truth outcome labeling for a local-first agent safety harness
**Milestone:** v1.4.0 "Adjudicated Corpus (Local Loop)"
**Researched:** 2026-06-13
**Confidence:** HIGH (PRD is authoritative source; prior-art claims verified against MISP, STIX, OSV, VirusTotal, MOTIF corpus, ThreatFox, MalwareBazaar)

---

## Scope Boundary

This file covers ONLY the NEW features introduced in v1.4.0. The following are already shipped and must NOT be re-researched or re-specced:

- Corroboration-based catalog matching (1/2/3-source tiers) — `internal/catalog`, `internal/policy`
- NDJSON audit log, sinks, rotation, tail/query/export — `internal/audit`
- Sentry detection rules SENTRY-001..008 with cluster correlation — `internal/sentry`
- First Responder reversible quarantine (move/restore/TUI card) — `internal/check`, `internal/tui`
- Bubble Tea v2 TUI quarantine card semantics (red=attacker, coral=Beekeeper) — `internal/tui`

---

## Prior-Art Survey

### How Prior Systems Model Confirmed-Label Provenance

**MISP (Malware Information Sharing Platform)**
MISP is the clearest prior art for adjudicated corpus design. It uses taxonomy-driven tags to express vetting state and analytic confidence per-attribute. Key patterns: (a) "confidence/vetting" tags differentiate automated from human-vetted indicators; (b) origin tags record whether a source was automated or manual — manual analysis supersedes automatic; (c) attribute-level tags can lower confidence on specific items within a high-confidence event; (d) false-positive contributions are first-class; (e) TLP tags govern sharing scope, decoupled from the confidence dimension. MISP's admiralty-scale taxonomy (A1..F6) maps directly to the concept of multi-source corroboration: A-grade (completely reliable source) with 1-Confirmed (confirmed by other independent sources) is effectively what Beekeeper calls `source_count >= 2` + `adjudication_source: catalog_confirmation`.
**Confidence: HIGH** (official MISP documentation at misp-project.org/best-practices-in-threat-intelligence)

**STIX 2.1**
STIX uses a `confidence` field (0-100, where 0 = no confidence, 100 = completely confident) on all STIX Domain Objects. Crucially, STIX also has a `revoked` boolean and a `defanged` property. The revoked flag is the equivalent of `true_label: benign` after a block — it marks an indicator as no longer valid. STIX decouples confidence from action: an indicator can have high confidence and no associated course of action, or low confidence with a watchlist action. This supports the Beekeeper pattern of `watch` weight at 1 source and `enforce` weight at 2+ sources.
**Confidence: MEDIUM** (STIX 2.1 OASIS spec structure confirmed via secondary sources; full spec truncated in direct fetch)

**OSV / GHSA (Open Source Vulnerability Schema)**
OSV uses a `withdrawn` timestamp for records that were retracted (false positive or incorrect advisory). It does NOT have a native confidence or adjudication field — database-specific metadata goes in the `database_specific` block. This is instructive as an anti-pattern: OSV's lack of a native confidence/provenance layer means downstream consumers cannot distinguish a community-reported vs analyst-confirmed advisory without parsing database-specific extensions. Beekeeper avoids this by making `adjudication_source` and `source_count` first-class schema fields.
**Confidence: HIGH** (official OSV schema at ossf.github.io/osv-schema)

**ThreatFox / MalwareBazaar (abuse.ch)**
ThreatFox uses a 0-100% confidence field per IOC. Default is 50 when submitter does not specify. Trusted Reporters (manually vetted contributors) bypass human review and go live immediately; standard submissions require human review first. MalwareBazaar only accepts confirmed malware (no benign/adware). This maps directly to the Beekeeper model: `adjudication_source: forensic_review` with manual analysis produces high confidence; `adjudication_source: catalog_confirmation` from a community feed (like Bumblebee) is intermediate confidence.
**Confidence: HIGH** (threatfox.abuse.ch/faq, abuse.ch blog introducing ThreatFox)

**VirusTotal multi-engine consensus**
VirusTotal's detection ratio (e.g., 6/72) is the canonical example of source-count-based corroboration. Research literature uses thresholds of t=2 to t=5 detections before labeling a sample malicious. The key insight: even t=1 (any detection) is meaningful as a watchlist signal, while t=2+ is required before actioning. This is exactly the Beekeeper warn(1)/enforce(2+) boundary. No single engine alone produces `enforce` weight.
**Confidence: HIGH** (multiple published papers on VirusTotal threshold selection; SecureScan arxiv 2602.10750)

**MOTIF Malware Reference Dataset**
The MOTIF dataset (3,095 samples, 454 families) established ground-truth labels through manual analysis of open-source threat intelligence reports. Key finding: manual analysis takes roughly 10 hours per sample, but the error rate is negligible and considered ground truth. Open-source TI reports from reputable organizations produce high-confidence labels. This validates Beekeeper's `forensic_review` (local/analyst inspection) as the highest-confidence adjudication source.
**Confidence: HIGH** (arxiv.org/abs/2111.15031, peer-reviewed, ScienceDirect published)

**SOC Alert Triage (async adjudication pattern)**
SOC best practice separates the detection event from the adjudication outcome. Analysts triage asynchronously, document investigative steps, and feed outcomes back into detection tuning. False positives are never silently closed — each is a labeled data point. This directly validates Beekeeper's design: the adjudication engine runs off the hot path, async after the verdict, and records `resolved_at` when the label is assigned. The `unresolved` state is the expected initial state. Research identifies a "true positive benign" (TPB) category — correct detection of threat-indicative behavior that investigation reveals was benign — which maps directly to Beekeeper's `policy_correct` label.
**Confidence: HIGH** (cyberdefenders.org, prophetsecurity.ai, ACM SIGOPS false positive fingerpointing paper)

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features without which the corpus system is incomplete or useless. These are non-negotiable for v1.

| Feature | Why Expected | Complexity | Notes | Depends On (existing) |
|---------|--------------|------------|-------|-----------------------|
| Four-layer event schema (behavior/decision/outcome/context) | Any corpus system requires a schema capturing what happened, what was decided, what the truth was, and the context. Without the outcome layer it is just a log. | MEDIUM | PRD §3.1 defines all fields. Conditional fields per `source_surface` (6 branches). `cluster_id` binds non-agent correlated events. | Existing NDJSON audit log structure |
| `true_label` field — four values: `malicious`, `benign`, `policy_correct`, `unresolved` | Ground-truth labeling is the entire value proposition. Without it the corpus is indistinguishable from a raw log. | LOW | Initial state is always `unresolved`. PRD §3.2. `policy_correct` distinguishes deterministic policy hits from actual attacks (SOC TPB pattern). | Schema (above) |
| `adjudication_source` field — six named values | Downstream consumers must know HOW a label was established to weight it correctly. A forensic review has different reliability than a user override. OSV's omission of this field is the key anti-pattern to avoid. | LOW | Six values: `catalog_confirmation`, `forensic_review`, `breach_confirmation`, `user_override`, `downstream_clean`, `benign_explained`. PRD §3.2. | `true_label` |
| `resolved_at` timestamp | Async adjudication needs a timestamp recording when the outcome was determined, separate from the event timestamp. Absent means still `unresolved`. | LOW | RFC3339. Pattern mirrors OSV's `withdrawn` field. | `true_label` |
| `was_correct` boolean | Operator-facing summary: did Beekeeper's verdict match the ground truth? Synthesized from `true_label` + `verdict`. Stored for query efficiency. | LOW | `policy_correct` counts as was_correct=true even if no attack occurred. | `true_label`, `verdict` |
| `source_count` recorded explicitly | VirusTotal/MISP prior art confirms: downstream consumers must NOT re-derive corroboration count. Record it at adjudication time. Enables warn-vs-enforce distinction without re-querying the catalog layer. | LOW | Integer. 1 = warn weight (`confidence_tier: watch`), 2+ = enforce weight (`confidence_tier: enforce`). PRD §3.2. | Existing corroboration engine in `internal/catalog` |
| `confidence_tier` in push envelope: `watch` vs `enforce` | Push consumers cannot safely re-derive the tier. It must be frozen at emission time alongside `source_count`. | LOW | Maps directly to existing 1-source/2-source corroboration semantics. PRD §3.5. | `source_count` |
| Async adjudication off the hot path | The hot path (beekeeper check, gateway, sentry) must not block on corpus adjudication. Table stakes for a safety harness whose core SLA is sub-100ms. | MEDIUM | Adjudication engine runs after verdict, not before. No I/O allowed in `internal/policy` (architecture invariant). | Pure-function policy engine constraint (shipped) |
| Append-only local corpus store | Forensic integrity requires append-only. Any mutation invalidates the corpus as evidence. MISP, OSV, and MalwareBazaar all use append/withdrawal rather than update. | MEDIUM | Extends existing NDJSON audit log. Owner-only (0600). Same sink model as audit log. PRD §3.3. | Existing audit log (`internal/audit`) |
| Records emitted in push-envelope shape from day one | The moat is built on the first run. Records in a different shape require a migration that touches every historical record. | MEDIUM | No transport in v1, but the wire format is frozen and emitted locally. PRD §§3.3, 3.5. | Push-envelope schema (§3.5) |
| `scope` tag on every record from birth: `org_only` (default) vs `community_shareable` | A re-tagging migration when push paths exist is expensive and error-prone. Default `org_only` prevents accidental sharing. TLP pattern from MISP: scope is a first-class field, not an afterthought. | LOW | Default `org_only`. Promotion requires anonymization (v2.0 gate). PRD §3.6. | None |
| `cluster_id` binding for non-agent incidents | `file_watcher`, `sentry`, and `scan` incidents are correlated event clusters, not single events. Adjudicating a single event in isolation is incorrect for these surfaces. | MEDIUM | `file_watcher`/`sentry`/`scan` surfaces adjudicate the cluster. `hook`/`mcp_gateway`/`shim` adjudicate per event. PRD §3.1. | Existing Sentry correlation window |
| First Responder corpus binding: confirmed-malicious arms TUI quarantine card | The corpus is only useful if it drives action. Confirmed malicious adjudication must propagate to the quarantine layer without requiring manual re-run. | MEDIUM | Arms card for any matching local install. Does NOT auto-purge. PRD §3.4. | Existing First Responder quarantine (shipped) |
| Purge remains human/org-policy gated — never automatic | If purge were automatic, a poisoned adjudication could silently destroy a developer's environment. Non-negotiable safety property. | LOW | Purge stays a local human-confirmed or org-policy-gated action in ALL versions. PRD §3.4. | Existing First Responder purge gate (shipped) |
| Restore intact after adjudication | Reversibility is already shipped. Adjudication must not remove the ability to restore a quarantined item. | LOW | Red = attacker action, coral = Beekeeper response per locked TUI semantic. | Existing quarantine/restore (shipped) |
| `action_hint: watch_and_block` as the ONLY pushable action | Constraining the pushable action set at format-definition time prevents a future transport from accidentally carrying destructive actions. This is a blast-radius guard baked into the schema. | LOW | `auto_purge` is NEVER present in a pushable envelope. Enforced in the schema definition and any serialization code. PRD §3.5. | Push-envelope schema |

### Differentiators (Competitive Advantage)

Features that make this corpus system better than logging without outcome labels, and that build the irreplaceable moat the PRD describes.

| Feature | Value Proposition | Complexity | Notes | Depends On |
|---------|-------------------|------------|-------|------------|
| Six semantically distinct `adjudication_source` values with defined confidence implications | Most incident databases (OSV, raw SIEMs) do not capture HOW a label was established. The source of the label is as important as the label itself. `breach_confirmation` is qualitatively different from `user_override`. MISP's admiralty scale shows the prior art for this distinction. | LOW | Confidence mapping: `breach_confirmation` = highest; `forensic_review` = high; `catalog_confirmation` = medium; `benign_explained` = medium benign; `downstream_clean` = weak benign; `user_override` = weak FP signal only. | `adjudication_source` schema field |
| `behavior_signature_hash` in push envelope | Attackers change package names but reuse behavior. A hash over the behavior pattern enables matching across renamed variants — a key advantage over pure catalog name-matching. | HIGH | Hash over `action_type` + `target_resource` pattern + `network_destination` pattern. Must be stable across schema versions (a breaking change if it changes). | Four-layer schema |
| IOC fields in push envelope: `domains`, `dns_tunnel_pattern`, `dead_drop_pattern` | Most local corpus systems capture package identity only. Network behavior IOCs from Sentry mean a confirmed incident contributes pattern-level detection, not just name-level blocklisting. | HIGH | `iocs` block in push signature. Populated from Sentry DNS/network events (SENTRY-003/007). Requires Sentry event source data to be present in the cluster. | Sentry SENTRY-003/007 (shipped) |
| `repo_fingerprint` (hashed, non-reversible) in context layer | Enables per-repository corpus queries without exposing the repository name. Supports "has this repo seen similar incidents?" without transmitting the repo path. | MEDIUM | SHA-256 of canonical repo root path. Non-reversible by design. | Context layer schema |
| `fleet_node_id` (anonymized) in context layer | Enables multi-machine corpus correlation when push exists — same attack pattern on multiple machines — without exposing machine identity. Correct from v1. | MEDIUM | Stable anonymized host identifier. Not per-run. Complies with OTX/MISP pattern of stripping contributor identity while preserving fleet signal. | Context layer schema |
| `policy_correct` as a distinct `true_label` (not just `benign`) | Standard incident databases use true/false positive binary. `policy_correct` captures correct deterministic policy rule fires (e.g., credential path block) where no actual attack occurred. This prevents false-positive remediation of correct rules. SOC literature calls this "true positive benign." | LOW | Derived from: `verdict == block` AND deterministic policy rule (not catalog-based) AND no downstream evidence of malicious intent. | `true_label`, `verdict`, `policy_matched` |
| `unresolved` as a first-class state (not just absence of label) | An incident should never be implicitly treated as benign because no one labeled it. `unresolved` is an explicit pending state that drives analyst queue ordering and prevents silent false-negative accumulation. | LOW | Default for all new records. Transitions to other states via adjudication engine based on subsequent evidence. | Adjudication engine |
| Push-envelope signing fields pre-defined even when transport is absent | By defining `signing.issuer`, `signing.signature`, `signing.issued_at`, `signing.nonce` in the envelope schema now, the v1.1 transport implementation is wiring, not redesign. Same pattern used in v1.0.0 for Sigstore (reproducible from first commit). | LOW | Signing fields populated when transport exists. Empty/null in v1. Frozen schema prevents a breaking wire format change at v1.1. | Push-envelope schema |
| Corpus feedback loop: confirmed-malicious → Sentry watch elevation | A confirmed-malicious adjudication can raise a Sentry watch on the specific process tree associated with the incident, providing detection coverage beyond the initial block event. The corpus becomes an active defense signal, not just a passive record. | HIGH | Adjudication engine emits a watch directive to Sentry rule engine. Not just TUI card — active detection elevation. PRD §4 Phase 3 acceptance criteria. | Sentry rule engine (`internal/sentry`, shipped) |
| Corpus feedback loop: confirmed-malicious → local catalog overlay | A confirmed-malicious adjudication can add a local-only catalog entry for the package/extension, so future installs are caught without waiting for upstream catalog sync lag (which the PRD identifies as an hour or more). | HIGH | Local catalog overlay — does not mutate the synced catalog. Owner-only. Survives catalog re-sync. | `internal/catalog` (shipped) |

### Anti-Features (Commonly Requested, Often Problematic)

Features that must be explicitly excluded from this milestone — and from all future versions where noted.

| Anti-Feature | Why It Seems Good | Why Problematic | Scope of Exclusion | Alternative |
|--------------|-------------------|-----------------|--------------------|-------------|
| `auto_purge` as a pushable action | "If we know a package is malicious, shouldn't we automatically remove it?" | A fleet-wide destructive action triggered by a remote signal is itself an attack surface. A poisoned or compromised push channel could silently destroy developer environments. Even a legitimate but incorrect adjudication (user_override false positive) would cause data loss with no recovery path. | ALL versions — never in the pushable set. PRD §3.5 states this explicitly. | `watch_and_block` is the pushable action. Purge stays human/org-policy gated locally. |
| Network transmission of corpus data in v1 | "Why not send data to a central server while we build the corpus?" | Transmitting incident data (which includes `target_resource`, `network_destination`, `actor_lineage`) before anonymization is complete and before a trust relationship with the user is established is a privacy violation. Rushing transmission before the schema is hardened means a breaking wire change at v1.1. | v1 ONLY. Transmission is planned for v1.1-1.9 (org) and v2.0 (community). | Records are emitted in push-envelope shape locally, so transport is wiring-only when it arrives. |
| Automatic `community_shareable` promotion | "Once confirmed malicious, why not share it automatically?" | `community_shareable` records include behavior signatures and IOCs that, even partially anonymized, can be re-identified if the attacker knows which packages they deployed. Promotion must be gated by an explicit anonymization step verified by the operator. | v1 and v2.0 — promotion is always explicit, never automatic. PRD §3.6. | Explicit `scope` upgrade gated by anonymization pipeline (v2.0 step). |
| Org aggregator or push fan-out in v1 | "Wouldn't it be valuable to push to all my machines immediately?" | The push channel is a high-blast-radius attack surface. It must be built and hardened post-launch across the v1.1-1.9 band before carrying enforce-weight signals to other machines. | v1 ONLY. | v1.1-1.9 scope per PRD §6. `watch_and_block` constraint plus corroboration gate guard the blast radius when it ships. |
| Community shared corpus feed in v1 | "A shared community feed would grow the corpus faster." | Requires maintained infrastructure, anonymization pipeline, abuse resistance, signed pushes, and rate limiting. Rushing these to v1 adds complexity without the foundation. | v1 ONLY. | v2.0 scope per PRD §7. Local corpus builds the asset; community feed propagates it. |
| SENTRY-008 CI runner OIDC theft detection | "Can we detect token theft from CI runners in the corpus?" | CI runners typically have no editor/host agent. Detection would require out-of-scope event sources (DNS, process memory). Named as a documented residual gap in PRD §8, not a detection target. | v1.4.0 and near-term future. | Architectural mitigation: scoped/ephemeral tokens, trusted-publisher policy. Document in threat model. |
| GitHub API dead-drop exfil detection | "Can we detect exfil via legitimate GitHub API calls?" | A host-level tool cannot reliably distinguish a legitimate push from malicious repo creation with stolen secrets. The signal is architecturally undetectable from host scope. | v1.4.0 and near-term future. | Architectural mitigation only. Named in threat model per PRD §8. |
| Corpus data mutation or retroactive relabeling | "What if the ground truth changes? We should update labels." | Append-only is a forensic requirement. A mutable corpus can be tampered with after the fact. OSV uses `withdrawn` timestamp rather than deletion or edit for this reason. | ALL versions — schema is append-only. | Superseding adjudication records: a new record references the prior `event_id` and overrides the outcome layer. Original record is preserved. |
| Weighted fractional corroboration scores (0.5 source weight) | "Some sources are more reliable; shouldn't we weight them?" | Identified as TM-B-01 in the threat audit (2026-06-05) — a core semantics change requiring designed change. The current binary 1/2+ model is intentional and auditable. Fractional weights make the warn/enforce boundary non-obvious and harder to reason about. | v1.4.0; deferred per v1.2.0 audit. | Document as a future research item. Current model aligns with VirusTotal t=2 pattern and MISP 2-source confirmation. |

---

## Feature Dependencies

```
[Four-layer schema (behavior/decision/outcome/context)]
    └──required-by──> [true_label field]
    └──required-by──> [adjudication_source field]
    └──required-by──> [was_correct boolean]
    └──required-by──> [resolved_at timestamp]
    └──required-by──> [source_count field]
    └──required-by──> [cluster_id binding]
    └──required-by──> [scope tag]
    └──required-by──> [push-envelope shape emission]

[source_count field]
    └──feeds──> [confidence_tier: watch vs enforce]
    └──depends-on──> [existing corroboration engine in internal/catalog — shipped]

[true_label: malicious]
    └──arms──> [First Responder TUI quarantine card]
    └──optionally-elevates──> [Sentry watch directive (corpus->Sentry feedback)]
    └──optionally-adds-to──> [local catalog overlay]

[First Responder TUI quarantine card]
    └──depends-on──> [existing quarantine/restore — shipped v1.0.0]
    └──depends-on──> [existing TUI red/coral semantic — locked]
    └──gate──> [human/org-policy confirmation for purge — NEVER automatic]

[push-envelope schema]
    └──requires──> [action_hint constraint: watch_and_block ONLY]
    └──requires──> [scope field on every record]
    └──requires──> [confidence_tier derived from source_count]
    └──defers──> [signing fields populated only when transport exists]
    └──invariant──> [auto_purge NEVER present — all versions]

[scope: org_only (default)]
    └──explicit-promotion-only──> [community_shareable]
    └──gate──> [anonymization step — v2.0]
    └──NOT-automatic──> [community_shareable promotion]

[cluster_id]
    └──depends-on──> [existing Sentry correlation window — shipped]
    └──governs──> [adjudication unit for file_watcher/sentry/scan surfaces]
    └──NOT-for──> [hook/mcp_gateway/shim — these adjudicate per event]

[append-only corpus store]
    └──extends──> [existing NDJSON audit log — internal/audit]
    └──same-sink-model-as──> [audit log — syslog, OTLP, HTTPS]
    └──no-transmission-in-v1──> [all records stay local]
    └──owner-only──> [0600 permissions, same as audit log]

[adjudication engine]
    └──runs──> [async, off hot path]
    └──NOT-in──> [internal/policy — pure function library invariant]
    └──reads──> [audit log events and catalog state]
    └──writes──> [outcome layer on corpus records]
    └──triggers-on-malicious──> [First Responder card arm]
```

### Key Dependency Notes

- **Schema must be frozen before corpus store is built.** A schema change after corpus records exist requires a migration; there is no migration tooling in v1. PRD Phase 0 gate: every field in the Nx Console worked trace maps to a schema field with no gaps.
- **`source_count` depends on the existing corroboration engine.** The adjudication engine reads the corroboration result already computed at verdict time; it does not re-query sources.
- **`cluster_id` binding depends on existing Sentry correlation.** The Sentry correlation window and cluster key are already in place (SENTRY-001..008); the corpus records the cluster identity.
- **Adjudication engine must NOT be in `internal/policy`.** That package is a pure function library (architecture invariant, shipped). The adjudication engine is I/O-bound (reads audit records, writes corpus records, arms TUI) and must live in an adapter layer — likely `internal/corpus/` or `internal/adjudication/`.
- **First Responder binding depends on existing quarantine card infrastructure.** The card arming is new behavior; the card itself, restore, and purge gate are already shipped.
- **`behavior_signature_hash` and IOC fields depend on Sentry event data being present.** These fields can only be populated for incidents where Sentry produced network/DNS events (SENTRY-003, SENTRY-007). Other `source_surface` values will have these fields absent or empty.

---

## MVP Definition

### v1: Launch With (Scope = PRD §3 only)

All items below are required for v1.4.0. None are deferrable within this milestone per the PRD non-goals.

- [ ] Four-layer event schema frozen — behavior/decision/outcome/context with all conditional fields per `source_surface`. `cluster_id` binds correlated non-agent incidents.
- [ ] Push-envelope wire format frozen and emitted locally — all fields including signing stubs, `action_hint: watch_and_block` constraint baked in, `auto_purge` absent.
- [ ] Adjudication engine: async assignment of `true_label`, `adjudication_source`, `was_correct`, `resolved_at`, `source_count`, `confidence_tier` — off the hot path, not in `internal/policy`.
- [ ] Append-only corpus store extending the existing NDJSON audit log — owner-only (0600), no transmission, records emitted in push-envelope shape.
- [ ] `scope` tag on every record from birth — `org_only` default, promotion explicit and not automated.
- [ ] First Responder corpus binding — confirmed-malicious adjudication arms TUI quarantine card for any matching local install; purge stays human/org-policy gated; restore intact.
- [ ] All 8 Sentry patterns (SENTRY-001..008) produce moat-grade records with all four layers populated (PRD §4 Phase 3 acceptance criterion).
- [ ] Offline/disconnected machine remains fully protective on last synced catalog — corpus loop is out of the hot path.

### v1.1-1.9: Add After Launch (Org Self-Host)

- [ ] Org aggregator endpoint — self-hosted, receives signed signatures from org machines
- [ ] Push fan-out within org — `watch_and_block` only, corroboration gate before enforce weight
- [ ] Signed push channel — per-push signature verification, authenticated pub/sub
- [ ] Offline fallback when push channel is absent
- [ ] Anonymization pipeline for `org_only` to `community_shareable` promotion

### v2.0: Future (Community Shared Corpus Feed)

- [ ] Maintained shared host (community ingestion and fan-out endpoint)
- [ ] Community catalog as a pull source alongside Bumblebee/OSV/Socket/Pollen
- [ ] Community push fan-out with `watch_and_block` constraint and corroboration gate
- [ ] Abuse resistance — signed pushes, rate limiting, poisoning resistance

---

## Feature Prioritization Matrix

| Feature | Moat Value | Implementation Cost | v1 Priority | Category |
|---------|------------|---------------------|-------------|----------|
| Four-layer schema freeze | CRITICAL | MEDIUM | P0 (Phase 0) | Table Stakes |
| Push-envelope format freeze | CRITICAL | MEDIUM | P0 (Phase 0) | Table Stakes |
| `true_label` + `adjudication_source` + `was_correct` + `resolved_at` | CRITICAL | LOW | P0 | Table Stakes |
| `source_count` + `confidence_tier` recorded explicitly | HIGH | LOW | P0 | Table Stakes |
| `scope` tag on every record | HIGH | LOW | P0 | Table Stakes |
| `action_hint: watch_and_block` ONLY constraint | CRITICAL (safety) | LOW | P0 | Table Stakes |
| `auto_purge` never pushable (invariant) | CRITICAL (safety) | LOW | P0 | Anti-Feature guard |
| Append-only corpus store | CRITICAL | MEDIUM | P1 (Phase 1) | Table Stakes |
| Adjudication engine (async, off hot path) | CRITICAL | MEDIUM | P1 | Table Stakes |
| `cluster_id` binding for non-agent incidents | HIGH | MEDIUM | P1 | Table Stakes |
| First Responder corpus binding (arm quarantine card) | HIGH | MEDIUM | P2 (Phase 2) | Table Stakes |
| Purge stays human/policy gated | CRITICAL (safety) | LOW | P2 | Table Stakes |
| `policy_correct` as distinct label from `benign` | MEDIUM | LOW | P1 | Differentiator |
| `unresolved` as explicit first-class state | HIGH | LOW | P1 | Differentiator |
| Six `adjudication_source` values with defined confidence | HIGH | LOW | P1 | Differentiator |
| `repo_fingerprint` + `fleet_node_id` in context | MEDIUM | MEDIUM | P1 | Differentiator |
| Signing fields pre-defined in envelope (empty in v1) | HIGH (v1.1 unlock) | LOW | P1 | Differentiator |
| `behavior_signature_hash` in push envelope | HIGH (moat depth) | HIGH | P1 | Differentiator |
| IOC fields in push envelope | HIGH (moat depth) | HIGH | P1 | Differentiator |
| Corpus feedback to Sentry watch elevation | MEDIUM | HIGH | P2 | Differentiator |
| Corpus feedback to local catalog overlay | MEDIUM | HIGH | P2 | Differentiator |
| Org aggregator / push fan-out | HIGH (v1.1) | HIGH | OUT OF v1 SCOPE | Deferred |
| Community feed / anonymization pipeline | HIGH (v2.0) | VERY HIGH | OUT OF v1 SCOPE | Deferred |

**Priority key per PRD §4 phase plan:**
- P0: Schema-freeze / safety-guard — Phase 0 gate (schema and envelope lock). Must complete before any Phase 1 work.
- P1: Must ship in Phase 1 (corpus store + adjudication engine) or Phase 2 (First Responder binding).
- P2: Phase 2 or Phase 3 (launch readiness).
- OUT OF v1 SCOPE: PRD explicitly defers these to v1.1-1.9 or v2.0.

---

## Adjudication Source Confidence Mapping

Prior-art-informed mapping from each `adjudication_source` to its implied confidence level and establishment method. Feeds REQUIREMENTS.md adjudication category.

| adjudication_source | Established By | Confidence Level | Effect on source_count | Typical Latency | Prior Art Basis |
|--------------------|---------------|-----------------|------------------------|-----------------|-----------------|
| `catalog_confirmation` | A tracked catalog (Bumblebee/OSV/Socket/Pollen) adds an entry matching the incident after the event | MEDIUM | Adds 1 to source_count (may push from warn to enforce if count was 1 at verdict time) | Hours to days (catalog lag) | OSV withdrawal pattern; ThreatFox community vetting |
| `forensic_review` | Local or analyst inspection of payload or behavior | HIGH | Standalone basis for true_label assignment | Minutes to hours | MOTIF ground-truth methodology; MISP origin:manual supersedes origin:automated |
| `breach_confirmation` | Downstream incident validates the alert — exfil proven, data loss confirmed | HIGHEST | Definitive confirmation | Hours to days after incident | MISP admiralty scale A1 (completely reliable, confirmed); ThreatFox: manual vet at 100% confidence |
| `user_override` | Developer allowed a block | LOW (weak FP signal) | Does NOT change confidence_tier alone; signals likely false positive | Immediate (user action) | MISP: attribute-level confidence lowering; SOC: false positive feedback loop |
| `downstream_clean` | An allow produced no subsequent incident within a defined observation window | LOW (weak benign signal) | Weak benign signal | Days to weeks | Absence of evidence is not evidence of absence; observation window must be defined |
| `benign_explained` | Apparent exfil traced to a known-good source via active investigation | MEDIUM (benign) | Clears suspicious verdict | Minutes to hours | More confident than `downstream_clean`; requires active investigation to establish the known-good path |

---

## REQUIREMENTS.md Category Structure

The downstream REQUIREMENTS.md should group requirements into these categories with the following prefix codes:

| Category Prefix | Covers |
|----------------|--------|
| `SCHEMA` | Field definitions per `source_surface`, conditional fields, `cluster_id`, `scope`, push-envelope format freeze, `action_hint` constraint, signing stub fields |
| `ADJUDICATION` | Async off-hot-path engine, `true_label` transitions, `adjudication_source` assignment rules, `was_correct` derivation, `resolved_at` timestamping, `source_count` recording, `confidence_tier` mapping |
| `CORPUS-STORE` | NDJSON append-only store, owner-only permissions, same-sink model as audit log, no transmission in v1, records in push-envelope shape from first write |
| `FIRST-RESPONDER-BINDING` | Confirmed-malicious adjudication → TUI card arm, purge gate (human or org-policy, never automatic), restore intact, red/coral TUI semantic preserved |
| `PUSH-ENVELOPE` | Local emission in push-envelope shape, `watch_and_block` ONLY in pushable set, `auto_purge` never emitted, `scope` field present, `confidence_tier` from `source_count` |
| `SCOPE-TAGGING` | `org_only` default, explicit promotion to `community_shareable`, promotion gated (anonymization v2.0), never automatic, scope present from record birth |

---

## Sources

- PRD: `beekeeper-corpus-milestone-prd.md` (repo root) — authoritative scope for v1.4.0; HIGH confidence
- MISP best practices: https://www.misp-project.org/best-practices-in-threat-intelligence.html — confidence/vetting taxonomy patterns, admiralty scale, TLP scope decoupling; HIGH confidence
- OSV schema: https://ossf.github.io/osv-schema/ — `withdrawn` field, absence of native confidence layer as instructive anti-pattern; HIGH confidence
- ThreatFox FAQ: https://threatfox.abuse.ch/faq/ — 0-100% confidence, Trusted Reporter vetting bypass; HIGH confidence
- MOTIF malware corpus: https://arxiv.org/abs/2111.15031 — ground-truth labeling methodology, manual analysis as gold standard, ~10h per sample; HIGH confidence
- VirusTotal threshold research: SecureScan (arxiv 2602.10750) and related papers — t=2 as standard enforce threshold, t=1 as watchlist threshold; HIGH confidence
- SOC alert triage: cyberdefenders.org/blog/alert-triage-process, prophetsecurity.ai/blog/alert-triage — async adjudication, outcome documentation, TPB (true positive benign) category; HIGH confidence
- ACM false positive fingerpointing: https://dl.acm.org/doi/fullHtml/10.1145/3370084 — continuous improvement feedback loop from labeled outcomes; HIGH confidence
- STIX 2.1 spec: https://docs.oasis-open.org/cti/stix/v2.1/os/stix-v2.1-os.html — confidence field 0-100, `revoked` boolean, decoupled action constraint; MEDIUM confidence (full spec partially truncated in fetch)
- OTX (AlienVault Open Threat Exchange) Wikipedia — victim identity stripping pattern for community sharing; MEDIUM confidence

---

*Feature research for: Beekeeper v1.4.0 Adjudicated Corpus (Local Loop)*
*Researched: 2026-06-13*
