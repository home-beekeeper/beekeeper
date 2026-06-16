package corpus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// defaultThresholds returns the standard PLCY-01 corroboration thresholds.
func defaultThresholds() policy.CorroborationThresholds {
	return policy.CorroborationThresholds{
		WarnAt:         1,
		BlockAt:        2,
		QuarantineAt:   3,
		CatalogHealthy: true,
	}
}

// makeUnresolvedRecord creates a minimal CorpusRecord with TrueLabel="unresolved"
// and a given ClusterID, suitable for adjudicator tests.
func makeUnresolvedRecord(clusterID string) CorpusRecord {
	return CorpusRecord{
		AuditRecord: audit.AuditRecord{
			RecordID:  "test-record-" + clusterID,
			ClusterID: clusterID,
			ToolName:  "Bash",
			Decision:  "block",
		},
		TrueLabel:           "unresolved",
		CorpusSchemaVersion: CorpusSchemaVersion,
		PushEnvelope: &PushEnvelope{
			TrueLabel:      "unresolved",
			ConfidenceTier: "watch",
			SourceCount:    0,
			ActionHint:     ActionHintWatchAndBlock,
		},
	}
}

// TestAdjudicationTrueLabelTransition verifies ADJ-02:
//   - A fresh record has TrueLabel "unresolved".
//   - After Adjudicate with a catalog-confirmation signal it becomes "malicious".
//   - Only the 4-value true_label set is reachable: malicious|benign|policy_correct|unresolved.
func TestAdjudicationTrueLabelTransition(t *testing.T) {
	rec := makeUnresolvedRecord("cluster-adj-01")

	// Start as unresolved.
	if rec.TrueLabel != "unresolved" {
		t.Fatalf("initial TrueLabel = %q, want \"unresolved\"", rec.TrueLabel)
	}

	// Adjudicate with catalog_confirmation signal.
	signals := AdjudicationSignals{
		CatalogConfirmed: true,
		Matches:          []policy.CatalogMatch{{CatalogSource: "bumblebee", Signed: true}},
		Thresholds:       defaultThresholds(),
		Now:              time.Now().UTC(),
	}
	result := Adjudicate(rec, signals)

	// TrueLabel must transition to "malicious" on catalog confirmation.
	if result.TrueLabel != "malicious" {
		t.Fatalf("TrueLabel after catalog confirmation = %q, want \"malicious\"", result.TrueLabel)
	}
	if result.AdjudicationSource != AdjSourceCatalogConfirmation {
		t.Fatalf("AdjudicationSource = %q, want %q", result.AdjudicationSource, AdjSourceCatalogConfirmation)
	}

	// Only the 4-value set is reachable.
	validLabels := map[string]bool{
		"malicious":     true,
		"benign":        true,
		"policy_correct": true,
		"unresolved":    true,
	}
	if !validLabels[result.TrueLabel] {
		t.Fatalf("TrueLabel = %q is outside the valid 4-value set", result.TrueLabel)
	}

	// No catalog confirmation → stays unresolved.
	noop := AdjudicationSignals{
		CatalogConfirmed:      false,
		DownstreamCleanElapsed: false,
		Now:                   time.Now().UTC(),
	}
	noopResult := Adjudicate(rec, noop)
	if noopResult.TrueLabel != "unresolved" {
		t.Fatalf("TrueLabel without any signal = %q, want \"unresolved\"", noopResult.TrueLabel)
	}
}

// TestAdjudicationSources verifies ADJ-03: all 6 adjudication_source values with
// documented confidence mapping:
//   - high:   forensic_review, breach_confirmation
//   - medium: catalog_confirmation, benign_explained
//   - weak:   downstream_clean, user_override
func TestAdjudicationSources(t *testing.T) {
	cases := []struct {
		source     string
		wantConf   string
	}{
		{AdjSourceForensicReview,      AdjConfidenceHigh},
		{AdjSourceBreachConfirmation,  AdjConfidenceHigh},
		{AdjSourceCatalogConfirmation, AdjConfidenceMedium},
		{AdjSourceBenignExplained,     AdjConfidenceMedium},
		{AdjSourceDownstreamClean,     AdjConfidenceWeak},
		{AdjSourceUserOverride,        AdjConfidenceWeak},
	}
	for _, tc := range cases {
		got := AdjudicationSourceConfidence(tc.source)
		if got != tc.wantConf {
			t.Errorf("AdjudicationSourceConfidence(%q) = %q, want %q", tc.source, got, tc.wantConf)
		}
	}
}

