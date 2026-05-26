package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mzansi-agentive/beekeeper/internal/platform"
)

// Writer is an append-only NDJSON sink for AuditRecords. It enforces owner-only
// file permissions (0600 on Unix, owner-only DACL on Windows) on open and
// re-applies them after every write so that a recreated or externally-modified
// file is never left world-readable (Pitfall 5).
type Writer struct {
	path string
	file *os.File
}

// NewWriter opens (creating if needed) the audit log at path for appending and
// enforces owner-only permissions immediately. The parent directory is created
// with 0700 if absent. The file is opened with O_APPEND|O_CREATE|O_WRONLY:
// O_APPEND avoids truncating or recreating an existing log (Pitfall 5), so prior
// decision records are never lost.
func NewWriter(path string) (*Writer, error) {
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

	return &Writer{path: path, file: f}, nil
}

// Write marshals rec to a single NDJSON line and appends it to the audit log,
// then re-applies owner-only permissions. Permissions are re-enforced on every
// write so that if the file was recreated or its DACL reset between writes it is
// re-locked before the next record is observed (Pitfall 5).
//
// Write returns any error verbatim and never downgrades a decision on failure —
// the hook handler owns the fail-closed semantics for audit-write errors.
func (w *Writer) Write(rec AuditRecord) error {
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

	return nil
}

// Close closes the underlying audit-log file.
func (w *Writer) Close() error {
	return w.file.Close()
}
