# Research Summary -- v1.4.0 "Adjudicated Corpus (Local Loop)"

**Project:** Beekeeper
**Domain:** Adjudicated incident corpus with ground-truth outcome labeling for a local-first agent safety harness
**Researched:** 2026-06-13
**Confidence:** HIGH (all four research files code-grounded; zero new external deps; all stack decisions are stdlib-only or already in go.mod)

---

## Executive Summary

Beekeeper v1.4.0 introduces an adjudicated incident corpus -- an append-only local store where every detection event is promoted into a four-layer record (behavior / decision / outcome / context) and the outcome layer is filled in asynchronously as corroborating evidence arrives. The moat thesis driving this milestone is that the outcome layer -- confirmed true_label, established adjudication_source, and frozen source_count -- is the only part of a corpus that cannot be retrofitted. An identical detection engine run by a competitor is just logs. The same engine with outcome labels attached is an asset. This makes the schema lock and the first write correct the highest-priority deliverables in the entire milestone: every field must be present, including true_label: "unresolved" as an explicit placeholder, from the very first run.

The recommended approach is zero new external dependencies: crypto/ed25519 (stdlib) for the push-envelope signing block, crypto/hmac + crypto/sha256 for non-reversible repo_fingerprint and fleet_node_id generation, and plain O_APPEND NDJSON for the corpus store extending the existing audit.Writer pattern. The new internal/corpus package is deliberately impure (it does file I/O, time reads, and random operations) but consumes internal/policy and internal/sentry as read-only pure-function dependencies, preserving the architecture invariant that neither of those packages may have I/O or goroutines. The corpus store implements the existing audit.Sink interface so the MultiSink fan-out wires it in with zero per-surface code changes.

The key risks are all schema-time risks: a non-retrofittable outcome layer if the schema is not frozen before the corpus store is built; secrets leaking into the corpus NDJSON if audit.RedactRecord is not called on every write; and the auto_purge blast-radius failure mode if the push envelope action_hint allowlist is not a compile-time typed constant from day one. All three are Phase 22 and Phase 23 concerns that cannot be fixed retroactively. The phase ordering below is derived from these non-retrofittable dependencies.

---

## Key Findings

### Recommended Stack

All v1.4.0 corpus capabilities are achievable with Go stdlib only and the packages already present in go.mod. No new entries to go.mod are required for any Phase 22-25 deliverable. The critical stack decisions are:

- internal/corpus is a **new impure package** (types, store, emitter, adjudicator, signer, fingerprint, scope). It has I/O and goroutines -- unlike internal/policy and internal/sentry which must remain pure.
- CorpusRecord **embeds audit.AuditRecord** via an inline field (json:",inline"), carrying the behavior and decision layers from the existing audit infrastructure and adding the outcome and context layers. No duplication of the behavior/decision schema.
- The corpus store is an **audit.Sink implementation** (corpus.StoreSink) -- added to audit.MultiSink at daemon startup when cfg.Corpus.Enabled == true. All six source surfaces gain corpus writing with zero per-surface code changes.
- **Ed25519 stdlib** (crypto/ed25519) for the signing block in the push envelope. Key generated once into ~/.beekeeper/corpus-signing.key (0600). Signing fields populated locally but not transmitted in v1. Cosign/sigstore stays CI-only.
- **HMAC-SHA256** (crypto/hmac + crypto/sha256) with a per-install random 32-byte salt stored in state.json under corpus.local_salt for repo_fingerprint and fleet_node_id. Non-reversible without the salt. The v2.0 anonymization step is a one-function swap (replace local_salt with community_salt).
- **Plain O_APPEND NDJSON**, no hash chaining in v1. Hash chaining adds value only for multi-party audit trails with an external verifier. A prev_hash: "" placeholder field is reserved in the schema for v1.1 activation.

**Core technologies:**
- internal/corpus (new): impure adapter package -- four-layer types, append-only store, async adjudication engine, emitter adapter, Ed25519 signer, HMAC fingerprinter, scope guard
- crypto/ed25519 stdlib: push-envelope signing block -- 64-byte sig, deterministic, zero new deps, forward-compatible with sigstore
- crypto/hmac + crypto/sha256 stdlib: repo_fingerprint and fleet_node_id -- non-reversible without per-install salt, v2.0 rewire is 1-function swap
- audit.Sink interface (existing): corpus store integration point -- zero per-surface changes, inherits MultiSink fan-out

