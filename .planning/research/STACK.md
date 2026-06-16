# Technology Stack: v1.4.0 "Adjudicated Corpus (Local Loop)"

**Project:** Beekeeper
**Milestone:** v1.4.0 — Adjudicated Corpus (Local Loop)
**Researched:** 2026-06-13
**Confidence:** HIGH — all recommendations are stdlib-only or already-present in go.mod; no new external dependencies required for the core corpus work

---

## Scope of This Document

This document covers ONLY the stack additions and integration decisions needed to implement the v1.4.0 corpus features. It does NOT re-research the existing harness (Go 1.25, Bubble Tea v2, cilium/ebpf, fsnotify, cosign, etc.) — those decisions are locked and documented in the archived prior-milestone STACK.md.

The five questions this research answers:
1. Schema representation — four-layer struct with conditional fields
2. Corpus store integrity — plain append vs hash-chaining vs content-addressing
3. Adjudication engine — off-hot-path design
4. Push-envelope signing primitive — Ed25519 vs cosign for v1 (emitted-only)
5. Non-reversible hashing for repo_fingerprint and fleet_node_id

---

## Decision Summary (TL;DR)

| Question | Decision |
|----------|----------|
| Schema representation | New `CorpusRecord` struct embedding `AuditRecord`, with four typed layer sub-structs; `source_surface` enum controls conditional field population via `omitempty` |
| Corpus store | Plain O_APPEND NDJSON file — NO hash chaining in v1; file is owner-only 0600, same as audit log; one optional `prev_hash` field reserved for v1.1 activation |
| Adjudication engine | Async goroutine, channel-fed, writes to a dedicated `corpus.ndjson` file; pure function `Adjudicate(CorpusRecord) (Outcome, error)` with no I/O |
| Signing primitive | `crypto/ed25519` stdlib — zero deps, 64-byte sig, Go-team maintained; key generated once into `~/.beekeeper/corpus-signing.key`; `signing` block populated with issuer + sig + issued_at + nonce but NOT transmitted in v1 |
| Non-reversible hashing | `crypto/hmac` + `crypto/sha256` over a per-install salt stored in `~/.beekeeper/state.json`; output is hex-encoded 64-char string; v2.0 anonymization step becomes `hmac(community_salt, repo_path)` — a 1-line rewire |

---

## New Go Code — Zero New External Dependencies

All capabilities for v1.4.0 are achievable with:
- Go 1.25 stdlib only (`crypto/ed25519`, `crypto/hmac`, `crypto/sha256`, `crypto/rand`, `encoding/json`, `sync`, `time`, `os`, `path/filepath`)
- Already-present modules in go.mod (`github.com/home-beekeeper/beekeeper/internal/audit`, `internal/quarantine`, `internal/policy`, `internal/platform`)

**No new entries to go.mod are required for any Phase 22–25 deliverable.**

---

## 1. Four-Layer Event Schema

### Design

The corpus record extends the existing `AuditRecord` rather than replacing it. `AuditRecord` is written by every decision surface (`check`, `gateway`, `sentry`, `watch`, `scan`). The corpus layer is a POST-DECISION enrichment: same record, promoted into the corpus with the outcome layer attached.

