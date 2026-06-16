package corpus

import "github.com/home-beekeeper/beekeeper/internal/audit"

// CorpusRecord is one NDJSON line in the corpus store. It embeds the existing
// audit.AuditRecord (behavior + decision layers) via UNNAMED embedding so that
// all AuditRecord fields are promoted to the top-level JSON object — they
// serialize as if they were directly on CorpusRecord, not nested under an
// "AuditRecord" key.
//
// DO NOT use an inline json tag here (the kind used by YAML/mapstructure) —
// that tag is NOT recognized by stdlib encoding/json. Unnamed embedding is the
// correct Go idiom for JSON field promotion (Pitfall 1 in PITFALLS.md).
//
// Field placement rationale:
//
//   - behavior fields: promoted from embedded audit.AuditRecord
//     (AgentName, ToolName, AgentID, AgentLineage, SentryFilesAccessed, etc.)
//   - decision fields: promoted from embedded audit.AuditRecord
//     (Decision, Reason, RuleIDs, CorroborationCount, SourcesAgreed, etc.)
//   - source_surface: promoted from AuditRecord.SourceSurface (additive Phase 22 field)
//   - cluster_id: promoted from AuditRecord.ClusterID (additive Phase 22 field)
//   - ruleset_version: promoted from AuditRecord.RulesetVersion (additive Phase 22 field)
//   - outcome fields: on CorpusRecord directly (corpus-specific; non-retrofittable)
//   - context fields: on CorpusRecord directly (corpus-specific enrichments)
//
// NON-RETROFITTABLE: The outcome layer (TrueLabel, WasCorrect, etc.) must be
// present as placeholders from the first write. TrueLabel is NOT omitempty —
// it is always "unresolved" initially. WasCorrect is *bool (nil = unresolved,
// distinct from false = incorrect). Adding these fields later would leave
// run-1 records without them, breaking adjudication queries (Pitfall 5).
//
// JSON namespace collision check: AuditRecord has no scope, baseline_deviation,
// repo_fingerprint, fleet_node_id, was_correct, true_label, adjudication_source,
// resolved_at, or corpus_schema_version fields. No promoted-field collision exists.
type CorpusRecord struct {
	audit.AuditRecord // promoted — carries behavior + decision layers; all json tags promoted

	// --- Outcome layer (THE MOAT — non-retrofittable) ---

	// TrueLabel is always present in JSON (no omitempty). Initial value is
	// "unresolved". Values: malicious|benign|policy_correct|unresolved.
	// Set by the adjudication engine in Phase 23.
	TrueLabel string `json:"true_label"`

	// AdjudicationSource records which authority set TrueLabel.
	// Examples: "human_review", "downstream_clean_30d", "forensic_confirmation".
	// Absent until adjudicated.
	AdjudicationSource string `json:"adjudication_source,omitempty"`

	// WasCorrect is nil until adjudicated. true = the original verdict matched
	// the ground-truth label. false = the verdict was wrong. nil = unresolved.
	// Pointer semantics with omitempty: absent from JSON when nil.
	WasCorrect *bool `json:"was_correct,omitempty"`

	// ResolvedAt is the RFC3339 timestamp when TrueLabel left "unresolved".
	// Absent until adjudicated.
	ResolvedAt string `json:"resolved_at,omitempty"`

	// --- Context layer ---

	// BaselineDeviation is the behavioral baseline deviation level at event time.
	// Values: "low"|"medium"|"high"|"" (absent = not measured).
	BaselineDeviation string `json:"baseline_deviation,omitempty"`

	// RepoFingerprint is HMAC-SHA256(per_install_secret, canonical_repo_id) as a
	// hex string. NON-REVERSIBLE: the per-install secret is never stored in the
	// corpus record; the fingerprint cannot be reversed to recover the repo path
	// without the secret. Populated by Phase 23 STORE-05. Absent until Phase 23.
	RepoFingerprint string `json:"repo_fingerprint,omitempty"`

	// FleetNodeID is HMAC-SHA256(per_install_secret, hostname) as a hex string.
	// NON-REVERSIBLE: same contract as RepoFingerprint. Populated by Phase 23
	// STORE-05. Absent until Phase 23.
	FleetNodeID string `json:"fleet_node_id,omitempty"`

	// Scope is always present in JSON (no omitempty). The zero value ("") is
	// guaranteed to serialize as "org_only" via CorpusScope.MarshalJSON,
	// preventing scope-tag leakage from uninitialized records (SCOPE-01).
	Scope CorpusScope `json:"scope"`

	// --- Schema versioning ---

	// CorpusSchemaVersion is always the value of the CorpusSchemaVersion constant
	// ("1.0"). Embedded in every record so consumers can detect schema evolution.
	CorpusSchemaVersion string `json:"corpus_schema_version"`

	// --- Push envelope ---

	// PushEnvelope is nil until the Phase 23 emitter populates it. It is NOT
	// transmitted in v1 (no transport in v1); the field is defined here so the
	// wire format is frozen before any record is written.
	PushEnvelope *PushEnvelope `json:"push_envelope,omitempty"`
}

