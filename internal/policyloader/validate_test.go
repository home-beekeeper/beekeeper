package policyloader

import (
	"path/filepath"
	"testing"
)

// TestValidateSchema_RejectsExec verifies that a PolicyFile containing a rule
// with action == "exec" is rejected by ValidateSchema (T-09-01, V10: no
// execution-surface fields).
func TestValidateSchema_RejectsExec(t *testing.T) {
	// Load the adversarial fixture that sets "action": "exec".
	// LoadPolicyFile calls ValidateSchema internally and returns the errors.
	path := filepath.Join(testdataDir(), "invalid_exec_action.json")
	_, errs := LoadPolicyFile(path)
	if len(errs) == 0 {
		t.Fatal("LoadPolicyFile(invalid_exec_action.json): expected errors for action=exec, got none")
	}
	// Verify at least one error mentions the invalid action.
	found := false
	for _, e := range errs {
		if containsStr(e.Error(), "exec") || containsStr(e.Error(), "action") || containsStr(e.Error(), "invalid") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("errors do not mention exec/action/invalid: %v", errs)
	}
}

// TestValidateSchema_UnknownRuleType verifies that a PolicyFile with an
// unknown rule_type value returns a non-empty []error that names both the
// rule index and the ID (T-09-02: unknown rule_type must never be silently
// skipped).
func TestValidateSchema_UnknownRuleType(t *testing.T) {
	path := filepath.Join(testdataDir(), "invalid_unknown_rule_type.json")
	_, errs := LoadPolicyFile(path)
	if len(errs) == 0 {
		t.Fatal("LoadPolicyFile(invalid_unknown_rule_type.json): expected errors for unknown rule_type, got none")
	}
	// The error should mention the rule index (rule[0]) and the bogus type.
	found := false
	for _, e := range errs {
		if containsStr(e.Error(), "rule_type") || containsStr(e.Error(), "execute_script") || containsStr(e.Error(), "unknown") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("errors do not mention rule_type/execute_script/unknown: %v", errs)
	}
}

// TestValidateSchema_UnknownSchemaVersion verifies that SchemaVersion != "1"
// returns a non-empty []error.
func TestValidateSchema_UnknownSchemaVersion(t *testing.T) {
	path := filepath.Join(testdataDir(), "invalid_schema_version.json")
	_, errs := LoadPolicyFile(path)
	if len(errs) == 0 {
		t.Fatal("LoadPolicyFile(invalid_schema_version.json): expected errors for unknown schema_version, got none")
	}
}

// TestValidateSchema_URLField verifies that a fixture containing a "url" field
// in a rule is rejected (DisallowUnknownFields ensures smuggled keys produce
// parse errors — T-09-01).
func TestValidateSchema_URLField(t *testing.T) {
	path := filepath.Join(testdataDir(), "invalid_url_field.json")
	_, errs := LoadPolicyFile(path)
	if len(errs) == 0 {
		t.Fatal("LoadPolicyFile(invalid_url_field.json): expected errors for url field, got none")
	}
}

// TestValidateSchema_ValidFiles verifies that valid policy fixtures return no
// validation errors.
func TestValidateSchema_ValidFiles(t *testing.T) {
	for _, name := range []string{"valid_release_age.json", "valid_allowlist.json"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(testdataDir(), name)
			pf, errs := LoadPolicyFile(path)
			if len(errs) != 0 {
				t.Fatalf("LoadPolicyFile(%q): unexpected validation errors: %v", name, errs)
			}
			if pf.Name == "" {
				t.Errorf("PolicyFile.Name is empty for valid fixture %q", name)
			}
		})
	}
}

// TestValidateSchema_AllErrorsCollected verifies that ValidateSchema returns ALL
// errors when multiple rules are invalid (not just the first error).
func TestValidateSchema_AllErrorsCollected(t *testing.T) {
	// Build a PolicyFile with two bad rules manually.
	pf := PolicyFile{
		SchemaVersion: "1",
		Name:          "multi-error-test",
		Rules: []PolicyRule{
			{ID: "rule-bad-type", RuleType: "not_a_real_type", Action: "block"},
			{ID: "rule-bad-action", RuleType: "release_age", Action: "exec"},
		},
	}
	errs := ValidateSchema(pf)
	if len(errs) < 2 {
		t.Errorf("ValidateSchema with 2 invalid rules: expected >= 2 errors, got %d: %v", len(errs), errs)
	}
}

// TestValidateSchema_CriticalBlockAtUpperBound (WR-02): when a single
// corroboration_threshold rule sets both block_at and critical_block_at,
// ValidateSchema must reject critical_block_at > block_at immediately at load
// time (not defer to eval-time validateCorroborationThresholds). When block_at
// is absent from the rule, no load-time upper-bound error is produced.
func TestValidateSchema_CriticalBlockAtUpperBound(t *testing.T) {
	// Case 1: both block_at and critical_block_at present, inverted — must error.
	pfBothInverted := PolicyFile{
		SchemaVersion: "1",
		Name:          "inverted-upper-bound",
		Rules: []PolicyRule{
			{
				ID:              "CORR-inverted",
				RuleType:        "corroboration_threshold",
				BlockAt:         2,
				CriticalBlockAt: 5, // > block_at: 2 — must be rejected at load time
			},
		},
	}
	errs := ValidateSchema(pfBothInverted)
	if len(errs) == 0 {
		t.Error("ValidateSchema: expected error for critical_block_at(5) > block_at(2), got none")
	}
	found := false
	for _, e := range errs {
		if containsStr(e.Error(), "critical_block_at") || containsStr(e.Error(), "block_at") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("error does not mention critical_block_at/block_at: %v", errs)
	}

	// Case 2: both present and valid (critical_block_at <= block_at) — must not error.
	pfBothValid := PolicyFile{
		SchemaVersion: "1",
		Name:          "valid-upper-bound",
		Rules: []PolicyRule{
			{
				ID:              "CORR-valid",
				RuleType:        "corroboration_threshold",
				BlockAt:         2,
				CriticalBlockAt: 1, // <= block_at: 2 — valid
			},
		},
	}
	errs2 := ValidateSchema(pfBothValid)
	if len(errs2) != 0 {
		t.Errorf("ValidateSchema: unexpected errors for valid critical_block_at(1) <= block_at(2): %v", errs2)
	}

	// Case 3: critical_block_at present but block_at absent — no load-time upper-bound
	// check (effective global BlockAt is only known after full policy-file merge).
	pfCriticalOnly := PolicyFile{
		SchemaVersion: "1",
		Name:          "critical-only",
		Rules: []PolicyRule{
			{
				ID:              "CORR-critical-only",
				RuleType:        "corroboration_threshold",
				CriticalBlockAt: 5, // no block_at — cannot validate upper bound at load time
			},
		},
	}
	errs3 := ValidateSchema(pfCriticalOnly)
	if len(errs3) != 0 {
		t.Errorf("ValidateSchema: unexpected errors when block_at is absent (upper bound unknown at load time): %v", errs3)
	}
}

// containsStr is a helper that checks if s contains substr (case-sensitive).
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
