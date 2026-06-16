---
phase: 22-schema-envelope-lock
verified: 2026-06-13T00:00:00Z
status: passed
score: 8/8 must-haves verified
overrides_applied: 0
signed_off: "2026-06-13 — maintainer approved the schema-freeze sign-off (PRD §4 Phase 0). Format FROZEN for Phase 23. The IPv6 normalizeNetworkDest quirk (::1 -> ::) was accepted as-is and documented to resurface for a future CorpusSchemaVersion bump (see .planning/todos/pending/corpus-behavior-sig-ipv6-normalization.md)."
human_verification:
  - test: "Schema-freeze sign-off: review internal/corpus types, PushEnvelope fields, and TestSchemaLockNxConsoleTrace output; confirm no field gaps and the format is frozen for Phase 23 to build on"
    expected: "Maintainer confirms the four-layer schema and envelope wire format are correct and may not be changed without a CorpusSchemaVersion bump"
    why_human: "PRD §4 Phase 0 explicitly defines this as a human gate: 'Sign-off freezes the format.' No automated test can substitute for the maintainer's domain judgment about schema completeness."
  - test: "IPv6 bare-address normalization quirk acknowledged: normalizeNetworkDest(\"::1\") strips the trailing \":1\" and produces \"::\" (since \"1\" is all ASCII digits). See behavior_sig.go:207. For the production corpus the only realistic destinations are FQDN attacker hosts (not bare IPv6 loopback), but if any future attack pattern uses a bare IPv6 C2 address the hash will mis-normalize it."
    expected: "Maintainer acknowledges this frozen behavior and confirms it is acceptable for the Phase 22 schema lock. If unacceptable, the normalization rule must be changed NOW (before the format freeze) — changing it after is a breaking schema change."
    why_human: "The normalizer is explicitly frozen in Phase 22 (any future change requires CorpusSchemaVersion bump). The behavior is documented in code but requires maintainer awareness before the freeze sign-off."
---

# Phase 22: Schema & Envelope Lock — Verification Report

**Phase Goal:** Freeze the four-layer event-record schema and push-envelope wire format as Go types before any record is written, with the `auto_purge`-unrepresentable compile-time guard and a schema-lock gate (Nx Console trace maps with no gaps; envelope can represent a `watch_and_block` push with `confidence_tier` + `source_count`). Sign-off freezes the format.

**Verified:** 2026-06-13
**Status:** human_needed (all 8 automated must-haves VERIFIED; 2 human items pending freeze sign-off)
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `CorpusRecord` embeds `audit.AuditRecord` via UNNAMED embedding; four layers present; outcome fields default to unresolved/nil | VERIFIED | `internal/corpus/types.go:37` — `audit.AuditRecord` field with no name, no json tag; `TrueLabel string json:"true_label"` (no omitempty); `WasCorrect *bool json:"was_correct,omitempty"`. TestCorpusRecordSchema PASS. |
| 2 | `ScanClusterID` is stable and deterministic; per-package key | VERIFIED | `internal/corpus/behavior_sig.go:84-92` — SHA-256 over three NUL-separated inputs, truncated to 16 hex chars. TestScanClusterID: 5/5 sub-tests PASS (stability, version-sensitivity, pkg-sensitivity, 16-char hex, NUL separation). |
| 3 | `PushEnvelope` JSON round-trips with all fields; signing block zero-value | VERIFIED | `internal/corpus/types.go:104-131`. TestPushEnvelopeRoundTrip: 2/2 sub-tests PASS (signing_nil, signing_zero_value). `Signing *SigningBlock json:"signing,omitempty"` is nil in v1. |
| 4 | `ActionHint` is a typed string with ONLY `ActionHintWatchAndBlock`; no `auto_purge` anywhere in `internal/corpus/`; `PushEnvelope.ActionHint` typed (not string) | VERIFIED | `internal/corpus/action_hint.go:22,36` — `type ActionHint string`, `const ActionHintWatchAndBlock ActionHint = "watch_and_block"`. `PushEnvelope.ActionHint ActionHint` (types.go:126). `grep -rc "auto_purge" internal/corpus/` = exit 1 (0 matches in all 9 files). |
| 5 | `BehaviorSigHash` deterministic over frozen normalized inputs; `ruleset_version` present | VERIFIED | `internal/corpus/behavior_sig.go:54-62` — SHA-256, NUL-separated, 64-char hex. Three frozen normalizers (normalizeActionType, normalizeTargetResource, normalizeNetworkDest). `RulesetVersion string json:"ruleset_version,omitempty"` on AuditRecord (types.go:107). TestBehaviorSigHash: 9/9 sub-tests PASS. |
| 6 | `TestSchemaLockNxConsoleTrace` passes — Nx Console trace maps with no gaps; envelope = watch_and_block / enforce / source_count:2 | VERIFIED | `go test ./internal/corpus/... -run TestSchemaLockNxConsoleTrace` = PASS. 18-field check with 6 documented conditional/Phase-23 skips. All non-skipped fields verified against fixture values. Envelope JSON asserts 8 required fragments. |
| 7 | `CorpusRecord{}` zero-value serializes `"scope":"org_only"` (CorpusScope.MarshalJSON) | VERIFIED | `internal/corpus/scope.go:37-42` — MarshalJSON returns `[]byte("\"org_only\"")` when `s == ""`. TestScopeZeroValue PASS. TestSchemaLockNxConsoleTrace Step 5 independently re-asserts this. |
| 8 | `PromoteScope` returns non-nil error in v1; scope unchanged | VERIFIED | `internal/corpus/scope.go:55-57` — returns `errors.New(...)` unconditionally, mutates nothing. TestPromoteScopeReturnsErrorInV1 PASS. |

