# Pitfalls Research

**Domain:** Adjudicated corpus + adjudication engine + push envelope — added to an existing local-first Go security harness (Beekeeper v1.4.0)
**Researched:** 2026-06-13
**Confidence:** HIGH (all claims grounded in existing codebase audit findings, PRD §3/§5/§8, and hard-won lessons from prior milestones)

---

## Critical Pitfalls

### Pitfall 1: Non-Retrofittable Schema — Outcome Fields Missing on Run 1

**What goes wrong:**
The `outcome` layer (`was_correct`, `true_label`, `adjudication_source`, `resolved_at`) and the corroboration/confidence fields (`source_count`, `confidence_tier`) are omitted from the initial corpus record because "adjudication happens later anyway." When users later want to label old incidents, there is no record to attach the label to — the moat thesis requires ground-truth attached to the original event. Discarded events cannot be relabelled retroactively. The whole proposition that "the outcome layer is the expensive and irreplaceable part" (PRD §1) collapses.

**Why it happens:**
Developers defer outcome fields to Phase 23 (adjudication engine) and forget that Phase 22 (schema lock) must emit them — even as `unresolved` placeholders — so the adjudication engine in Phase 23 can fill them in-place rather than re-scanning a lossy log. This is the canonical "design for it late, lose the data forever" mistake.

**How to avoid:**
- Phase 22 (schema and envelope lock) MUST emit all four layers from the first write, with `true_label: "unresolved"` and `adjudication_source: ""` as explicit placeholders, not omitted fields.
- The Phase 22 evaluator gate must assert: every field in the PRD §3.1 schema (all 18 named fields across behavior/decision/outcome/context layers) maps to a field in the written NDJSON record, including `was_correct`, `resolved_at`, `cluster_id`, `scope`, `repo_fingerprint`, `fleet_node_id`, and `source_count`. No gap. No conditional omission of outcome fields.
- The `push_envelope` format (PRD §3.5) must also be frozen in Phase 22 — `confidence_tier` and `source_count` are fields the adjudication engine fills, so their schema slots must exist in the first emitted record even if unpopulated until Phase 23.
- Treat the schema as append-only: adding optional fields in Phase 23+ is safe; removing or renaming fields already emitted requires a migration plan (there is none in v1 scope).

**Warning signs:**
- Phase 22 plan says "outcome layer will be wired up in Phase 23" — this is the anti-pattern. Phase 23 populates it; Phase 22 must define and emit the slot.
- Any corpus record in the evaluator gate that lacks `true_label` as a field (even if its value is `"unresolved"`) is a schema violation, not a normal pending state.
- The push envelope in Phase 22 does not carry `confidence_tier` — if the envelope test asserts only `signature.*` fields, the evaluator gate is insufficient.

**Phase to address:** Phase 22 (schema and envelope lock) is the ONLY phase that can prevent this. There is no recovery.

---

### Pitfall 2: Corpus Store Bypasses RedactRecord — Secrets Leak into NDJSON

**What goes wrong:**
The corpus store appends records to NDJSON without routing through `audit.RedactRecord` + `DefaultRedactPatterns`. Attacker-influenced fields — package names carrying credential-shaped strings, command-line arguments, network destinations, file paths — reach the corpus file raw. If the corpus is later promoted, exported, or its remote sink receives it (OTLP/syslog/HTTPS), secrets escape the machine.

**Why it happens:**
The corpus store is treated as a "new, separate thing" and the author wires up a fresh `os.OpenFile` + `json.Encoder` path without importing the existing `audit.RedactRecord` chokepoint. The existing audit log (F-1 finding from the 2026-06-12 security review) required remediation precisely because a new audit path bypassed redaction. The same mistake is easy to repeat with the corpus.

**How to avoid:**
- Every corpus write MUST call `audit.RedactRecord(record, audit.DefaultRedactPatterns)` before marshalling to NDJSON. There is no exception.
- The corpus store implementation in Phase 23 should reuse `internal/audit`'s write path — either call the existing `Sink.Write` interface with a corpus-typed record, or call `RedactRecord` explicitly before any disk/sink write.
- Phase 23 must include a test that writes a corpus record whose `behavior.target_resource` contains a string matching a `DefaultRedactPattern` (e.g., an AWS key-shaped string) and asserts the persisted NDJSON has the field redacted, not the raw value.
- The `internal/audit` package's redaction coverage gap (§8 of THREAT-MODEL.md: "Sentry-derived fields written verbatim") already applies to the audit log; the corpus store must not replicate this gap for the additional behavioral fields it introduces.