### Expected Features

The feature set is organized into six requirement categories: SCHEMA / ADJUDICATION / CORPUS-STORE / FIRST-RESPONDER-BINDING / PUSH-ENVELOPE / SCOPE-TAGGING.

**Must have -- table stakes (all required for v1.4.0):**
- Four-layer event schema frozen (behavior/decision/outcome/context) with all conditional fields per source_surface -- the entire moat thesis depends on this being correct from run 1
- true_label field (malicious / benign / policy_correct / unresolved), adjudication_source (6 named values), resolved_at, was_correct -- ground-truth labeling is the value proposition
- source_count and confidence_tier (watch=1 source, enforce=2+ sources) recorded explicitly at adjudication time -- downstream consumers must not re-derive these
- scope tag on every record from birth: org_only default, explicit promotion only -- avoids re-tagging migration when transport arrives
- action_hint: watch_and_block as the ONLY value in the pushable set; auto_purge never present -- compile-time typed constant from Phase 22
- Push-envelope wire format frozen and emitted locally with signing stubs populated -- transport in v1.1 is wiring, not redesign
- Append-only local corpus store (beekeeper-corpus.ndjson, 0600, owner-only) extending existing audit log pattern
- Async adjudication engine off the hot path -- hook handler must never block on corpus writes
- cluster_id binding for non-agent incidents (file_watcher, sentry, scan adjudicate the cluster, not individual events)
- First Responder corpus binding: confirmed-malicious arms TUI quarantine card; purge stays human/org-policy gated; restore intact

**Should have -- differentiators:**
- Six semantically distinct adjudication_source values with defined confidence implications -- absence of this is the OSV anti-pattern
- policy_correct as a distinct label from benign -- prevents false-positive remediation of correct deterministic rules
- unresolved as explicit first-class state, not absence of label -- drives analyst queue ordering
- behavior_signature_hash in push envelope -- matches renamed attack variants, not just package names
- IOC fields (domains, DNS-tunnel pattern, dead-drop pattern) from Sentry SENTRY-003/007 events
- repo_fingerprint + fleet_node_id (HMAC-derived, non-reversible) in context layer
- Corpus feedback: confirmed-malicious adjudication arms Sentry watch elevation on matching process tree
- Corpus feedback: confirmed-malicious adds local catalog overlay for immediate protection ahead of catalog sync lag

**Defer to v2+:**
- Org aggregator / push fan-out (v1.1-1.9 scope per PRD section 6)
- Community shared corpus feed (v2.0 scope per PRD section 7)
- community_shareable scope promotion with anonymization pipeline (v2.0 gate)
- Hash chaining in corpus store (v1.1+ once multi-party verifier exists)
- Weighted fractional corroboration scores (TM-B-01 deferred per v1.2.0 audit)

**Anti-features -- never implement:**
- auto_purge in any pushable envelope -- ever, in any version. Blast radius is unbounded.
- Automatic community_shareable scope promotion -- anonymization step is required
- Network transmission of corpus data in v1 -- schema must be hardened first
- Corpus data mutation or retroactive relabeling -- append-only is a forensic requirement

### Architecture Approach

internal/corpus is a new impure package that sits between the existing pure-function core (policy, sentry) and the existing I/O infrastructure (audit, quarantine, tui). The key architectural seam is audit.MultiSink: the corpus store is added to the fan-out graph at daemon startup so all six source surfaces write corpus records without any per-surface code changes. The adjudication engine runs as an async background goroutine (channel-fed, 1024-record buffer) draining off the hot path. Confirmed-malicious adjudications propagate to the TUI via a corpus_quarantine_armed AuditRecord written to the standard audit log, which the existing 2-second poll cycle picks up without any new IPC channel.

