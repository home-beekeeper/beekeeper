# Phase 22: Schema & Envelope Lock - Context

**Gathered:** 2026-06-13
**Status:** Ready for planning
**Source:** Generated from `beekeeper-corpus-milestone-prd.md` §3 + the milestone Locked Decisions (OQ-1/2/3) + `.planning/research/`. (No separate discuss-phase; the key decisions were locked during milestone setup.)
**Requirement IDs (every one MUST be covered):** SCHEMA-01, SCHEMA-02, SCHEMA-03, SCHEMA-04, SCHEMA-05, SCHEMA-06, SCOPE-01, SCOPE-02

<domain>
## Phase Boundary

Phase 22 freezes the **non-retrofittable foundation**: the four-layer event-record schema and the push-envelope wire format, as Go types, before any corpus record is ever written. The moat depends on capturing the outcome + corroboration fields from the first run, so the format must be right and frozen here.

**This phase is type-definitions + invariant guards + a schema-lock gate. It does NOT:**
- write or persist any corpus record (the append-only store is Phase 23),
- run any adjudication / assign any `true_label` (Phase 23),
- emit, transmit, or sign anything (no transport in v1; signing block stays zero-value),
- touch First Responder / TUI (Phase 24),
- change the behavior of any existing surface (`hook`/`gateway`/`shim`/`watch`/`sentry`/`scan`). Adding the additive `source_surface` field to the base record is allowed; altering decision logic is not.

**Deliverable:** the typed schema + envelope + the compile-time `action_hint` guard + a **schema-lock evaluator gate** (a test proving the Nx Console worked trace maps to schema fields with no gaps, and that the envelope can represent a `watch_and_block` push carrying `confidence_tier` + `source_count`). Sign-off freezes the format.

</domain>

<decisions>
## Implementation Decisions (LOCKED)

### Package & boundary
- New package `internal/corpus` holds the schema types, the envelope types, the `ActionHint` constant, and a pure constructor/validator. In Phase 22 this package is pure (types + a compile-time guard + pure validation) — the impure store/engine arrive in Phase 23.
- `internal/policy` / `internal/nudge` / `internal/pkgparse` stay I/O-free; do not add anything to them.
- `CorpusRecord` **embeds the existing `internal/audit` `AuditRecord`** (which already carries the behavior + decision layers) and adds the **outcome** and **context** layers as new fields. Do not duplicate behavior/decision fields. (research/STACK.md, research/ARCHITECTURE.md)

### Four-layer schema (SCHEMA-01, SCHEMA-02) — PRD §3.1
- Layers: **behavior** (source_surface, action_type, actor_lineage, target_resource, network_destination, agent_id*), **decision** (verdict, policy_matched, rule_id*, correlation_window*, confidence, ruleset_version), **outcome** (was_correct, true_label, adjudication_source, resolved_at), **context** (cluster_id, baseline_deviation, repo_fingerprint, fleet_node_id, scope). `*` = conditional, populated only for the relevant `source_surface`.
- `source_surface` is an **additive** branch-key field on the base record with six values: `hook | mcp_gateway | shim | file_watcher | sentry | scan`.
- Outcome-layer fields are present as **`unresolved` placeholders from the first write** (`true_label: "unresolved"`, empty `resolved_at`) — non-retrofittable, so the slots exist now even though Phase 23 populates them.
- `cluster_id` binds correlated events. Agent-mediated surfaces (`hook`/`mcp_gateway`/`shim`) adjudicate **per event**; non-agent surfaces (`file_watcher`/`sentry`/`scan`) adjudicate **per cluster**. **Scan cluster_id (OQ-2 locked):** per-package-hit with a **stable key** `cluster_id = hash(package_or_extension_id + version + repo_fingerprint)`, so re-scans are idempotent and each flagged package is independently adjudicable.

### Push-envelope format (SCHEMA-03, SCHEMA-04) — PRD §3.5
- Envelope = `signature{package_or_extension_id, version, behavior_signature_hash, iocs{domains, dns_tunnel_pattern, dead_drop_pattern}}`, `true_label`, `confidence_tier` (`watch|enforce`), `source_count`, `scope`, `action_hint`, `signing{issuer, signature, issued_at, nonce}` (signing zero-value in v1, populated only when transport exists).
- **`action_hint` is a typed constant whose ONLY pushable value is `watch_and_block`.** `auto_purge` must be **unrepresentable** in a pushable envelope — enforce this at compile time (e.g. an unexported/closed type with no `auto_purge` member, or a builder that cannot emit it), NOT a runtime string check. This is the blast-radius guard baked in at format-freeze. (research/PITFALLS.md item 5)

### Behavior signature + versioning (SCHEMA-05)
- `behavior_signature_hash` input is **frozen** in this phase: a hash over `action_type` + normalized `target_resource` + normalized `network_destination`. Changing it later is a breaking schema change, so the normalization rules must be defined and documented now.
- Record `ruleset_version` on every record so later schema/rule evolution is detectable.