**Warning signs:**
- Corpus writer imports `os` and `encoding/json` directly without importing `internal/audit`.
- The evaluator gate for Phase 23 does not include a redaction assertion test.
- Corpus records in the test fixture contain the literal package name `evil-package-aws-key-AKIAIOSFODNN7EXAMPLE` unredacted.

**Phase to address:** Phase 23 (corpus store and adjudication engine). Must be in the first corpus write implementation, not a follow-on.

---

### Pitfall 3: source_count Double-Counting and Single-Source ENFORCE Weight

**What goes wrong:**
Two variants, both fatal to the 2FA corroboration invariant:

Variant A — Double-counting: The adjudication engine increments `source_count` once per catalog match event, but the same source (e.g., Bumblebee) can emit multiple match events for the same package in one correlation window (e.g., from the mmap index hit AND a catalog-delta re-scan). The engine counts both hits, increments `source_count` to 2, and marks the record `confidence_tier: "enforce"` from a single source. One compromised catalog source now drives enforce weight.

Variant B — First-match enforce: The engine uses the verdict from `internal/policy` (which already has `source_count` logic) to populate the corpus `confidence_tier`, but maps "block" to "enforce" without checking whether the block came from `critical`-severity single-source escalation (per-severity override). A single-source critical block is legitimate enforcement, but corpus records should not export it as `confidence_tier: "enforce"` with `source_count: 1` because downstream consumers (v1.1+ org aggregators) will interpret that as two-source corroboration.

**Why it happens:**
The adjudication engine is written fresh and re-implements source counting without reusing the existing `internal/catalog/corroboration.go` logic, or it reuses it but maps results to corpus fields with a lossy conversion.

**How to avoid:**
- The corpus `source_count` MUST be derived from the same `CorroborationResult.Sources` de-duplicated set that drives the policy engine's verdict, not re-counted from log events or match callbacks.
- De-duplicate by canonical source identifier: Bumblebee is one source regardless of how many entries it contributed in the correlation window. `source_count` = `len(unique source identifiers in the match set)`.
- `confidence_tier: "enforce"` requires `source_count >= 2`, always. A single-source block (whether from per-severity escalation or any other mechanism) MUST emit `confidence_tier: "watch"` in the corpus record, even if `beekeeper check` itself returned a block verdict. The corpus records the corroboration tier, not the enforcement action.
- Add a table-driven test: inputs of (sources=["bumblebee","bumblebee","bumblebee"], verdict=block) yields corpus record with `source_count: 1`, `confidence_tier: "watch"`. And (sources=["bumblebee","osv"], verdict=block) yields `source_count: 2`, `confidence_tier: "enforce"`.

**Warning signs:**
- `source_count` in a test fixture is 2 but the test input fed only one named source.
- The adjudication engine reads `CorroborationResult.Count` (an integer) instead of `len(CorroborationResult.Sources)` (a deduplicated set).
- A corpus record with `confidence_tier: "enforce"` has `source_count: 1`.

**Phase to address:** Phase 23 (adjudication engine). Gate the phase on a specific test: single-source-block yields watch-tier; dual-source-block yields enforce-tier.

---

### Pitfall 4: Scope-Tag Leakage — org_only Data Emitted as community_shareable

**What goes wrong:**
The default `scope` field is omitted or defaults to `""` in the schema, and the push envelope serializer falls back to `"community_shareable"` when the field is absent (or the promotion codepath has a logic inversion: `if scope != "org_only" { emit as community_shareable }`). When v1.1+ transport is added, data that should be org-scoped leaks to a community endpoint.

**Why it happens:**
Developers write `scope: scope || "community_shareable"` as a fallback thinking "unset means we haven't restricted it yet." The correct invariant is the opposite: unset means the most restrictive tier applies.

