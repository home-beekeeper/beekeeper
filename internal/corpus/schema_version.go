package corpus

// CorpusSchemaVersion is the current version of the CorpusRecord JSON schema.
// It is embedded in every CorpusRecord as the corpus_schema_version field.
//
// BREAKING CHANGE WARNING: Bumping this constant is a breaking schema change.
// Any change to the CorpusRecord field set, json tag names, or CorpusScope
// values requires a version bump AND a migration plan before the store can
// accept mixed-version records. Phase 22 freezes the schema at "1.0".
const CorpusSchemaVersion = "1.0"
