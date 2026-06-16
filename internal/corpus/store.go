package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// StoreSink is an append-only, owner-only NDJSON sink for CorpusRecords. It
// implements audit.Sink so it can be added to the audit.MultiSink fan-out graph.
//
// StoreSink mirrors the audit.Writer pattern:
//   - O_APPEND|O_CREATE|O_WRONLY open (never truncates, preserves prior records)
//   - platform.SetOwnerOnly enforced on open and re-enforced after every write
//   - sync.Mutex guards the json.Encoder for safe concurrent use
//   - Long-lived *os.File — NOT per-record open/close (Pitfall 4)
//
// Redaction-first invariant (T-23-01, F-1 security lesson):
// StoreSink.Write calls audit.RedactRecordWithDefaults as its FIRST operation
// before any further processing. Credential-shaped strings (AKIA keys, JWT
// tokens, Bearer headers) are replaced with placeholders before the record is
// ever marshalled to NDJSON.
//
// Phase 23 seam note: StoreSink currently constructs a minimal CorpusRecord
// inline. Plan 23-02 will replace the inline mapping by delegating to the real
// corpus.MapToCorpusRecord function, which wires the full emitter adapter
// (BehaviorSigHash, ScanClusterID, RepoFingerprint, BuildPushEnvelope). The
// executor of 23-02 should replace the inline corpusRec construction below with
// a call to MapToCorpusRecord(redacted, cfg) and remove the direct PushEnvelope
// construction here.
type StoreSink struct {
	path    string
	file    *os.File
	mu      sync.Mutex
	encoder *json.Encoder
}

// NewStoreSink opens (creating if necessary) the corpus NDJSON file at
// corpusPath for appending, enforces owner-only permissions, and returns a
// ready-to-use StoreSink. The parent directory is created with 0o700 if absent.
//
// On error, any opened file handle is closed before returning so there is no
// resource leak.
func NewStoreSink(corpusPath string) (*StoreSink, error) {
	if err := os.MkdirAll(filepath.Dir(corpusPath), 0o700); err != nil {
		return nil, fmt.Errorf("create corpus directory: %w", err)
	}

	f, err := os.OpenFile(corpusPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open corpus file %q: %w", corpusPath, err)
	}

	// Enforce owner-only permissions immediately after open. On Windows,
	// os.OpenFile does not produce a 0600-equivalent DACL; SetOwnerOnly applies
	// the correct owner-only DACL (mirrors audit.Writer behaviour, T-23-03).
	if err := platform.SetOwnerOnly(corpusPath); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("enforce owner-only permissions on corpus file: %w", err)
	}

	return &StoreSink{
		path:    corpusPath,
		file:    f,
		mu:      sync.Mutex{},
		encoder: json.NewEncoder(f),
	}, nil
}

// Write persists rec to the corpus NDJSON file as a single CorpusRecord line.
//
// Invariant order (all three steps must happen in this order):
//  1. RedactRecordWithDefaults — credential-shaped strings are removed FIRST.
//  2. Map the redacted AuditRecord to a CorpusRecord (minimal inline mapping;
//     Phase 23-02 replaces this with MapToCorpusRecord).
//  3. Encode to NDJSON under the mutex, then re-enforce owner-only permissions.
//
// A Write error is returned verbatim. The caller (MultiSink) owns fail-closed
// semantics; StoreSink never panics. A Write error MUST NOT change the hook
// exit code (verified by TestCorpusWriteErrorDoesNotChangeExitCode in 23-03).
func (s *StoreSink) Write(rec audit.AuditRecord) error {
	// Step 1 — REDACTION FIRST (T-23-01, non-negotiable).
	redacted := audit.RedactRecordWithDefaults(rec)

	// Step 2 — Minimal CorpusRecord construction.
	// Phase 23-02 seam: replace this inline mapping with MapToCorpusRecord(redacted, cfg).
	corpusRec := CorpusRecord{
		AuditRecord:         redacted,
		TrueLabel:           "unresolved",
		CorpusSchemaVersion: CorpusSchemaVersion,
		// Scope zero-value serialises as "org_only" via CorpusScope.MarshalJSON (SCOPE-01).
		// PushEnvelope: minimal non-nil placeholder so STORE-04 holds from the
		// first write (before Phase 23-02 wires the full emitter).
		PushEnvelope: &PushEnvelope{
			TrueLabel:      "unresolved",
			ConfidenceTier: "watch",
			SourceCount:    0,
			ActionHint:     ActionHintWatchAndBlock,
		},
	}

	// Step 3 — Encode under mutex + re-enforce permissions.
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.encoder.Encode(corpusRec); err != nil {
		return fmt.Errorf("encode corpus record: %w", err)
	}

	// Re-enforce owner-only permissions after every write. This catches the case
	// where an external process reset the DACL or permissions between writes
	// (mirrors audit.Writer re-enforcement, T-23-03).
	if err := platform.SetOwnerOnly(s.path); err != nil {
		return fmt.Errorf("re-enforce owner-only permissions on corpus file: %w", err)
	}

	return nil
}

