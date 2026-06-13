# Architecture Research — v1.4.0 Adjudicated Corpus (Local Loop)

**Domain:** Integration architecture for an off-hot-path adjudication engine + append-only corpus store into the existing Beekeeper safety harness
**Researched:** 2026-06-13
**Confidence:** HIGH (grounded in live source code reads of internal/audit, internal/sentry, internal/quarantine, internal/watch/crossref.go, internal/policy, internal/tui, internal/check/handler.go, and the full PRD)

---

## System Overview

```
 HOT PATH (synchronous, fail-closed, sub-100ms)
 +---------------------------------------------------------------------------+
 |  source_surfaces: hook | mcp_gateway | shim | file_watcher | scan        |
 |                                                                           |
 |   policy.Evaluate()      --> internal/policy (pure, no I/O)              |
 |   sentry.EvaluateEvent() --> internal/sentry (pure, no I/O)              |
 |                       |                                                   |
 |              audit.Sink.Write(AuditRecord)  -------------------------+   |
 |              [record_type: policy_decision | sentry_alert | ...]     |   |
 +---------------------------------------------------------------------------+
                                                                        |
 OFF-HOT-PATH CORPUS LOOP (async, never blocks hook path)              |
 +--------------------------------------------------------------------+--+
 |                                                                    v  |
 |   internal/corpus/adjudicator.go                                      |
 |      <- watches corpus store for unresolved records                   |
 |      <- calls policy.CorroborateOutcome() (reuses pure lib)           |
 |      -> assigns outcome layer (true_label, adjudication_source,       |
 |           was_correct, source_count, confidence_tier)                 |
 |      -> builds push_envelope fields                                   |
 |      -> writes CorpusRecord update to corpus store (append-only NDJSON)|
 |                                                                        |
 |   internal/corpus/store.go  (NEW)                                     |
 |      <- extends audit.Sink interface                                  |
 |      -> StateDir/corpus/beekeeper-corpus.ndjson  (0600, owner-only)  |
 |      -> emits records in push_envelope shape with scope tag           |
 |                                                                        |
 +------------------------------------------------------------------------+
                     |
                     | confirmed-malicious adjudication
                     v
 FIRST RESPONDER BINDING (Phase 24)
 +------------------------------------------------------------------------+
 |   existing seam: sentry.TargetList (internal/sentry/targets.go)       |
 |      <- adjudicator calls TargetList.AddTarget() when                 |
 |           true_label == "malicious"                                    |
 |      <- sentry.SaveTargets() persists updated TargetList              |
 |                                                                        |
 |   existing seam: internal/tui (incidents.go / quarantine_panel.go)   |
 |      <- adjudicator emits AuditRecord{record_type:"corpus_quarantine_armed"}|
 |         so the TUI stateTick picks it up on its normal 2s poll cycle  |
 |      -> QuarantinePanel.loadItems() surfaces the armed card           |
 |      -> [P]urge / [R]estore gated to human keypress (no auto-purge)  |
 +------------------------------------------------------------------------+
```

---

## Component Boundaries

### New components (v1.4.0)

| Component | Package | File(s) | Responsibility |
|-----------|---------|---------|----------------|
| Corpus record type | `internal/corpus` | `types.go` | Four-layer schema: behavior / decision / outcome / context. Includes push_envelope sub-struct. Immutable after construction. |
| Corpus store | `internal/corpus` | `store.go` | Append-only NDJSON writer at StateDir/corpus/beekeeper-corpus.ndjson. Extends `audit.Sink` so the existing `MultiSink` fan-out can route to it. Owner-only (0600). |
| Adjudication engine | `internal/corpus` | `adjudicator.go` | Async worker that reads unresolved CorpusRecords, applies adjudication logic (6 adjudication_source values), calls `policy.CorroborateOutcome()` for confidence_tier, writes back the outcome layer. Never touched by the hook handler. |
| Corpus emitter adapter | `internal/corpus` | `emitter.go` | Thin impure adapter: consumes an `audit.AuditRecord` (from any source_surface), maps it to a `CorpusRecord` with `outcome.true_label="unresolved"`, and hands it to the corpus store. This is the impure boundary that keeps `internal/policy` and `internal/sentry` pure. |

### Modified components (v1.4.0)