**How to avoid:**
- The Go struct for the corpus record must define `Scope` with a typed constant, not a raw string. The zero value of that type must compile to `org_only` behavior. Either use a `string` with an explicit sentinel check (`if r.Scope == "" { r.Scope = ScopeOrgOnly }`) enforced in the constructor, or use an `iota` where the zero value is `ScopeOrgOnly`.
- The envelope serializer must reject any envelope where `scope == ""` (or the zero value is not `org_only`). This is a hard error, not a fallback.
- Promotion to `community_shareable` requires an explicit call to a `PromoteScope(record)` function that (a) checks anonymization preconditions (v2.0 gate), (b) strips victim-identifying fields (`repo_fingerprint`, `fleet_node_id`, raw traces), and (c) sets `scope = "community_shareable"`. No other code path changes scope.
- In v1, `PromoteScope` always returns an error: `"community_shareable promotion requires anonymization gate (v2.0)"`. This makes the function real from day one but permanently gated until v2.0.
- Phase 22 evaluator gate must include a test: a freshly-constructed corpus record with no explicit scope set must serialize as `"org_only"`, not `""` or `"community_shareable"`.

**Warning signs:**
- `Scope string` in the struct without a constructor that enforces the default.
- Test fixture NDJSON files contain `"scope": ""` or `"scope": "community_shareable"` for records not explicitly promoted.
- No `PromoteScope` function exists, and scope is set by direct field assignment.

**Phase to address:** Phase 22 (schema lock — default must be enforced at struct construction time, before any serializer exists).

---

### Pitfall 5: Push Envelope Carries auto_purge or Other Destructive action_hint

**What goes wrong:**
The push envelope's `action_hint` field is populated by a switch over verdict values. The switch has a case for `"auto_purge"` or the default case emits the verdict as-is, so a `purge`-class verdict in the adjudication engine becomes `action_hint: "auto_purge"` in the serialized envelope. When v1.1+ transport wires up, org machines receive a push that signals them to auto-purge a package. This is the highest blast-radius mistake in the entire milestone.

**Why it happens:**
The `action_hint` mapping is written as a convenience translation and the PRD constraint ("only `watch_and_block` is pushable") is not encoded as a compile-time or test-time assertion, only as a comment.

**How to avoid:**
- Define a typed constant or enum for `ActionHint` with exactly one value in the pushable set: `ActionHintWatchAndBlock`. All other action intents (purge, restore, quarantine) are not represented as `ActionHint` values in the `PushEnvelope` type.
- The envelope construction function must have the signature `BuildPushEnvelope(record CorpusRecord) (PushEnvelope, error)`. Any record whose adjudication implies a destructive action returns an error: `"purge actions are not representable in push envelopes"`. The envelope is not emitted.
- The Phase 22 schema lock must specify the full allowed set for `action_hint` in the frozen format: `{watch_and_block}`. Any deviation is a schema violation, not a configuration option.
- Add a fuzz target in Phase 25 (launch readiness) that generates arbitrary `CorpusRecord` values and asserts that `BuildPushEnvelope` never returns an envelope with `action_hint` outside the allowed set.

**Warning signs:**
- `action_hint` is a `string` without a compile-time constraint on its values.
- Test fixture envelopes contain `"action_hint": "auto_purge"` or `"action_hint": "purge"`.
- The envelope builder copies `verdict` to `action_hint` with a direct string assignment.

**Phase to address:** Phase 22 (schema lock — the allowed-set constraint must be in the frozen format). Phase 25 (fuzz gate for envelope construction).

---

### Pitfall 6: Anonymization Identity Leakage — repo_fingerprint and fleet_node_id Reversible

**What goes wrong:**
`repo_fingerprint` is derived as `sha256(repo_path)` where `repo_path` is the absolute filesystem path (e.g., `/home/alice/work/secret-project`). Because absolute paths on developer machines are highly predictable (username + project name), the fingerprint is reversible by brute-force over a dictionary of common paths. Similarly, `fleet_node_id` is derived from `sha256(hostname)` or `sha256(mac_address)`, which are also small, predictable domains.

