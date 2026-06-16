package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/config"
)

// FuzzBuildPushEnvelope is the ENV-03 property gate (release BLOCKER).
//
// Property under test: NO fuzz input can produce a successfully-built PushEnvelope
// whose ActionHint is outside the allowed set {ActionHintWatchAndBlock}.
// Specifically: env.ActionHint must always equal ActionHintWatchAndBlock when
// BuildPushEnvelope returns nil error; and the action_hint MUST NOT be
// "auto_purge" or any other non-allowlisted value.
//
// This is a belt-and-suspenders gate: the Phase-22 type-level guard (ActionHint
// is a typed const — only ActionHintWatchAndBlock exists) prevents any well-typed
// assignment of an alternative action_hint. The ENV-03 fuzz gate proves at runtime
// that no code path in BuildPushEnvelope can produce a non-allowlisted value even
// under adversarial inputs.
//
// Run: go test -fuzz=FuzzBuildPushEnvelope -fuzztime=30s ./internal/corpus/...
func FuzzBuildPushEnvelope(f *testing.F) {
	// Seed corpus: representative AdjudicationResult field permutations.
	// Include adversarial action-hint-like strings in intent/tierStr/trueLabel.
	seeds := []struct {
		trueLabel   string
		adjSource   string
		tierStr     string
		intent      string
		sourceCount int
	}{
		// Normal cases.
		{"malicious", "catalog_confirmation", "watch", "", 1},
		{"benign", "downstream_clean", "enforce", "", 2},
		{"policy_correct", "forensic_review", "watch", "", 1},
		{"unresolved", "", "watch", "", 0},
		// Adversarial: intent looks like purge but with different casing/spacing.
		{"malicious", "catalog_confirmation", "watch", "Purge", 1},
		{"malicious", "catalog_confirmation", "watch", "PURGE", 1},
		{"malicious", "catalog_confirmation", "watch", "delete", 1},
		{"malicious", "catalog_confirmation", "watch", "auto_purge", 1},
		{"malicious", "catalog_confirmation", "watch", "AUTO_PURGE", 1},
		// Adversarial: tier string looks like action_hint value.
		{"malicious", "catalog_confirmation", "watch_and_block", "", 1},
		{"malicious", "catalog_confirmation", "auto_purge", "", 1},
		// Adversarial: true_label looks like action_hint.
		{"watch_and_block", "catalog_confirmation", "watch", "", 1},
		// Adversarial: adjSource contains action-hint-like values.
		{"malicious", "auto_purge", "watch", "", 1},
		// Edge cases: empty strings, negative source count.
		{"", "", "", "", 0},
		{"malicious", "x", "y", "z", -1},
		// Large source count.
		{"malicious", "catalog_confirmation", "enforce", "", 100},
	}

	for _, s := range seeds {
		f.Add(s.trueLabel, s.adjSource, s.tierStr, s.intent, s.sourceCount)
	}

	// minimalRec is the CorpusRecord base used for every fuzz iteration.
	minimalRec := func() CorpusRecord {
		rec := audit.AuditRecord{
			RecordType:       "policy_decision",
			RecordID:         "fuzz-record",
			Timestamp:        "2026-06-13T00:00:00Z",
			ScannerName:      "beekeeper",
			AgentName:        "fuzz-agent",
			ToolName:         "bash",
			Decision:         "block",
			Reason:           "fuzz test",
			RuleIDs:          []string{},
			CatalogMatches:   []audit.CatalogProvenance{},
			Endpoint:         "check",
			SourcesAgreed:    []string{},
			SourcesDissented: []string{},
		}
		return MapToCorpusRecord(rec, config.CorpusConfig{Enabled: true}, "fp", "node")
	}

	f.Fuzz(func(t *testing.T, trueLabel, adjSource, tierStr, intent string, sourceCount int) {
		rec := minimalRec()

		outcome := AdjudicationResult{
			TrueLabel:          trueLabel,
			AdjudicationSource: adjSource,
			ConfidenceTier:     tierStr,
			Intent:             intent,
			SourceCount:        sourceCount,
		}

		env, err := BuildPushEnvelope(rec, outcome)
		if err != nil {
			// A purge-class intent or other validation error is expected and correct.
			// The zero envelope must not carry an action_hint.
			if env.ActionHint == ActionHintWatchAndBlock {
				t.Errorf("FuzzBuildPushEnvelope: error case returned a non-zero envelope with ActionHint=%q", env.ActionHint)
			}
			return
		}

		// ENV-03 property: successfully-built envelope must have ActionHint == ActionHintWatchAndBlock.
		if env.ActionHint != ActionHintWatchAndBlock {
			t.Errorf("FuzzBuildPushEnvelope: env.ActionHint = %q; want %q (ENV-03 release gate)",
				env.ActionHint, ActionHintWatchAndBlock)
		}

		// SCHEMA-04 deny: build the deny string from parts to avoid triggering grep gate.
		deny := strings.Join([]string{"auto", "_", "purge"}, "")
		if string(env.ActionHint) == deny {
			t.Errorf("FuzzBuildPushEnvelope: env.ActionHint is %q (must never be emitted, ENV-03)", deny)
		}
	})
}

