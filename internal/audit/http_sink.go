package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// HTTPSink delivers each AuditRecord as an individual HTTPS POST to the
// configured endpoint. The body is a single NDJSON line. Errors are treated as
// fire-and-forget: they are logged to stderr but never returned to the caller
// so a remote endpoint outage does not affect the local file audit trail.
type HTTPSink struct {
	endpoint string
	client   *http.Client
}

// NewHTTPSink returns an HTTPSink that POSTs each record to endpoint.
func NewHTTPSink(endpoint string) *HTTPSink {
	return &HTTPSink{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// Write serialises rec as NDJSON and POSTs it to the endpoint.
func (s *HTTPSink) Write(rec AuditRecord) error {
	data, _ := json.Marshal(rec)
	data = append(data, '\n')

	req, err := http.NewRequest(http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper https sink: request error: %v\n", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := s.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "beekeeper https sink: post error: %v\n", err)
		return nil
	}
	resp.Body.Close()
	return nil
}

// Close is a no-op for HTTPSink; each record is sent immediately.
func (s *HTTPSink) Close() error { return nil }