**Major components:**
1. internal/corpus/types.go -- CorpusRecord (embeds AuditRecord), PushEnvelope, SigningBlock, four-layer schema types; ActionHint typed const enforcing watch_and_block only
2. internal/corpus/store.go -- StoreSink implementing audit.Sink; StateDir/corpus/beekeeper-corpus.ndjson at 0600; calls audit.RedactRecord before every write
3. internal/corpus/emitter.go -- impure adapter; maps AuditRecord to CorpusRecord; computes cluster_id per source_surface; populates PushEnvelope; derives behavior_signature_hash
4. internal/corpus/adjudicator.go -- async goroutine; reads unresolved records; calls policy.CorroborateOutcome() (exported 2-line wrapper, no policy I/O); writes outcome layer; on confirmed-malicious: writes corpus_quarantine_armed AuditRecord + calls sentry.AddTarget()+sentry.SaveTargets()
5. internal/corpus/signer.go -- Ed25519 keygen/sign/key-persistence; crypto/ed25519 + crypto/rand
6. internal/corpus/fingerprint.go -- RepoFingerprint() and FleetNodeID() via HMAC-SHA256 with per-install salt
7. internal/corpus/scope.go -- ScopeOrgOnly const; PromoteScope() returns error in v1 ("gated until v2.0 anonymization")

**Modified components:**
- internal/audit/types.go -- add cluster_id, scope, baseline_deviation, repo_fingerprint, fleet_node_id, source_surface as omitempty fields; add corpus_quarantine_armed record_type
- internal/audit/sink.go -- NewMultiSink adds corpus.StoreSink when cfg.Corpus.Enabled
- internal/policy/corroboration.go -- export CorroborateOutcome() 2-line wrapper (no behavioral change)
- internal/config/config.go -- add CorpusConfig{Enabled, Path, Scope}
- internal/tui/incidents.go -- add CorpusAdjudicationIncidentFromRecord() constructor
- internal/tui/quarantine_panel.go -- loadItems() recognizes corpus_quarantine_armed record type

### Critical Pitfalls

All 8 pitfalls from PITFALLS.md are HIGH confidence (grounded in prior milestone hard-won lessons). The top 5 non-retrofittable ones:

1. **Non-retrofittable schema** (Phase 22) -- outcome fields (true_label, adjudication_source, was_correct, resolved_at) and source_count/confidence_tier must be emitted from the very first write as explicit "unresolved" placeholders. Deferring them to Phase 23 loses data forever. Evaluator gate for Phase 22 must assert all 18 PRD section 3.1 fields are present in the synthetic NDJSON record.

2. **Corpus bypasses audit.RedactRecord** (Phase 23) -- the corpus store must call audit.RedactRecord(record, audit.DefaultRedactPatterns) before every NDJSON write. The F-1 finding from the 2026-06-12 security review was exactly this mistake on a different code path. A test asserting that an AWS-key-shaped target_resource is redacted in the persisted corpus is a required Phase 23 gate.

3. **source_count double-counting and single-source enforce** (Phase 23) -- source_count must be len(CorroborationResult.Sources) (deduplicated set of source identifiers), NOT a count of match events. Three Bumblebee match events yield source_count: 1. A single-source block from per-severity escalation must emit confidence_tier: "watch" in the corpus even if beekeeper check returned a block verdict. Table-driven test is a required Phase 23 gate.

4. **Scope-tag leakage** (Phase 22) -- org_only must be the compile-time zero value of the Scope field (enforced at struct construction, not as a serializer fallback). PromoteScope() always returns an error in v1. A unit test asserting CorpusRecord{} serializes as "scope": "org_only" is a required Phase 22 gate.

5. **Adjudication on the hot path** (Phase 23) -- the corpus write path must be async (non-blocking channel send + background goroutine drain). For beekeeper check (short-lived process), the goroutine must sync.WaitGroup.Wait() with a 200ms hard deadline before os.Exit. A corpus write error must NOT change the hook exit code. BenchmarkRunCheck with corpus enabled must assert p99 under 100ms.

Additional pitfalls addressed in PITFALLS.md:
- **auto_purge in push envelope** (Phase 22/25) -- ActionHint typed constant; BuildPushEnvelope returns error for purge-class intents; fuzz gate in Phase 25
- **Reversible fingerprints** (Phase 23) -- HMAC-SHA256 with per-install salt, not bare SHA-256; two-key non-reversibility unit test required
- **Residual gaps treated as solved** (Phase 25) -- SENTRY-008 (CI runner OIDC) and GitHub API dead-drop must be named explicitly in THREAT-MODEL.md update

---

## Implications for Roadmap