### Context-layer fingerprint fields (SCHEMA-01 defines; STORE-05 populates in 23)
- `repo_fingerprint` and `fleet_node_id` are defined as fields in the context layer **now**, with the documented contract that they are **non-reversible** (`HMAC-SHA256` over the canonical id with a per-install secret). Phase 22 defines the fields + the non-reversibility contract; the actual HMAC population/per-install-secret wiring is **STORE-05 (Phase 23)** — do not pull the secret-generation I/O into Phase 22.

### Scope tagging (SCOPE-01, SCOPE-02) — PRD §3.6
- Every record carries `scope` ∈ {`org_only`, `community_shareable`}; the Go **zero-value resolves to `org_only`** (the safe default — never leave it unset such that it could serialize as community_shareable).
- The only code path that changes `scope` is a `PromoteScope`-style function that **always returns an error in v1** (real promotion + anonymization is v2.0). No automatic promotion.

### Schema-lock evaluator gate (SCHEMA-06) — PRD §4 Phase 0 gate
- A test asserts every field in the **Nx Console worked trace** maps to a schema field with no gaps, and that the envelope can represent a `watch_and_block` push carrying `confidence_tier` + `source_count`. This gate is the phase's definition of done; passing it "freezes the format."

### Stack (research/STACK.md — HIGH confidence)
- Zero new `go.mod` dependencies. Signing block uses `crypto/ed25519` (stdlib) when populated later; fingerprints use `crypto/hmac` + `crypto/sha256`. Phase 22 only needs the type defs + `encoding/json` tags + the constant guard + the gate test.

## Claude's Discretion
- Exact Go struct + field names, json tags, and file layout within `internal/corpus` (e.g. `schema.go`, `envelope.go`, `action_hint.go`).
- The precise mechanism for the `auto_purge`-unrepresentable guard (closed type vs builder-returns-error) — pick the one the plan-checker will agree is genuinely compile-time-safe.
- Representation of conditional per-`source_surface` fields (pointer fields, `omitempty`, or a sub-struct per surface).
- The Nx Console trace fixture format and where it lives (testdata).
- Whether `repo_fingerprint`/`fleet_node_id` get a documented stub/helper signature in 22 (definition only) vs left purely as fields.
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Authoritative scope
- `beekeeper-corpus-milestone-prd.md` §3.1 (four-layer schema), §3.5 (push-envelope format), §3.6 (scope tagging), §4 Phase 0 (the schema-lock gate)
- `.planning/REQUIREMENTS.md` — SCHEMA-01..06 + SCOPE-01..02 wording + the Locked Decisions table (OQ-1/2/3)

### Research (code-grounded, HIGH confidence)
- `.planning/research/STACK.md` — CorpusRecord embeds AuditRecord; zero new deps; Ed25519 + HMAC-SHA256 stdlib choices
- `.planning/research/ARCHITECTURE.md` — `internal/corpus` package boundary; `ActionHint` compile-time constant; cluster_id derivation; embeds-not-replaces AuditRecord
- `.planning/research/FEATURES.md` — field tables per layer; `adjudication_source` confidence mapping; anti-feature guards
- `.planning/research/PITFALLS.md` — non-retrofittable schema (#1), scope-tag leakage (#4), auto_purge blast radius (#5), non-reversible fingerprints (#6)

### Existing code to mirror / respect
- `internal/audit/` — the `AuditRecord` struct to embed (read its fields + json tags); `RedactRecord`/`DefaultRedactPatterns` (relevant to Phase 23, but the schema must be redaction-compatible)
- `internal/sentry/` — `rule_id` (e.g. SENTRY-005), correlation windows, SENTRY-001..008 (source of the conditional decision-layer fields + cluster semantics)
- `CLAUDE.md` — pure-lib boundary, fail-closed, StateDir layout, single-static-binary / no-CGO constraints
</canonical_refs>

<specifics>
## Specific Ideas
- The **Nx Console worked trace** is the schema-lock gate fixture (SCHEMA-06): a trojanized VS Code extension exfiltrating repos. Every field that incident produces (editor-descendant process lineage, file reads, first outbound, the catalog/forensic confirmation) must map to a schema field with no gaps. Use it as the test that "freezes the format."
- The envelope must be demonstrably able to represent a `watch_and_block` push for that incident carrying `confidence_tier: enforce` + `source_count: 2`.
</specifics>

<deferred>
## Deferred Ideas (NOT Phase 22)
- Append-only corpus store + `RedactRecord` write path — Phase 23 (STORE-01..05)
- Adjudication engine, `true_label` assignment, corroboration-gated confidence, `downstream_clean` 30-day window — Phase 23 (ADJ-01..07)
- Envelope **emission** in-shape + the `BuildPushEnvelope` purge-rejection fuzz gate — Phase 23 (ENV-01..03) [the *type-level* `auto_purge`-unrepresentable guard is in 22; the *builder/emit* path + fuzz gate are 23]
- HMAC population of `repo_fingerprint`/`fleet_node_id` + per-install secret generation — Phase 23 (STORE-05) [Phase 22 defines the fields + the non-reversibility contract only]
- First Responder corpus binding — Phase 24 (FRB-01..05)
- Any transport/signing-block population — out of v1 entirely
</deferred>

---

*Phase: 22-schema-envelope-lock*
*Context generated: 2026-06-13 from PRD §3 + Locked Decisions + research (no separate discuss-phase)*
