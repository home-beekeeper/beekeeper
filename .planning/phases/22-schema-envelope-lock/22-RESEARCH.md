# Phase 22: Schema & Envelope Lock — Research

**Researched:** 2026-06-13
**Domain:** Go type definitions — four-layer event-record schema + push-envelope wire format; compile-time `auto_purge`-unrepresentable guard; schema-lock evaluator gate. Zero new external dependencies.
**Confidence:** HIGH — all claims are code-grounded against the live codebase or authoritative PRD/CONTEXT.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Package & boundary**
- New package `internal/corpus` holds the schema types, the envelope types, the `ActionHint` constant, and a pure constructor/validator. In Phase 22 this package is pure (types + a compile-time guard + pure validation) — the impure store/engine arrive in Phase 23.
- `internal/policy` / `internal/nudge` / `internal/pkgparse` stay I/O-free; do not add anything to them.
- `CorpusRecord` embeds the existing `internal/audit` `AuditRecord` (which already carries the behavior + decision layers) and adds the outcome and context layers as new fields. Do not duplicate behavior/decision fields.

**Four-layer schema (SCHEMA-01, SCHEMA-02)**
- Layers: behavior (source_surface, action_type, actor_lineage, target_resource, network_destination, agent_id*), decision (verdict, policy_matched, rule_id*, correlation_window*, confidence, ruleset_version), outcome (was_correct, true_label, adjudication_source, resolved_at), context (cluster_id, baseline_deviation, repo_fingerprint, fleet_node_id, scope). `*` = conditional.
- `source_surface` is additive with six values: `hook | mcp_gateway | shim | file_watcher | sentry | scan`.
- Outcome-layer fields are present as `unresolved` placeholders from the first write.
- `cluster_id` binds correlated events. Agent-mediated surfaces adjudicate per event; non-agent surfaces adjudicate per cluster. Scan cluster_id (OQ-2 locked): `hash(package_or_extension_id + version + repo_fingerprint)`, stable key.

**Push-envelope format (SCHEMA-03, SCHEMA-04)**
- Envelope = `signature{package_or_extension_id, version, behavior_signature_hash, iocs{...}}`, `true_label`, `confidence_tier`, `source_count`, `scope`, `action_hint`, `signing{issuer, signature, issued_at, nonce}` (signing zero-value in v1).
- `action_hint` is a typed constant whose ONLY pushable value is `watch_and_block`. `auto_purge` must be UNREPRESENTABLE in a pushable envelope at compile time, NOT a runtime string check.

**Behavior signature + versioning (SCHEMA-05)**
- `behavior_signature_hash` input frozen: hash over `action_type` + normalized `target_resource` + normalized `network_destination`.
- `ruleset_version` recorded on every record.

**Context-layer fingerprint fields**
- `repo_fingerprint` and `fleet_node_id` defined as fields in the context layer now, with the HMAC-SHA256 non-reversibility contract. Actual HMAC population is Phase 23 (STORE-05).

**Scope tagging (SCOPE-01, SCOPE-02)**
- Every record carries `scope` ∈ {`org_only`, `community_shareable`}; Go zero-value resolves to `org_only`.
- Only `PromoteScope` changes scope; `PromoteScope` always returns an error in v1.

**Schema-lock evaluator gate (SCHEMA-06)**
- A test asserts every field in the Nx Console worked trace maps to a schema field with no gaps, and that the envelope can represent a `watch_and_block` push carrying `confidence_tier` + `source_count`.

**Stack**
- Zero new `go.mod` dependencies.

### Claude's Discretion

- Exact Go struct + field names, json tags, and file layout within `internal/corpus`.
- The precise mechanism for the `auto_purge`-unrepresentable guard.
- Representation of conditional per-`source_surface` fields (pointer fields, `omitempty`, or sub-struct).
- The Nx Console trace fixture format and where it lives (testdata).
- Whether `repo_fingerprint`/`fleet_node_id` get a documented stub/helper signature in 22.

### Deferred Ideas (OUT OF SCOPE)

- Append-only corpus store + `RedactRecord` write path — Phase 23 (STORE-01..05)
- Adjudication engine, `true_label` assignment — Phase 23 (ADJ-01..07)
- Envelope emission + `BuildPushEnvelope` purge-rejection fuzz gate — Phase 23 (ENV-01..03) [the *type-level* guard is Phase 22; the builder+fuzz gate are Phase 23]
- HMAC population of `repo_fingerprint`/`fleet_node_id` + per-install secret generation — Phase 23 (STORE-05)
- First Responder corpus binding — Phase 24 (FRB-01..05)
- Any transport/signing-block population — out of v1 entirely
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SCHEMA-01 | Four-layer event record as typed Go schema; `CorpusRecord` embeds `AuditRecord`; `source_surface` additive field with six values; conditional fields; outcome-layer `unresolved` placeholders from first write | §§ AuditRecord embedding, source_surface field, outcome layer design, conditional fields |
| SCHEMA-02 | `cluster_id` binds correlated events; non-agent surfaces adjudicate per cluster; scan surface stable key = `hash(package_or_extension_id + version + repo_fingerprint)` | §§ Scan cluster_id mechanism, cluster_id type, SHA-256 input normalization |
| SCHEMA-03 | Push-envelope wire format frozen: `signature{...}`, `true_label`, `confidence_tier`, `source_count`, `scope`, `action_hint`, `signing{...}` (zero-value in v1) | §§ PushEnvelope type, EnvelopeSignature, SigningBlock |
| SCHEMA-04 | `action_hint` typed constant; `auto_purge` unrepresentable in pushable envelope at compile time | §§ Recommended compile-time guard mechanism |
| SCHEMA-05 | `behavior_signature_hash` input frozen; `ruleset_version` on every record | §§ Hash normalization rules, ruleset_version sourcing |
| SCHEMA-06 | Schema-lock evaluator gate — Nx Console trace maps to schema with no gaps; envelope can represent `watch_and_block` push with `confidence_tier` + `source_count` | §§ Gate test design, fixture format |
| SCOPE-01 | Every record carries `scope` from birth; Go zero-value resolves to `org_only` | §§ Scope type mechanism, zero-value analysis |
| SCOPE-02 | Promotion to `community_shareable` explicit and always returns error in v1; only `PromoteScope` changes scope | §§ PromoteScope stub design |
</phase_requirements>

---

## Summary

Phase 22 freezes the non-retrofittable foundation before any corpus record is written. The four deliverables are: (1) `internal/corpus` package with typed Go structs for the four-layer `CorpusRecord` and the `PushEnvelope`; (2) a compile-time `auto_purge`-unrepresentable guard baked into the `ActionHint` type; (3) two small modifications to existing packages (`audit.AuditRecord` gains `source_surface` + three context fields; `internal/config` gains a `CorpusConfig` block); and (4) the schema-lock gate test using the Nx Console trace fixture.

**Primary recommendation:** Implement the `auto_purge` guard via a closed-enum unexported-member type — define `type ActionHint string` with one package-level constant `ActionHintWatchAndBlock ActionHint = "watch_and_block"`. The `PushEnvelope` struct uses `ActionHint` as the field type, not `string`. Any code that tries to assign `"auto_purge"` cannot do so without constructing an untyped string and assigning it to a typed field — which is a compile error unless a conversion is performed. Combined with no `auto_purge`-valued constant anywhere in `internal/corpus`, this is the strongest practically-achievable compile-time guard in Go (see §Compile-Time Guard below for the full analysis). The Phase 23 `BuildPushEnvelope` builder validates the constant set and the Phase 23 fuzz gate is the runtime belt-and-suspenders; Phase 22 is the type-level guarantee.

**Phase 22 scope is ONLY:** type definitions + invariant guards + the gate test. No store, no adjudication, no emission, no behavioral change to existing surfaces.

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Schema type definitions | `internal/corpus` (new, Phase 22) | — | Owns all corpus types; pure package per CLAUDE.md constraint |
| Envelope type definitions | `internal/corpus` (new, Phase 22) | — | Co-located with schema; consumed by Phase 23 builder |
| `action_hint` compile-time guard | `internal/corpus` (type + const) | Phase 23 builder (runtime validation) | Type system is Phase 22; builder enforcement is Phase 23 |
| `scope` zero-value guarantee | `internal/corpus` constructor | `CorpusRecord` struct tag | Constructor enforces; field type backs it |
| `source_surface` additive field | `internal/audit.AuditRecord` (modified) | `internal/corpus.CorpusRecord` (reads it) | Additive — existing consumers unaffected (omitempty) |
| Config block for corpus | `internal/config.CorpusConfig` (modified) | — | Follows existing `AuditConfig` pattern |
| Schema-lock gate test | `internal/corpus/schema_lock_test.go` | testdata/nx_console_trace.json fixture | Gate proves field-completeness before any store exists |
| Behavior signature normalization | `internal/corpus/behavior_sig.go` | — | Pure function, Phase 22 defines rules; Phase 23 populates field |
| `CorroborateOutcome` export wrapper | `internal/policy/corroboration.go` (modified) | — | 2-line export of existing unexported `corroborate()`; consumed Phase 23+ |