The PRD section 4 phase plan maps to GSD Phases 22-25. The dependency ordering is strict and non-retrofittable.

### Phase 22: Schema and Envelope Lock (PRD Phase 0)

**Rationale:** Non-retrofittable. Every field in the four-layer schema must be defined and emitted (even as placeholders) before any corpus record is written to disk. A schema change after the first corpus write requires a migration -- there is no migration tooling in v1 scope.

**Delivers:** internal/corpus/types.go with all four layers, typed ActionHint constant, ScopeOrgOnly zero-value enforcement, PushEnvelope struct with signing stubs; AuditRecord extension (cluster_id, scope, fleet_node_id, source_surface, baseline_deviation, repo_fingerprint); CorpusConfig struct; exported policy.CorroborateOutcome() 2-line wrapper.

**Addresses (SCHEMA, SCOPE-TAGGING, PUSH-ENVELOPE table stakes):** Four-layer schema freeze; push-envelope wire format freeze; action_hint: watch_and_block typed constant; scope: org_only zero-value enforcement; PromoteScope() permanently gated to error in v1; signing fields pre-defined (empty in v1).

**Avoids:** Pitfalls 1 (non-retrofittable schema), 4 (scope-tag leakage), 5 (auto_purge in envelope), 6 (reversible fingerprints -- derivation algorithm specified in schema lock before Phase 23 implements it).

**Self-defense:** ActionHint typed constant baked into the type; no code path can emit auto_purge.

**Evaluator gate (PRD section 4 Phase 0):** Every field in the Nx Console worked trace maps to a schema field with no gaps; PushEnvelope can represent a watch_and_block push with confidence_tier and source_count; CorpusRecord{} serializes as "scope": "org_only"; format sign-off freezes the schema for all subsequent phases.

**Research flag:** NONE -- all decisions are code-grounded and locked by the four researchers.

---

### Phase 23: Corpus Store + Adjudication Engine (PRD Phase 1)

**Rationale:** Builds on the frozen Phase 22 schema. The corpus store, emitter adapter, and adjudication engine are the core I/O layer. All three must ship together because the evaluator gate requires a full synthetic incident trace through all four layers with adjudication outcome assigned.

**Delivers:** corpus.StoreSink (audit.Sink implementation, 0600 NDJSON, RedactRecord on every write); corpus/emitter.go (AuditRecord to CorpusRecord mapping, cluster_id computation per source_surface, behavior_signature_hash derivation); corpus/adjudicator.go (async channel-fed goroutine, 1024-record buffer, 6 adjudication_source rules, policy.CorroborateOutcome() for confidence_tier, WaitGroup+200ms deadline for short-lived check process); HMAC-SHA256 fingerprints with per-install salt; Ed25519 keygen/sign; audit.MultiSink wired to add StoreSink when corpus enabled; fuzz gate (fuzz_test.go for NDJSON parser and emitter).

**Addresses (CORPUS-STORE, ADJUDICATION):** Append-only local store; async off-hot-path adjudication; true_label transitions; source_count deduplication by source identifier; confidence_tier mapping (watch=1 source, enforce=2+); RedactRecord routing on every write; HMAC fingerprints from first write; 0600 owner-only permissions; no network path.

**Avoids:** Pitfalls 2 (RedactRecord bypass), 3 (source_count double-counting), 6 (reversible fingerprints), 7 (adjudication on hot path).

**Self-defense:** Corpus store owner-only (0600); append-only (O_APPEND|O_CREATE); scope=org_only default enforced by constructor; no network path exists; fuzz gate for NDJSON parser and emitter is a release gate.

**Evaluator gate (PRD section 4 Phase 1):** Synthetic Nx Console incident records all four layers; 2-source adjudication emits confidence_tier: "enforce", 1-source emits "watch"; records in corpus NDJSON carry frozen push-envelope shape; AWS-key-shaped target_resource is redacted in persisted NDJSON; 3x Bumblebee events yield source_count: 1, confidence_tier: "watch".

**Research flag:** MINOR -- adjudicator lifecycle (background goroutine in daemon vs batch-on-next-check invocation) is Open Question (c); must be resolved before Phase 23 planning begins.

---

### Phase 24: First Responder Corpus Binding (PRD Phase 2)