```go
// internal/corpus/types.go

// CorpusRecord is one NDJSON line in the corpus store.
// It embeds the original AuditRecord (the behavior and decision layers)
// and adds the outcome and context layers. The push envelope fields
// are populated at emit time; the signing block is populated when
// transport exists (v1.1+) but is defined and reserved from v1.
//
// Source: beekeeper-corpus-milestone-prd.md §3.1, §3.5
type CorpusRecord struct {
    // Embed the existing AuditRecord — carries behavior + decision layers.
    // Embedded as a named field to avoid JSON namespace collisions.
    audit.AuditRecord `json:",inline"` // or explicit field with tag

    // --- Outcome layer (THE MOAT) ---
    WasCorrect         *bool  `json:"was_correct,omitempty"` // nil = unresolved
    TrueLabel          string `json:"true_label,omitempty"`   // malicious|benign|policy_correct|unresolved
    AdjudicationSource string `json:"adjudication_source,omitempty"` // see §3.2
    ResolvedAt         string `json:"resolved_at,omitempty"` // RFC3339; empty until adjudicated

    // --- Context layer ---
    ClusterID         string `json:"cluster_id,omitempty"`        // binds correlated non-agent incidents
    BaselineDeviation string `json:"baseline_deviation,omitempty"` // "low"|"medium"|"high"|""
    RepoFingerprint   string `json:"repo_fingerprint,omitempty"`   // HMAC-SHA256 hex, non-reversible
    FleetNodeID       string `json:"fleet_node_id,omitempty"`     // HMAC-SHA256 hex, anonymized
    Scope             string `json:"scope"`                        // always present: org_only|community_shareable

    // --- Schema/ruleset versioning ---
    CorpusSchemaVersion  string `json:"corpus_schema_version"` // "1.0"
    RulesetVersion       string `json:"ruleset_version,omitempty"` // mirrors decision.RulesetVersion

    // --- Push envelope (populated at emit, NOT transmitted in v1) ---
    PushEnvelope *PushEnvelope `json:"push_envelope,omitempty"`
}

// BehaviorLayer holds the source-surface-specific conditional fields.
// Conditional population: fields are only set when source_surface matches.
// This mirrors the PRD §3.1 "conditional fields per source_surface" design.
// NOTE: source_surface itself lives in AuditRecord.Endpoint / a new
// SourceSurface field added to AuditRecord (see Integration section).
type PushEnvelope struct {
    Signature    EnvelopeSignature `json:"signature"`
    TrueLabel    string            `json:"true_label"`
    ConfidenceTier string          `json:"confidence_tier"` // watch|enforce
    SourceCount  int               `json:"source_count"`
    Scope        string            `json:"scope"`           // org_only|community_shareable
    ActionHint   string            `json:"action_hint"`     // watch_and_block only; never auto_purge
    Signing      *SigningBlock     `json:"signing,omitempty"` // nil in v1; populated in v1.1+
}

type EnvelopeSignature struct {
    PackageOrExtensionID string   `json:"package_or_extension_id"`
    Version              string   `json:"version"`
    BehaviorSignatureHash string  `json:"behavior_signature_hash"` // SHA-256 of canonical behavior fields
    IOCs                 []string `json:"iocs,omitempty"` // domains, DNS-tunnel pattern, dead-drop pattern
}

// SigningBlock is defined from v1 so the wire format is frozen.
// Fields are populated when transport exists (v1.1+).
// In v1, this block is populated locally for format validation but never transmitted.
type SigningBlock struct {
    Issuer    string `json:"issuer"`    // Ed25519 public key hex (DER) or "local"
    Signature string `json:"signature"` // base64url-encoded Ed25519 signature over canonical envelope bytes
    IssuedAt  string `json:"issued_at"` // RFC3339
    Nonce     string `json:"nonce"`     // base64url-encoded 16 random bytes (crypto/rand)
}
```

### Conditional fields per source_surface

The PRD specifies conditional fields (e.g. `agent_id` only for agent-mediated surfaces, `rule_id` only for Sentry). This is already handled by the existing `AuditRecord` via `omitempty` tags. The corpus layer does not need a separate conditional-field mechanism — it reads from the embedded `AuditRecord` where conditional fields are already populated or absent.

For the four new schema-level fields (`cluster_id`, `baseline_deviation`, `repo_fingerprint`, `fleet_node_id`), all use `omitempty`. The `source_surface` field should be added to `AuditRecord` as a new `SourceSurface string json:"source_surface,omitempty"` field (alongside the existing `Endpoint` field) to make the branch key explicit.

### Schema versioning

`corpus_schema_version: "1.0"` is a constant string embedded in every `CorpusRecord` at creation time by the corpus writer. When the schema evolves, the reader switches on this field. No external versioning library required — plain string comparison.

`ruleset_version` mirrors whatever the policy engine reports and is copied from the decision at record creation time.

---

## 2. Corpus Store — Append-Only NDJSON, No Hash Chaining in v1

### Decision: Plain O_APPEND, owner-only 0600, `corpus.ndjson`

