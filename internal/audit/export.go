package audit

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ExportOpts controls format and record filtering for Export.
type ExportOpts struct {
	Format string // "ndjson" | "csv" | "otlp"
	QueryOpts
}

// Export reads NDJSON records from r, applies the filters embedded in opts, and
// writes matching records to out in the requested format. Supported formats:
//
//   - "ndjson" — raw NDJSON lines (delegates to Query)
//   - "csv"    — RFC 4180 CSV with a fixed header row
//   - "otlp"   — OpenTelemetry Logs OTLP/JSON envelope
//
// Unknown formats return an error immediately without reading from r.
func Export(ctx context.Context, r io.Reader, opts ExportOpts, out io.Writer) error {
	switch opts.Format {
	case "ndjson":
		return Query(ctx, r, opts.QueryOpts, out)
	case "csv":
		return exportCSV(ctx, r, opts, out)
	case "otlp":
		return exportOTLP(ctx, r, opts, out)
	default:
		return fmt.Errorf("export: unknown format %q (want ndjson, csv, or otlp)", opts.Format)
	}
}

func exportCSV(ctx context.Context, r io.Reader, opts ExportOpts, out io.Writer) error {
	w := csv.NewWriter(out)

	header := []string{
		"record_type", "record_id", "timestamp", "scanner_name",
		"agent_name", "tool_name", "decision", "reason", "rule_ids", "endpoint",
	}
	if err := w.Write(header); err != nil {
		return fmt.Errorf("csv: write header: %w", err)
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum%100 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		var rec AuditRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if !filterRecord(rec, opts.QueryOpts) {
			continue
		}

		row := []string{
			rec.RecordType,
			rec.RecordID,
			rec.Timestamp,
			rec.ScannerName,
			rec.AgentName,
			rec.ToolName,
			rec.Decision,
			rec.Reason,
			strings.Join(rec.RuleIDs, "|"),
			rec.Endpoint,
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("csv: write row: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("csv: scan: %w", err)
	}

	w.Flush()
	return w.Error()
}

// otlpLogRecord is the internal representation used to build the OTLP payload.
type otlpLogRecord struct {
	TimeUnixNano string         `json:"timeUnixNano"`
	Body         otlpStringVal  `json:"body"`
	Attributes   []otlpKV       `json:"attributes"`
}

type otlpStringVal struct {
	StringValue string `json:"stringValue"`
}

type otlpKV struct {
	Key   string        `json:"key"`
	Value otlpStringVal `json:"value"`
}

func exportOTLP(ctx context.Context, r io.Reader, opts ExportOpts, out io.Writer) error {
	const maxRecords = 100_000

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	var logRecords []otlpLogRecord
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum%100 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if len(logRecords) >= maxRecords {
			break
		}

		rawLine := scanner.Bytes()

		var rec AuditRecord
		if err := json.Unmarshal(rawLine, &rec); err != nil {
			continue
		}
		if !filterRecord(rec, opts.QueryOpts) {
			continue
		}

		var nanos int64
		if ts, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
			nanos = ts.UnixNano()
		}

		lr := otlpLogRecord{
			TimeUnixNano: strconv.FormatInt(nanos, 10),
			Body:         otlpStringVal{StringValue: string(rawLine)},
			Attributes: []otlpKV{
				{Key: "beekeeper.decision", Value: otlpStringVal{StringValue: rec.Decision}},
				{Key: "beekeeper.tool_name", Value: otlpStringVal{StringValue: rec.ToolName}},
				{Key: "beekeeper.agent_name", Value: otlpStringVal{StringValue: rec.AgentName}},
			},
		}
		logRecords = append(logRecords, lr)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("otlp: scan: %w", err)
	}

	payload := map[string]any{
		"resourceLogs": []map[string]any{
			{
				"resource": map[string]any{
					"attributes": []otlpKV{
						{Key: "service.name", Value: otlpStringVal{StringValue: "beekeeper"}},
					},
				},
				"scopeLogs": []map[string]any{
					{
						"scope": map[string]string{
							"name": "beekeeper/audit",
						},
						"logRecords": logRecords,
					},
				},
			},
		},
	}

	return json.NewEncoder(out).Encode(payload)
}