**Rationale:** Connects the adjudication engine's confirmed-malicious output to the already-shipped First Responder quarantine infrastructure. All binding is additive (new TUI constructor, new record_type filter in loadItems()); existing quarantine/restore/purge code is unchanged.

**Delivers:** CorpusAdjudicationIncidentFromRecord() in internal/tui/incidents.go (red for attacker action, coral for Beekeeper response per locked TUI semantic); corpus_quarantine_armed record_type filter in internal/tui/quarantine_panel.go loadItems(); adjudicator emits corpus_quarantine_armed AuditRecord to audit.Writer on confirmed-malicious; adjudicator calls sentry.AddTarget() + sentry.SaveTargets() on confirmed-malicious; no changes to quarantine.MoveTyped, targets.go, or rules.go.

**Addresses (FIRST-RESPONDER-BINDING):** Confirmed-malicious adjudication arms TUI quarantine card; purge stays human-gated (never automatic -- only via human [P] keypress); restore intact; red/coral TUI semantic preserved; Sentry TargetList tightened on confirmed-malicious process tree.

**Avoids:** Auto-purge (adjudicator arms the card, never calls MoveTyped autonomously); TUI poll reuses existing 2s audit tail cycle (no new IPC channels or shared memory).

**Evaluator gate (PRD section 4 Phase 2):** Confirmed local Nx Console match arms TUI quarantine card and does not auto-purge; restore reverses a purge cleanly; corpus_quarantine_armed record appears in beekeeper.ndjson when adjudication resolves as malicious with confidence_tier: "enforce".

**Research flag:** NONE -- all seams are code-verified against live source files.

---

### Phase 25: Launch Readiness E2E (PRD Phase 3)

**Rationale:** End-to-end validation of the full corpus loop. All 8 Sentry patterns must produce moat-grade records. Documentation must name residual gaps (SENTRY-008, GitHub API dead-drop) explicitly and without downplaying. BuildPushEnvelope fuzz gate closes the auto_purge blast-radius risk for the transport path.