// TestWasCorrectAndResolvedAt verifies ADJ-06:
//   - was_correct is derived from true_label vs verdict (policy_correct → true).
//   - resolved_at is RFC3339-parseable when leaving unresolved.
//   - resolved_at is empty while still unresolved.
func TestWasCorrectAndResolvedAt(t *testing.T) {
	now := time.Now().UTC()

	// Case 1: policy_correct verdict + malicious label → was_correct = false.
	rec1 := makeUnresolvedRecord("cluster-wc-01")
	rec1.AuditRecord.Decision = "block"
	signals1 := AdjudicationSignals{
		CatalogConfirmed: true,
		Matches:          []policy.CatalogMatch{{CatalogSource: "bumblebee", Signed: true}},
		Thresholds:       defaultThresholds(),
		Now:              now,
	}
	r1 := Adjudicate(rec1, signals1)
	if r1.WasCorrect == nil {
		t.Fatal("WasCorrect = nil after adjudication, want non-nil")
	}
	// block decision + malicious label → verdict was correct (block was right)
	if !*r1.WasCorrect {
		t.Errorf("WasCorrect = false for block+malicious, want true (block was correct)")
	}
	// resolved_at must be RFC3339-parseable.
	if r1.ResolvedAt == "" {
		t.Fatal("ResolvedAt empty after adjudication, want RFC3339 timestamp")
	}
	if _, err := time.Parse(time.RFC3339, r1.ResolvedAt); err != nil {
		t.Fatalf("ResolvedAt = %q is not valid RFC3339: %v", r1.ResolvedAt, err)
	}

	// Case 2: allow decision + malicious label → was_correct = false (allowed something malicious).
	rec2 := makeUnresolvedRecord("cluster-wc-02")
	rec2.AuditRecord.Decision = "allow"
	r2 := Adjudicate(rec2, signals1)
	if r2.WasCorrect == nil {
		t.Fatal("WasCorrect = nil after adjudication, want non-nil")
	}
	if *r2.WasCorrect {
		t.Error("WasCorrect = true for allow+malicious, want false (allowed a malicious tool call)")
	}

	// Case 3: still unresolved → resolved_at must be empty.
	noopSignals := AdjudicationSignals{Now: now}
	r3 := Adjudicate(rec1, noopSignals)
	if r3.ResolvedAt != "" {
		t.Fatalf("ResolvedAt = %q for unresolved record, want empty", r3.ResolvedAt)
	}
	if r3.WasCorrect != nil {
		t.Fatal("WasCorrect non-nil for unresolved record, want nil")
	}
}