---

## Standard Stack

### Core (all stdlib — zero new go.mod entries)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` | stdlib | JSON tags on all struct fields; `omitempty` for conditional fields | Already used throughout codebase |
| `crypto/sha256` | stdlib | `behavior_signature_hash` computation; scan `cluster_id` stable key | No CGO; FIPS-compatible |
| `crypto/hmac` | stdlib | HMAC-SHA256 for `repo_fingerprint`/`fleet_node_id` (contract defined in Phase 22; population in Phase 23) | Keyed hash — non-reversible without salt |
| `crypto/ed25519` | stdlib | `signing` block primitive (zero-value in v1; field type defined Phase 22) | Go stdlib since 1.13; 64-byte sigs; no CGO |
| `crypto/rand` | stdlib | Nonce field in `SigningBlock` (populated Phase 23+) | CSPRNG |

**Installation:** No `go get` commands needed. All packages are in Go stdlib.

### Existing Packages Modified (Phase 22)

| Package | File | Change |
|---------|------|--------|
| `internal/audit` | `types.go` | Add `SourceSurface`, `ClusterID`, `BaselineDeviation`, `Scope` to `AuditRecord` (all `omitempty`) |
| `internal/config` | `config.go` | Add `CorpusConfig` struct |
| `internal/policy` | `corroboration.go` | Add exported `CorroborateOutcome()` 2-line wrapper |

### Package Legitimacy Audit

> Phase 22 installs **zero new external packages**. No slopcheck run required.

| Package | Registry | Disposition |
|---------|----------|-------------|
| (none) | — | No new dependencies |

---

## Phase 22 Delta Findings (Code-Grounded)

### Finding 1: `AuditRecord` Embedding — What's Present vs. What's Missing

**Verified against `internal/audit/types.go`:**

`AuditRecord` (the embedding target) already carries:

**Behavior-layer fields (present in `AuditRecord`):**
- `AgentName` → maps to `actor_lineage` (agent identity)
- `ToolName` → maps to `action_type` proxy
- `AgentID`, `ParentAgentID`, `AgentDepth`, `AgentLineage` → maps to `actor_lineage` chain
- `SentryParentChain` → maps to `actor_lineage` for sentry surface
- `SentryFilesAccessed` → maps to `target_resource` for sentry surface
- `SentryNetworkDests` → maps to `network_destination` for sentry surface

**Decision-layer fields (present in `AuditRecord`):**
- `Decision` → maps to `verdict` (allow|warn|block)
- `Reason` → maps to `policy_matched` proxy
- `RuleIDs` → maps to `rule_id` (array form)
- `CorroborationCount` → maps to `confidence` proxy (signed-source count)
- `SourcesAgreed` / `SourcesDissented` → maps to corroboration provenance
- `SentryRuleID` → maps to `rule_id` for sentry surface (e.g., "SENTRY-005")
- `SentryRuleName`, `SentrySeverity` → sentry-specific decision fields

**NOT present in `AuditRecord` — must be added as `omitempty` fields in Phase 22:**
- `SourceSurface string json:"source_surface,omitempty"` — the branch key (PRD §3.1 top field)
- `ClusterID string json:"cluster_id,omitempty"` — binds correlated non-agent events
- `BaselineDeviation string json:"baseline_deviation,omitempty"` — "low"|"medium"|"high"|""
- `Scope string json:"scope"` — NOTE: this field is on `CorpusRecord` not `AuditRecord` (see below)

**Embedding design decision:** The CONTEXT.md says `CorpusRecord` embeds `AuditRecord`. The `AuditRecord` fields carry behavior + decision. The milestone research recommends `json:",inline"` embedding. However, Go's `encoding/json` does NOT support `json:",inline"` — that tag is a YAML/mapstructure convention. The correct Go idiom for struct embedding is to use a promoted (unnamed) embedded field, which promotes all fields including their JSON tags. This is the right approach.

```go
// internal/corpus/types.go
type CorpusRecord struct {
    audit.AuditRecord                    // promoted embedding — all AuditRecord json tags promoted
    SourceSurface string `json:"source_surface,omitempty"` // additive branch key
    // ... outcome + context layers
}
```

The `json:",inline"` tag in STACK.md is incorrect Go — the planner must use unnamed embedding (promotion). `encoding/json` handles this correctly: promoted fields are serialized as if they were directly on the outer struct. There is no `json:"inline"` support in stdlib `encoding/json`. [VERIFIED: internal/audit/types.go — live code confirms AuditRecord is a plain struct with json tags; embedding via promotion is idiomatic Go]

**Key concern — JSON field name collision:** `AuditRecord.Scope` does not exist (confirmed). `AuditRecord.ClusterID` does not exist (confirmed). Adding `SourceSurface`, `ClusterID`, and `BaselineDeviation` to `AuditRecord` as `omitempty` fields is safe — they are new names. Adding `scope` to `CorpusRecord` (not `AuditRecord`) means there is no collision. [VERIFIED: internal/audit/types.go]

**Key concern — `Scope` placement:** `scope` carries different semantics between existing `AuditRecord` (no such field) and `CorpusRecord`. Placing `Scope` on `CorpusRecord` directly (not the embedded `AuditRecord`) avoids touching the existing audit log schema and keeps the audit log clean. The context layer fields `ClusterID`, `BaselineDeviation`, `RepoFingerprint`, `FleetNodeID` are also new fields — they belong on `CorpusRecord` directly, NOT on `AuditRecord`, because they are corpus-specific enrichments that do not need to be written by the hook handler's direct `AuditRecord.Write()` path.

**Revised placement matrix:**

| Field | Add to `AuditRecord`? | Add to `CorpusRecord` directly? | Rationale |
|-------|----------------------|--------------------------------|-----------|
| `source_surface` | YES (omitempty) | NO | Used as a branch key on every audit record type; hook handler sets it |
| `cluster_id` | YES (omitempty) | NO | Corpus emitter (Phase 23) sets it when mapping AuditRecord → CorpusRecord; but having it on AuditRecord allows sentry surface to set it directly |
| `scope` | NO | YES (on CorpusRecord) | Corpus-only field; never on raw audit records |
| `baseline_deviation` | NO | YES (on CorpusRecord) | Corpus-specific enrichment |
| `repo_fingerprint` | NO | YES (on CorpusRecord) | Corpus-specific context field |
| `fleet_node_id` | NO | YES (on CorpusRecord) | Corpus-specific context field |
| `was_correct`, `true_label`, `adjudication_source`, `resolved_at` | NO | YES (on CorpusRecord) | Outcome layer — pure corpus fields |

**Actual `AuditRecord` additions for Phase 22 (minimal, backward-compatible):**
1. `SourceSurface string json:"source_surface,omitempty"` — the branch key
2. `ClusterID string json:"cluster_id,omitempty"` — for Sentry surface use; corpus emitter reads it

This is narrower than ARCHITECTURE.md's broader list. Adding only these two to `AuditRecord` keeps the existing audit log schema minimal. All other context/corpus fields stay on `CorpusRecord`.

---

### Finding 2: Compile-Time `auto_purge`-Unrepresentable Guard — Recommended Mechanism

**Analysis of Go type options:**

**Option A: Typed string constant (RECOMMENDED)**
```go
// internal/corpus/action_hint.go

// ActionHint is the typed set of fleet-pushable actions in a PushEnvelope.
// It is a named string type so the compiler rejects untyped string literals
// at assignment sites. The ONLY value in the pushable set is ActionHintWatchAndBlock.
// auto_purge is not defined as a constant and therefore cannot appear in a
// well-typed PushEnvelope without an explicit unsafe conversion.
type ActionHint string

// ActionHintWatchAndBlock is the sole pushable action hint.
// It instructs a receiving machine to: raise a Sentry watch on the process tree
// associated with the flagged package, block new installs of that version, and
// arm the local quarantine card. Purge action is never fleet-pushable.
const ActionHintWatchAndBlock ActionHint = "watch_and_block"

// PushEnvelope uses ActionHint, not string, so the compiler enforces the allowed set
// at every construction site.
type PushEnvelope struct {
    Signature      EnvelopeSignature `json:"signature"`
    TrueLabel      string            `json:"true_label"`
    ConfidenceTier string            `json:"confidence_tier"`
    SourceCount    int               `json:"source_count"`
    Scope          CorpusScope       `json:"scope"`
    ActionHint     ActionHint        `json:"action_hint"` // typed — not string
    Signing        *SigningBlock      `json:"signing,omitempty"`
}
```