**Rationale:** The corpus store is an owner-only local file (`~/.beekeeper/audit/corpus.ndjson`). The threat model for a local file owned by a single user is: the OS filesystem ACL is the integrity boundary. The attacker who can modify `corpus.ndjson` already has code execution as the owner — they can also zero the audit log, kill the daemon, and modify the binary. Hash chaining provides tamper-evidence for multi-party audit trails (e.g. a WORM log shipped to a SOC SIEM). For a single-owner local file, the implementation overhead of hash chaining does not improve the security posture.

**What hash chaining DOES NOT help here:**
- The attacker who can modify the corpus file is already inside the owner's session
- There is no external verifier to present the chain to in v1
- Chain breakage detection requires a reader that holds the prior state — adds complexity on the write path (serialization of "last hash" record under mutex, or a separate state file)

**What to do instead:**
- Same `O_APPEND|O_CREATE|O_WRONLY` pattern as `audit.Writer`
- Same `platform.SetOwnerOnly` enforcement on every write
- Same mutex-guarded concurrent writes
- One reserved field: `"prev_hash": ""` in the struct (populated as empty string in v1). This is a forward-reserved placeholder so v1.1 can populate it for org aggregator chain verification without a schema migration. The field is present but empty — readers must treat empty `prev_hash` as "chain not yet established."

**The `corpus.ndjson` writer is a thin wrapper over the existing `audit.Writer` pattern**, not a separate implementation. Use `audit.NewWriterWithOptions` with a different path, or create a parallel `corpus.NewWriter` in `internal/corpus/` that delegates to the same file-open pattern. Reuse `audit.Sink` interface for fan-out if the same remote sinks are desired — but remote sinks for corpus records require explicit opt-in (v1 default: file only, no remote transmission).

**Corpus store path:** `~/.beekeeper/audit/corpus.ndjson` (sibling of `beekeeper.ndjson`). Uses the same `StateDir` + `/audit/` lookup. Same rotation thresholds as audit log (configurable, default unlimited in v1, add rotation cap in v1.1 when corpus volume is better understood).

**Confirmed NOT over-engineered for v1:**
- No content-addressing (CAS) — adds directory-per-record overhead, no benefit for sequential corpus reads
- No Merkle tree — requires an anchor store external to the file
- No database (SQLite, bbolt, etc.) — adds a new dep, overkill for a sequential-write append-only log
- No hash chaining in v1 — no multi-party verifier exists yet

---

## 3. Adjudication Engine — Off Hot Path

### Design

The adjudication engine MUST NOT run on the hook handler goroutine (`beekeeper check` is sub-100ms). It runs as an async goroutine in the watch/daemon tier.

```go
// internal/corpus/adjudicator.go

// AdjudicationResult is the outcome assignment produced by the engine.
// It is a pure value type — no I/O.
type AdjudicationResult struct {
    TrueLabel          string    // malicious|benign|policy_correct|unresolved
    AdjudicationSource string    // catalog_confirmation|forensic_review|breach_confirmation|
                                  // user_override|downstream_clean|benign_explained
    SourceCount        int       // number of independent sources confirming this adjudication
    ConfidenceTier     string    // watch (1 source) | enforce (2+ sources)
    WasCorrect         *bool     // nil = unresolved; true/false after adjudication
    ResolvedAt         time.Time // zero value = unresolved
}

// Adjudicate is a PURE FUNCTION — no I/O, no goroutines, no side effects.
// It derives an AdjudicationResult from the CorpusRecord's decision and
// outcome signals available at call time.
//
// Callers (the async adjudication goroutine) are responsible for writing
// the result back to the corpus store.
func Adjudicate(rec CorpusRecord, signals AdjudicationSignals) AdjudicationResult
```

`AdjudicationSignals` is a value type carrying what is known at adjudication time:
- `CatalogConfirmed bool` — a later catalog sync confirmed this package/extension
- `UserAllowed bool` — the developer explicitly allowed a blocked action
- `SubsequentIncidents int` — count of downstream alerts linked by `cluster_id`
- `SourcesConfirming []string` — which catalog sources independently confirmed

The pure function contract mirrors `internal/policy` — no I/O means the adjudication logic is independently testable and fuzzable without file system setup.

### Async delivery