// TestSupersedingRecords verifies ADJ-07:
//   - An outcome update has a NEW RecordID but the SAME ClusterID as the original.
//   - The original "unresolved" line is preserved on disk (append-only).
func TestSupersedingRecords(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	corpusPath := filepath.Join(dir, "corpus", "beekeeper-corpus.ndjson")

	// Write an unresolved record manually using StoreSink.
	sink, err := NewStoreSink(corpusPath)
	if err != nil {
		t.Fatalf("NewStoreSink: %v", err)
	}
	originalClusterID := "cluster-sr-01"
	rec := audit.AuditRecord{
		RecordID:  "orig-record-id",
		ClusterID: originalClusterID,
		ToolName:  "Bash",
		Decision:  "block",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if err := sink.Write(rec); err != nil {
		t.Fatalf("StoreSink.Write: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("StoreSink.Close: %v", err)
	}

	// Run the adjudication batch pass with a fake catalog index that confirms the match.
	idx := &fakeCatalogIndex{alwaysMatch: true}
	thresholds := defaultThresholds()

	ctx := context.Background()
	if err := RunAdjudicationBatch(ctx, corpusPath, stateFile, idx, thresholds, 30); err != nil {
		t.Fatalf("RunAdjudicationBatch: %v", err)
	}

	// Read all lines from the corpus NDJSON.
	data, err := os.ReadFile(corpusPath)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	lines := splitNDJSON(data)
	if len(lines) < 2 {
		t.Fatalf("expected >= 2 lines in corpus (original + superseding), got %d:\n%s", len(lines), string(data))
	}

	// Parse all lines and verify.
	var foundOriginal, foundSuperseding bool
	for _, line := range lines {
		var cr CorpusRecord
		if err := json.Unmarshal([]byte(line), &cr); err != nil {
			t.Fatalf("unmarshal line %q: %v", line, err)
		}
		if cr.AuditRecord.ClusterID != originalClusterID {
			continue
		}
		if cr.TrueLabel == "unresolved" && cr.AuditRecord.RecordID == "orig-record-id" {
			// Wait — the StoreSink may have wrapped it in a CorpusRecord with a new ID.
			// Check cluster_id and true_label.
			foundOriginal = true
		}
		if cr.TrueLabel == "malicious" {
			// Superseding record: same ClusterID, new RecordID.
			foundSuperseding = true
			if cr.AuditRecord.RecordID == "" {
				t.Error("superseding record has empty RecordID")
			}
		}
	}
	if !foundOriginal {
		t.Error("original unresolved record not found in corpus (append-only violated)")
	}
	if !foundSuperseding {
		t.Error("superseding malicious record not written by RunAdjudicationBatch")
	}
}

// TestDownstreamCleanWindow verifies ADJ-07 / OQ-1:
//   - downstream_clean labels benign ONLY after cleanWindowDays have elapsed
//     since the original Timestamp with no correlated follow-on.
//   - Before the window it stays unresolved.
func TestDownstreamCleanWindow(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, "state.json")
	corpusPath := filepath.Join(dir, "corpus", "beekeeper-corpus.ndjson")

	// Write a record timestamped 40 days ago (past the 30-day window).
	oldTimestamp := time.Now().UTC().Add(-40 * 24 * time.Hour).Format(time.RFC3339)
	sink, err := NewStoreSink(corpusPath)
	if err != nil {
		t.Fatalf("NewStoreSink: %v", err)
	}
	clusterID := "cluster-dc-01"
	oldRec := audit.AuditRecord{
		RecordID:  "old-record-id",
		ClusterID: clusterID,
		ToolName:  "Bash",
		Decision:  "allow",
		Timestamp: oldTimestamp,
	}
	if err := sink.Write(oldRec); err != nil {
		t.Fatalf("StoreSink.Write old: %v", err)
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("StoreSink.Close: %v", err)
	}

	// Index returns no matches → no catalog_confirmation; downstream_clean logic applies.
	noMatchIdx := &fakeCatalogIndex{alwaysMatch: false}
	thresholds := defaultThresholds()

	ctx := context.Background()
	if err := RunAdjudicationBatch(ctx, corpusPath, stateFile, noMatchIdx, thresholds, 30); err != nil {
		t.Fatalf("RunAdjudicationBatch (old, past window): %v", err)
	}

	// Read all lines; the old record should have a superseding benign record.
	data, err := os.ReadFile(corpusPath)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	lines := splitNDJSON(data)
	var foundBenign bool
	for _, line := range lines {
		var cr CorpusRecord
		if err := json.Unmarshal([]byte(line), &cr); err != nil {
			continue
		}
		if cr.AuditRecord.ClusterID == clusterID && cr.TrueLabel == "benign" {
			foundBenign = true
		}
	}
	if !foundBenign {
		t.Error("expected benign label after 40-day window (past 30-day threshold), not found")
	}

	// Now test: a record within the window stays unresolved.
	dir2 := t.TempDir()
	stateFile2 := filepath.Join(dir2, "state.json")
	corpusPath2 := filepath.Join(dir2, "corpus", "beekeeper-corpus.ndjson")

	recentTimestamp := time.Now().UTC().Add(-5 * 24 * time.Hour).Format(time.RFC3339)
	sink2, err := NewStoreSink(corpusPath2)
	if err != nil {
		t.Fatalf("NewStoreSink2: %v", err)
	}
	recentRec := audit.AuditRecord{
		RecordID:  "recent-record-id",
		ClusterID: "cluster-dc-02",
		ToolName:  "Bash",
		Decision:  "allow",
		Timestamp: recentTimestamp,
	}
	if err := sink2.Write(recentRec); err != nil {
		t.Fatalf("StoreSink2.Write: %v", err)
	}
	if err := sink2.Close(); err != nil {
		t.Fatalf("StoreSink2.Close: %v", err)
	}

	if err := RunAdjudicationBatch(ctx, corpusPath2, stateFile2, noMatchIdx, thresholds, 30); err != nil {
		t.Fatalf("RunAdjudicationBatch (recent, within window): %v", err)
	}

	data2, err := os.ReadFile(corpusPath2)
	if err != nil {
		t.Fatalf("read corpus2: %v", err)
	}
	lines2 := splitNDJSON(data2)
	for _, line := range lines2 {
		var cr CorpusRecord
		if err := json.Unmarshal([]byte(line), &cr); err != nil {
			continue
		}
		if cr.AuditRecord.ClusterID == "cluster-dc-02" && cr.TrueLabel == "benign" {
			t.Error("recent record (5 days, within 30-day window) was incorrectly labeled benign")
		}
	}
}

// splitNDJSON splits NDJSON bytes into non-empty lines.
func splitNDJSON(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := string(data[start:i])
			if len(line) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(data) {
		line := string(data[start:])
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

// fakeCatalogIndex is a test-only MultiCatalogLookup.
//   - alwaysMatch=true  → every LookupAll returns a signed Bumblebee match
//   - alwaysMatch=false → every LookupAll returns nil (no match)
// It does NOT implement io.Closer — it's passed as policy.MultiCatalogLookup.
type fakeCatalogIndex struct {
	alwaysMatch bool
}

func (f *fakeCatalogIndex) LookupAll(ecosystem, pkg string) []policy.CatalogMatch {
	if f.alwaysMatch {
		return []policy.CatalogMatch{
			{CatalogSource: "bumblebee", Signed: true, Ecosystem: ecosystem, Package: pkg, Version: "18.95.0"},
		}
	}
	return nil
}
