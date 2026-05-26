package check

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/config"
)

// BenchmarkCheck measures RunCheck latency against a realistic ~200-entry index,
// exercising the full stdin→mmap→policy→audit path. This is the empirical gate
// toward the sub-100ms p95 target (HOOK-01), run in CI on all three platforms.
func BenchmarkCheck(b *testing.B) {
	dir := b.TempDir()

	entries := make([]catalog.Entry, 0, 200)
	for i := 0; i < 199; i++ {
		entries = append(entries, catalog.Entry{
			ID:            fmt.Sprintf("synthetic-%03d", i),
			Name:          fmt.Sprintf("synthetic package %d", i),
			Ecosystem:     "npm",
			Package:       fmt.Sprintf("malicious-pkg-%03d", i),
			Versions:      []string{"1.0.0"},
			Severity:      "high",
			CatalogSource: "bumblebee",
		})
	}
	entries = append(entries, catalog.Entry{
		ID:            "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised",
		Name:          "nrwl.angular-console compromise",
		Ecosystem:     "editor-extension",
		Package:       "nrwl.angular-console",
		Versions:      []string{"18.95.0"},
		Severity:      "critical",
		CatalogSource: "bumblebee",
	})

	idxPath := filepath.Join(dir, "bumblebee.idx")
	if err := catalog.BuildIndex(idxPath, entries); err != nil {
		b.Fatalf("BuildIndex: %v", err)
	}
	auditPath := filepath.Join(dir, "audit", "beekeeper.ndjson")
	cfg := config.Config{FailMode: config.FailModeClosed}

	const payload = `{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install express@4.18.2"}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := RunCheck(context.Background(), strings.NewReader(payload), cfg, idxPath, auditPath)
		if !res.Decision.Allow {
			b.Fatalf("clean package unexpectedly blocked: %+v", res.Decision)
		}
	}
}
