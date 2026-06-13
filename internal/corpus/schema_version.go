package corpus

// CorpusSchemaVersion is the current version of the CorpusRecord JSON schema.
// It is embedded in every CorpusRecord as the corpus_schema_version field.
//
// BREAKING CHANGE WARNING: Bumping this constant is a breaking schema change.
// Any change to the CorpusRecord field set, json tag names, or CorpusScope
// values requires a version bump AND a migration plan before the store can
// accept mixed-version records. Phase 22 freezes the schema at "1.0".
//
// KNOWN DEFERRED ISSUE (accepted at the 2026-06-13 Phase 22 freeze sign-off):
// normalizeNetworkDest in behavior_sig.go strips a trailing ":<digits>" as a
// port, so a bare IPv6 address (e.g. "::1") mis-normalizes to "::". This is a
// frozen hash input — the fix (bracket-aware [host]:port parsing) is a BREAKING
// change and MUST ride the next CorpusSchemaVersion bump. When you bump this
// constant, address it: see .planning/todos/pending/corpus-behavior-sig-ipv6-normalization.md
const CorpusSchemaVersion = "1.0"
