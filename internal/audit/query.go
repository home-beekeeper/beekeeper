package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// QueryOpts controls the streaming filter applied by Query.
type QueryOpts struct {
	Since    time.Time // zero value = no lower bound
	Agent    string    // empty = no filter
	Tool     string    // empty = no filter
	Decision string    // empty = no filter (allow|warn|block)
	Limit    int       // 0 = no limit
}

// Query streams NDJSON lines from r, applies the filters in opts, and writes
// matching raw lines to out. It never re-marshals records — the raw line bytes
// are forwarded verbatim so downstream tooling can re-parse without loss.
//
// Malformed lines are silently skipped; a summary count is printed after the
// loop when any lines were skipped. Query returns nil unless a context
// cancellation or write error occurs.
func Query(ctx context.Context, r io.Reader, opts QueryOpts, out io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	skipped := 0
	written := 0
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum%100 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		rawLine := scanner.Bytes()

		var rec AuditRecord
		if err := json.Unmarshal(rawLine, &rec); err != nil {
			skipped++
			continue
		}

		if !filterRecord(rec, opts) {
			continue
		}

		if _, err := out.Write(rawLine); err != nil {
			return fmt.Errorf("write audit record: %w", err)
		}
		if _, err := out.Write([]byte{'\n'}); err != nil {
			return fmt.Errorf("write audit record newline: %w", err)
		}
		written++

		if opts.Limit > 0 && written >= opts.Limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan audit log: %w", err)
	}

	if skipped > 0 {
		fmt.Fprintf(out, "# %d malformed line(s) skipped\n", skipped)
	}

	return nil
}

// filterRecord returns true when rec passes all non-zero filters in opts.
func filterRecord(rec AuditRecord, opts QueryOpts) bool {
	if !opts.Since.IsZero() {
		ts, err := time.Parse(time.RFC3339, rec.Timestamp)
		if err != nil || ts.Before(opts.Since) {
			return false
		}
	}
	if opts.Agent != "" && rec.AgentName != opts.Agent {
		return false
	}
	if opts.Tool != "" && rec.ToolName != opts.Tool {
		return false
	}
	if opts.Decision != "" && rec.Decision != opts.Decision {
		return false
	}
	return true
}