// PushEnvelope is the frozen v1 wire format for a corpus push operation.
// The Signing block is zero-value (nil) in v1 — defined here to freeze the
// format before any corpus record is written. Populated by Phase 23
// BuildPushEnvelope (ENV-02).
//
// SCHEMA-04 guard: ActionHint is typed ActionHint (not string). Assigning any
// unrecognized string literal to PushEnvelope.ActionHint is a compile error.
// The only well-typed value is ActionHintWatchAndBlock.
type PushEnvelope struct {
	// Signature carries the event fingerprint and IOC block that identifies
	// the flagged package or extension.
	Signature EnvelopeSignature `json:"signature"`

	// TrueLabel is the adjudicated label at push time.
	// Values: malicious|benign|policy_correct.
	TrueLabel string `json:"true_label"`

	// ConfidenceTier is the corroboration confidence tier.
	// Values: "watch" (1 source) | "enforce" (2+ sources, >= BlockAt threshold).
	ConfidenceTier string `json:"confidence_tier"`

	// SourceCount is the number of distinct signed sources that agreed on the label.
	SourceCount int `json:"source_count"`

	// Scope is the sharing scope for this push. Typed CorpusScope; zero value
	// resolves to "org_only" via MarshalJSON (same SCOPE-01 guarantee as CorpusRecord).
	Scope CorpusScope `json:"scope"`

	// ActionHint is the typed fleet action hint. Only ActionHintWatchAndBlock is
	// defined. Assigning any other string is a compile error (SCHEMA-04).
	ActionHint ActionHint `json:"action_hint"`

	// Signing is the envelope signature block. Nil in v1 (no transport in v1).
	// The type is defined here to freeze the wire format.
	Signing *SigningBlock `json:"signing,omitempty"`
}

// EnvelopeSignature carries the cryptographic fingerprint of the flagged event
// and any associated indicators of compromise.
type EnvelopeSignature struct {
	// PackageOrExtensionID is the canonical identifier of the flagged package or
	// extension (e.g. "npm:malicious-pkg" or "vscode:nx-console").
	PackageOrExtensionID string `json:"package_or_extension_id"`

	// Version is the flagged version string.
	Version string `json:"version"`

	// BehaviorSignatureHash is SHA-256(action_type_normalized + NUL +
	// target_resource_normalized + NUL + network_destination_normalized),
	// hex-encoded. The normalization rules are frozen in Phase 22
	// (see internal/corpus/behavior_sig.go, Plan 03). This hash is the stable
	// attacker-behavior fingerprint across victims.
	BehaviorSignatureHash string `json:"behavior_signature_hash"`

	// IOCs holds optional indicators of compromise extracted from the event.
	IOCs IOCBlock `json:"iocs,omitempty"`
}

// IOCBlock holds optional indicators of compromise associated with the flagged event.
// All fields are omitempty — the block is absent from JSON when all fields are zero.
type IOCBlock struct {
	// Domains is the list of attacker-controlled domains observed in the event.
	Domains []string `json:"domains,omitempty"`

	// DNSTunnelPattern is the regex or glob pattern matching DNS tunnel queries.
	DNSTunnelPattern string `json:"dns_tunnel_pattern,omitempty"`

	// DeadDropPattern is the regex or glob pattern matching dead-drop C2 URLs.
	DeadDropPattern string `json:"dead_drop_pattern,omitempty"`
}

// SigningBlock is the envelope signing block. In v1 this block is zero-value
// (nil pointer on PushEnvelope.Signing) because no transport layer exists yet.
// The type is defined here to freeze the wire format for v1.1+ when transport
// and signing keys arrive.
//
// Signing is intended to use Ed25519 (stdlib crypto/ed25519) with Sigstore/cosign
// for key distribution — consistent with the Beekeeper release signing strategy.
type SigningBlock struct {
	// Issuer is the identity that issued this signature (e.g. a Sigstore OIDC issuer URL).
	Issuer string `json:"issuer"`

	// Signature is the Ed25519 signature over the canonical envelope bytes, base64-encoded.
	Signature string `json:"signature"`

	// IssuedAt is the RFC3339 timestamp when the signature was issued.
	IssuedAt string `json:"issued_at"`

	// Nonce is a CSPRNG-generated nonce (crypto/rand) to prevent replay attacks.
	Nonce string `json:"nonce"`
}
