package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/nudge"
)

// TestNudgeCheckCmd_NpmInstall verifies that `nudge check "npm install chalk"`
// produces a decision/reason/action output line.
func TestNudgeCheckCmd_NpmInstall(t *testing.T) {
	// Inject a fake DetectStateFn that returns a known state so the test is
	// deterministic (no real pnpm/bun exec on the test machine).
	prev := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		// pnpm installed, hardened, Node meets floor → expect soft advise.
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.0.0",
			PnpmHardened:  true,
			NodeVersion:   "22.0.0",
		}
	}
	defer func() { nudge.DetectStateFn = prev }()

	// Set BEEKEEPER_HOME to a temp dir so resolveConfig reads no real config.
	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	cmd := newNudgeCheckCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, []string{"npm install chalk"})
	if err != nil {
		t.Fatalf("RunE returned unexpected error: %v (stderr: %q)", err, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "decision:") {
		t.Errorf("expected 'decision:' in output, got %q", output)
	}
	if !strings.Contains(output, "reason:") {
		t.Errorf("expected 'reason:' in output, got %q", output)
	}
	if !strings.Contains(output, "action:") {
		t.Errorf("expected 'action:' in output, got %q", output)
	}
}

// TestNudgeCheckCmd_NonInstall verifies that a non-install command returns an
// error (not applicable).
func TestNudgeCheckCmd_NonInstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	cmd := newNudgeCheckCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, []string{"npm run build"})
	if err == nil {
		t.Error("RunE: expected non-nil error for non-install command, got nil")
	}
}

// TestNudgeStatusCmd verifies that `nudge status` prints human-readable PM state.
func TestNudgeStatusCmd(t *testing.T) {
	// Inject a known PM state.
	prev := nudge.DetectStateFn
	nudge.DetectStateFn = func(_ context.Context, _ nudge.Config) nudge.PMState {
		return nudge.PMState{
			PnpmInstalled: true,
			PnpmVersion:   "11.2.0",
			PnpmHardened:  true,
			NodeVersion:   "22.5.0",
		}
	}
	defer func() { nudge.DetectStateFn = prev }()

	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	cmd := newNudgeStatusCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE returned unexpected error: %v (stderr: %q)", err, errOut.String())
	}

	output := out.String()
	// Must be human-readable (not NDJSON) — check for plain-text headers.
	if !strings.Contains(output, "Package Manager State") {
		t.Errorf("expected 'Package Manager State' header, got %q", output)
	}
	if !strings.Contains(output, "pnpm:") {
		t.Errorf("expected 'pnpm:' in output, got %q", output)
	}
	if !strings.Contains(output, "Nudge Configuration") {
		t.Errorf("expected 'Nudge Configuration' header, got %q", output)
	}
	if !strings.Contains(output, "enabled:") {
		t.Errorf("expected 'enabled:' config line, got %q", output)
	}
}

// TestNudgeAuditCmd_FiltersToNudgeRecords verifies that `nudge audit --since=1h`
// filters to record_type:"nudge" records only.
func TestNudgeAuditCmd_FiltersToNudgeRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", dir)

	// Create an audit dir and seed the NDJSON log with one nudge record and one
	// policy_decision record.
	// BEEKEEPER_HOME=dir → StateDir = dir/beekeeper → AuditDir = dir/beekeeper/audit
	auditDir := filepath.Join(dir, "beekeeper", "audit")
	if err := os.MkdirAll(auditDir, 0700); err != nil {
		t.Fatalf("create audit dir: %v", err)
	}
	logPath := filepath.Join(auditDir, "beekeeper.ndjson")

	now := time.Now().UTC()
	nudgeRec := audit.AuditRecord{
		RecordType:      "nudge",
		RecordID:        "nudge-001",
		Timestamp:       now.Format(time.RFC3339),
		ScannerName:     "beekeeper",
		Decision:        "warn",
		OriginalCommand: "npm install chalk",
		NudgeAction:     "advise",
	}
	policyRec := audit.AuditRecord{
		RecordType:  "policy_decision",
		RecordID:    "policy-001",
		Timestamp:   now.Format(time.RFC3339),
		ScannerName: "beekeeper",
		Decision:    "allow",
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("create audit log: %v", err)
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(nudgeRec); err != nil {
		f.Close()
		t.Fatalf("write nudge record: %v", err)
	}
	if err := enc.Encode(policyRec); err != nil {
		f.Close()
		t.Fatalf("write policy record: %v", err)
	}
	f.Close()

	cmd := newNudgeAuditCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	// Parse flags to pick up --since default.
	if err := cmd.Flags().Parse([]string{"--since=1h"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	runErr := cmd.RunE(cmd, nil)
	if runErr != nil {
		t.Fatalf("RunE returned unexpected error: %v (stderr: %q)", runErr, errOut.String())
	}

	output := out.String()
	// Must contain the nudge record.
	if !strings.Contains(output, "nudge-001") {
		t.Errorf("expected nudge-001 record in output, got %q", output)
	}
	// Must NOT contain the policy_decision record.
	if strings.Contains(output, "policy-001") {
		t.Errorf("policy_decision record should be filtered out, but found in output: %q", output)
	}
}