When v2.0 promotes a record to `community_shareable`, the fingerprint is included as "anonymized" but is trivially linked back to the victim organization's developer by hostname dictionary attack.

**Why it happens:**
Developers use a bare hash because "hashing makes it private." Hashing without a secret input or sufficient entropy is not anonymization — it is pseudonymization with a recoverable mapping.

**How to avoid:**
- `repo_fingerprint` must be `HMAC-SHA256(secret_key, canonical_repo_id)` where `secret_key` is a per-installation random 32-byte value stored in `~/.beekeeper/state.json` (generated on first run, never transmitted). The canonical repo identifier should be the git remote URL (normalized) or a UUID-form project ID, not the filesystem path.
- `fleet_node_id` must be `HMAC-SHA256(secret_key, hostname)` using the same per-installation key. The key is never in any corpus record or push envelope.
- Since the key is local-only and per-installation, the fingerprint is non-reversible externally: even if an attacker obtains the corpus record, they cannot determine the original repo path or hostname without the secret key.
- The Phase 22 schema lock must specify the derivation algorithm for both fields. The Phase 23 implementation must generate these fields using the locked algorithm, with a test that verifies: same path + same key yields same fingerprint (determinism); different keys yield different fingerprints for the same path (non-reversibility across installations).
- For v2.0 promotion: the `PromoteScope` function must assert that `repo_fingerprint` and `fleet_node_id` are present as HMAC-derived values and not raw path/hostname. The promotion gate is "anonymization-verified," not "trusting the field is already anonymized."

**Warning signs:**
- `repo_fingerprint` computation uses `sha256.Sum256([]byte(repoPath))` without a keyed HMAC.
- The `secret_key` is not generated or stored on first run.
- Two different Beekeeper installations with the same `repo_path` would produce the same `repo_fingerprint`.

**Phase to address:** Phase 22 (specify the derivation algorithm in the schema lock). Phase 23 (implement with HMAC, generate the per-install key, write the cross-installation non-reversibility test).

---

### Pitfall 7: Adjudication Blocks or Slows the Synchronous Hook Handler

**What goes wrong:**
The adjudication engine is called synchronously from `beekeeper check` before the hook exits. Since adjudication involves I/O (corpus store write, possible catalog lookups for `catalog_confirmation` adjudication source), the hook's sub-100ms target is violated. Worse, if the corpus store write fails (disk full, permission error), the hook handler fails closed — blocking the agent from any tool call until the disk issue is resolved. This is an availability-vs-security inversion: the corpus store (a recording system) should never affect the enforcement path (the blocking system).

**Why it happens:**
The adjudication engine is implemented as a function call from `internal/check/handler.go` in the same goroutine as the policy evaluation, rather than as a goroutine with a channel or a post-hook callback. It "seems natural" to do everything in one place.

**How to avoid:**
- The adjudication engine and corpus store write MUST be off the hot path. The hook handler fires the hook verdict (exit 0/1/2) FIRST, then queues the corpus record for async write.
- The correct pattern: after `RunCheck()` returns a verdict, the hook handler sends the `CorpusRecord` to a buffered channel (non-blocking, drop-on-full) and returns immediately. A separate goroutine drains the channel and writes to the corpus store.
- For `beekeeper check` (which is a short-lived process, not a daemon), the async goroutine must `sync.WaitGroup.Wait()` before the process exits — but this wait must have a hard deadline (e.g., 200ms) after which the corpus write is abandoned with a warning, not block forever.
- `internal/policy` remains pure (no I/O, no goroutines) per the architecture constraint in CLAUDE.md. The corpus record construction that reads policy-engine outputs may happen in the check handler, but disk I/O belongs in a separate layer.
- Phase 22 defines the corpus record struct. Phase 23's adjudication engine must be reviewed explicitly for any synchronous I/O call in the code path between `RunCheck()` returning and the hook exit.

**Warning signs:**
- `internal/check/handler.go` imports `internal/corpus` and calls `corpus.Write(record)` synchronously before `os.Exit`.
- Hook latency in tests increases by more than 5ms when a corpus record is being written.
- A corpus store write error causes `beekeeper check` to exit non-zero (other than its policy-derived exit code).

