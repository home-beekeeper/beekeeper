package policyloader

import "fmt"

// SupportedSchemaVersion is the only accepted schema_version value for policy
// files. Unknown versions are rejected (not silently parsed) so a breaking
// schema change is detected immediately rather than producing silently malformed
// policies.
const SupportedSchemaVersion = "1"

// legalRuleTypes is the complete enum of valid rule_type values. Any value not
// in this set is rejected by ValidateSchema (T-09-02: unknown rule_type silently
// skipped is a security hole — default switch branch must be an error).
var legalRuleTypes = map[string]bool{
	"release_age":                  true,
	"package_allowlist":            true,
	"sensitive_path":               true,
	"lifecycle_script_allowlist":   true,
	"corroboration_threshold":      true,
}

// legalActions is the complete enum of valid action values. "exec" is
// explicitly NOT included so that ValidateSchema rejects it (T-09-01, V10).
var legalActions = map[string]bool{
	"block": true,
	"warn":  true,
	"allow": true,
}

// ValidateSchema checks pf for structural correctness:
//   - schema_version must equal SupportedSchemaVersion
//   - each rule's rule_type must be one of the five legal values
//   - each rule's action (if non-empty) must be "block", "warn", or "allow"
//     (this rejects "exec" — T-09-01)
//
// All errors are collected and returned together (not just the first) so that
// `policy validate` gives a complete picture of what needs to be fixed.
// An empty []error means the file is valid.
func ValidateSchema(pf PolicyFile) []error {
	var errs []error

	if pf.SchemaVersion != SupportedSchemaVersion {
		errs = append(errs, fmt.Errorf("unsupported schema_version %q (want %q)",
			pf.SchemaVersion, SupportedSchemaVersion))
	}

	for i, r := range pf.Rules {
		if !legalRuleTypes[r.RuleType] {
			errs = append(errs, fmt.Errorf("rule[%d] %q: unknown rule_type %q",
				i, r.ID, r.RuleType))
		}
		if r.Action != "" && !legalActions[r.Action] {
			errs = append(errs, fmt.Errorf("rule[%d] %q: invalid action %q (want \"block\", \"warn\", or \"allow\")",
				i, r.ID, r.Action))
		}
	}

	return errs
}