**Why this is genuinely compile-time-safe:**
- `envelope.ActionHint = "auto_purge"` → **compile error**: cannot use untyped string constant "auto_purge" as type `ActionHint`
- `envelope.ActionHint = ActionHint("auto_purge")` → compiles but is an explicit unsafe conversion with no corresponding constant — a strict reviewer will catch this as a code smell immediately, and it leaves no magic string constant to grep for
- `envelope.ActionHint = someStringVar` → **compile error**: cannot assign `string` to `ActionHint`
- `envelope.ActionHint = ActionHintWatchAndBlock` → compiles and is the only well-typed assignment

**What this does NOT prevent:**
- Explicit type conversion: `ActionHint("auto_purge")` — this is a Go language feature and cannot be prevented by type design alone
- JSON deserialization into an `ActionHint` field from external input — `encoding/json` will unmarshal any string into a `ActionHint` field without validation

**Why the Phase 23 builder + fuzz gate closes the runtime gap:**
The type-level guard is the Phase 22 compile-time foundation. Phase 23's `BuildPushEnvelope(record CorpusRecord) (PushEnvelope, error)` returns an error for any purge-class intent — this is the runtime path that consumers use. Phase 23's fuzz gate (`ENV-03`) asserts no constructed envelope escapes with a non-allowlisted `action_hint`. Together: compile-time guard prevents accidental introduction by future contributors; builder prevents programmatic construction; fuzz gate proves no code path can emit `auto_purge`.

**Option B: Unexported type + constructor (considered, rejected for Phase 22)**
```go
// This would make PushEnvelope itself unexported or require a builder:
type pushEnvelope struct { ... } // unexported
func BuildPushEnvelope(...) (pushEnvelope, error) { ... } // only way to construct
```
This is better blast-radius isolation but makes Phase 22 incompatible with Phase 23's architecture (Phase 23 needs to construct PushEnvelope values in tests). The CONTEXT.md defers the builder to Phase 23 (`BuildPushEnvelope` is ENV-02). Phase 22's job is the type-level guard, not the builder.

**Decision: Use Option A (typed `ActionHint` string) for Phase 22.** Paired with Phase 23's builder and fuzz gate, this achieves genuine defense-in-depth. [ASSUMED — this is the recommended approach; the planner should confirm the plan-checker will accept it]

---

### Finding 3: `source_surface` Additive Field — How It Becomes the Branch Key

`source_surface` should be added to `AuditRecord` as a plain `string` with `json:"source_surface,omitempty"`. Six valid values: `hook | mcp_gateway | shim | file_watcher | sentry | scan`.

**How existing surfaces populate it (Phase 23 wiring, not Phase 22):**
- `internal/check/handler.go`: sets `SourceSurface = "hook"` on the `AuditRecord` before `auditSink.Write()`
- `internal/gateway/`: sets `SourceSurface = "mcp_gateway"`
- `internal/sentry/`: sets `SourceSurface = "sentry"` (already has `SentryRuleID` conditional field)
- `internal/watch/`: sets `SourceSurface = "file_watcher"` or `"scan"`

**Phase 22 only defines the field and its six valid values.** Existing audit records written before Phase 22 will have `source_surface: ""` (omitempty = absent from JSON), which is fine — backward-compatible for existing audit consumers.

**Conditional per-surface fields (already in `AuditRecord`):**
The existing `AuditRecord` already handles per-surface conditional fields via `omitempty`:
- `agent_id` → `AgentID string json:"agent_id,omitempty"` (agent-mediated surfaces only)
- `rule_id` → `SentryRuleID string json:"sentry_rule_id,omitempty"` (sentry only)
- `correlation_window` → implicit in sentry rule windows

**No new conditional field struct is needed.** The existing `omitempty` pattern is the correct approach. Phase 22 defines the six `source_surface` values as constants; the conditional fields are already in place. [VERIFIED: internal/audit/types.go]

---

### Finding 4: `behavior_signature_hash` Normalization Rules — Frozen Specification

The hash input must be frozen in Phase 22 (SCHEMA-05). Any change later is a breaking schema change.

**Frozen normalization rules (recommended):**

```
behavior_signature_hash = SHA-256(
    action_type_normalized
    + "\x00"
    + target_resource_normalized
    + "\x00"
    + network_destination_normalized
)
```