| Component | Package | File | Change |
|-----------|---------|------|--------|
| AuditRecord | `internal/audit` | `types.go` | Add `cluster_id`, `baseline_deviation`, `repo_fingerprint`, `fleet_node_id`, `scope` fields. Add `record_type` value `"corpus_quarantine_armed"`. No removals — existing consumers unaffected (omitempty). |
| audit MultiSink | `internal/audit` | `sink.go` | `NewMultiSink` adds `corpus.StoreSink` to the sink graph when a corpus config block is present. The existing Sink interface and fan-out logic are unchanged. |
| Hook handler | `internal/check` | `handler.go` | No behavioral change. After `audit.Sink.Write()` returns (unchanged), the hook exits. The corpus store write is synchronous within the existing `Sink.Write()` call. The handler is unaware of adjudication. |
| Config | `internal/config` | `config.go` | Add `CorpusConfig` struct: `enabled bool`, `path string` (override for corpus file, defaults to StateDir/corpus/beekeeper-corpus.ndjson), `scope string` (org_only | community_shareable, default org_only). |
| policy.corroboration | `internal/policy` | `corroboration.go` | Export `CorroborateOutcome(matches []CatalogMatch, t CorroborationThresholds) (sourceCount int, confidenceTier string)` wrapper (2-line addition). No behavioral change to existing unexported `corroborate()`. |
| Sentry TargetList | `internal/sentry` | `targets.go` | No code change. The adjudicator calls the existing `AddTarget()` + `SaveTargets()` after a confirmed-malicious adjudication. The seam is already clean. |
| TUI incidents | `internal/tui` | `incidents.go` | Add `CorpusAdjudicationIncidentFromRecord()` constructor that renders a confirmed-adjudication card using the new `record_type:"corpus_quarantine_armed"` record. Red for attacker action, coral for Beekeeper response (existing color contract). |
| TUI quarantine panel | `internal/tui` | `quarantine_panel.go` | `loadItems()` adds a filter for `corpus_quarantine_armed` record type from the audit tail. A confirmed-malicious adjudication surfaces as a pending card. No auto-purge. |

---

## Integration Points: Named at Package/File Level

### 1. Where the four-layer record is emitted (all six source_surfaces)

The record is emitted at the `audit.Sink.Write()` call site in each surface handler. The corpus store is added as an additional sink in the `MultiSink` graph. The hook handler (`internal/check/handler.go`) calls `auditSink.Write(auditRec)` already; adding the corpus store to the `MultiSink` means the four-layer record (with behavior + decision populated, outcome = unresolved, context = cluster_id/scope) is written atomically with the existing audit log write -- no hot-path latency added beyond an NDJSON append to a second file.

Concrete call chain:

```
internal/check/handler.go  -->  audit.MultiSink.Write(AuditRecord)
                                    |
                                    +-- audit.WriterSink.Write()   [existing beekeeper.ndjson]
                                    +-- corpus.StoreSink.Write()   [NEW beekeeper-corpus.ndjson]
                                          |
                                          +-- maps AuditRecord --> CorpusRecord{
                                                behavior: {source_surface, action_type,
                                                           actor_lineage, target_resource,
                                                           agent_id},
                                                decision: {verdict, policy_matched, rule_id,
                                                           correlation_window, confidence,
                                                           ruleset_version},
                                                outcome:  {true_label: "unresolved"},
                                                context:  {cluster_id, scope: "org_only",
                                                           repo_fingerprint, fleet_node_id}
                                              }
```

The same pattern applies to `internal/gateway`, `internal/shim`, `internal/watch/watcher.go`, `internal/sentry` (daemon), and `internal/scan` -- each already writes to an `audit.Sink`; adding the corpus store to the graph is config-level wiring, not per-surface code change.

Fail-closed invariant: if `corpus.StoreSink.Write()` returns an error, `MultiSink.Write()` logs the error and continues -- corpus store failure must NOT block the local audit write (same contract as the existing syslog/OTLP/HTTPS sinks).

### 2. How cluster_id binds correlated non-agent incidents

The Sentry correlation engine already groups related events by assigning them to the same `RuleState` windows (CredAccessByPID, PhoneHomeByPID, etc.). The `SentryAlert.RuleID` + `SentryAlert.ProcessPID` + a wall-clock window form a natural cluster key. `cluster_id` is derived at corpus record construction time in `corpus/emitter.go`:

