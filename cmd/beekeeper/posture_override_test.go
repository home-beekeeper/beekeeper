package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// stagePostureHome points the whole Beekeeper state tree (config + audit + the
// allow-once store) at a temp dir via BEEKEEPER_HOME, which platform.StateDir
// honors under `go test`. Returns the resolved state dir.
func stagePostureHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("BEEKEEPER_HOME", home)
	stateDir, err := platform.StateDir()
	if err != nil {
		t.Fatalf("resolve state dir: %v", err)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	return stateDir
}

// readPostureOverrideRecords scans the audit log and returns every
// posture_override record as a generic map.
func readPostureOverrideRecords(t *testing.T) []map[string]any {
	t.Helper()
	auditPath, err := configAuditPath()
	if err != nil {
		t.Fatalf("resolve audit path: %v", err)
	}
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	var out []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			t.Fatalf("audit record not valid JSON: %v\nline: %s", err, raw)
		}
		if rec["record_type"] == "posture_override" {
			out = append(out, rec)
		}
	}
	return out
}

// execPostureAllow runs `beekeeper posture allow <args>` against the staged home.
func execPostureAllow(t *testing.T, args ...string) {
	t.Helper()
	cmd := newPostureAllowCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("posture allow %v: %v\noutput: %s", args, err, buf.String())
	}
}

// execPostureEnforce runs `beekeeper posture enforce <args>` against the staged home.
func execPostureEnforce(t *testing.T, args ...string) {
	t.Helper()
	cmd := newPostureEnforceCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("posture enforce %v: %v\noutput: %s", args, err, buf.String())
	}
}

// TestPostureAllowOnceWritesDistinctRecord: `posture allow <pkg> --once` records a
// one-shot token AND writes a posture_override record with action allow_once.
func TestPostureAllowOnceWritesDistinctRecord(t *testing.T) {
	stateDir := stagePostureHome(t)

	execPostureAllow(t, "left-pad", "--ecosystem", "npm", "--once", "--reason", "trying it once")

	recs := readPostureOverrideRecords(t)
	if len(recs) != 1 {
		t.Fatalf("posture_override records = %d, want 1; got %+v", len(recs), recs)
	}
	rec := recs[0]
	if rec["posture_override_action"] != "allow_once" {
		t.Errorf("posture_override_action = %v, want allow_once", rec["posture_override_action"])
	}
	if rec["posture_package"] != "left-pad" {
		t.Errorf("posture_package = %v, want left-pad", rec["posture_package"])
	}
	if rec["posture_ecosystem"] != "npm" {
		t.Errorf("posture_ecosystem = %v, want npm", rec["posture_ecosystem"])
	}
	if !strings.Contains(asString(rec["reason"]), "trying it once") {
		t.Errorf("reason = %v, want it to contain the recorded justification", rec["reason"])
	}

	// The one-shot token must actually be on disk in the staged state dir.
	if _, err := os.Stat(filepath.Join(stateDir, "posture-allow-once.json")); err != nil {
		t.Errorf("allow-once store not written: %v", err)
	}
}

// TestPostureAllowAlwaysWritesDistinctRecordAndConfig: `posture allow <pkg>
// --always --reason` appends to config.Posture.Allow (NOT package_allowlist) and
// writes a posture_override record with action allow_always.
func TestPostureAllowAlwaysWritesDistinctRecordAndConfig(t *testing.T) {
	stagePostureHome(t)

	execPostureAllow(t, "vetted-pkg", "--ecosystem", "npm", "--rule", "release-age", "--always", "--reason", "vetted by security")

	recs := readPostureOverrideRecords(t)
	if len(recs) != 1 {
		t.Fatalf("posture_override records = %d, want 1; got %+v", len(recs), recs)
	}
	rec := recs[0]
	if rec["posture_override_action"] != "allow_always" {
		t.Errorf("posture_override_action = %v, want allow_always", rec["posture_override_action"])
	}
	if rec["posture_rule"] != "release-age" {
		t.Errorf("posture_rule = %v, want release-age", rec["posture_rule"])
	}

	// The config must carry a POSTURE-SCOPED allow entry, never a package_allowlist.
	cfgPath, err := platform.ConfigPath()
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config after allow --always: %v", err)
	}
	if cfg.Posture == nil || len(cfg.Posture.Allow) != 1 {
		t.Fatalf("config.Posture.Allow = %+v, want exactly one posture-scoped allow entry", cfg.Posture)
	}
	if cfg.Posture.Allow[0].Package != "vetted-pkg" || cfg.Posture.Allow[0].Rule != "release-age" {
		t.Errorf("allow entry = %+v, want {Package:vetted-pkg, Rule:release-age}", cfg.Posture.Allow[0])
	}
	// The entry must feed PostureRuleExcludes (the posture-scoped path), proving it
	// is wired to the posture evaluators and not the general package_allowlist.
	if ex := cfg.PostureRuleExcludes("release-age", "npm"); len(ex) != 1 || ex[0] != "vetted-pkg" {
		t.Errorf("PostureRuleExcludes = %v, want [vetted-pkg]", ex)
	}
}

// TestPostureEnforceBlockWritesDistinctRecordAndConfig: `posture enforce
// release-age --block` sets the rule action AND writes a posture_override record
// with action enforce_block.
func TestPostureEnforceBlockWritesDistinctRecordAndConfig(t *testing.T) {
	stagePostureHome(t)

	execPostureEnforce(t, "release-age", "--block")

	recs := readPostureOverrideRecords(t)
	if len(recs) != 1 {
		t.Fatalf("posture_override records = %d, want 1; got %+v", len(recs), recs)
	}
	rec := recs[0]
	if rec["posture_override_action"] != "enforce_block" {
		t.Errorf("posture_override_action = %v, want enforce_block", rec["posture_override_action"])
	}
	if rec["posture_rule"] != "release-age" {
		t.Errorf("posture_rule = %v, want release-age", rec["posture_rule"])
	}

	cfgPath, err := platform.ConfigPath()
	if err != nil {
		t.Fatalf("resolve config path: %v", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config after enforce --block: %v", err)
	}
	if got := cfg.PostureRuleAction("release-age"); got != config.PostureActionBlock {
		t.Errorf("release-age action = %q, want block", got)
	}
}

// TestPostureAllowAlwaysRequiresReason: `--always` without `--reason` is rejected.
func TestPostureAllowAlwaysRequiresReason(t *testing.T) {
	stagePostureHome(t)
	cmd := newPostureAllowCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"some-pkg", "--always"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("posture allow --always with no --reason: err = nil, want a required-reason error")
	}
}

// TestPostureAllowRejectsBothModes: exactly one of --once / --always is required.
func TestPostureAllowRejectsBothModes(t *testing.T) {
	stagePostureHome(t)
	cmd := newPostureAllowCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"some-pkg", "--once", "--always", "--reason", "r"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("posture allow --once --always: err = nil, want a mutually-exclusive error")
	}
}

// TestPostureEnforceRejectsBadRule: an unknown rule is rejected fail-closed.
func TestPostureEnforceRejectsBadRule(t *testing.T) {
	stagePostureHome(t)
	cmd := newPostureEnforceCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"not-a-rule", "--block"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("posture enforce not-a-rule: err = nil, want an invalid-rule error")
	}
}

// asString coerces a JSON-decoded value to a string for assertions.
func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