**Phase to address:** Phase 23 (adjudication engine design — async queue must be explicit in the implementation plan). Phase 25 (launch readiness gate must benchmark hook latency with corpus enabled and assert it stays under 100ms).

---

### Pitfall 8: Residual Gaps Treated as Solved — SENTRY-008 and GitHub API Dead-Drop

**What goes wrong:**
During Phase 25 (launch readiness), the documentation says "the eight Sentry patterns each produce a moat-grade record" and implicitly implies the corpus covers the full threat landscape. SENTRY-008 (CI runner OIDC theft from process memory) and GitHub API dead-drop exfil are NOT detectable by the corpus or any v1 mechanism — they are explicitly documented residual gaps in PRD §8 and the existing THREAT-MODEL.md §8. If Phase 25 documentation omits or downplays these gaps, users have false confidence that the adjudicated corpus resolves threats that are architecturally outside host-agent scope.

**Why it happens:**
Milestone pride: "we shipped a corpus that catches 8 Sentry patterns" is a compelling claim, and the temptation is to list all 8 without asterisks. The residual gaps are uncomfortable to document prominently.

**How to avoid:**
- The Phase 25 documentation and THREAT-MODEL.md update MUST include a named-residual-gaps section that explicitly states:
  - SENTRY-008 (CI runner OIDC theft): tokens extracted from runner process memory when no editor/agent is present on the CI host. The corpus captures what Sentry sees, but Sentry's scope is editor- and agent-CLI descendants only. CI runners outside that tree are unmonitored by construction. Mitigation is architectural (scoped and ephemeral tokens, OIDC trusted-publisher policy), not a corpus or Sentry rule. Named in the threat model; not a v1 feature.
  - GitHub API dead-drop exfil: a host-level tool (including the Sentry) cannot reliably distinguish a legitimate `git push` or GitHub API call from malicious repo creation or gist write using stolen secrets. The Sentry would see an outbound network connection, but the destination is `api.github.com` — a legitimate endpoint for virtually every developer machine. There is no behavioral signal that separates malicious API-dead-drop from normal developer activity at the host level. Architectural mitigation only (network egress policies, GitHub token scoping, audit logging at the GitHub organization level). Not a v1 corpus feature.
- The Phase 25 evaluator gate must include a documentation review checkpoint that verifies both gaps are named (not just mentioned in passing) in the shipped documentation.
- Do NOT add SENTRY-009 or SENTRY-010 to "solve" these gaps in v1.4.0 scope. That would be false precision: the events needed to detect these patterns (process memory reads, GitHub API intent classification) are not available to a host-level agent without kernel-mode access.

**Warning signs:**
- Phase 25 launch checklist says "corpus covers all 8 Sentry patterns" without a caveat.
- The THREAT-MODEL.md update for v1.4.0 removes or weakens the SENTRY-008/dead-drop residual gap language currently in §8.
- A new Sentry rule (SENTRY-009/010) appears in Phase 25 claiming to detect CI runner exfil or GitHub dead-drop.