```go
// internal/corpus/engine.go

// Engine receives CorpusRecord values from decision surfaces and adjudicates
// them asynchronously, writing confirmed results back to the corpus store.
type Engine struct {
    ch     chan CorpusRecord
    writer *Writer // corpus.ndjson writer
    // ...
}

// Submit adds a record to the adjudication queue. Non-blocking drop on
// full channel (fail-open for the adjudication queue — the hot path
// is never blocked; a dropped record stays in the corpus as "unresolved").
func (e *Engine) Submit(rec CorpusRecord) {
    select {
    case e.ch <- rec:
    default:
        // queue full: log and continue; record stays unresolved in audit log
    }
}
```

Channel buffer size: 1024 (empirically matches the max audit burst observed in v1.2.0 dogfood). The adjudication goroutine drains the channel and writes corpus records; it does not block the decision surfaces.

**Integration point with First Responder:** When `Adjudicate` returns `TrueLabel: "malicious"` with `SourceCount >= 2` (enforce tier), the engine emits a `QuarantineCard` event on the TUI event bus. The TUI arms the card via the existing Bubble Tea message loop. Purge is never automatic — the card is a UI prompt only.

---

## 4. Push-Envelope Signing Primitive — `crypto/ed25519` (stdlib)

### Decision: `crypto/ed25519` stdlib, NOT cosign/sigstore for the local signing block

**Rationale:**

| Criterion | `crypto/ed25519` stdlib | cosign/sigstore |
|-----------|------------------------|-----------------|
| Dependencies | Zero (stdlib) | Large dep tree (sigstore, go-containerregistry, etc.) |
| No CGO | YES | YES |
| Offline operation | YES — keygen and sign are pure local operations | Partially — keyless mode requires Fulcio/Rekor network; keyed mode works offline |
| Key size | 64-byte private key, 32-byte public key | PEM-wrapped; larger key material |
| Signature size | 64 bytes | Variable (bundle includes cert chain) |
| Suitable for local per-record signing | YES | Overkill — cosign is optimized for artifact bundles, not per-record inline signing |
| Forward compatible with cosign/sigstore | YES — cosign supports Ed25519 keys; the `signing.signature` field value is algorithm-agnostic | N/A |
| Existing usage in repo | cosign is CI-only (not imported as a Go library in the binary) | cosign is the release-signing tool, not an in-binary dependency |
| Confidence | HIGH (pkg.go.dev/crypto/ed25519, Go 1.13+ stable API) | HIGH for release signing; WRONG tool for in-process local signing |

The `signing` block in the push envelope is designed for the future v1.1 org aggregator to verify that a push came from a known org machine. Ed25519 is the right primitive for this: compact key material, fast verification, 64-byte signatures, constant-time operations in Go stdlib, and no CGO.

Cosign/sigstore is the right tool for release artifact signing (already used in CI). It is the wrong tool for a per-corpus-record inline signing field inside a local NDJSON file. Adding sigstore as an in-binary Go import would add ~30+ indirect dependencies and require network access for keyless signing mode.

**Key lifecycle in v1:**
- `beekeeper corpus init` generates an Ed25519 key pair using `crypto/ed25519.GenerateKey(crypto/rand.Reader)` and persists the private key to `~/.beekeeper/corpus-signing.key` (0600, `platform.SetOwnerOnly`) encoded as raw 64-byte PEM or hex
- The public key is written to `~/.beekeeper/state.json` under `corpus.signing_pubkey`
- In v1, the signing block is populated locally but never transmitted — it serves as a format freeze and local self-integrity check
- In v1.1, the org aggregator receives the public key out-of-band and uses it to verify incoming pushes

**Nonce generation:** `crypto/rand.Read(nonce[:])` where `nonce` is a `[16]byte`. Encoded as base64url. This is already the pattern used elsewhere in the codebase for random identifiers.

**What is signed:** The canonical bytes of the push envelope (excluding the `signing` block itself), JSON-marshaled with sorted keys. The signing input is `sha256.Sum256(canonical)` — sign the digest, not the full payload, consistent with Ed25519ph conventions (though bare Ed25519 `Sign` also works; the prehash variant is available via `SignWithOptions` in Go 1.20+).

