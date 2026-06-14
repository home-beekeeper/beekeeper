// Package corpus — reader.go
//
// ReadMaliciousRecords reads the confirmed-malicious adjudication records from
// the corpus NDJSON file. It is the signal source for First Responder (FRB-01).
//
// CALLER CONSTRAINT (ADJ-01 / Pitfall 5): ReadMaliciousRecords MUST only be
// called from off-hot-path code (runCatalogsSync, RunFirstResponder). It MUST
// NEVER be called from internal/check/handler.go or any synchronous hook path,
// as it performs file I/O that would violate the sub-100ms hook budget.
package corpus

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

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
// There is no context parameter — this is a fast read of an already-small
// drained file. The caller (runCatalogsSync) already has a 5s deadline on the
// preceding RunAdjudicationBatch; this function runs after that and is a single
// pass over a sub-10MB file.
//
// CALLER CONSTRAINT (ADJ-01 / Pitfall 5): MUST NOT be called from handler.go.
func ReadMaliciousRecords(corpusPath string) ([]CorpusRecord, error) {
	f, err := os.Open(corpusPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no corpus file yet — not an error
		}
		return nil, fmt.Errorf("open corpus for reading: %w", err)
	}

	var allRecords []CorpusRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB line buffer (matches RunAdjudicationBatch)
	scanned := 0
	for scanner.Scan() {
		if scanned >= maxRecordsToScan {
			break
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec CorpusRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue // skip malformed lines — do not abort
		}
		allRecords = append(allRecords, rec)
		scanned++
	}
	if err := scanner.Err(); err != nil {
		f.Close()
		return nil, fmt.Errorf("scan corpus: %w", err)
	}
	f.Close() // close before any further work (Windows cannot rename open-for-read files)

	// Latest-per-cluster collapse (mirrors RunAdjudicationBatch lines 293–301):
	// last NDJSON line wins (NDJSON is append-only; superseding records appear later).
	latestByCluster := make(map[string]CorpusRecord)
	for _, rec := range allRecords {
		id := clusterKeyOf(rec)
		if id == "" {
			continue
		}
		latestByCluster[id] = rec // last write wins
	}

	// Filter to malicious-only; skip any entry that resolved to another label.
	var out []CorpusRecord
	for _, rec := range latestByCluster {
		if rec.TrueLabel == "malicious" {
			out = append(out, rec)
		}
	}
	return out, nil
}
