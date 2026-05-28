package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// OTLPSink batches AuditRecords and flushes them as OTLP LogsData JSON to the
// configured endpoint via HTTP POST. Batches are flushed when they reach 100
// records or when Close is called. Flush errors are logged to stderr and
// treated as fire-and-forget — they are never returned to the caller — so a
// remote collector outage does not affect the local file audit trail.
type OTLPSink struct {
	endpoint string
	client   *http.Client
	mu       sync.Mutex
	batch    []AuditRecord
}

// NewOTLPSink returns an OTLPSink that POSTs to endpoint.
func NewOTLPSink(endpoint string) *OTLPSink {
	return &OTLPSink{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
		batch:    make([]AuditRecord, 0, 100),
	}
}

// Write appends rec to the current batch. If the batch reaches 100 records it
// is flushed immediately (fire-and-forget on flush error).
func (s *OTLPSink) Write(rec AuditRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batch = append(s.batch, rec)
	if len(s.batch) >= 100 {
		return s.flushLocked()
	}
	return nil
}

// Close flushes any remaining records.
func (s *OTLPSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.batch) > 0 {
		return s.flushLocked()
	}
	return nil
}

// flushLocked builds an OTLP LogsData envelope and POSTs it to the endpoint.
// Must be called with s.mu held.
func (s *OTLPSink) flushLocked() error {
	logRecords := make([]otlpLogRecord, 0, len(s.batch))
	for _, rec := range s.batch {
		var nanos int64
		if ts, err := time.Parse(time.RFC3339, rec.Timestamp); err == nil {
			nanos = ts.UnixNano()
		}
		body, _ := json.Marshal(rec)
		lr := otlpLogRecord{
			TimeUnixNano: strconv.FormatInt(nanos, 10),
			Body:         otlpStringVal{StringValue: string(body)},
			Attributes: []otlpKV{
				{Key: "beekeeper.decision", Value: otlpStringVal{StringValue: rec.Decision}},
				{Key: "beekeeper.tool_name", Value: otlpStringVal{StringValue: rec.ToolName}},
				{Key: "beekeeper.agent_name", Value: otlpStringVal{StringValue: rec.AgentName}},
			},
		}
		logRecords = append(logRecords, lr)
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

	data, err := json.Marshal(payload)
	// Reset batch regardless of outcome so we do not re-send on the next flush.
	s.batch = s.batch[:0]
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper otlp sink: marshal error: %v\n", err)
		return nil
	}

	req, err := http.NewRequest(http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper otlp sink: request error: %v\n", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper otlp sink: flush error: %v\n", err)
		return nil
	}
	resp.Body.Close()
	return nil
}