- For `source_surface: sentry` records: `cluster_id = sha256(RuleID + strconv(ProcessPID) + wall_time_bucket_60s)[:16]`. The 60-second bucket matches the existing `CredAccessWindowSec` default in `sentry.RuleConfig`. Events sharing the same PID, rule, and 60s bucket share a `cluster_id`. This reuses the existing window concept without touching `EvaluateEvent`.
- For `source_surface: hook | mcp_gateway | shim` records (agent-mediated): `cluster_id = AgentID` if present (already in AuditRecord), else a per-invocation UUID. Agent tool calls are already correlated by `AgentID`.
- For `source_surface: file_watcher | scan` records: `cluster_id = sha256(Package + Version + scan_time_bucket_5min)[:16]` using the scan pass timestamp bucketed to 5-minute windows.

No changes to `internal/sentry/rules.go` or `EvaluateEvent` -- cluster_id is computed in the emitter adapter from existing `SentryAlert` and `AuditRecord` fields.

### 3. How a confirmed-malicious adjudication arms the TUI quarantine card without auto-purge

The adjudicator writes two things when `true_label` resolves to `"malicious"`:

**First:** An `AuditRecord` with `record_type:"corpus_quarantine_armed"` to the existing `audit.Writer` (beekeeper.ndjson). The TUI's `stateTick` already polls the audit log on a 2-second cycle via `recentAuditRecords()` (the bounded 512KB tail reader established in v1.3.0). The TUI surfaces an `IncidentModel` card using a new `CorpusAdjudicationIncidentFromRecord()` constructor in `internal/tui/incidents.go`. This is the same mechanism as the existing `CatalogQuarantineIncidentFromRecord()` -- no new TUI polling infrastructure.

**Second:** A call to `sentry.AddTarget()` + `sentry.SaveTargets()` to tighten the Sentry correlation thresholds for the matching process tree. The `internal/sentry/targets.go` `AddTarget`/`SaveTargets` seam is already designed for exactly this write-outside-the-hot-path pattern (per the doc comment: "Called by the first-responder after each scan hit update").

What does NOT happen: the adjudicator never calls `quarantine.MoveTyped()` autonomously. Move is only triggered by explicit [P] keypress in `internal/tui/quarantine_panel.go`. The corpus adjudication arms a card; it does not execute the move. This preserves the "purge stays human/org-policy gated" invariant unconditionally.

### 4. Push-envelope shape emitted now so transport is wiring later

The `CorpusRecord` type in `internal/corpus/types.go` contains a nested `PushEnvelope` struct populated at record construction time. The `Signing` sub-struct fields are always zero-value in v1. When v1.1 transport arrives, it populates `Signing` on outbound pushes without any schema migration.

The `ActionHint` allowlist is enforced at construction time: `emitter.go` asserts `action_hint == "watch_and_block"` or leaves it empty; `"auto_purge"` is rejected as an invalid value by a compile-time constant. This is not a runtime check -- a future contributor cannot accidentally introduce it.

`BehaviorSignatureHash` is `sha256(action_type + target_resource + ecosystem + package + version)` -- a stable fingerprint of the attacker behavior, not the victim context. Computed in `corpus/emitter.go` at record construction time.

### 5. Package boundary: new impure package consuming existing pure corroboration logic

`internal/corpus` is a NEW package. It is impure (reads/writes files, calls time.Now()) but consumes `internal/policy` as a read-only dependency for the corroboration logic:

```
internal/corpus  -->  internal/policy   (calls CorroborateOutcome(), pure)
internal/corpus  -->  internal/audit    (reads AuditRecord, uses Sink interface)
internal/corpus  -->  internal/sentry   (calls AddTarget, SaveTargets -- I/O helpers only)
internal/corpus  -->  internal/config   (reads CorpusConfig)
internal/corpus  -->  internal/platform (reads StateDir)
```

`internal/policy` gains NO new dependency. `internal/audit` gains NO new dependency. `internal/sentry/rules.go` and `EvaluateEvent` are untouched. The pure-lib boundary is unbroken.

The PRD's `source_count` and `confidence_tier` map directly to `corroborate()`'s return values: `count` -> `source_count`; `level == "warn"` -> `confidence_tier:"watch"`; `level == "block"` -> `confidence_tier:"enforce"`. The new `policy.CorroborateOutcome()` export is a 2-line wrapper that calls the existing unexported function and returns only the two fields `internal/corpus` needs.

---

## Data Flow