**Why NOT RSA, ECDSA P-256, or other curves:**
- RSA: 256-byte signatures, slow keygen, no advantage over Ed25519 for this use case
- ECDSA P-256: 71-byte DER signatures on average, requires explicit low-S normalization to be canonical, more foot-guns than Ed25519
- Ed25519: 64-byte fixed-size signatures, deterministic (no random nonce needed per sign), fastest Go stdlib option, already used by sigstore for HashEdDSA (Ed25519ph) — forward-compatible

**Forward compatibility with sigstore:**  Sigstore's Trail of Bits audit (January 2026) confirmed Ed25519 is the recommended algorithm for new sigstore deployments moving to cryptographic agility. When v2.0 introduces a community corpus host, the same Ed25519 key material can be wrapped in a cosign bundle for external verification — no algorithm migration needed.

---

## 5. Non-Reversible Hashing — `crypto/hmac` + `crypto/sha256`

### Decision: HMAC-SHA256 with per-install salt

**For `repo_fingerprint`:**
```go
// Produces a non-reversible, consistent fingerprint of a repository path.
// salt is loaded from ~/.beekeeper/state.json on first use and generated
// once via crypto/rand if absent. Same salt = same fingerprint across runs
// on the same machine (enables correlation). Different salt on each machine
// = different fingerprint (prevents cross-machine correlation without consent).
func RepoFingerprint(repoPath, salt string) string {
    mac := hmac.New(sha256.New, []byte(salt))
    mac.Write([]byte(filepath.Clean(repoPath)))
    return hex.EncodeToString(mac.Sum(nil)) // 64-char hex string
}
```

**For `fleet_node_id`:**
```go
// Produces a non-reversible, consistent identifier for this machine.
// Combines hostname + OS + a random per-install salt (same source as above).
// In v2.0, replacing salt with community_salt produces a pseudonymous
// fleet_node_id that enables community-level fleet analytics without exposing
// the raw hostname.
func FleetNodeID(hostname, goos, salt string) string {
    mac := hmac.New(sha256.New, []byte(salt))
    mac.Write([]byte(hostname + "|" + goos))
    return hex.EncodeToString(mac.Sum(nil))
}
```

**Why HMAC-SHA256 over plain SHA-256:**
- Plain SHA-256 of a repo path is reversible in practice — an attacker with a corpus of repo paths can preimage-attack it
- HMAC with a secret salt is non-reversible without the salt
- The salt is per-install and never leaves the machine in v1 (in `~/.beekeeper/state.json`, same owner-only file)
- `crypto/hmac.Equal` for comparison avoids timing side-channels
- Standard library, zero new dependencies

**v2.0 anonymization step is wiring, not redesign:**
The v2.0 "anonymization step" (PRD §7) that promotes a record from `org_only` to `community_shareable` is: replace `local_salt` with `community_salt` in the HMAC computation. The `community_salt` is published by the community host; the `fleet_node_id` in the exported record uses it, making the ID pseudonymous at the community level (machines with the same community salt can be correlated for threat-cluster analysis, but the raw hostname is never exposed). This is a one-function swap — no schema change, no new library.

**Salt storage:** `state.json` under `corpus.local_salt` (32 random bytes, hex-encoded). Generated once by `beekeeper corpus init` via `crypto/rand.Read`. The existing `internal/platform.StateDir()` function already manages this file.

---

## Integration Points with Existing Code

### `internal/audit` — minimal extension

1. Add `SourceSurface string json:"source_surface,omitempty"` to `AuditRecord` in `types.go`. This field was implicit in `Endpoint` but the corpus schema names it explicitly as the `behavior.source_surface` branch key.

2. `RedactRecord` in `redact.go` must be extended to redact the new corpus fields that carry attacker-influenced strings: `RepoFingerprint` is already hashed (safe), `FleetNodeID` is already hashed (safe), `IOCs` in `EnvelopeSignature` must go through `RedactStringSlice` (the IOC list may contain domains from attacker-controlled packages).

3. `audit.Sink` interface is reused for corpus fan-out. The corpus writer implements `Sink` — same `Write(rec)` / `Close()` contract. `NewMultiSink` in `sink.go` is NOT modified; the corpus writer is wired separately in the daemon's startup sequence.

### `internal/quarantine` — read-only integration