**Phase to address:** Phase 25 (launch readiness — documentation gate must verify both gaps are named). Phase 23 (adjudication engine must not claim `catalog_confirmation` for SENTRY-008-class events).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Store `scope: ""` and fill it in later | Defer the default-enforcement decision | When v1.1+ transport lands, the fill-in logic is ambiguous — is empty "not yet tagged" or "intentionally unscoped"? Every consumer must special-case it | Never — the zero value must be `org_only` from Phase 22 |
| Derive `repo_fingerprint` from `sha256(path)` now, fix it in v2.0 | Simpler Phase 23 implementation | v1.0 corpus records have reversible fingerprints; v2.0 cannot re-derive them without the original paths, so the fix is a schema break or a silent security downgrade for old records | Never — HMAC derivation must be in Phase 23 from the first write |
| Adjudication engine calls `corpus.Write` synchronously | No goroutine complexity | Hook latency degrades; corpus write errors block the enforcement path; the test suite becomes slow | Never — async queue is required from Phase 23 |
| `action_hint` as a raw `string` field | Flexible, easy to add new values | Any future code touching the field can accidentally set `auto_purge`; no compile-time guard | Never — typed constant from Phase 22 |
| Corpus store reuses audit log NDJSON file (single file) | No new file to manage | Corpus records and audit records become interleaved; downstream consumers cannot distinguish them; rotation policy applies to both without type discrimination | Acceptable only if corpus records carry a `record_type: "corpus"` discriminator AND audit tail/query commands filter by type |
| Skip RedactRecord on corpus writes "since corpus is local-only in v1" | Simpler implementation | When v1.1+ fan-out or OTLP sink is added, secrets are already persisted in the corpus file and cannot be retroactively redacted | Never — RedactRecord is mandatory on the first write |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `internal/catalog` corroboration results to corpus `source_count` | Copy `CorroborationResult.Count` (an integer that may include per-entry multiples) | Copy `len(CorroborationResult.Sources)` after deduplication by source name |
| `internal/audit` sink to corpus writes | Write corpus records directly to `os.File` bypassing sink | Route corpus records through the same `Sink` interface so OTLP/syslog/HTTPS fanout is inherited when enabled |
| First Responder quarantine TUI to corpus binding | Fire quarantine card on any `true_label == "malicious"` record | Gate on `true_label == "malicious" AND confidence_tier == "enforce"` — a watch-tier (single-source) malicious adjudication should not auto-arm a quarantine card |
| Push envelope serializer to `action_hint` mapping | Translate adjudication verdict directly to `action_hint` | Translate through the pushable-action allowlist: only `watch_and_block` is a valid value; anything else returns an error |
| `beekeeper check` (short-lived process) with async corpus write | Fire goroutine and `os.Exit` immediately | Fire goroutine + `sync.WaitGroup.Wait()` with a 200ms hard deadline before `os.Exit` |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Corpus store grows unbounded (no rotation) | Disk fills; `beekeeper check` hook latency degrades as append seeks grow | Reuse the audit log rotation policy; corpus file rotates on the same schedule | After ~30 days of continuous use on a busy agent workstation |
| Full corpus file parse on adjudication lookup | Phase 23 adjudication engine reads all corpus records to find the cluster_id match | Maintain an in-memory index (cluster_id to file offset) built on startup using the bounded tail approach (`recentAuditRecords` 512KB tail pattern from internal/tui) | After ~10,000 corpus records (~1MB NDJSON) |
| Corpus write inside the hook handler goroutine | Hook latency exceeds 100ms on slow disks; antivirus-intercepted writes on Windows | Async queue + bounded deadline as specified in Pitfall 7 | Immediately on any NFS mount or AV-intercepted write path on Windows |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| `community_shareable` as default scope | Org-confidential incident data (package names, behavior traces, cluster patterns) leaks to community endpoint in v1.1+ | `org_only` as compile-time zero value; promotion requires explicit `PromoteScope` call with anonymization gate |
| `auto_purge` in pushable envelope | A compromised org aggregator sends a push that purges a package from every fleet machine | `action_hint` typed constant with only `watch_and_block` in the allowed set; envelope construction rejects purge-class intents |
| Corpus store world-readable permissions | An attacker (or the agent itself) reads the corpus to learn what Beekeeper knows | `0600` owner-only on the corpus file, same as the audit log; self-protection (`selfprotect.go`) blocks agent reads of StateDir prefix which includes the corpus file |
| Unredacted secrets in corpus NDJSON | Credential-shaped strings in `behavior.target_resource` or `decision.reason` leak into corpus; if OTLP sink is enabled, secrets leave the machine | `audit.RedactRecord(record, audit.DefaultRedactPatterns)` called before every corpus write, no exceptions |
| `repo_fingerprint` and `fleet_node_id` derived from bare hash | Reversible by brute-force dictionary attack; victim identification possible even in "anonymized" community records | HMAC-SHA256 with per-install random secret key, generated on first run, never transmitted |
| Corpus file path outside StateDir | Self-protection (`selfprotect.go`) guards the StateDir prefix; a corpus path outside it is unguarded, writable by the agent | Corpus file lives under `platform.StateDir() + "/corpus/"` — same prefix as audit/, catalogs/, quarantine/ |
| Single-source adjudication marked `confidence_tier: "enforce"` | Downstream consumers (v1.1+ org aggregator) treat enforce-tier as two-source corroboration, bypassing the 2FA requirement | `confidence_tier: "enforce"` requires `source_count >= 2`; test that per-severity single-source block emits `watch` tier in corpus |