### Event-to-corpus-record (v1 local loop)

```
Source surface produces event
         |
         v
[surface handler] calls audit.Sink.Write(AuditRecord)
         |
         v  (synchronous, within existing Sink.Write call)
corpus.StoreSink.Write(AuditRecord)
         |
         +-- maps to CorpusRecord {behavior, decision, outcome={unresolved}, context}
         +-- computes cluster_id (surface-specific derivation, see section 2 above)
         +-- populates PushEnvelope fields (behavior_signature_hash, scope=org_only)
         +-- appends NDJSON line to StateDir/corpus/beekeeper-corpus.ndjson (0600)
         |
         v  (hook exits; corpus file now has an unresolved record)
corpus.Adjudicator (background goroutine OR next beekeeper check invocation)
         |
         +-- reads new unresolved records from corpus store
         +-- for each: consult catalog state and secondary signals
         |     - catalog_confirmation: re-query catalog index for same package/version
         |     - downstream_clean:    N minutes elapsed with no follow-on alert
         |     - user_override:       AuditRecord with decision=allow found for same pkg
         +-- calls policy.CorroborateOutcome(matches, thresholds)
         |     -> source_count, confidence_tier
         +-- assigns true_label, adjudication_source, resolved_at, was_correct
         +-- updates CorpusRecord outcome layer (append-only: new NDJSON record with
         |   same cluster_id + outcome set; consumers take the latest by cluster_id)
         |
         v  (if true_label == "malicious")
         +-- write AuditRecord{record_type:"corpus_quarantine_armed"} to audit.Writer
         |     (TUI stateTick picks this up on its 2s poll, arms the quarantine card)
         +-- sentry.AddTarget() + sentry.SaveTargets()
               (Sentry daemon reloads TargetList on next EvaluateEvent call)
```

### Corpus store file layout

```
StateDir/
  audit/
    beekeeper.ndjson         [existing -- policy_decision, sentry_alert, nudge, ...]
  corpus/
    beekeeper-corpus.ndjson  [NEW -- CorpusRecord NDJSON, append-only, 0600]
  sentry/
    sentry-targets.json      [existing -- updated by adjudicator on confirmed-malicious]
  quarantine/
    extensions/              [existing]
    packages/                [existing]
```

---

## Recommended Package Structure

```
internal/
  corpus/
    types.go            # CorpusRecord, PushEnvelope, four-layer schema types
    store.go            # StoreSink: implements audit.Sink, writes corpus NDJSON
    emitter.go          # Maps AuditRecord -> CorpusRecord; computes cluster_id,
                        #   behavior_signature_hash, scope tag
    adjudicator.go      # Adjudication engine: reads unresolved records, assigns
                        #   outcome layer, calls policy.CorroborateOutcome,
                        #   triggers TargetList + TUI arm on confirmed-malicious
    adjudicator_test.go
    store_test.go
    types_test.go
    fuzz_test.go        # Fuzz the emitter and adjudicator record parser
    e2e_test.go         # //go:build e2e -- full corpus loop E2E (Phase 25)
```

Additional file-level changes:

- `internal/policy/corroboration.go` -- add exported `CorroborateOutcome()` wrapper (Phase 22, 2-line addition)
- `internal/audit/types.go` -- add cluster_id, scope, baseline_deviation, repo_fingerprint, fleet_node_id fields (Phase 22)
- `internal/config/config.go` -- add `CorpusConfig` struct (Phase 22)
- `internal/audit/sink.go` -- `NewMultiSink` adds StoreSink when corpus enabled (Phase 23)
- `internal/tui/incidents.go` -- add `CorpusAdjudicationIncidentFromRecord()` (Phase 24)
- `internal/tui/quarantine_panel.go` -- `loadItems()` filter for corpus_quarantine_armed (Phase 24)

---

## Build Order Aligned to PRD Phases

### Phase 22 -- Schema and envelope lock (PRD "Phase 0")

**Goal:** Freeze the four-layer schema and push-envelope wire format. Every subsequent phase emits records in this exact shape.

**New files:**
- `internal/corpus/types.go` -- CorpusRecord, PushEnvelope (with ActionHint allowlist enforced by const), cluster_id derivation helpers

