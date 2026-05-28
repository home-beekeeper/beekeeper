package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mzansi-agentive/beekeeper/internal/platform"
)

// Writer is an append-only NDJSON sink for AuditRecords. It enforces owner-only
// file permissions (0600 on Unix, owner-only DACL on Windows) on open and
// re-applies them after every write so that a recreated or externally-modified
// file is never left world-readable (Pitfall 5).
//
// Phase 6 additions (AUDT-03/04): Writer now carries a sync.Mutex (safe for
// concurrent hook-handler calls), an optional maxBytes rotation threshold, and
// a list of additional remote sinks (syslog, OTLP, HTTPS) that receive a
// fan-out copy of each record after the local file write succeeds.
type Writer struct {
	path     string
	file     *os.File
	mu       sync.Mutex
	maxBytes int64  // 0 = no rotation
	sinks    []Sink // additional remote sinks; file write already happened
}

// NewWriter opens (creating if needed) the audit log at path for appending and
// enforces owner-only permissions immediately. The parent directory is created
// with 0700 if absent. The file is opened with O_APPEND|O_CREATE|O_WRONLY:
// O_APPEND avoids truncating or recreating an existing log (Pitfall 5), so prior
// decision records are never lost.
//
// NewWriter is equivalent to NewWriterWithOptions(path, 0, nil).
func NewWriter(path string) (*Writer, error) {
	return NewWriterWithOptions(path, 0, nil)
}

// NewWriterWithOptions is like NewWriter but also configures log rotation and
// additional remote sinks. maxBytes controls when Rotate is called (0 = never).
// sinks receives a fan-out copy of each record after the file write; errors from
// remote sinks are logged to stderr and do not affect the caller's error return.
func NewWriterWithOptions(path string, maxBytes int64, sinks []Sink) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create audit directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit log %q: %w", path, err)
	}

	// Enforce owner-only perms on the freshly-opened file. On Windows os.OpenFile
	// does not produce a 0600-equivalent DACL, so SetOwnerOnly applies one.
	if err := platform.SetOwnerOnly(path); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("enforce owner-only permissions on audit log: %w", err)
	}

	return &Writer{
		path:     path,
		file:     f,
		maxBytes: maxBytes,
		sinks:    sinks,
	}, nil
}

// Write marshals rec to a single NDJSON line and appends it to the audit log,
// then re-applies owner-only permissions. Permissions are re-enforced on every
// write so that if the file was recreated or its DACL reset between writes it is
// re-locked before the next record is observed (Pitfall 5).
//
// Phase 6: Write acquires the Writer mutex before any I/O, making it safe for
// concurrent callers. After the file write, if maxBytes > 0 a rotation check is
// performed (errors are logged to stderr but do not fail the write). Finally,
// the record is fanned out to any additional sinks (remote sinks log errors to
// stderr internally and never surface them here).
//
// Write returns any file-write or permissions error verbatim and never
// downgrades a decision on failure — the hook handler owns the fail-closed
// semantics for audit-write errors.
func (w *Writer) Write(rec AuditRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal audit record: %w", err)
	}

	data = append(data, '\n')
	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("append audit record: %w", err)
	}

	if err := platform.SetOwnerOnly(w.path); err != nil {
		return fmt.Errorf("re-enforce owner-only permissions on audit log: %w", err)
	}

	// Rotation: checked after a successful write so the threshold is accurate.
	if w.maxBytes > 0 {
		if rerr := Rotate(w.path, w.maxBytes, 30); rerr != nil {
			fmt.Fprintf(os.Stderr, "beekeeper audit: rotation error: %v\n", rerr)
		}
	}

	// Fan-out to remote sinks. Errors are fire-and-forget; each remote sink
	// logs its own errors to stderr. The mutex is still held here so sinks
	// that are not concurrency-safe are protected; well-behaved sinks (OTLPSink,
	// HTTPSink) manage their own locking internally.
	for _, s := range w.sinks {
		if serr := s.Write(rec); serr != nil {
			fmt.Fprintf(os.Stderr, "beekeeper audit: sink error: %v\n", serr)
		}
	}

	return nil
}

// Close closes the underlying audit-log file and all additional sinks.
func (w *Writer) Close() error {
	fileErr := w.file.Close()
	for _, s := range w.sinks {
		if serr := s.Close(); serr != nil {
			fmt.Fprintf(os.Stderr, "beekeeper audit: sink close error: %v\n", serr)
		}
	}
	return fileErr
}