---

## "Looks Done But Isn't" Checklist

- [ ] **Schema completeness:** All four layers (behavior/decision/outcome/context) present in every corpus record — verify a synthetic NDJSON record has all 18 named fields from PRD §3.1, including `was_correct`, `true_label`, `resolved_at`, `cluster_id`, `scope`, `repo_fingerprint`, `fleet_node_id`, `source_count`
- [ ] **Redaction:** Corpus NDJSON does not contain unredacted credential patterns — verify by writing a record with a fake AWS-key-shaped `target_resource` and asserting the persisted value is redacted
- [ ] **Scope default:** A freshly-constructed `CorpusRecord{}` (zero value) serializes as `"scope": "org_only"` — verify in a unit test before any other logic runs
- [ ] **action_hint constraint:** `BuildPushEnvelope` called with a purge-class adjudication returns an error, not an envelope — verify with a negative unit test
- [ ] **source_count deduplication:** Feeding three Bumblebee match events into the adjudication engine results in `source_count: 1`, not `source_count: 3` — verify with a table-driven test
- [ ] **Off-hot-path:** `beekeeper check` with corpus enabled benchmarks at under 100ms — verify with `go test -bench=BenchmarkRunCheck -benchtime=10s` and confirm p99 is under 100ms
- [ ] **Corpus outside hook path:** A corpus write error (injected via test double) does not change the hook exit code — verify the hook exits with its policy-derived code even when the corpus write fails
- [ ] **HMAC fingerprint:** `repo_fingerprint` for the same repo path with two different install keys produces two different values — verify with a two-key unit test
- [ ] **Residual gaps documented:** THREAT-MODEL.md update includes named sections for SENTRY-008 CI runner OIDC and GitHub API dead-drop — verify the strings "SENTRY-008" and "dead-drop" appear in the Phase 25 threat model update
- [ ] **Envelope format frozen:** The `push_envelope` JSON shape emitted in Phase 22/23 matches the PRD §3.5 field list exactly — verify by deserializing a Phase 23 test fixture into the Phase 22-locked struct without errors

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Outcome fields missing from run-1 corpus records | HIGH — data loss, no recovery | Accept loss of pre-fix records; add a migration marker `"schema_version": 2` to post-fix records; downstream consumers skip v1 records for label-accuracy metrics |
| Secrets leaked into existing corpus NDJSON before redaction was added | HIGH — secrets on disk | Rotate any credentials that appeared in corpus; delete the pre-fix corpus file; re-derive `fleet_node_id` key if the key was in the leaked data |
| scope field emitted as `""` in existing records | MEDIUM — fix before v1.1 transport | One-time migration script: read corpus, set `scope = "org_only"` for any record with empty scope, rewrite file (append-only is a property of the live path, not a migration constraint); add `schema_version` bump |
| auto_purge found in existing envelope test fixtures | LOW — no transport yet, no real blast radius | Delete the fixture; add the negative test from Pitfall 5 to prevent recurrence; audit any serializer that writes `action_hint` directly |
| source_count double-counted in existing corpus records | MEDIUM — affects adjudication accuracy | Re-derive source_count from the raw match provenance stored in the record (if captured); if provenance not stored, mark affected records `confidence_tier: "unresolved"` and re-adjudicate |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Non-retrofittable schema (Pitfall 1) | Phase 22 | Evaluator gate: every PRD §3.1 field present in synthetic record, including `true_label: "unresolved"` placeholder |
| Corpus bypasses RedactRecord (Pitfall 2) | Phase 23 | Test: AWS-key-shaped `target_resource` yields redacted value in persisted NDJSON |
| source_count double-counting (Pitfall 3) | Phase 23 | Table test: 3x Bumblebee events yields `source_count: 1, confidence_tier: "watch"` |
| Scope-tag leakage default (Pitfall 4) | Phase 22 | Unit test: `CorpusRecord{}` serializes as `"scope": "org_only"` |
| auto_purge in push envelope (Pitfall 5) | Phase 22 (typed constant) + Phase 25 (fuzz gate) | Fuzz: `BuildPushEnvelope` never returns envelope with `action_hint` outside allowed set |
| Reversible fingerprint (Pitfall 6) | Phase 23 | Two-key unit test for non-reversibility of `repo_fingerprint` |
| Adjudication on hot path (Pitfall 7) | Phase 23 | Benchmark: `BenchmarkRunCheck` under 100ms p99 with corpus enabled; corpus error injection test |
| Residual gaps treated as solved (Pitfall 8) | Phase 25 | Documentation review gate: "SENTRY-008" and "dead-drop" present in THREAT-MODEL.md update |