**Modified files:**
- `internal/audit/types.go` -- add cluster_id, scope, baseline_deviation, repo_fingerprint, fleet_node_id as omitempty fields (non-breaking)
- `internal/policy/corroboration.go` -- export `CorroborateOutcome()` 2-line wrapper
- `internal/config/config.go` -- add `CorpusConfig{Enabled bool, Path string, Scope string}`

**Self-defense delivered in this phase:** ActionHint allowlist (watch_and_block only, auto_purge absent) baked into the type at Phase 22, enforced by compile-time constant.

**Evaluator gate:** every field in the Nx Console worked trace maps to a schema field; PushEnvelope can represent a watch_and_block push with confidence_tier and source_count; format frozen.

---

### Phase 23 -- Corpus store + adjudication engine (PRD "Phase 1")

**Goal:** Append-only local corpus store live; adjudication engine assigns true_label and confidence_tier; records emit in push-envelope shape.

**New files:**
- `internal/corpus/store.go` -- StoreSink implementing audit.Sink; StateDir/corpus/beekeeper-corpus.ndjson at 0600; `NewCorpusStoreSink(cfg CorpusConfig)` constructor
- `internal/corpus/emitter.go` -- maps AuditRecord to CorpusRecord; computes cluster_id per source_surface; populates PushEnvelope
- `internal/corpus/adjudicator.go` -- background worker; reads unresolved records; calls `policy.CorroborateOutcome()`; writes outcome update; checks 6 adjudication_source rules; does NOT touch quarantine or TargetList yet (that is Phase 24)
- `internal/corpus/fuzz_test.go` -- fuzz the NDJSON parser and emitter (release gate)

**Modified files:**
- `internal/audit/sink.go` -- `NewMultiSink` adds `corpus.StoreSink` to the sink graph when `cfg.Corpus.Enabled == true`

**Self-defense delivered in this phase:** corpus store owner-only (0600); append-only (O_APPEND|O_CREATE); scope=org_only default; no network path exists.

**Evaluator gate:** a synthetic Nx Console incident records all four layers; a 2-source adjudication emits `confidence_tier:"enforce"`, a 1-source emits `"watch"`; records in corpus NDJSON carry a frozen push-envelope shape.

---

### Phase 24 -- First Responder corpus binding (PRD "Phase 2")

**Goal:** Confirmed-malicious adjudication arms TUI quarantine card; purge stays human-gated; restore intact.

**Modified files:**
- `internal/corpus/adjudicator.go` -- after `true_label = "malicious"`: write `corpus_quarantine_armed` AuditRecord to `audit.Writer`; call `sentry.AddTarget()` + `sentry.SaveTargets(targetsPath, tl)`
- `internal/tui/incidents.go` -- add `CorpusAdjudicationIncidentFromRecord(rec audit.AuditRecord) IncidentModel`; red for attacker action, coral for Beekeeper response per locked TUI semantic
- `internal/tui/quarantine_panel.go` -- `loadItems()` recognizes `corpus_quarantine_armed` records in the TUI poll cycle; surfaces them as pending-quarantine cards (same pattern as existing `CatalogQuarantineIncidentFromRecord(..., pending=true)` in `incidents.go`)

**No changes to:**
- `internal/quarantine/quarantine.go` -- MoveTyped/Restore/Purge are unchanged; purge remains a human [P] keypress only
- `internal/sentry/rules.go` / `EvaluateEvent` -- TargetList tightening already wired; no rule changes
- `internal/sentry/targets.go` -- AddTarget/SaveTargets are called by the adjudicator with no file changes

**Evaluator gate:** a confirmed local Nx Console match arms the TUI card and does not auto-purge; restore reverses a purge cleanly.

---

### Phase 25 -- Launch readiness E2E (PRD "Phase 3")

**Goal:** End-to-end run of the full corpus loop; all 8 Sentry patterns produce moat-grade records; no corpus data leaves the machine; offline run is fully protective; documentation honest.

**New files:**
- `internal/corpus/e2e_test.go` (build tag `//go:build e2e`) -- synthetic Nx Console incident trace through all 8 SENTRY rules -> corpus emit -> adjudication -> TargetList update -> TUI arm

**Modified files:**
- `docs/THREAT-MODEL.md` -- add corpus residual gaps: SENTRY-008 CI runner OIDC, GitHub API dead-drop, and catalog lag as documented non-goals (from PRD §8)
- Web docs (content/docs/) -- local-first, offline-protective, named residual gaps (accuracy gate passes only when these are documented)