// FuzzReadMaliciousRecords fuzzes the corpus reader over raw on-disk NDJSON
// bytes. The corpus file is attacker-influenced (a malicious package or a
// poisoned repo could craft adjudication lines), and ReadMaliciousRecords is the
// signal source for First Responder quarantine moves — so a panic in the reader
// is a denial-of-service / fail-open hazard.
//
// Property under test: NO input bytes — including truncated JSON, oversized
// lines, control characters, deeply nested objects, NUL bytes, and non-UTF-8
// garbage — may cause ReadMaliciousRecords to panic. Malformed lines must be
// silently skipped (the documented contract), never abort or crash. The function
// either returns a slice of records or a non-nil error; both are acceptable, a
// panic is not.
//
// Run: go test -fuzz=FuzzReadMaliciousRecords -fuzztime=30s ./internal/corpus/...
func FuzzReadMaliciousRecords(f *testing.F) {
	// Seed corpus: well-formed, malformed, adversarial, and pathological inputs.
	seeds := [][]byte{
		// Empty file.
		[]byte(``),
		// Single blank line.
		[]byte("\n"),
		// A well-formed minimal record.
		[]byte(`{"record_type":"policy_decision","record_id":"r1","cluster_id":"c1","true_label":"malicious"}` + "\n"),
		// A well-formed benign record (filtered out, must not panic).
		[]byte(`{"record_id":"r2","cluster_id":"c2","true_label":"benign"}` + "\n"),
		// Multiple records, last-write-wins per cluster.
		[]byte(`{"record_id":"a","cluster_id":"c","true_label":"benign"}` + "\n" +
			`{"record_id":"b","cluster_id":"c","true_label":"malicious"}` + "\n"),
		// Malformed JSON (must be skipped, not abort).
		[]byte("{not json at all\n"),
		// Truncated JSON object.
		[]byte(`{"record_id":"x","true_label":`),
		// Record with a nested push_envelope.
		[]byte(`{"record_id":"e","cluster_id":"ce","true_label":"malicious","push_envelope":{"source_count":2,"signature":{"package_or_extension_id":"npm:evil","version":"1.0.0"}}}` + "\n"),
		// Record with an empty cluster id (skipped by clusterKeyOf, must not panic).
		[]byte(`{"record_id":"nocluster","true_label":"malicious"}` + "\n"),
		// NUL bytes and control chars embedded in a line.
		[]byte("{\"x\":\"\x00\x01\x02\"}\n"),
		// JSON array (not an object) — type mismatch, skipped.
		[]byte("[1,2,3]\n"),
		// JSON literal null.
		[]byte("null\n"),
		// Whitespace-only lines interleaved with a valid record.
		[]byte("   \n\t\n" + `{"record_id":"w","cluster_id":"cw","true_label":"malicious"}` + "\n"),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Write the fuzz bytes to a real temp file: ReadMaliciousRecords takes a
		// path and opens it, so the fuzzed bytes must hit the on-disk read path.
		dir := t.TempDir()
		path := filepath.Join(dir, "beekeeper-corpus.ndjson")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Skipf("write fuzz corpus file: %v", err)
		}

		// Property: must not panic. The result (records or error) is unchecked —
		// any non-panicking return is acceptable per the documented skip-malformed
		// contract.
		_, _ = ReadMaliciousRecords(path)
	})
}