**Delivers:** internal/corpus/e2e_test.go (build tag //go:build e2e) -- synthetic Nx Console incident trace through all 8 SENTRY rules to corpus emit to adjudication to TargetList update to TUI arm; THREAT-MODEL.md update adding named corpus residual gaps section (SENTRY-008 CI runner OIDC, GitHub API dead-drop, DNS-TXT tunneling); web docs update (local-first, offline-protective, named gaps); BuildPushEnvelope fuzz target asserting action_hint never outside allowed set.

**Addresses (PUSH-ENVELOPE fuzz, launch documentation):** 8 Sentry patterns produce moat-grade CorpusRecords; no corpus data leaves the machine; offline run fully protective on last synced catalog; SENTRY-008 and GitHub API dead-drop named explicitly in threat model.

**Avoids:** Pitfall 8 (residual gaps treated as solved); auto_purge fuzz gate closes Pitfall 5 for the future transport path.

**Evaluator gate (PRD section 4 Phase 3):** 8 Sentry patterns each produce moat-grade CorpusRecord; go test -tags e2e ./internal/corpus/... exits 0; BenchmarkRunCheck with corpus enabled p99 under 100ms; THREAT-MODEL.md contains strings "SENTRY-008" and "dead-drop" in corpus gaps section; web docs accuracy gate (accuracy_spec.py) passes.

**Research flag:** NONE for E2E test structure. Documentation review is a human checkpoint and cannot be automated.

---

### Phase Ordering Rationale

- **Phase 22 before Phase 23:** schema is non-retrofittable; the emitter and store must use a frozen schema or early corpus records may have gaps that can never be filled retroactively.
- **Phase 23 before Phase 24:** the adjudication engine must be proven to produce correct true_label/confidence_tier values before its output is wired to the TUI and Sentry TargetList. An incorrect adjudication that auto-arms quarantine cards is worse than no integration.
- **Phase 24 before Phase 25:** the First Responder binding must be integrated and smoke-tested before the E2E harness validates the full loop across all 8 Sentry patterns.
- **Additive AuditRecord extension is safe:** schema fields added in Phase 22 use omitempty -- no existing consumer (hook handler, gateway, TUI, audit CLI) is broken.
- **Corpus store as audit.Sink means zero per-surface changes in Phases 23-25:** the MultiSink fan-out pattern is proven across syslog/OTLP/HTTPS sinks already; adding StoreSink is config-level wiring.

### Research Flags

All phases have well-documented patterns and code-grounded architecture. No phase requires /gsd-plan-phase --research-phase:
- **Phase 22** -- schema decisions code-grounded and locked; Go struct/omitempty conventions are standard; no research needed during planning
- **Phase 23** -- audit.Sink pattern, O_APPEND NDJSON, HMAC-SHA256 stdlib all well-documented; architecture verified against live source files; async goroutine pattern is idiomatic
- **Phase 24** -- TUI stateTick poll and CatalogQuarantineIncidentFromRecord template are proven in the codebase; AddTarget/SaveTargets seam is documented and clean
- **Phase 25** -- E2E test follows established //go:build e2e convention in the codebase; documentation review is a human gate (not researchable)

---

## Open Questions

Must be resolved during requirements and roadmap authoring. Items (b), (c), and (d) are blockers for Phase 23 planning. Item (a) is a Phase 22 internal decision. Item (e) can be validated during Phase 22 evaluator gate.

**(a) Freeze behavior_signature_hash definition in Phase 22.**
Current candidate: sha256(action_type + target_resource + ecosystem + package + version). Alternative: sha256(action_type + network_destination_pattern + ecosystem + package + version) to capture exfil pattern more precisely. This is a one-way decision -- changing the hash input post-freeze invalidates all prior corpus records for behavior-matching purposes and requires a schema version bump. Must be decided in Phase 22 before the emitter is written.

**(b) downstream_clean observation window value.**
The downstream_clean adjudication_source requires a defined observation window: "an allow that produced no subsequent incident within N days." The PRD does not specify N. Candidate values: 7 days (aggressive, risks premature benign labeling), 14 days (moderate), 30 days (conservative, delays adjudication). Affects the adjudication engine timer logic in Phase 23. Must be decided before Phase 23 implementation begins.

**(c) Adjudicator lifecycle: background goroutine in catalogs daemon vs batch on next beekeeper check.**
Option A (daemon goroutine): adjudicator runs continuously in beekeeper catalogs daemon (or a new beekeeper corpus daemon), watching the corpus file via fsnotify, processing unresolved records as they arrive. Provides real-time adjudication; requires a daemon to be running.
Option B (batch on next invocation): adjudicator runs as a batch step at the start of each beekeeper check invocation (after the hook verdict is returned), processing unresolved records from the previous N minutes. No daemon required; adjudication is eventually consistent on the next hook invocation.
Decision significantly affects Phase 23 architecture and which daemon or command owns the adjudicator goroutine. Must be resolved before Phase 23 planning begins.

**(d) scan-surface cluster_id unit: synthetic cluster vs per-package-hit.**
Current candidate: cluster_id = sha256(Package + Version + scan_time_bucket_5min)[:16]. If a single scan run hits the same package version twice (once from a direct dep, once from a transitive dep), both hits share a cluster_id and are adjudicated as one incident. Whether this is correct behavior (one incident per package version per time window, regardless of how many dep paths hit it) or incorrect (two separate incidents from different dep paths) needs a decision. Affects Phase 23 emitter logic.

**(e) cluster_id bucket windows need empirical validation.**
The 60-second window for Sentry events (matching CredAccessWindowSec default) and 5-minute window for scan events are reasonable starting points. The 60-second window has not been validated against a real multi-event Sentry incident trace (the only empirical reference is the Nx Console incident timeline). A synthetic multi-event Sentry trace in Phase 22's evaluator gate should validate this window before Phase 23 ships. If the window is too narrow, correlated Sentry events split into separate clusters (reducing corroboration signal). If too wide, unrelated events merge (inflating corroboration signal).

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All decisions are stdlib-only or already in go.mod; confirmed against live go.mod and internal/audit/writer.go; Ed25519 and HMAC-SHA256 are stable Go 1.13+ APIs; zero new deps is a firm constraint |
| Features | HIGH | PRD is authoritative; prior-art survey (MISP, STIX, OSV, VirusTotal, MOTIF, ThreatFox) all HIGH confidence; warn/enforce boundary has strong multi-source prior-art basis (VirusTotal t=2, MISP admiralty scale) |
| Architecture | HIGH | All integration points verified against live source files (handler.go, sink.go, incidents.go, quarantine_panel.go, targets.go, crossref.go, corroboration.go); audit.Sink fan-out pattern proven across syslog/OTLP/HTTPS sinks |
| Pitfalls | HIGH | All 8 pitfalls grounded in prior milestone hard-won lessons (F-1 redaction finding 2026-06-12, TM-B-01 source_count semantics, v1.3.0 async audit tail, self-protection StateDir scope); recovery costs clearly characterized with explicit recovery steps |

**Overall confidence:** HIGH

### Gaps to Address

- **behavior_signature_hash definition** -- must be frozen in Phase 22 before emitter is written; see Open Question (a)
- **downstream_clean observation window** -- must be specified before Phase 23; affects adjudication engine timer; see Open Question (b)
- **Adjudicator lifecycle** -- daemon goroutine vs batch-on-invocation significantly affects Phase 23 architecture; must be resolved before Phase 23 planning begins; see Open Question (c)
- **scan-surface cluster_id unit** -- per-package-hit vs per-scan-pass deduplication; affects Phase 23 emitter; see Open Question (d)
- **60-second Sentry cluster window** -- needs validation against synthetic multi-event trace in Phase 22 evaluator gate before Phase 23 ships; see Open Question (e)

---

## Sources

### Primary (HIGH confidence)
- beekeeper-corpus-milestone-prd.md (repo root) -- authoritative v1.4.0 scope; sections 3.1-3.6 (schema, adjudication, store, First Responder, push envelope, scope tagging); section 4 (phase plan and evaluator gates); section 8 (residual gaps)
- internal/audit/types.go, internal/audit/writer.go, internal/audit/sink.go -- AuditRecord embedding target, O_APPEND pattern, MultiSink fan-out, Sink interface contract; confirmed live source
- internal/policy/corroboration.go, internal/policy/engine.go -- pure function boundary confirmed; corroborate() return values map directly to source_count/confidence_tier
- internal/sentry/targets.go, internal/sentry/rules.go -- AddTarget/SaveTargets seam confirmed; EvaluateEvent pure confirmed; cluster derivation approach validated
- internal/tui/incidents.go, internal/tui/quarantine_panel.go -- stateTick poll pattern, CatalogQuarantineIncidentFromRecord template, bounded 512KB audit tail confirmed
- internal/check/handler.go -- sub-100ms hot path confirmed; no room for synchronous adjudication
- internal/quarantine/quarantine.go -- MoveTyped/Restore/Purge never auto-triggered; human-gated via TUI [P] key confirmed
- go.mod -- zero existing sigstore/cosign Go library import confirmed; cosign is CI-only
- pkg.go.dev/crypto/ed25519 -- GenerateKey, Sign, Verify; stdlib since Go 1.13; constant-time operations
- pkg.go.dev/crypto/hmac -- HMAC keyed hashing, hmac.Equal timing-safe comparison; stdlib
- MISP best practices (misp-project.org/best-practices-in-threat-intelligence) -- confidence/vetting taxonomy, admiralty scale, TLP scope decoupling; HIGH
- MOTIF malware corpus (arxiv.org/abs/2111.15031) -- ground-truth labeling methodology; forensic_review as highest-confidence source; peer-reviewed
- ThreatFox FAQ (threatfox.abuse.ch/faq) -- 0-100% confidence, Trusted Reporter vetting pattern; HIGH

### Secondary (MEDIUM confidence)
- VirusTotal threshold research (SecureScan arxiv 2602.10750 and related) -- t=2 as standard enforce threshold, t=1 as watchlist signal; multiple papers agree
- STIX 2.1 spec (OASIS) -- confidence field 0-100, revoked boolean; partially truncated in fetch so MEDIUM
- Trail of Bits sigstore audit (January 2026) -- Ed25519 recommended for new sigstore deployments
- Hash chaining audit log literature (tracehold.ai, dev.to) -- confirms value only for multi-party audit trails with external verifier; blog sources

### Tertiary (LOW confidence)
- OSV schema (ossf.github.io/osv-schema) -- withdrawn field; absence of native confidence/provenance layer as anti-pattern; schema itself is HIGH confidence but the "anti-pattern" framing is interpretive inference

---

*Research completed: 2026-06-13*
*Ready for roadmap: yes*