**Evaluator gate:** 8 Sentry patterns each produce a moat-grade CorpusRecord; beekeeper-corpus.ndjson contains zero records with data leaving the machine; `go test -tags e2e ./internal/corpus/...` exits 0.

---

## Architectural Patterns

### Pattern 1: Impure adapter wrapping pure core

**What:** `internal/corpus/emitter.go` is the impure adapter; `internal/policy/corroboration.go` and `internal/sentry/rules.go` remain pure. The adjudicator calls the exported `policy.CorroborateOutcome()` for confidence-tier derivation. The adapter handles all file, time, and random operations.

**When to use:** Any time a new capability needs corroboration or policy logic. Do not add I/O to `internal/policy` or `internal/sentry`.

**Why:** Matches existing architecture constraint in CLAUDE.md: "internal/policy must be a pure function library -- No I/O, no goroutines, no side effects." The same constraint extends to `internal/sentry/rules.go` (EvaluateEvent is documented as pure). Adding I/O would break the three-consumer model (hook handler, gateway middleware, Sentry correlation).

### Pattern 2: Sink extension for parallel record writing

**What:** `corpus.StoreSink` implements the existing `audit.Sink` interface. It is added to `audit.MultiSink` at startup via config. All six source_surfaces gain corpus writing with zero per-surface code changes.

**When to use:** Any new output target for audit data (future: push transport sink for v1.1). Add a new `Sink` implementation; add it to `NewMultiSink`; no surface changes.

**Why:** `MultiSink` already fans out to syslog/OTLP/HTTPS sinks with this pattern (`internal/audit/sink.go:NewMultiSink`). The corpus store is architecturally identical to the HTTPS sink -- a local file rather than a remote endpoint.

### Pattern 3: TUI card arming via audit record poll

**What:** The adjudicator writes a `corpus_quarantine_armed` AuditRecord to the standard audit log. The TUI's `stateTick` (already polling the audit log every 2s via the bounded 512KB tail reader) picks it up and renders the quarantine card. No IPC, no new goroutine, no shared memory.

**When to use:** Any time a background job needs to surface state to the TUI. Write an AuditRecord with a distinguished `record_type`; the TUI poll handles it.

**Why:** Avoids adding IPC channels or shared state between the adjudicator goroutine and the Bubble Tea model. The audit log is already the single source of truth for TUI state. The existing `CatalogQuarantineIncidentFromRecord` in `internal/tui/incidents.go` is the proven template.

---

## Anti-Patterns

### Anti-Pattern 1: Blocking the hook path on adjudication

**What people do:** Call the adjudication engine synchronously from `internal/check/handler.go` to get an immediate `true_label` before returning the decision.

**Why it's wrong:** Adjudication is inherently latent (it waits for secondary signals like catalog_confirmation, downstream_clean, user_override). Blocking on it would violate the sub-100ms target and the fail-closed contract -- a slow adjudication would cause a timeout that blocks the tool call.

**Do this instead:** The hook handler writes the CorpusRecord with `true_label:"unresolved"` synchronously (fast NDJSON append). The adjudicator runs off-hot-path (background goroutine or next-invocation batch scan).

### Anti-Pattern 2: Adding I/O to internal/policy or internal/sentry

**What people do:** Add a `ReadCorpusRecord()` call inside `policy.Evaluate()` or `sentry.EvaluateEvent()` to incorporate past adjudications into the current decision.

**Why it's wrong:** Both functions are explicitly pure. This is an architecture constraint in CLAUDE.md. Adding I/O makes them untestable as unit functions and breaks the three-consumer model.

**Do this instead:** Feed pre-loaded adjudication data as input to evaluation (e.g., through a catalog TargetList or policy overlay). Keep the loading in the impure adapter layer.

### Anti-Pattern 3: auto_purge in PushEnvelope

**What people do:** Add an `auto_purge` action_hint for high-confidence adjudications to allow future fleet-wide automated purge.

**Why it's wrong:** The PRD explicitly forbids auto_purge in any pushable envelope across all versions. A compromised push channel carrying auto_purge would have unbounded blast radius.

**Do this instead:** Keep destructive action local and human-gated. A confirmed-malicious adjudication arms a TUI card; purge requires [P] keypress. The allowlist (watch_and_block only) is enforced at record construction time in `corpus/emitter.go` as a compile-time constant.

### Anti-Pattern 4: Premature scope promotion