---

## Residual Gaps — Document, Do Not Detect

These gaps must be named in Phase 25 launch documentation and THREAT-MODEL.md. They are not v1.4.0 deliverables.

**SENTRY-008 — CI Runner OIDC Token Theft:**
Tokens are extracted from runner process memory on a machine that typically has no editor, no agent, and no Beekeeper hook. Sentry's scope is editor- and agent-CLI descendants only. A CI runner process tree rooted outside those ancestry paths is unmonitored by construction. Detection would require: (a) a host agent on every CI runner (high deployment burden), (b) process-memory event sources (not available without kernel-mode access), and (c) OIDC token pattern recognition in memory reads (eBPF uprobe on Go runtime or similar, v3-scope work). Mitigation is architectural: ephemeral OIDC tokens with narrow scope, OpenID Connect trusted-publisher policy on the package registry (PyPI, npm), short token TTL, and audit logging at the CI platform level.

**GitHub API Dead-Drop Exfil:**
A stolen credential used to create a private repo or write a gist on `api.github.com` is indistinguishable at the host-network level from normal developer activity. Every developer machine makes legitimate HTTPS connections to `api.github.com`. The Sentry's SENTRY-003 (first-outbound phone-home) would not fire because `api.github.com` is in the expected outbound set for most agent workstations. SENTRY-007 (exfil fusion) requires a correlated file read + outbound pattern; a token extracted from environment variables (not a file) is not correlated. Mitigation is architectural: GitHub organization audit log, anomalous-new-repo alerts (GitHub Advanced Security), network egress allow-lists that block `api.github.com` from agent processes (hard in practice), and secret scanning on the GitHub side. Not a host-agent detection primitive.

**DNS-TXT Tunneling:**
DNS query events are ingested (eBPF kprobe on Linux, ETW DNS-Client on Windows) but no correlation rule consumes them (THREAT-MODEL.md §8, point 3). A DNS-TXT exfil channel emits many queries to an attacker-controlled subdomain. Detection requires a high-query-rate rule over the DNS event stream with domain entropy scoring. This is future Sentry work (v1.5+ scope), not a corpus-adjudication feature. Named here because corpus records of Sentry alert events will not include DNS-tunnel confidence signals in v1.4.0.

---

## Sources

- `beekeeper-corpus-milestone-prd.md` — §1 (moat thesis), §3 (v1 schema, envelope, scope), §5 (acceptance criteria), §8 (risks and residual gaps) — HIGH confidence (primary spec)
- `.planning/PROJECT.md` — current milestone scope, architecture constraints, enforcement-stays-local invariant — HIGH confidence
- `docs/THREAT-MODEL.md` — §8 (known gaps), §11 (CORR / source_count semantics, TM-B-01/TM-B-02), §12 (first-responder security review, F-1 RedactRecord finding, F-2/F-4/F-8 controls) — HIGH confidence (codebase-grounded)
- `CLAUDE.md` — architecture constraints (pure internal/policy, fail-closed, off-hot-path invariants) — HIGH confidence
- Prior milestone lessons: F-1 audit redaction finding (2026-06-12), source_count corroboration semantics (v1.2.0 CORR phases), async audit tail bounded-512KB pattern (v1.3.0 TUI de-mock), self-protection StateDir scope (v1.3.0 seed commits) — HIGH confidence (hard-won in this codebase)

---
*Pitfalls research for: Beekeeper v1.4.0 Adjudicated Corpus (Local Loop)*
*Researched: 2026-06-13*
