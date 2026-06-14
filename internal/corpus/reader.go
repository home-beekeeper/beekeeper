// Package corpus — reader.go
//
// ReadMaliciousRecords reads the confirmed-malicious adjudication records from
// the corpus NDJSON file. It is the signal source for First Responder (FRB-01).
//
// CALLER CONSTRAINT (ADJ-01 / Pitfall 5): ReadMaliciousRecords MUST only be
// called from off-hot-path code (runCatalogsSync, RunFirstResponder). It MUST
// NEVER be called from internal/check/handler.go or any synchronous hook path.
package corpus

// ReadMaliciousRecords returns the latest-per-cluster CorpusRecords whose
// TrueLabel is "malicious". These are the confirmed-malicious adjudications
// that First Responder uses to arm the TUI quarantine card and Sentry watch.
//
// Reads the corpus NDJSON from corpusPath (the same file RunAdjudicationBatch
// writes). Returns (nil, nil) when the file does not exist — no corpus yet is
// not an error. Malformed NDJSON lines are silently skipped. The scan is capped
// at maxRecordsToScan (50 000) with a 1 MB per-line buffer (same bounds as
// RunAdjudicationBatch).
//
// CALLER CONSTRAINT (ADJ-01 / Pitfall 5): MUST NOT be called from handler.go.
// Implementation is in Task 2 (feat(24-01): ReadMaliciousRecords).
func ReadMaliciousRecords(corpusPath string) ([]CorpusRecord, error) {
	// RED stub: always returns an empty slice so tests fail until Task 2 implements this.
	return []CorpusRecord{}, nil
}