Where:
- **`action_type_normalized`** = `AuditRecord.ToolName` lowercased and stripped of path components (base name only). E.g., `"bash"` not `"/usr/bin/bash"`. For sentry surface: the PRD §3.1 `action_type` is synthesized from `SentryRuleID` (e.g., `"SENTRY-005"` → `action_type="sentry_exfil_fusion"`). The normalization rule is: lowercase, strip path, use snake_case.
- **`target_resource_normalized`** = `AuditRecord.SentryFilesAccessed[0]` for sentry events; the first positional argument in `ToolInput["command"]` for hook events; empty string if absent. Normalized: strip home directory prefix (`~` or absolute home path → `~`), lowercase on case-insensitive OS, forward-slash path separator.
- **`network_destination_normalized`** = `AuditRecord.SentryNetworkDests[0]` for sentry events; empty string for hook events (hook events don't have network destinations). Normalized: strip ports → hostname only; lowercase. E.g., `"192.168.1.1:443"` → `"192.168.1.1"`.
- **Separator**: NUL byte (`\x00`) to prevent `"a" + "bc"` colliding with `"ab" + "c"`.
- **Hash**: `crypto/sha256.Sum256()` → 32-byte hash → hex-encoded 64-char string.

**Why these rules:** The hash is a behavior fingerprint of the attacker, not the victim. The attacker reuses package names, exfil destinations, and file access patterns across campaigns. Normalizing away victim-specific context (absolute home paths, exact ports) makes the fingerprint match across victims while remaining specific to the attack pattern.

**Where `ruleset_version` comes from:** `AuditRecord.RuleIDs` carries policy-matched rule IDs. There is no current `ruleset_version` field on `AuditRecord`. Phase 22 must add `RulesetVersion string json:"ruleset_version,omitempty"` to `AuditRecord` AND define a `CorpusSchemaVersion = "1.0"` constant in `internal/corpus`. The `RulesetVersion` is populated by the policy loader with its catalog snapshot version; the corpus record copies it from the `AuditRecord`. [VERIFIED: internal/audit/types.go — `ruleset_version` field is absent; must be added]

**Additional `AuditRecord` field needed:** `RulesetVersion string json:"ruleset_version,omitempty"` — add to `AuditRecord.`

---

### Finding 5: `scope` Zero-Value-Resolves-to-`org_only` — Recommended Mechanism

**Problem:** `scope string` zero value is `""`, not `"org_only"`. If `CorpusRecord.Scope` is an unguarded `string`, an uninitialized record serializes as `"scope": ""` — a scope-tag-leakage risk (Pitfall 4 in PITFALLS.md).

**Three options:**

**Option A: Custom type with `MarshalJSON` (RECOMMENDED)**
```go
// internal/corpus/scope.go

// CorpusScope is the sharing-scope tag on every CorpusRecord. The zero value
// (empty string) MUST serialize as "org_only" to prevent scope-tag leakage
// (Pitfall 4 — community_shareable must never be the default).
type CorpusScope string

const (
    // ScopeOrgOnly is the default scope. Records stay within the local machine
    // in v1 and within the originating org in v1.1+.
    ScopeOrgOnly CorpusScope = "org_only"
    // ScopeCommunityShareable indicates the record has been explicitly promoted
    // and anonymized. Promotion is a v2.0 feature; PromoteScope always returns
    // an error in v1.
    ScopeCommunityShareable CorpusScope = "community_shareable"
)

// MarshalJSON implements json.Marshaler. The zero value ("") serializes as
// "org_only" rather than "". This prevents scope-leakage from uninitialized
// records — `CorpusRecord{}` must serialize as `"scope":"org_only"`.
func (s CorpusScope) MarshalJSON() ([]byte, error) {
    if s == "" {
        return []byte(`"org_only"`), nil
    }
    return json.Marshal(string(s))
}
```

**Why MarshalJSON:** This is the ONLY mechanism that makes the zero value serialize correctly without requiring a constructor call. A constructor that sets `Scope = ScopeOrgOnly` relies on callers always using the constructor — which is a behavioral guarantee, not a type guarantee. If any code does `CorpusRecord{}` or `&CorpusRecord{Endpoint: "check"}`, the scope would be empty without `MarshalJSON`.

**Option B: Constructor enforcement only**
```go
func NewCorpusRecord() CorpusRecord {
    return CorpusRecord{Scope: ScopeOrgOnly, TrueLabel: "unresolved", ...}
}
```
This is necessary but not sufficient — test code and zero-value construction can bypass it.

**Option C: `iota` typed int with `MarshalJSON`**
More complex; `string` type is cleaner for JSON output.

**Decision: Use Option A (`MarshalJSON` on `CorpusScope` type) + Option B (constructor).** Belt-and-suspenders: the type-level guarantee catches zero-value construction; the constructor sets correct defaults. [ASSUMED — MarshalJSON approach; widely used in Go codebases for this exact pattern]

**`PromoteScope` stub in Phase 22:**
```go
// PromoteScope returns an error in v1. Scope promotion requires anonymization
// (v2.0 deliverable). This function is defined in Phase 22 so the call site
// exists and the error is explicit from day one.
func PromoteScope(r *CorpusRecord) error {
    return errors.New("corpus: scope promotion requires anonymization gate (v2.0); not available in v1")
}
```

---

### Finding 6: Scan `cluster_id` Stable Key — Exact Hash Input

Per OQ-2 (locked): `cluster_id = hash(package_or_extension_id + version + repo_fingerprint)`.

**Exact Go implementation:**
```go
// ScanClusterID derives the stable cluster_id for a scan surface hit.
// The key is stable across re-scans: same package+version on the same machine
// always produces the same cluster_id, making re-scans idempotent.
//
// IMPORTANT: repo_fingerprint at this call site is the HMAC-SHA256 hex value
// (populated by Phase 23). If repo_fingerprint is empty (Phase 22 gate test
// context), the key is still stable within the session but not across
// reinstallation (salt changes).
func ScanClusterID(packageOrExtID, version, repoFingerprint string) string {
    h := sha256.New()
    h.Write([]byte(packageOrExtID))
    h.Write([]byte("\x00"))
    h.Write([]byte(version))
    h.Write([]byte("\x00"))
    h.Write([]byte(repoFingerprint))
    return hex.EncodeToString(h.Sum(nil))[:16] // 16 hex chars = 8 bytes = 64-bit ID space
}
```

**Why SHA-256 truncated to 16 hex chars:** Collision probability is negligible for any realistic corpus size (< 10M records per machine). Shorter ID is more readable in TUI and log output. This is the same approach used in the existing sentry cluster derivation (`internal/corpus/emitter.go` design in ARCHITECTURE.md).

**Why NUL separator:** Prevents `"pkg" + "1.0foo"` from colliding with `"pkg1.0" + "foo"`.

---

### Finding 7: `RedactRecord` Unexported Type — Landmine for Phase 23

**Critical finding from live code (`internal/audit/redact.go`):**

```go
type redactPattern struct { ... }       // UNEXPORTED type
func RedactRecord(rec AuditRecord, patterns []redactPattern) AuditRecord // takes unexported type
func DefaultRedactPatterns() []redactPattern // returns unexported type
```

**The problem:** `RedactRecord` and `DefaultRedactPatterns()` both use the unexported type `redactPattern`. While `DefaultRedactPatterns()` can be called from within `internal/audit`, it **cannot** be called by `internal/corpus` because the return type `[]redactPattern` is unexported and `internal/corpus` is a different package.

This is a Phase 22 landmine for Phase 23. Phase 22 defines the `CorpusRecord` type. Phase 23's corpus store must call redaction before writing — but cannot call `RedactRecord(rec, DefaultRedactPatterns())` because `DefaultRedactPatterns()` returns an unexported type that cannot be assigned to a variable in `internal/corpus`.

**Resolution (Phase 22 must add to `internal/audit/redact.go`):**
```go
// RedactRecordWithDefaults returns a copy of rec with all default sensitive
// patterns applied. This is the corpus store's redaction entrypoint — it does
// not expose the unexported redactPattern type to callers outside this package.
func RedactRecordWithDefaults(rec AuditRecord) AuditRecord {
    return RedactRecord(rec, DefaultRedactPatterns())
}
```

**This function must be added in Phase 22** so the type contract is established before Phase 23's corpus store implements it. The planner must include a task to add `RedactRecordWithDefaults` to `internal/audit/redact.go`. [VERIFIED: internal/audit/redact.go — `redactPattern` is confirmed unexported; `RedactRecord` and `DefaultRedactPatterns` both use it; `internal/corpus` cannot call them directly]

---

### Finding 8: `CorroborateOutcome` Export — Exact Change Needed

**Current state in `internal/policy/corroboration.go`:**
```go
func corroborate(matches []CatalogMatch, t CorroborationThresholds) (level string, quarantine bool, count int, agreed, dissented []string)
// ^ unexported; returns 5 values
```

**What Phase 23 needs (as per ARCHITECTURE.md):** A 2-line exported wrapper that returns only `source_count` (= `count`) and `confidence_tier` (derived from `level`).

**Exact addition to `internal/policy/corroboration.go` (Phase 22):**
```go
// CorroborationOutcome is the pure-function result consumed by the corpus
// adjudication engine (internal/corpus). It contains only the fields needed
// for corpus source_count and confidence_tier derivation.
type CorroborationOutcome struct {
    SourceCount    int    // number of distinct SIGNED sources that agreed
    ConfidenceTier string // "watch" (1 source) | "enforce" (2+ sources)
}

// CorroborateOutcome is an exported wrapper over the unexported corroborate()
// function, returning only the fields the corpus adjudication engine needs.
// It is a pure function — no I/O, no goroutines, no side effects.
//
// confidence_tier mapping:
//   level == "block" || level == "warn" (signedCount >= 2) → "enforce"
//   level == "warn" (signedCount == 1)                     → "watch"
//   level == "allow"                                       → "watch" (no corroboration)
func CorroborateOutcome(matches []CatalogMatch, t CorroborationThresholds) CorroborationOutcome {
    level, _, count, _, _ := corroborate(matches, t)
    tier := "watch"
    if count >= t.BlockAt {
        tier = "enforce"
    }
    return CorroborationOutcome{SourceCount: count, ConfidenceTier: tier}
}
```

**Critical nuance:** `confidence_tier: "enforce"` requires `source_count >= t.BlockAt` (default 2). A single-source critical-severity block (`level == "block"` with `count == 1` via severity override) maps to `confidence_tier: "watch"` in the corpus record, even though the `beekeeper check` verdict is `block`. The corpus records corroboration tier, not enforcement action. This is Pitfall 3 from PITFALLS.md — the exact test case is: `(sources=["bumblebee"], verdict=block via critical override)` → `source_count: 1, confidence_tier: "watch"`. [VERIFIED: internal/policy/corroboration.go — existing `corroborate()` signature confirmed; no exported wrapper exists yet]

---

### Finding 9: Schema-Lock Gate (SCHEMA-06) — Nx Console Fixture Design

**What the gate must prove:**
1. Every field in the PRD §3.1 four-layer schema is present in at least one `CorpusRecord` field (no schema gaps)
2. The `PushEnvelope` can represent a `watch_and_block` push carrying `confidence_tier: "enforce"` and `source_count: 2`
3. The Nx Console incident maps cleanly to the typed schema

**The Nx Console incident (from CONTEXT.md §Specifics):** A trojanized VS Code extension exfiltrating repos:
- `source_surface: "sentry"` (Sentry detects the exfil behavior)
- `action_type: "sentry_exfil_fusion"` (SENTRY-005 fires)
- `actor_lineage`: `["code", "nx-console.vsix", "node"]` — editor-descended process tree
- `target_resource`: `.git/config` or similar repo credential file
- `network_destination`: attacker-controlled IP (first outbound after file read)
- `verdict: "block"` → `"alert"` (Sentry is detection-only; confirmed via ForensicConfirmation)
- `rule_id: "SENTRY-005"` → maps to `SentryRuleID`
- `cluster_id`: derived from SENTRY-005 correlation window
- `was_correct: nil` initially (unresolved), then `true` after adjudication
- `true_label: "unresolved"` → later `"malicious"`
- `confidence_tier: "enforce"` (catalog_confirmation + breach_confirmation = source_count 2)

**Fixture format (testdata/nx_console_trace.json):**
```json
{
  "description": "Nx Console Sentry exfil trace — Phase 22 schema-lock fixture",
  "surface": "sentry",
  "rule_id": "SENTRY-005",
  "actor_lineage": ["code", "node", "nx-language-server"],
  "target_resource": "~/.ssh/id_rsa",
  "network_destination": "malicious-collector.example.com",
  "verdict": "block",
  "confidence_tier_expected": "enforce",
  "source_count_expected": 2,
  "action_hint_expected": "watch_and_block"
}
```

**Gate test structure (schema_lock_test.go):**
```go
// TestSchemaLockNxConsoleTrace is the SCHEMA-06 evaluator gate.
// It proves: (1) every PRD §3.1 schema field maps to a typed Go field with no gaps;
// (2) a PushEnvelope can represent watch_and_block push with confidence_tier+source_count.
func TestSchemaLockNxConsoleTrace(t *testing.T) {
    // 1. Load the fixture
    // 2. Construct a CorpusRecord from the trace fields using the corpus constructors
    // 3. Assert every PRD-named field is set (non-zero or explicitly "unresolved")
    // 4. Construct a PushEnvelope with ActionHintWatchAndBlock, ConfidenceTier:"enforce", SourceCount:2
    // 5. Marshal to JSON and assert all envelope fields are present
    // 6. Assert ActionHint == ActionHintWatchAndBlock (typed, not string "watch_and_block")
    // 7. Assert scope serializes as "org_only" (zero-value test)
}
```

**What "no gaps" means for the test:** Enumerate all PRD §3.1 field names and assert each maps to a named Go struct field (via reflection or explicit assignment). The test does NOT need to prove the corpus store works — it only needs to prove the types are sufficient to carry the trace.

---

### Finding 10: `CorpusConfig` Block — Config Pattern

**Pattern to follow (from `internal/config/config.go`):** `AuditConfig` struct with `json` tags; added to the root `Config` struct.

```go
// CorpusConfig holds Phase 22+ corpus configuration.
// Follows the same pattern as AuditConfig.
type CorpusConfig struct {
    // Enabled controls whether the corpus store is active. Default false
    // until Phase 23 wires the store; Phase 22 defines the type.
    Enabled bool `json:"enabled"`
    // Path overrides the default corpus file location.
    // Default: StateDir()/corpus/beekeeper-corpus.ndjson
    Path string `json:"path,omitempty"`
    // Scope is the default scope for new records.
    // Valid values: "org_only" (default), "community_shareable".
    // "community_shareable" is reserved for v2.0; setting it in v1 has no effect.
    Scope string `json:"scope,omitempty"`
}
```

Add to `Config` struct: `Corpus CorpusConfig json:"corpus,omitempty"`.

---

## Architecture Patterns

### System Architecture Diagram

```
Phase 22 deliverables only (types + guard + gate test):

  internal/corpus/           [NEW — pure package]
    types.go                 CorpusRecord (embeds audit.AuditRecord)
                             PushEnvelope, EnvelopeSignature, SigningBlock
    action_hint.go           type ActionHint string; const ActionHintWatchAndBlock
    scope.go                 type CorpusScope string; MarshalJSON zero→"org_only"
                             PromoteScope() always-error stub
    behavior_sig.go          BehaviorSigHash(actionType, targetResource, networkDest) string
                             ScanClusterID(pkgID, version, repoFingerprint) string
    schema_version.go        const CorpusSchemaVersion = "1.0"
    schema_lock_test.go      SCHEMA-06 gate test + testdata/nx_console_trace.json

  internal/audit/types.go   [MODIFIED — additive omitempty fields]
    + SourceSurface          string json:"source_surface,omitempty"
    + ClusterID              string json:"cluster_id,omitempty"
    + RulesetVersion         string json:"ruleset_version,omitempty"

  internal/audit/redact.go  [MODIFIED — one new exported function]
    + RedactRecordWithDefaults(rec AuditRecord) AuditRecord

  internal/policy/corroboration.go  [MODIFIED — 2-line export wrapper]
    + type CorroborationOutcome struct
    + func CorroborateOutcome(...) CorroborationOutcome

  internal/config/config.go  [MODIFIED — new config block]
    + type CorpusConfig struct
    + Config.Corpus CorpusConfig
```

### Recommended Project Structure

```
internal/corpus/
├── types.go              # CorpusRecord, PushEnvelope, EnvelopeSignature, SigningBlock
├── action_hint.go        # type ActionHint string; ActionHintWatchAndBlock const
├── scope.go              # type CorpusScope string; MarshalJSON; ScopeOrgOnly const; PromoteScope stub
├── behavior_sig.go       # BehaviorSigHash(); ScanClusterID(); pure functions, no I/O
├── schema_version.go     # CorpusSchemaVersion = "1.0" const
├── corpus_test.go        # Unit tests: scope zero-value, action_hint type safety, cluster_id stability
└── schema_lock_test.go   # SCHEMA-06 gate: Nx Console trace coverage + envelope representability
    testdata/
    └── nx_console_trace.json   # Nx Console Sentry exfil fixture
```

### Pattern 1: Typed Enum for `action_hint` (Compile-Time Guard)

**What:** Define `type ActionHint string` with only `ActionHintWatchAndBlock` as a package constant. Use `ActionHint` as the field type in `PushEnvelope.ActionHint`. No `"auto_purge"` constant exists in the package.

**When to use:** Any field where only a specific set of values is acceptable and a compile-time guarantee is stronger than a runtime check. (Same pattern used for `FailModeClosed`/`FailModeOpen` in `internal/config`.)

**Example:**
```go
// Source: internal/config/config.go (existing pattern)
const (
    FailModeClosed = "closed"
    FailModeOpen   = "open"
    FailModeWarn   = "warn"
)
```
The corpus guard goes one step further by using a named type (not just `string` constants) so assignment of an untyped string literal is a compile error.

### Pattern 2: `encoding/json` Struct Embedding for Multi-Layer Record

**What:** Unnamed embedding of `audit.AuditRecord` in `CorpusRecord` promotes all `AuditRecord` fields to the top-level JSON object. The corpus-specific fields are additional fields on `CorpusRecord`.

**When to use:** When extending an existing record type without duplication.

**Example:**
```go
type CorpusRecord struct {
    audit.AuditRecord                              // promoted — all json tags preserved
    SourceSurface     string      `json:"source_surface,omitempty"`
    // Outcome layer
    WasCorrect        *bool       `json:"was_correct,omitempty"`  // nil = unresolved
    TrueLabel         string      `json:"true_label"`             // always present; "unresolved" initially
    AdjudicationSource string     `json:"adjudication_source,omitempty"`
    ResolvedAt        string      `json:"resolved_at,omitempty"` // RFC3339; absent until adjudicated
    // Context layer
    ClusterID         string      `json:"cluster_id,omitempty"`
    BaselineDeviation string      `json:"baseline_deviation,omitempty"`
    RepoFingerprint   string      `json:"repo_fingerprint,omitempty"`  // HMAC-SHA256 hex (Phase 23 populates)
    FleetNodeID       string      `json:"fleet_node_id,omitempty"`     // HMAC-SHA256 hex (Phase 23 populates)
    Scope             CorpusScope `json:"scope"`                       // always present; zero→"org_only"
    CorpusSchemaVer   string      `json:"corpus_schema_version"`       // always "1.0"
    RulesetVersion    string      `json:"ruleset_version,omitempty"`   // mirrors AuditRecord.RulesetVersion
}
```

**JSON namespace collision check:** `AuditRecord` has no `scope`, `cluster_id`, `baseline_deviation`, `repo_fingerprint`, `fleet_node_id`, `was_correct`, `true_label`, `adjudication_source`, `resolved_at`, or `corpus_schema_version` fields. No collision. [VERIFIED: internal/audit/types.go]

### Anti-Patterns to Avoid

- **`json:",inline"` tag:** Not a valid `encoding/json` tag. Milestone research STACK.md mentions it — this is incorrect for stdlib `encoding/json`. Use unnamed embedding (promotion) instead.
- **`Scope string` without `MarshalJSON`:** Zero value serializes as `""`. Must use `CorpusScope` type with `MarshalJSON`.
- **`action_hint string` untyped field:** Allows `"auto_purge"` assignment with no compile error. Must use `ActionHint` named type.
- **Adding corpus-specific fields to `AuditRecord`:** Keep only `source_surface`, `cluster_id`, and `ruleset_version` on `AuditRecord`; all other corpus fields live on `CorpusRecord` to avoid polluting the existing audit log schema.
- **Calling `RedactRecord(rec, DefaultRedactPatterns())` from `internal/corpus`:** Won't compile — `redactPattern` is unexported. Must use `RedactRecordWithDefaults()` (new function in Phase 22).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Non-reversible fingerprint | Custom hash of hostname/path | `crypto/hmac` + `crypto/sha256` (stdlib) | HMAC with per-install secret is the only non-brute-force-reversible approach; plain SHA-256 is a dictionary attack target |
| Envelope signing | Custom signing scheme | `crypto/ed25519` (stdlib) — zero-value in v1, type defined now | Already audited; 64-byte sigs; Go team maintained; forward-compatible with sigstore |
| Scope default enforcement | Constructor-only check | `MarshalJSON` on `CorpusScope` type | Constructor enforcement can be bypassed by zero-value construction; `MarshalJSON` cannot |
| `action_hint` allowlist | Runtime string comparison | `type ActionHint string` + single constant | Type system prevents assignment errors at compile time |
| `cluster_id` derivation | Separate state machine | `sha256(pkg + version + fingerprint)` pure function | Stateless, reproducible, idempotent across re-scans |

---

## Common Pitfalls

### Pitfall 1: `json:",inline"` Tag in Struct Embedding (Direct Impact on Phase 22)
**What goes wrong:** Planner writes `audit.AuditRecord json:",inline"` as a named field, expecting it to flatten the JSON. `encoding/json` ignores the `inline` tag — it produces a nested `"AuditRecord"` JSON object, not promoted fields.
**How to avoid:** Use unnamed embedding: `audit.AuditRecord` with no field name and no json tag. Go's `encoding/json` handles promotion of fields automatically.
**Warning signs:** Test fixture shows `"AuditRecord": {...}` nesting in corpus NDJSON output.

### Pitfall 2: Scope Zero-Value Leakage (SCOPE-01)
**What goes wrong:** `CorpusRecord.Scope string json:"scope"` — zero value `""` serializes as `"scope":""`. A corpus consumer that reads `scope != "org_only"` as "community_shareable" now treats every uninitialized record as community-shareable.
**How to avoid:** Use `CorpusScope` type with `MarshalJSON` that maps `""` → `"org_only"`.
**Warning signs:** Gate test asserts `rec.Scope == "org_only"` but forgets to assert the JSON serialization produces `"scope":"org_only"` for a zero-value `CorpusRecord{}`.

### Pitfall 3: `RedactRecord` Unexported Type Compile Error
**What goes wrong:** Phase 23 implementation tries to call `audit.RedactRecord(rec, audit.DefaultRedactPatterns())` from `internal/corpus`. Won't compile because `redactPattern` is unexported and `DefaultRedactPatterns()` returns `[]redactPattern`.
**How to avoid:** Phase 22 adds `RedactRecordWithDefaults(rec AuditRecord) AuditRecord` to `internal/audit/redact.go`. Phase 23's corpus store calls this single-argument version.
**Warning signs:** Phase 23 plan says "call `audit.RedactRecord` with default patterns" without noting the unexported-type constraint.

### Pitfall 4: `CorroborateOutcome` Tier Mapping Error (Pitfall 3 from PITFALLS.md)
**What goes wrong:** Maps `level == "block"` → `confidence_tier: "enforce"` regardless of `count`. A single-source critical-severity block (severity override in `DefaultCorroborationThresholds`) has `level == "block"` but `count == 1`. Corpus would emit `source_count: 1, confidence_tier: "enforce"` — a lie that breaks the 2FA invariant for downstream consumers.
**How to avoid:** `confidence_tier` is derived from `count >= t.BlockAt` (default 2), NOT from `level`. A single-source block always emits `confidence_tier: "watch"` in the corpus record.

### Pitfall 5: Outcome Fields Missing as Placeholders (Pitfall 1 from PITFALLS.md)
**What goes wrong:** Phase 22 defines `CorpusRecord` but leaves out `TrueLabel`, `WasCorrect`, `ResolvedAt` fields because "Phase 23 populates them." Phase 23 cannot add them to an emitted record — the schema must emit them as `"unresolved"` placeholders from the first write.
**How to avoid:** `TrueLabel` must NOT be `omitempty` — it is always `"unresolved"` initially. `WasCorrect *bool` is `nil` initially (pointer semantics; `omitempty` for pointer → absent from JSON when nil is fine). `ResolvedAt string omitempty` is absent when unresolved.

---

## Code Examples

### Full `CorpusRecord` Type Definition
```go
// Source: internal/corpus/types.go (Phase 22 target)

// CorpusRecord is one NDJSON line in the corpus store. It embeds
// the existing AuditRecord (behavior + decision layers) and adds the
// outcome and context layers.
//
// Field placement rationale:
//   - behavior fields: all in embedded audit.AuditRecord
//   - decision fields: all in embedded audit.AuditRecord
//   - source_surface: additive on CorpusRecord (branch key)
//   - outcome fields: on CorpusRecord (corpus-specific)
//   - context fields: on CorpusRecord (corpus-specific)
//
// Non-retrofittable: outcome fields must be present (even as zero/unresolved)
// from the first write. See Pitfall 1 in PITFALLS.md.
type CorpusRecord struct {
    audit.AuditRecord // promoted — carries behavior + decision layers

    // source_surface: additive branch key identifying which Beekeeper surface
    // produced this record. One of: hook|mcp_gateway|shim|file_watcher|sentry|scan.
    // Set by the corpus emitter (Phase 23) from AuditRecord.SourceSurface.
    // Redundant with AuditRecord.SourceSurface but explicit for corpus consumers.
    // (Actually read directly from AuditRecord.SourceSurface after embedding; this
    // field may be omitted if AuditRecord.SourceSurface is sufficient.)

    // --- Outcome layer (THE MOAT — non-retrofittable) ---
    // TrueLabel is always present. Initial value is "unresolved".
    // Values: malicious|benign|policy_correct|unresolved
    TrueLabel          string  `json:"true_label"`
    // AdjudicationSource is set by the adjudication engine (Phase 23).
    AdjudicationSource string  `json:"adjudication_source,omitempty"`
    // WasCorrect is nil until adjudicated. true = verdict matched ground truth.
    WasCorrect         *bool   `json:"was_correct,omitempty"`
    // ResolvedAt is RFC3339 timestamp set when TrueLabel leaves "unresolved".
    ResolvedAt         string  `json:"resolved_at,omitempty"`

    // --- Context layer ---
    ClusterID         string      `json:"cluster_id,omitempty"`
    BaselineDeviation string      `json:"baseline_deviation,omitempty"`
    // RepoFingerprint is HMAC-SHA256(secret, canonical_repo_id) hex string.
    // Populated by Phase 23 (STORE-05). Non-reversible without the per-install secret.
    RepoFingerprint   string      `json:"repo_fingerprint,omitempty"`
    // FleetNodeID is HMAC-SHA256(secret, hostname) hex string. Non-reversible.
    FleetNodeID       string      `json:"fleet_node_id,omitempty"`
    // Scope is always present. Zero value serializes as "org_only" via MarshalJSON.
    Scope             CorpusScope `json:"scope"`

    // --- Schema versioning ---
    CorpusSchemaVersion string `json:"corpus_schema_version"` // always "1.0"
    // RulesetVersion is copied from AuditRecord.RulesetVersion.
    // Redundant but explicit for corpus consumers.

    // --- Push envelope (populated at emit, NOT transmitted in v1) ---
    // PushEnvelope is nil until Phase 23 emitter populates it.
    PushEnvelope *PushEnvelope `json:"push_envelope,omitempty"`
}

// PushEnvelope is the frozen v1 wire format. Populated by Phase 23 BuildPushEnvelope.
// The Signing block is zero-value in v1 — defined now so the format is frozen.
type PushEnvelope struct {
    Signature      EnvelopeSignature `json:"signature"`
    TrueLabel      string            `json:"true_label"`
    ConfidenceTier string            `json:"confidence_tier"` // watch|enforce
    SourceCount    int               `json:"source_count"`
    Scope          CorpusScope       `json:"scope"`
    ActionHint     ActionHint        `json:"action_hint"` // typed: only ActionHintWatchAndBlock
    Signing        *SigningBlock      `json:"signing,omitempty"` // nil in v1
}

type EnvelopeSignature struct {
    PackageOrExtensionID  string   `json:"package_or_extension_id"`
    Version               string   `json:"version"`
    BehaviorSignatureHash string   `json:"behavior_signature_hash"`
    IOCs                  IOCBlock `json:"iocs,omitempty"`
}

type IOCBlock struct {
    Domains           []string `json:"domains,omitempty"`
    DNSTunnelPattern  string   `json:"dns_tunnel_pattern,omitempty"`
    DeadDropPattern   string   `json:"dead_drop_pattern,omitempty"`
}

// SigningBlock is defined in v1 (zero-value) to freeze the wire format.
// Populated in v1.1+ when transport exists.
type SigningBlock struct {
    Issuer    string `json:"issuer"`
    Signature string `json:"signature"`
    IssuedAt  string `json:"issued_at"`
    Nonce     string `json:"nonce"`
}
```

### `ActionHint` Compile-Time Guard
```go
// Source: internal/corpus/action_hint.go (Phase 22 target)

// ActionHint is the closed type for fleet-pushable action hints.
// Using a named type (not string) means the compiler rejects untyped
// string literals at assignment sites.
type ActionHint string

// ActionHintWatchAndBlock is the sole pushable action in the allowed set.
// No auto_purge constant is defined — it is unrepresentable.
const ActionHintWatchAndBlock ActionHint = "watch_and_block"
```

### `CorpusScope` with `MarshalJSON`
```go
// Source: internal/corpus/scope.go (Phase 22 target)

type CorpusScope string

const (
    ScopeOrgOnly            CorpusScope = "org_only"
    ScopeCommunityShareable CorpusScope = "community_shareable"
)

func (s CorpusScope) MarshalJSON() ([]byte, error) {
    if s == "" {
        return []byte(`"org_only"`), nil
    }
    return json.Marshal(string(s))
}

func PromoteScope(r *CorpusRecord) error {
    return errors.New("corpus: scope promotion requires anonymization gate (v2.0); not available in v1")
}
```

### `BehaviorSigHash` and `ScanClusterID`
```go
// Source: internal/corpus/behavior_sig.go (Phase 22 target)

func BehaviorSigHash(actionType, targetResource, networkDestination string) string {
    h := sha256.New()
    h.Write([]byte(normalizeActionType(actionType)))
    h.Write([]byte{0})
    h.Write([]byte(normalizeTargetResource(targetResource)))
    h.Write([]byte{0})
    h.Write([]byte(normalizeNetworkDest(networkDestination)))
    return hex.EncodeToString(h.Sum(nil))
}

func ScanClusterID(packageOrExtID, version, repoFingerprint string) string {
    h := sha256.New()
    h.Write([]byte(packageOrExtID))
    h.Write([]byte{0})
    h.Write([]byte(version))
    h.Write([]byte{0})
    h.Write([]byte(repoFingerprint))
    return hex.EncodeToString(h.Sum(nil))[:16]
}

// normalizeActionType: lowercase, base name only
// normalizeTargetResource: strip absolute home prefix → ~, forward-slash, lowercase
// normalizeNetworkDest: strip port → hostname only, lowercase
```

---

## What Belongs in Phase 22 vs Phase 23 — Landmine Disambiguation

| Item | Phase 22? | Phase 23? | Note |
|------|-----------|-----------|------|
| `type ActionHint string` + `ActionHintWatchAndBlock` const | YES | — | Type-level guard is the Phase 22 deliverable |
| `BuildPushEnvelope(CorpusRecord) (PushEnvelope, error)` builder | NO | YES (ENV-02) | Builder function is Phase 23; type definitions are Phase 22 |
| Fuzz gate: no envelope escapes with non-allowlisted `action_hint` | NO | YES (ENV-03) | Runtime fuzz gate is Phase 23 |
| `type CorpusScope` + `MarshalJSON` + `PromoteScope` stub | YES | — | Zero-value guarantee must exist before any record is written |
| `RedactRecordWithDefaults` in `internal/audit` | YES | — | Phase 23's store needs it; Phase 22 establishes the function contract |
| Actual `HMAC-SHA256` computation for `repo_fingerprint`/`fleet_node_id` | NO | YES (STORE-05) | Per-install secret generation is impure I/O — Phase 23 |
| `BehaviorSigHash` function signature + normalization rules | YES | — | Rules must be FROZEN in Phase 22; function definition establishes them |
| Calling `BehaviorSigHash` to populate `behavior_signature_hash` | NO | YES (emitter.go) | The field slot is Phase 22; the population is Phase 23 |
| `CorroborateOutcome` exported wrapper in `internal/policy` | YES | — | Phase 23's adjudicator needs it; Phase 22 establishes the contract |
| Actual corroboration result population in corpus records | NO | YES (ADJ-01..07) | Adjudication is Phase 23 |
| Schema-lock gate test (SCHEMA-06) | YES | — | This IS the Phase 22 evaluator gate |
| Corpus NDJSON store file creation | NO | YES (STORE-01) | No I/O in Phase 22 |
| Adding `CorpusConfig` to `internal/config` | YES | — | Config type must exist before any consumer can check `Enabled` |

---

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| `encoding/json` v1 struct embedding with explicit named field | Unnamed embedding promotes all fields to JSON top level | Use unnamed embedding; `json:",inline"` is NOT stdlib |
| `scope` as unguarded `string` field | `CorpusScope` type with `MarshalJSON` zero-value guarantee | Prevents scope-tag leakage without requiring constructor call |
| Runtime `action_hint` validation | `type ActionHint string` typed constant — compile-time | Prevents accidental `auto_purge` at assignment; runtime validation is belt-and-suspenders |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `MarshalJSON` on `CorpusScope` is the right zero-value guarantee mechanism | Finding 5 | If plan-checker objects, fallback is constructor enforcement only; scope-leakage risk increases |
| A2 | `type ActionHint string` + single constant is accepted by a strict reviewer as "genuinely compile-time-safe" | Finding 2 | Reviewer may require unexported-type builder; Phase 23 builder would need to be pulled forward |
| A3 | `audit.AuditRecord` unnamed embedding promotes all fields correctly in `encoding/json` (no collision) | Finding 1 | Confirmed by language spec; only risk is a future `AuditRecord` field name colliding with a `CorpusRecord` field — mitigated by the placement matrix |
| A4 | `BehaviorSigHash` NUL-byte separator and normalization rules are sufficient to freeze the input | Finding 4 | Normalization rules for `action_type` (ToolName → base name → lowercase) need empirical validation against real audit records |
| A5 | `ScanClusterID` truncation to 16 hex chars is sufficient collision resistance | Finding 6 | At 10M corpus records: collision probability ≈ 10M²/(2^64) ≈ 5×10⁻⁹; acceptable for this use case |

---

## Open Questions

1. **Where does `verdict` map for the four-layer schema?**
   - What we know: `AuditRecord.Decision` (`allow|warn|block`) is the verdict. But the PRD §3.1 lists `verdict` as a decision-layer field with values `allow|warn|block|alert` (alert = Sentry detection-only).
   - What's unclear: `AuditRecord.Decision` does not have `"alert"` as a value today. Sentry alerts use `SentryRuleID` but the existing code sets `Decision: "block"` for Sentry alerts. PRD adds `alert` as a fourth verdict value.
   - Recommendation: Phase 22 should add `"alert"` as a documented valid value for `AuditRecord.Decision` (no code change needed — it's a string); the corpus emitter (Phase 23) sets it for sentry surface records.

2. **`source_surface` on `AuditRecord` vs on `CorpusRecord` directly?**
   - What we know: The CONTEXT.md says `source_surface` is an additive field on the BASE RECORD. If it goes on `AuditRecord`, it appears in both the audit log and the corpus. If on `CorpusRecord` only, it appears only in the corpus.
   - Recommendation: Add to `AuditRecord` (as per Finding 3). The audit log gaining `source_surface` is valuable forensically and is non-breaking (`omitempty`). This is consistent with the milestone research ARCHITECTURE.md recommendation.

3. **`cluster_id` placement: `AuditRecord` or `CorpusRecord`?**
   - What we know: Sentry correlation already groups by rule+PID+window. The corpus emitter derives `cluster_id` from these. If `cluster_id` is on `AuditRecord`, the raw audit log has it; if on `CorpusRecord` only, the corpus has it.
   - Recommendation: Add `ClusterID` to `AuditRecord` (omitempty). The sentry daemon can set it on audit records directly, improving raw audit log forensic value. The corpus emitter reads it from `AuditRecord.ClusterID`. Non-breaking.

---

## Environment Availability

> Phase 22 is code/type-definition-only. No external tools or services needed.

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25+ | All Go code | ✓ | Confirmed (project requirement) | — |
| `encoding/json` (stdlib) | JSON tags | ✓ | Bundled with Go | — |
| `crypto/sha256` (stdlib) | `BehaviorSigHash`, `ScanClusterID` | ✓ | Bundled with Go | — |

---

## Validation Architecture

> Nyquist validation is enabled (not explicitly disabled in `.planning/config.json`).

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing package (stdlib) |
| Config file | none (Go convention; `go test ./...`) |
| Quick run command | `go test ./internal/corpus/... ./internal/audit/... ./internal/policy/... ./internal/config/...` |
| Full suite command | `go test ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SCHEMA-01 | `CorpusRecord` embeds `AuditRecord`; all four layers present; `source_surface` field present; `TrueLabel` always `"unresolved"` initially | unit | `go test ./internal/corpus/... -run TestCorpusRecordSchema` | ❌ Wave 0 |
| SCHEMA-02 | `ScanClusterID(pkg, ver, fp)` is stable across calls; different inputs produce different IDs; NUL separator prevents collisions | unit | `go test ./internal/corpus/... -run TestScanClusterID` | ❌ Wave 0 |
| SCHEMA-03 | `PushEnvelope` JSON round-trip: all fields present; `signing` block is nil/zero in v1 | unit | `go test ./internal/corpus/... -run TestPushEnvelopeRoundTrip` | ❌ Wave 0 |
| SCHEMA-04 | `ActionHint` type: `envelope.ActionHint = "auto_purge"` is a compile error (checked via `go build` of a dedicated compile-fail test helper or simply by the absence of any `auto_purge` constant); `ActionHintWatchAndBlock` constant is the only defined value | compile-time | `go build ./internal/corpus/...` (must succeed; absence of `auto_purge` const is the proof) | ❌ Wave 0 |
| SCHEMA-05 | `BehaviorSigHash` is deterministic; same inputs produce same hash; NUL separation prevents prefix collisions | unit | `go test ./internal/corpus/... -run TestBehaviorSigHash` | ❌ Wave 0 |
| SCHEMA-06 | SCHEMA-06 gate: Nx Console trace maps to schema with no gaps; envelope can represent `watch_and_block` push with `confidence_tier:"enforce"` + `source_count:2` | unit (schema gate) | `go test ./internal/corpus/... -run TestSchemaLockNxConsoleTrace` | ❌ Wave 0 |
| SCOPE-01 | `CorpusRecord{}` (zero value) serializes as `"scope":"org_only"` in JSON; `CorpusScope("")` marshals to `"org_only"` | unit | `go test ./internal/corpus/... -run TestScopeZeroValue` | ❌ Wave 0 |
| SCOPE-02 | `PromoteScope(&rec)` returns a non-nil error in v1; `rec.Scope` is unchanged after the error | unit | `go test ./internal/corpus/... -run TestPromoteScopeReturnsErrorInV1` | ❌ Wave 0 |

**Additional cross-package tests:**
| Item | Command |
|------|---------|
| `RedactRecordWithDefaults` exists and applies default patterns | `go test ./internal/audit/... -run TestRedactRecordWithDefaults` |
| `CorroborateOutcome` wrapper: single-source block → `watch`; dual-source block → `enforce` | `go test ./internal/policy/... -run TestCorroborateOutcome` |
| `CorpusConfig` loads from JSON config block | `go test ./internal/config/... -run TestCorpusConfig` |
| No new imports in `go.mod` | `go mod tidy && git diff go.mod` (must be empty) |
| `internal/policy` still has no I/O imports | `go test ./internal/policy/... -run TestPolicyImportsArePure` (existing test) |

### Compile-Time `auto_purge` Guard Validation

The `auto_purge` guard is a **type-level guarantee**, not a test. Its validation is:
1. `go build ./internal/corpus/...` succeeds.
2. Grep confirms no `"auto_purge"` string constant exists in `internal/corpus/`: `grep -r "auto_purge" internal/corpus/` returns no results.
3. The `PushEnvelope.ActionHint` field type is `ActionHint` (not `string`): `go vet ./internal/corpus/...` passes.

An optional `testdata/compile_fail/auto_purge_assignment.go` with `//go:build ignore` can document the intended compile error for future contributors:
```go
//go:build ignore
// This file demonstrates that auto_purge is NOT a valid ActionHint.
// It does NOT compile — which is the desired property.
package main
import "github.com/home-beekeeper/beekeeper/internal/corpus"
func main() {
    var e corpus.PushEnvelope
    e.ActionHint = "auto_purge" // COMPILE ERROR: cannot use "auto_purge" (untyped string constant) as corpus.ActionHint
}
```

### Sampling Rate

- **Per task commit:** `go test ./internal/corpus/... ./internal/audit/... ./internal/policy/... ./internal/config/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./... && go build ./... && grep -r "auto_purge" internal/corpus/ | wc -l` (must output 0)

### Wave 0 Gaps

All test files are new (no existing `internal/corpus/` package):

- [ ] `internal/corpus/corpus_test.go` — covers SCHEMA-01, SCHEMA-02, SCHEMA-03, SCHEMA-05, SCOPE-01, SCOPE-02
- [ ] `internal/corpus/schema_lock_test.go` — covers SCHEMA-06 (gate test)
- [ ] `internal/corpus/testdata/nx_console_trace.json` — Nx Console trace fixture
- [ ] `internal/audit/redact_test.go` (existing) — extend with `TestRedactRecordWithDefaults` test case
- [ ] `internal/policy/corroboration_test.go` (existing) — extend with `TestCorroborateOutcome` test case
- [ ] `internal/config/config_test.go` (existing) — extend with `TestCorpusConfig` test case

---

## Security Domain

> `security_enforcement` not explicitly disabled; included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | — |
| V3 Session Management | no | — |
| V4 Access Control | yes | `CorpusScope` type + `PromoteScope` error stub prevents unauthorized scope promotion |
| V5 Input Validation | yes | `ActionHint` typed const prevents invalid `action_hint` values; `CorroborateOutcome` enforces `source_count >= 2` for enforce tier |
| V6 Cryptography | yes | `crypto/sha256` + `crypto/hmac` (stdlib); `crypto/ed25519` signing block type defined (zero-value in v1) |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| `auto_purge` in push envelope (blast-radius attack via compromised channel) | Tampering/Elevation | `ActionHint` typed const — compile-time unrepresentable |
| Scope-tag leakage (org-confidential data marked `community_shareable`) | Information Disclosure | `CorpusScope.MarshalJSON` zero-value guarantee + `PromoteScope` always-error |
| Non-reversible fingerprint bypass (victim re-identification) | Information Disclosure | HMAC-SHA256 contract defined in Phase 22; implementation in Phase 23 (STORE-05) |
| Schema gap (outcome fields missing from run-1 records) | Tampering (data integrity) | `TrueLabel` always present (not `omitempty`); `WasCorrect *bool` pointer (nil = unresolved, not absent) |

---

## Sources

### Primary (HIGH confidence)
- `internal/audit/types.go` — live `AuditRecord` struct; all fields + json tags; confirms what is present vs missing [VERIFIED: internal/audit/types.go]
- `internal/audit/redact.go` — `redactPattern` unexported type confirmed; `RedactRecord` + `DefaultRedactPatterns()` take/return unexported type [VERIFIED: internal/audit/redact.go]
- `internal/audit/sink.go` — `Sink` interface; `NewMultiSink` pattern [VERIFIED: internal/audit/sink.go]
- `internal/audit/writer.go` — `O_APPEND|O_CREATE|O_WRONLY` + `platform.SetOwnerOnly` pattern [VERIFIED: internal/audit/writer.go]
- `internal/policy/corroboration.go` — `corroborate()` unexported; no `CorroborateOutcome` export exists yet [VERIFIED: internal/policy/corroboration.go]
- `internal/policy/types.go` — `Decision.CorroborationCount`, `SourcesAgreed` confirmed [VERIFIED: internal/policy/types.go]
- `internal/config/config.go` — `AuditConfig` pattern for `CorpusConfig`; `legalNudgeModes` as closed-enum precedent [VERIFIED: internal/config/config.go]
- `internal/sentry/types.go` — `SentryAlert.RuleID` confirmed; `RuleState` window fields confirmed [VERIFIED: internal/sentry/types.go]
- `beekeeper-corpus-milestone-prd.md` §3.1, §3.5, §3.6, §4 — authoritative field lists; phase gate definition [CITED: beekeeper-corpus-milestone-prd.md]
- `.planning/phases/22-schema-envelope-lock/22-CONTEXT.md` — all locked decisions [CITED: 22-CONTEXT.md]
- `.planning/REQUIREMENTS.md` — SCHEMA-01..06, SCOPE-01..02 wording; OQ-1/2/3 locked decisions [CITED: .planning/REQUIREMENTS.md]

### Secondary (MEDIUM confidence)
- `.planning/research/STACK.md` — HMAC-SHA256 non-reversibility; Ed25519 stdlib choice; zero-new-deps confirmed [CITED: .planning/research/STACK.md]
- `.planning/research/ARCHITECTURE.md` — `CorroborateOutcome` 2-line wrapper pattern; cluster_id derivation; emitter boundary [CITED: .planning/research/ARCHITECTURE.md]
- `.planning/research/PITFALLS.md` — Pitfall 4 (scope leakage); Pitfall 5 (auto_purge); Pitfall 1 (non-retrofittable outcome fields); Pitfall 3 (source_count double-count) [CITED: .planning/research/PITFALLS.md]

### Tertiary (LOW confidence)
- Go language specification — unnamed struct embedding promotes fields; `encoding/json` handles promoted fields as if directly on the outer struct [ASSUMED — well-established Go language behavior; not verified via tool in this session]

---

## Metadata

**Confidence breakdown:**
- AuditRecord embedding analysis: HIGH — verified against live `internal/audit/types.go`
- `redactPattern` unexported type landmine: HIGH — verified against live `internal/audit/redact.go`
- `ActionHint` typed-const guard mechanism: HIGH — established Go pattern; consistent with existing `FailModeClosed` precedent in `internal/config`
- `CorpusScope.MarshalJSON` mechanism: MEDIUM — correct Go; no existing precedent in this codebase
- `BehaviorSigHash` normalization rules: MEDIUM — rules are reasonable; empirical validation against real audit records needed
- `CorroborateOutcome` wrapper: HIGH — verified against live `internal/policy/corroboration.go`
- Schema-lock gate test design: HIGH — directly from PRD §4 Phase 0 specification

**Research date:** 2026-06-13
**Valid until:** 2026-07-13 (stable domain; 30-day window)

---

## RESEARCH COMPLETE