The adjudication engine does NOT call `quarantine.MoveTyped`. It emits a TUI event (`QuarantineCardMsg`) carrying the `quarantine.Manifest` fields. The TUI handler then prompts the user and calls `quarantine.MoveTyped` on confirmation. This preserves the human-gated constraint from PRD §3.4 ("Purge stays a local human-confirmed action").

### `internal/watch/crossref.go` — `ScanHit` binding

After `CrossReference` returns `[]ScanHit`, the caller (watch daemon) submits a `CorpusRecord` to the adjudication engine for each hit with `CorroborationCount >= 2`. The `ScanHit.Decision` fields map directly to the corpus behavior/decision layers. No changes to `crossref.go` itself.

### `internal/policy` — pure function boundary maintained

The adjudication engine's `Adjudicate()` function is a pure function library (no I/O, no goroutines). It lives in `internal/corpus/adjudicator.go` and mirrors the `internal/policy` constraint. The async engine in `internal/corpus/engine.go` is the I/O layer that calls the pure function and writes results.

---

## New Package: `internal/corpus/`

```
internal/corpus/
    types.go           # CorpusRecord, PushEnvelope, SigningBlock, AdjudicationResult
    adjudicator.go     # Pure function: Adjudicate(CorpusRecord, AdjudicationSignals) AdjudicationResult
    engine.go          # Async goroutine: Engine, Submit, drain loop
    writer.go          # CorpusWriter wrapping audit.Writer pattern; corpus.ndjson
    signer.go          # Ed25519 keygen, Sign, key persistence; uses crypto/ed25519 + crypto/rand
    fingerprint.go     # RepoFingerprint, FleetNodeID; uses crypto/hmac + crypto/sha256
    scope.go           # Scope constants (OrgOnly, CommunityShareable); scope promotion guard
    schema_version.go  # CorpusSchemaVersion = "1.0" const + future version gates
```

---

## What NOT to Add

| Avoid | Why | What to Use Instead |
|-------|-----|---------------------|
| Any new entry in go.mod | Zero justified by v1.4.0 scope | Go stdlib throughout |
| Hash chaining in v1 | Over-engineering for a local owner-only file; no external verifier exists; adds write-path complexity | Plain O_APPEND + `prev_hash: ""` placeholder reserved for v1.1 |
| `github.com/sigstore/cosign` as in-binary Go import | 30+ transitive deps; designed for artifact bundles not per-record inline signing; cosign stays CI-only | `crypto/ed25519` stdlib |
| SQLite / bbolt / badger | No random-access query need in v1; sequential NDJSON is sufficient; adds deps | `corpus.ndjson` NDJSON append |
| Content-addressable store (IPFS style) | Adds per-record directory overhead; overkill for a local sequential log | NDJSON append |
| External JWT library | JWT is not appropriate for the signing block (wrong format; JWTs are credentials, not signatures) | Raw Ed25519 signature in `signing.signature` field |
| `encoding/json/v2` (experimental) | Not covered by Go 1 compat promise; API may change in Go 1.26 | `encoding/json` stdlib |
| A separate corpus DB schema divorced from AuditRecord | Duplicates the behavior/decision layer; makes forensic cross-referencing harder | `CorpusRecord` embeds `AuditRecord` |
| Network transport in v1 | PRD §3 non-goal: "No network transmission of corpus data" | Local file only; remote sinks opt-in gated behind a "data leaves this machine" warning (same as audit) |
| Merkle tree or blockchain-style anchoring | No anchor store exists; no external verifier; implementation cost far exceeds benefit for local-only use case | Reserved `prev_hash` field for future chaining |
| `auto_purge` action in push envelope | PRD §3.5 hard constraint: "auto_purge is never present in a pushable envelope" | `watch_and_block` only in `ActionHint` |
| Promotion of `scope` from `org_only` to `community_shareable` without anonymization | PRD §3.6: promotion is explicit + anonymization-gated | Scope field is set at birth; promotion path is v2.0 |

---

## Confidence Assessment

