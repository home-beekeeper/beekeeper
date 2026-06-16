package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/audit"
)

// writeCorpusLines writes CorpusRecord values as NDJSON lines to a temp file
// and returns the file path.
func writeCorpusLines(t *testing.T, dir string, records []CorpusRecord) string {
	t.Helper()
	path := filepath.Join(dir, "beekeeper-corpus.ndjson")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create corpus file: %v", err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			t.Fatalf("encode corpus record: %v", err)
		}
	}
	return path
}

// makeRecord constructs a minimal CorpusRecord with the given ClusterID and TrueLabel.
func makeRecord(clusterID, trueLabel string) CorpusRecord {
	return CorpusRecord{
		AuditRecord: audit.AuditRecord{
			RecordID:   "test-" + clusterID,
			ClusterID:  clusterID,
			ToolName:   "Bash",
			Decision:   "block",
			RecordType: "policy_decision",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		},
		TrueLabel:           trueLabel,
		CorpusSchemaVersion: CorpusSchemaVersion,
		PushEnvelope: &PushEnvelope{
			Signature: EnvelopeSignature{
				PackageOrExtensionID: "npm:test-pkg",
				Version:              "1.0.0",
			},
			TrueLabel:      trueLabel,
			ConfidenceTier: "enforce",
			SourceCount:    2,
			ActionHint:     ActionHintWatchAndBlock,
		},
	}
}

// TestReadMaliciousRecords verifies that ReadMaliciousRecords:
//   - returns only TrueLabel=="malicious" records
//   - latest-per-cluster line wins when the same ClusterID appears multiple times
//   - returns (nil, nil) on a missing file (not an error)
//   - skips malformed NDJSON lines and returns the remaining valid malicious records
//   - does not introduce secret-shaped fields (redaction safety)
func TestReadMaliciousRecords(t *testing.T) {
	dir := t.TempDir()

	t.Run("filters_to_malicious_only", func(t *testing.T) {
		recs := []CorpusRecord{
			makeRecord("cluster-A", "malicious"),
			makeRecord("cluster-B", "benign"),
			makeRecord("cluster-C", "unresolved"),
			makeRecord("cluster-D", "policy_correct"),
			makeRecord("cluster-E", "malicious"),
		}
		path := writeCorpusLines(t, dir, recs)
		got, err := ReadMaliciousRecords(path)
		if err != nil {
			t.Fatalf("ReadMaliciousRecords: unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 malicious records, got %d: %+v", len(got), got)
		}
		for _, r := range got {
			if r.TrueLabel != "malicious" {
				t.Errorf("got TrueLabel=%q, want malicious", r.TrueLabel)
			}
		}
	})

	t.Run("latest_per_cluster_wins", func(t *testing.T) {
		// Same ClusterID written twice; second line (malicious) must win.
		recs := []CorpusRecord{
			makeRecord("cluster-flip", "unresolved"),
			makeRecord("cluster-flip", "malicious"),
		}
		path := filepath.Join(dir, "flip.ndjson")
		f, _ := os.Create(path)
		enc := json.NewEncoder(f)
		for _, r := range recs {
			_ = enc.Encode(r)
		}
		f.Close()

		got, err := ReadMaliciousRecords(path)
		if err != nil {
			t.Fatalf("ReadMaliciousRecords: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 record (latest wins), got %d", len(got))
		}
		if got[0].TrueLabel != "malicious" {
			t.Errorf("TrueLabel = %q, want malicious (latest wins)", got[0].TrueLabel)
		}
	})

	t.Run("missing_file_returns_nil_nil", func(t *testing.T) {
		got, err := ReadMaliciousRecords(filepath.Join(dir, "nonexistent.ndjson"))
		if err != nil {
			t.Fatalf("missing file must return nil error, got: %v", err)
		}
		if got != nil {
			t.Fatalf("missing file must return nil slice, got %+v", got)
		}
	})

	t.Run("malformed_line_skipped_valid_returned", func(t *testing.T) {
		path := filepath.Join(dir, "malformed.ndjson")
		f, _ := os.Create(path)
		f.WriteString("{this is not valid json}\n")
		enc := json.NewEncoder(f)
		_ = enc.Encode(makeRecord("cluster-valid", "malicious"))
		f.Close()

		got, err := ReadMaliciousRecords(path)
		if err != nil {
			t.Fatalf("malformed line must not produce error, got: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("want 1 valid malicious record, got %d", len(got))
		}
		if got[0].TrueLabel != "malicious" {
			t.Errorf("TrueLabel = %q, want malicious", got[0].TrueLabel)
		}
	})

	t.Run("redaction_safety_no_secret_fields", func(t *testing.T) {
		// The corpus is redacted at write time (WR-03). Verify the reader does
		// not introduce any secret-shaped field (token, password, key) in the
		// returned slice. We marshal the returned records back to JSON and check
		// that no such field name appears.
		recs := []CorpusRecord{makeRecord("cluster-safe", "malicious")}
		path := filepath.Join(dir, "safe.ndjson")
		f, _ := os.Create(path)
		json.NewEncoder(f).Encode(recs[0])
		f.Close()

		got, err := ReadMaliciousRecords(path)
		if err != nil || len(got) == 0 {
			t.Fatalf("unexpected: err=%v, len=%d", err, len(got))
		}
		blob, _ := json.Marshal(got[0])
		lower := strings.ToLower(string(blob))
		for _, forbidden := range []string{"password", "secret", "private_key", "api_key"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("returned record JSON contains secret-shaped field %q", forbidden)
			}
		}
	})
}

// TestReadMaliciousRecordsLatestPerCluster verifies that when two lines share
// the same ClusterID (first unresolved, then malicious), exactly one record is
// returned and it is the later (malicious) one.
func TestReadMaliciousRecordsLatestPerCluster(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "supersede.ndjson")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(f)
	// First line: unresolved
	first := makeRecord("nx-console-cluster", "unresolved")
	if err := enc.Encode(first); err != nil {
		t.Fatal(err)
	}
	// Second line: superseding malicious adjudication (same ClusterID)
	second := makeRecord("nx-console-cluster", "malicious")
	if err := enc.Encode(second); err != nil {
		t.Fatal(err)
	}
	f.Close()

	got, err := ReadMaliciousRecords(path)
	if err != nil {
		t.Fatalf("ReadMaliciousRecords: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly 1 record (latest-per-cluster), got %d: %+v", len(got), got)
	}
	if got[0].TrueLabel != "malicious" {
		t.Errorf("TrueLabel = %q, want malicious (second line wins)", got[0].TrueLabel)
	}
	if got[0].AuditRecord.ClusterID != "nx-console-cluster" {
		t.Errorf("ClusterID = %q, want nx-console-cluster", got[0].AuditRecord.ClusterID)
	}
}