// Close closes the underlying corpus file under the mutex.
func (s *StoreSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}

// maxCorpusRecordBytes bounds the size of a single NDJSON corpus line before it
// is appended (WR-06). A tool call with very large SentryFilesAccessed/Reason
// content could otherwise exceed the ~4KB the O_APPEND atomicity assumption
// relies on; records over this cap are rejected (the caller logs + skips) rather
// than risking a torn line that the reader silently drops. The cap is generous
// (64KB) so legitimate records are never lost.
const maxCorpusRecordBytes = 64 << 10

// AppendCorpusRecordLine is the single shared writer for fully-resolved
// CorpusRecords (IN-03). Both the hook hot path (writeCorpusRecordDirect in
// internal/check/handler.go) and the adjudicator's superseding-record path
// (appendCorpusRecord in adjudicator.go) call it, so the redaction-first
// invariant (WR-03) and the open-flags/size-cap behaviour live in exactly one
// place and cannot drift.
//
// It lives in store.go (no adjudicator symbols) so handler.go can import it
// without pulling in RunAdjudicationBatch (ADJ-01 / Pitfall 3).
//
// Invariant order:
//  1. Redact the embedded AuditRecord with RedactRecordWithDefaults (WR-03):
//     every write path — store and adjudicator — honors redaction-first, even
//     for records re-read off disk that may have been written by another tool.
//  2. Marshal to a single NDJSON line and enforce the size cap (WR-06).
//  3. Open with O_APPEND|O_CREATE|O_WRONLY (IN-04: O_CREATE, never O_TRUNC) and
//     write the full record+"\n" buffer in a SINGLE f.Write call.
//
// Windows append-atomicity caveat (WR-06): POSIX guarantees O_APPEND write
// atomicity up to filesystem limits, but Windows provides no such guarantee for
// FILE_APPEND_DATA writes from multiple concurrent processes. The single-Write
// of a size-capped buffer minimizes the torn-line window; a cross-process file
// lock would be the fully robust fix but is deliberately out of scope here to
// avoid regressing the sub-100ms hot-path latency. The reader tolerates the
// residual risk by skipping malformed lines (adjudicator Step 1).
func AppendCorpusRecordLine(corpusPath string, rec CorpusRecord) error {
	// Step 1 — REDACTION FIRST (T-23-01 / WR-03, non-negotiable, uniform across
	// every write path).
	rec.AuditRecord = audit.RedactRecordWithDefaults(rec.AuditRecord)

	// Step 2 — marshal + size cap.
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal corpus record: %w", err)
	}
	if len(data)+1 > maxCorpusRecordBytes {
		return fmt.Errorf("corpus record is %d bytes, exceeds cap %d: refusing to append (WR-06 torn-line guard)", len(data)+1, maxCorpusRecordBytes)
	}
	data = append(data, '\n')

	// Step 3 — single-Write append (IN-04: O_CREATE without O_TRUNC).
	f, err := os.OpenFile(corpusPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open corpus file for append: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write corpus record: %w", err)
	}
	return nil
}

// ResolveCorpusPath resolves the corpus NDJSON file path from the provided
// config and stateDir.
//
// Rules:
//   - When cfg.Path is empty, the default path is stateDir/corpus/beekeeper-corpus.ndjson.
//   - When cfg.Path is set, it is cleaned and validated to be under stateDir
//     (Pitfall 7 / T-23-04: a path outside stateDir bypasses the self-protection
//     guard which only covers the StateDir prefix).
//
// ResolveCorpusPath never calls platform.StateDir() internally — the caller
// passes stateDir so that the corpus package remains testable with t.TempDir().
func ResolveCorpusPath(cfg config.CorpusConfig, stateDir string) (string, error) {
	if cfg.Path == "" {
		return filepath.Join(stateDir, "corpus", "beekeeper-corpus.ndjson"), nil
	}

	clean := filepath.Clean(cfg.Path)
	// Ensure the resolved path is under stateDir with a boundary-safe prefix
	// check (use os.PathSeparator to avoid matching a sibling directory that
	// starts with the same prefix).
	prefix := stateDir
	if !strings.HasPrefix(clean, prefix+string(filepath.Separator)) && clean != prefix {
		return "", fmt.Errorf("corpus path %q is outside state directory %q: refusing to open (T-23-04 self-protection boundary)", clean, stateDir)
	}

	return clean, nil
}