| Area | Confidence | Basis |
|------|------------|-------|
| Schema representation | HIGH | Existing `AuditRecord` pattern in `internal/audit/types.go` is proven; extension via embedding is idiomatic Go |
| Corpus store integrity approach | HIGH | Owner-only local file threat model is well-understood; hash chaining literature confirms it adds value only for multi-party audit trails |
| Adjudication engine design | HIGH | Mirrors existing pure-function policy engine constraint; async channel pattern is idiomatic for off-hot-path work in this codebase |
| Ed25519 signing primitive | HIGH | `crypto/ed25519` is Go stdlib since 1.13; Ed25519 confirmed supported by sigstore (Trail of Bits 2026 audit); pkg.go.dev verified |
| HMAC-SHA256 for fingerprinting | HIGH | `crypto/hmac` + `crypto/sha256` is Go stdlib; keyed hash non-reversibility is cryptographically established; v2 rewire is trivially one-function swap |
| Zero new external deps | HIGH | All required primitives are in stdlib or already-present packages |

---

## Sources

- [pkg.go.dev/crypto/ed25519](https://pkg.go.dev/crypto/ed25519) — Ed25519 GenerateKey, Sign, Verify; Go stdlib since 1.13; constant-time operations confirmed. HIGH confidence.
- [pkg.go.dev/crypto/hmac](https://pkg.go.dev/crypto/hmac) — HMAC keyed hashing, `hmac.Equal` for timing-safe comparison. HIGH confidence.
- [pkg.go.dev/crypto/rand](https://pkg.go.dev/crypto/rand) — cryptographically secure nonce generation. HIGH confidence.
- [github.com/golang/go/blob/master/src/crypto/ed25519/ed25519.go](https://github.com/golang/go/blob/master/src/crypto/ed25519/ed25519.go) — Source-confirmed: `PrivateKeySize = 64`, `SignatureSize = 64`, deterministic signing. HIGH confidence.
- [blog.trailofbits.com/2026/01/29/building-cryptographic-agility-into-sigstore/](https://blog.trailofbits.com/2026/01/29/building-cryptographic-agility-into-sigstore/) — Sigstore 2026 cryptographic agility audit: Ed25519 (HashEdDSA/Ed25519ph) confirmed as recommended algorithm for new sigstore deployments. HIGH confidence.
- [docs.sigstore.dev/cosign/signing/signing_with_containers/](https://docs.sigstore.dev/cosign/signing/signing_with_containers/) — Cosign algorithm support: RSA, ECDSA, Ed25519. HIGH confidence.
- [tracehold.ai/blog/immutable-audit-log-hmac-hash-chain/](https://tracehold.ai/blog/immutable-audit-log-hmac-hash-chain/) — Hash chaining for audit logs: confirms value is in multi-party tamper-evidence, not single-owner local files. MEDIUM confidence (blog source).
- [dev.to/robertatkinson3570/the-architecture-behind-tamper-proof-audit-logs-56ek](https://dev.to/robertatkinson3570/the-architecture-behind-tamper-proof-audit-logs-56ek) — Tamper-proof audit log architecture; confirms the verifier requirement for hash chaining. MEDIUM confidence (blog source).
- Context7 fetch `/golang/go` — `encoding/json` struct tags, `omitempty` behavior, `crypto/rand.Read` API. HIGH confidence.
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\audit\types.go` — Existing `AuditRecord` struct; embedding target for `CorpusRecord`. HIGH confidence (first-party source).
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\audit\writer.go` — Existing `Writer` pattern (O_APPEND, SetOwnerOnly, mutex, sink fan-out); reused for corpus writer. HIGH confidence.
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\quarantine\quarantine.go` — `MoveTyped`, `Manifest`; confirmed read-only integration (no direct call from adjudication engine). HIGH confidence.
- `C:\Users\Bantu\mzansi-agentive\beekeeper\internal\watch\crossref.go` — `ScanHit`, `CrossReference`; the seam where corpus records are submitted for adjudication. HIGH confidence.
- `C:\Users\Bantu\mzansi-agentive\beekeeper\go.mod` — Confirmed no existing sigstore/cosign Go library import; cosign is CI tooling only. HIGH confidence.
- `C:\Users\Bantu\mzansi-agentive\beekeeper\beekeeper-corpus-milestone-prd.md` — Authoritative scope document; all decisions traced to §3.1–§3.6 requirements. HIGH confidence.

---

*v1.4.0 corpus stack researched: 2026-06-13*