**Score:** 8/8 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/corpus/types.go` | CorpusRecord (embeds AuditRecord) + PushEnvelope + EnvelopeSignature + IOCBlock + SigningBlock | VERIFIED | Exists, substantive (187 lines), wired — TestCorpusRecordSchema + TestPushEnvelopeRoundTrip import and exercise it. |
| `internal/corpus/action_hint.go` | `type ActionHint string` + `ActionHintWatchAndBlock` const | VERIFIED | Exists, substantive (37 lines), wired — PushEnvelope.ActionHint field typed ActionHint; gate test uses ActionHintWatchAndBlock. |
| `internal/corpus/scope.go` | `type CorpusScope` + `MarshalJSON` zero-value guard + `PromoteScope` always-error stub | VERIFIED | Exists, substantive (57 lines), wired — CorpusRecord.Scope and PushEnvelope.Scope both typed CorpusScope. |
| `internal/corpus/schema_version.go` | `CorpusSchemaVersion = "1.0"` const | VERIFIED | Exists, substantive (11 lines), wired — set on CorpusRecord in gate test; referenced in doc comments as the breaking-change bump trigger. |
| `internal/corpus/behavior_sig.go` | `BehaviorSigHash` + `ScanClusterID` + three frozen normalizers | VERIFIED | Exists, substantive (229 lines), wired — exercised in schema_lock_test.go Step 4. No os/net/filepath imports. |
| `internal/corpus/schema_lock_test.go` | `TestSchemaLockNxConsoleTrace` (SCHEMA-06 gate) | VERIFIED | Exists, substantive (413 lines), PASS. Loads testdata/nx_console_trace.json, constructs CorpusRecord, 18-field no-gaps check, PushEnvelope JSON assertions. |
| `internal/corpus/testdata/nx_console_trace.json` | Nx Console Sentry exfil fixture | VERIFIED | Exists, valid JSON. Contains SENTRY-005, watch_and_block, source_count_expected:2, all four layer sections. |
| `internal/audit/types.go` | SourceSurface, ClusterID, RulesetVersion additive omitempty fields | VERIFIED | Lines 97, 103, 107 — correct json tags, all omitempty. Decision field comment updated to include `alert`. |
| `internal/audit/redact.go` | `RedactRecordWithDefaults` exported with cross-package-safe signature | VERIFIED | Line 175 — `func RedactRecordWithDefaults(rec AuditRecord) AuditRecord`, body calls `RedactRecord(rec, DefaultRedactPatterns())`. TestRedactRecordWithDefaults: 3/3 sub-tests PASS. |
| `internal/policy/corroboration.go` | `CorroborationOutcome` + `CorroborateOutcome` wrapper; tier from count not level | VERIFIED | Lines 131-149 — tier = "enforce" only when `count >= t.BlockAt`, never from `level == "block"`. TestCorroborateOutcome: 3/3 PASS (enforce case, watch-from-critical-single-source, no-match). |
| `internal/config/config.go` | `CorpusConfig struct` + `Config.Corpus` field | VERIFIED | Lines 473, 571. TestCorpusConfig: 3/3 sub-tests PASS (JSON round-trip, missing key = zero value, community_shareable). |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `internal/corpus/types.go` | `audit.AuditRecord` | unnamed embedding (promotion) | VERIFIED | Line 37: `audit.AuditRecord` with no field name, no json tag. TestCorpusRecordSchema asserts `"source_surface":"sentry"` at top level and absence of `"AuditRecord"` key. |
| `internal/corpus/types.go` | `ActionHint` | `PushEnvelope.ActionHint` field typed `ActionHint` | VERIFIED | Line 126: `ActionHint ActionHint`. Not `string` — assigning an unrecognized string literal is a compile error. |
| `internal/corpus/types.go` | `CorpusScope` | `CorpusRecord.Scope` and `PushEnvelope.Scope` both typed `CorpusScope` | VERIFIED | Line 80 (CorpusRecord.Scope) and line 122 (PushEnvelope.Scope). Both `CorpusScope` typed, not `string`. |
| `internal/corpus/schema_lock_test.go` | `testdata/nx_console_trace.json` | `os.ReadFile("testdata/nx_console_trace.json")` | VERIFIED | Line 41: fixture loaded; JSON parsed to map; fixture values drive CorpusRecord construction. |
| `internal/corpus/schema_lock_test.go` | `PushEnvelope` + `ActionHintWatchAndBlock` | constructs watch_and_block envelope with enforce + source_count:2 | VERIFIED | Lines 341-356: PushEnvelope literal with ActionHintWatchAndBlock, ConfidenceTier:"enforce", SourceCount:2. |
| `internal/audit/redact.go` | `RedactRecord(rec, DefaultRedactPatterns())` | `RedactRecordWithDefaults` body | VERIFIED | Line 176: `return RedactRecord(rec, DefaultRedactPatterns())`. |
| `internal/policy/corroboration.go` | `corroborate(matches, t)` | `CorroborateOutcome` body | VERIFIED | Line 144: `_, _, count, _, _ := corroborate(matches, t)`. |

---

### Gate Command Results

| Command | Result | Notes |
|---------|--------|-------|
| `go build ./...` | EXIT:0 | All 27 packages build clean. |
| `go vet ./internal/corpus/...` | EXIT:0 | No vet warnings. |
| `go test ./internal/corpus/... -v` | EXIT:0, PASS | 14 BehaviorSigHash sub-tests + 5 ScanClusterID sub-tests + 5 corpus_test.go tests + TestSchemaLockNxConsoleTrace PASS. Total: all PASS. |
| `go test ./internal/audit/... -run TestRedactRecordWithDefaults` | EXIT:0, PASS | 3/3 sub-tests. |
| `go test ./internal/policy/... -run "TestCorroborateOutcome\|TestCorroborationImportsArePure"` | EXIT:0, PASS | 3/3 + purity check PASS. |
| `go test ./internal/config/... -run TestCorpusConfig` | EXIT:0, PASS | 3/3 sub-tests. |
| `go test ./...` | EXIT:0 | 27 packages (internal/version has no test files), 0 failures, 0 regressions. |
| `go mod tidy && git diff --exit-code go.mod` | EXIT:0 | Zero new dependencies. |
| `grep -rc "auto_purge" internal/corpus/` | EXIT:1 (0 matches) | 0 matches across all 9 files (action_hint.go, behavior_sig.go, behavior_sig_test.go, corpus_test.go, schema_lock_test.go, schema_version.go, scope.go, types.go, testdata/nx_console_trace.json). |
| `go list -f "{{.Imports}}" ./internal/corpus/` | [crypto/sha256 encoding/hex encoding/json errors github.com/home-beekeeper/beekeeper/internal/audit path strings] | Pure package: no os, net, path/filepath. |
| `go list -f "{{.Imports}}" ./internal/policy/` | [fmt github.com/home-beekeeper/beekeeper/internal/pkgparse math net/url regexp sort strings] | Note: policy package itself imports net/url (existing, not added by Phase 22). corroboration.go adds no new imports — verified by TestCorroborationImportsArePure PASS. |

---

### Per-Requirement Evidence Table

| Req ID | Must-Have | Status | Evidence in Code | Test |
|--------|-----------|--------|------------------|------|
| SCHEMA-01 | `CorpusRecord` embeds `audit.AuditRecord` (unnamed, promoted); four layers present; `TrueLabel` NOT omitempty; `WasCorrect *bool` | VERIFIED | `types.go:37` — `audit.AuditRecord` (unnamed). `TrueLabel string json:"true_label"` line 44, no omitempty. `WasCorrect *bool json:"was_correct,omitempty"` line 54. `SourceSurface`, `ClusterID`, `RulesetVersion` on AuditRecord (types.go:97,103,107 via Plan 01). | TestCorpusRecordSchema PASS |
| SCHEMA-02 | `ScanClusterID(pkg, version, fp)` stable, idempotent, version-sensitive, NUL-separated | VERIFIED | `behavior_sig.go:84-92` — hash(pkg + NUL + version + NUL + fp)[:16]. `ClusterID` on AuditRecord (types.go:103) for the promotion path. | TestScanClusterID 5/5 PASS |
| SCHEMA-03 | `PushEnvelope` JSON round-trips with all fields; `signing` nil in v1; `SigningBlock` type frozen | VERIFIED | `types.go:104-186` — all envelope, signature, IOC, and signing types defined. `Signing *SigningBlock json:"signing,omitempty"` (line 130). | TestPushEnvelopeRoundTrip 2/2 PASS |
| SCHEMA-04 | `ActionHint` typed string; single `ActionHintWatchAndBlock` const; no `auto_purge` token; `PushEnvelope.ActionHint` typed ActionHint | VERIFIED | `action_hint.go:22,36`. `types.go:126` — field typed `ActionHint`. grep -rc auto_purge = 0. | TestActionHintTypeSafety PASS; `go build` PASS |
| SCHEMA-05 | `BehaviorSigHash` deterministic; frozen normalization rules; NUL-separated; `ruleset_version` on every record | VERIFIED | `behavior_sig.go:54-62` (BehaviorSigHash). Frozen normalizers: `normalizeActionType` (path.Base+lowercase), `normalizeTargetResource` (home-prefix collapse, no runtime os.UserHomeDir), `normalizeNetworkDest` (port-strip+lowercase). `RulesetVersion` on AuditRecord (types.go:107). | TestBehaviorSigHash 9/9 PASS |
| SCHEMA-06 | `TestSchemaLockNxConsoleTrace` PASS; trace maps with no gaps; envelope = watch_and_block / enforce / source_count:2 | VERIFIED | `schema_lock_test.go` — 18-field fieldCheck table; 6 documented skips (agent_id, was_correct, adjudication_source, resolved_at, repo_fingerprint, fleet_node_id); all non-skipped fields pass exact-value checks. Envelope JSON asserts 8 required fragments including action_hint, confidence_tier, source_count. | TestSchemaLockNxConsoleTrace PASS |
| SCOPE-01 | `CorpusRecord{}` zero-value serializes `"scope":"org_only"` | VERIFIED | `scope.go:37-42` — MarshalJSON returns `"org_only"` when `s == ""`. | TestScopeZeroValue PASS; TestSchemaLockNxConsoleTrace Step 5 PASS |
| SCOPE-02 | `PromoteScope` always returns non-nil error in v1; scope unchanged | VERIFIED | `scope.go:55-57` — single `return errors.New(...)` statement, no mutation. | TestPromoteScopeReturnsErrorInV1 PASS |

---

### Phase-Boundary Fidelity Check

The CONTEXT.md boundary defines what Phase 22 must NOT include. Verified against the actual codebase:

| Boundary Item | Checked | Result |
|---------------|---------|--------|
| No corpus store / append-only writer | grep for BuildPushEnvelope, EmitCorpus, WriteCorpus in internal/corpus/ | PASS — only appears in doc comments as Phase 23 references |
| No adjudication engine / true_label assignment | grep for adjudication logic (non-doc) in internal/corpus/ | PASS — TrueLabel field defined; no assignment logic |
| No BuildPushEnvelope builder (Phase 23 ENV-02) | grep for "func BuildPushEnvelope" in internal/corpus/ | PASS — absent; mentioned in doc comments only |
| No HMAC population of repo_fingerprint/fleet_node_id | grep for "crypto/hmac" or HMAC calls in internal/corpus/*.go | PASS — fields defined with documented contract; no HMAC imports |
| No fuzz gate (Phase 23 ENV-03) | No fuzz test functions in internal/corpus/ | PASS — fuzz gate deferred |
| No existing-surface decision-behavior change | AuditRecord additions are additive omitempty; FromDecision unchanged | PASS — FromDecision body unchanged (verified in types.go:144-196); new fields are caller-set only |

---

### Anti-Patterns Found

| File | Pattern | Severity | Assessment |
|------|---------|----------|------------|
| `internal/corpus/types.go:92` | `PushEnvelope *PushEnvelope json:"push_envelope,omitempty"` — nil pointer until Phase 23 emitter | Info | Intentional Phase 22 boundary stub, documented in comment. Not a defect. |
| `internal/corpus/scope.go:55` | `PromoteScope` always returns error | Info | Intentional always-error stub per SCOPE-02 and the locked decision. The stub IS the deliverable. |
| `behavior_sig.go:207` | `"::1" → "::"` (bare IPv6 loopback normalizes to "::" via the port-strip rule) | Warning (non-blocking) | See Human Verification item 2 below. Documented in code; frozen behavior. Needs freeze sign-off awareness. |

No `TBD`, `FIXME`, or `XXX` markers found in Phase 22 modified files.

---

### Human Verification Required

#### 1. Schema-Freeze Sign-Off (PRD §4 Phase 0 Gate)

**Test:** Review `internal/corpus/types.go` (CorpusRecord + PushEnvelope + supporting types), `internal/corpus/action_hint.go`, `internal/corpus/scope.go`, and the output of `go test ./internal/corpus/... -run TestSchemaLockNxConsoleTrace -v`. Confirm the four-layer schema and the envelope wire format are correct and complete.

**Expected:** Maintainer confirms: (a) the four-layer field set matches PRD §3.1 intent; (b) the 6 documented skips (agent_id, was_correct, adjudication_source, resolved_at, repo_fingerprint, fleet_node_id) are acceptable Phase 23 deferrals; (c) the format is frozen — no field may be added, removed, or renamed after this without bumping CorpusSchemaVersion and writing a migration plan.

**Why human:** PRD §4 Phase 0 explicitly requires a human sign-off to freeze the format. Automated tests prove the schema compiles and the gate passes, but cannot substitute for the maintainer's domain judgment about whether the field set is complete and correct before Phase 23 builds the store on top of it.

#### 2. `normalizeNetworkDest` IPv6 Quirk — Freeze Sign-Off Awareness

**Test:** Note that `normalizeNetworkDest("::1")` produces `"::"` (the function strips the trailing `:1` because `"1"` satisfies the all-digits guard). This behavior is documented in `behavior_sig.go:207` and in the 22-03-SUMMARY.md Deviations section.

**Expected:** Maintainer confirms this is acceptable for the Phase 22 freeze. The realistic corpus network destinations are attacker FQDN/IP hosts (not bare IPv6 loopback addresses), so this edge case has no practical impact on corpus fingerprinting accuracy. Bracket-form IPv6 with a port (`[::1]:443`) normalizes correctly to `[::1]`.

**Why human:** The normalization rules are explicitly frozen in Phase 22 (behavior_sig.go package-level doc: "Changing them is a breaking schema change"). If this behavior is unacceptable, the rule must be revised NOW before the freeze. After the freeze, any change requires a CorpusSchemaVersion bump and a migration plan for existing corpus records.

---

### Gaps Summary

None. All 8 automated must-haves are VERIFIED. The two human verification items are freeze sign-off gates required by the PRD, not defects or missing implementations.

---

_Verified: 2026-06-13_
_Verifier: Claude (gsd-verifier)_
