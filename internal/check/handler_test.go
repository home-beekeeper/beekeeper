package check

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/config"
)

// buildTestIndex writes a small real mmap index in dir containing the
// compromised Nx Console entry and returns the index path.
func buildTestIndex(t *testing.T, dir string) string {
	t.Helper()
	entries := []catalog.Entry{
		{
			ID:            "stepsecurity-2026-05-18-vscode-nrwl-angular-console-compromised",
			Name:          "nrwl.angular-console compromise",
			Ecosystem:     "editor-extension",
			Package:       "nrwl.angular-console",
			Versions:      []string{"18.95.0"},
			Severity:      "critical",
			CatalogSource: "bumblebee",
		},
	}
	idxPath := filepath.Join(dir, "bumblebee.idx")
	if err := catalog.BuildIndex(idxPath, entries); err != nil {
		t.Fatalf("BuildIndex: %v", err)
	}
	return idxPath
}

func closedConfig() config.Config { return config.Config{FailMode: config.FailModeClosed} }

func auditPathIn(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "audit", "beekeeper.ndjson")
}

func TestHookHandlerAllow(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install express@4.18.2"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t))

	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d", res.ExitCode, exitAllow)
	}
	if res.Decision.Level != "allow" {
		t.Fatalf("Level = %q, want allow", res.Decision.Level)
	}
	if !res.Decision.Allow {
		t.Fatal("Allow = false, want true for clean package")
	}
}

func TestCatalogMatchWarns(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Install","tool_input":{"ecosystem":"editor-extension","package":"nrwl.angular-console","version":"18.95.0"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t))

	// Phase 1: single-source catalog match is warn, NOT block — exit 0.
	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d (single-source warn does not block in Phase 1)", res.ExitCode, exitAllow)
	}
	if res.Decision.Level != "warn" {
		t.Fatalf("Level = %q, want warn", res.Decision.Level)
	}
	if !res.Decision.Allow {
		t.Fatal("Allow = false, want true for Phase 1 warn")
	}
	if len(res.Decision.CatalogMatches) == 0 {
		t.Fatal("expected at least one CatalogMatch")
	}
}

func TestFailClosedOnPanic(t *testing.T) {
	// Inject an opener that panics, exercising the top-level recover guard.
	panicOpener := func(string) (catalogIndex, error) {
		panic("boom")
	}
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)

	res := runCheck(context.Background(), stdin, closedConfig(), "ignored", auditPathIn(t), panicOpener)

	if res.Decision.Allow {
		t.Fatal("Allow = true on panic, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on panic", res.ExitCode)
	}
}

func TestTimeoutFailClosed(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)

	// Already-cancelled context: the deadline check must short-circuit to block.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := RunCheck(ctx, stdin, closedConfig(), idxPath, auditPathIn(t))

	if res.Decision.Allow {
		t.Fatal("Allow = true with cancelled context, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on timeout", res.ExitCode)
	}
	r := strings.ToLower(res.Decision.Reason)
	if !strings.Contains(r, "timeout") && !strings.Contains(r, "fail-closed") {
		t.Fatalf("Reason = %q, want it to mention timeout/fail-closed", res.Decision.Reason)
	}
}

func TestStdinCapEnforced(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)

	// Craft a syntactically valid but >1MB JSON object so decode does not fail
	// on syntax — the size cap must be what blocks it.
	var buf bytes.Buffer
	buf.WriteString(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"`)
	buf.WriteString(strings.Repeat("A", 2<<20)) // 2MB of payload
	buf.WriteString(`"}}`)

	res := RunCheck(context.Background(), &buf, closedConfig(), idxPath, auditPathIn(t))

	if res.Decision.Allow {
		t.Fatal("Allow = true on oversized stdin, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on oversized stdin", res.ExitCode)
	}
	r := strings.ToLower(res.Decision.Reason)
	if !strings.Contains(r, "1mb") && !strings.Contains(r, "cap") {
		t.Fatalf("Reason = %q, want it to mention 1MB/cap", res.Decision.Reason)
	}
}

func TestMalformedJSONFailsClosed(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader("{this is not valid json")

	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t))

	if res.Decision.Allow {
		t.Fatal("Allow = true on malformed JSON, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero on malformed JSON", res.ExitCode)
	}
}

func TestMissingIndexFailsClosed(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope", "bumblebee.idx")
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)

	res := RunCheck(context.Background(), stdin, closedConfig(), missing, auditPathIn(t))

	if res.Decision.Allow {
		t.Fatal("Allow = true with missing index, want false (fail-closed)")
	}
	if res.ExitCode == exitAllow {
		t.Fatalf("ExitCode = %d, want non-zero with missing index", res.ExitCode)
	}
}

func TestFailOpenModeAllowsOnFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope", "bumblebee.idx")
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install x"}}`)
	openCfg := config.Config{FailMode: config.FailModeOpen}

	res := RunCheck(context.Background(), stdin, openCfg, missing, auditPathIn(t))

	// fail_open deliberately reduces security: a failure ALLOWS.
	if !res.Decision.Allow {
		t.Fatal("Allow = false with fail_open + missing index, want true (reduced-security opt-in)")
	}
	if res.ExitCode != exitAllow {
		t.Fatalf("ExitCode = %d, want %d with fail_open", res.ExitCode, exitAllow)
	}
}

func TestAuditRecordWrittenOnEveryPath(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install express@4.18.2"}}`)

	RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPath)

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log not written: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		t.Fatal("audit log is empty, want one record")
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("audit record not valid JSON: %v", err)
	}
	if rec["record_type"] != "policy_decision" {
		t.Fatalf("record_type = %v, want policy_decision", rec["record_type"])
	}
}

func TestMalformedJSONStillAudits(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	auditPath := auditPathIn(t)

	RunCheck(context.Background(), strings.NewReader("{bad"), closedConfig(), idxPath, auditPath)

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("audit log not written on fail-closed path: %v", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		t.Fatal("expected a best-effort audit record on malformed-JSON fail-closed path")
	}
}

// guard against an accidentally too-short timeout: a real evaluation must
// complete well within the budget.
func TestNormalEvaluationWithinDeadline(t *testing.T) {
	dir := t.TempDir()
	idxPath := buildTestIndex(t, dir)
	stdin := strings.NewReader(`{"agent_name":"a","tool_name":"Bash","tool_input":{"command":"npm install express@4.18.2"}}`)

	start := time.Now()
	res := RunCheck(context.Background(), stdin, closedConfig(), idxPath, auditPathIn(t))
	if time.Since(start) > execTimeout {
		t.Fatal("evaluation exceeded the execution timeout for a trivial input")
	}
	if !res.Decision.Allow {
		t.Fatal("trivial clean input should allow")
	}
}