**What people do:** Set `scope:"community_shareable"` on records at adjudication time for high-confidence malicious findings to pre-stage them for v2.0 push.

**Why it's wrong:** Community-shareable records require an anonymization step (stripping victim context) that is a v2.0 deliverable. A premature scope promotion embeds un-anonymized victim data in records labeled community_shareable.

**Do this instead:** All v1 records default to `scope:"org_only"`. Promotion is explicit, operator-initiated, and gated by the anonymization step in v2.0. Never automatic.

---

## Integration Points Summary

| Integration point | Existing component | New/modified | Phase |
|-------------------|--------------------|--------------|-------|
| Four-layer schema types | `internal/audit/types.go` (AuditRecord) | MODIFIED: add cluster_id, scope, fleet_node_id | 22 |
| CorroborateOutcome export | `internal/policy/corroboration.go` | MODIFIED: 2-line export wrapper | 22 |
| CorpusConfig in layered config | `internal/config/config.go` | MODIFIED: add CorpusConfig block | 22 |
| Corpus record types | none | NEW: `internal/corpus/types.go` | 22 |
| Corpus store as audit.Sink | `internal/audit/sink.go` (MultiSink) | MODIFIED: add StoreSink to sink graph | 23 |
| Corpus store file | StateDir/corpus/beekeeper-corpus.ndjson | NEW: owner-only NDJSON sink | 23 |
| Emitter adapter | none | NEW: `internal/corpus/emitter.go` | 23 |
| Adjudicator async worker | none | NEW: `internal/corpus/adjudicator.go` | 23 |
| Corpus fuzz gate | none | NEW: `internal/corpus/fuzz_test.go` | 23 |
| TargetList arming | `internal/sentry/targets.go` AddTarget/SaveTargets | CALLED BY adjudicator (no file change to targets.go) | 24 |
| TUI quarantine card | `internal/tui/incidents.go` | MODIFIED: add CorpusAdjudicationIncidentFromRecord | 24 |
| TUI poll for corpus_quarantine_armed | `internal/tui/quarantine_panel.go` | MODIFIED: loadItems recognizes new record_type | 24 |
| E2E corpus loop test | none | NEW: `internal/corpus/e2e_test.go` (-tags e2e) | 25 |
| THREAT-MODEL.md residual gaps | `docs/THREAT-MODEL.md` | MODIFIED: add corpus gaps section | 25 |

---

## Sources

- Live code: `internal/audit/types.go`, `internal/audit/sink.go`, `internal/audit/writer.go` -- confirmed AuditRecord structure, MultiSink fan-out, Sink interface contract
- Live code: `internal/sentry/rules.go`, `internal/sentry/types.go` -- confirmed EvaluateEvent pure function signature, RuleState windows, SentryAlert shape, cluster derivation approach
- Live code: `internal/sentry/targets.go` -- confirmed AddTarget/SaveTargets seam; LoadTargets called at daemon startup only (I/O outside hot path)
- Live code: `internal/quarantine/quarantine.go` -- confirmed MoveTyped/Restore/Purge never auto-triggered; human-gated via TUI [P] key
- Live code: `internal/watch/crossref.go` -- confirmed CrossReference is read-only, uses audit.Writer for findings, same append-only pattern the corpus store will follow
- Live code: `internal/policy/corroboration.go`, `internal/policy/engine.go` -- confirmed pure, no I/O; corroborate() return values map directly to source_count/confidence_tier
- Live code: `internal/tui/incidents.go`, `internal/tui/quarantine_panel.go` -- confirmed stateTick poll pattern; existing CatalogQuarantineIncidentFromRecord pending/armed pattern is the template for corpus binding
- Live code: `internal/check/handler.go` -- confirmed sub-100ms hot path; audit.Sink.Write is the only output call; no room for synchronous adjudication
- Live code: `internal/config/config.go` -- confirmed AuditConfig pattern; CorpusConfig follows same struct convention
- PRD: `beekeeper-corpus-milestone-prd.md` sections 3.1-3.6 -- schema, adjudication_source values, push_envelope shape, scope tagging, First Responder integration spec
- CLAUDE.md architecture constraints -- pure-lib boundary, fail-closed, StateDir layout, Bubble Tea v2 import path

---
*Architecture research for: Beekeeper v1.4.0 Adjudicated Corpus (Local Loop)*
*Researched: 2026-06-13*
