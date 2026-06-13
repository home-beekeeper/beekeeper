---
created: 2026-06-13
title: Fix behavior_signature_hash IPv6 bare-address normalization (needs CorpusSchemaVersion bump)
area: internal/corpus
files:
  - internal/corpus/behavior_sig.go
  - internal/corpus/behavior_sig_test.go
---

# Fix `normalizeNetworkDest` IPv6 bare-address handling

**Problem:** The Phase-22 FROZEN `behavior_sig.go` `normalizeNetworkDest` treats any trailing `:<digits>` as a port, so a bare IPv6 address like `::1` normalizes to `::` (and e.g. `fe80::1` -> `fe80:`). It only affects which bucket `BehaviorSigHash` maps a record into (never a correctness or security property), and the realistic corpus destinations are FQDN / registry / dead-drop hosts, so it was **accepted as-is at the Phase 22 schema-freeze sign-off (2026-06-13)**.

**Why deferred (not fixed at freeze):** the maintainer chose to freeze the schema as-is and resurface this later. The normalization is a FROZEN hash input — changing it is a BREAKING change (records hashed before vs after won't match across the corpus), so it MUST ride a **`CorpusSchemaVersion` bump**, never a silent patch.

**Solution (future):** make `normalizeNetworkDest` bracket-aware — parse `[host]:port` and bare-IPv6 per RFC 3986 (`net.SplitHostPort` semantics; only strip a port when the host is bracketed, or a single trailing `:port` on a non-IPv6 host). Bump `CorpusSchemaVersion`. Add an IPv6 case to `TestBehaviorSigHash`. Re-confirm the SCHEMA-06 gate (`TestSchemaLockNxConsoleTrace`).

**Provenance:** `internal/corpus/behavior_sig.go` (~line 207, documented in code); `22-03-SUMMARY.md` Deviations; `22-VERIFICATION.md` `human_verification[2]` + `signed_off`. Accepted at the Phase 22 freeze sign-off 2026-06-13.
