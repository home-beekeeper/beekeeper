// Package policyloader is the I/O-side package that loads, validates, and
// dry-run-tests declarative JSON policy files (CODE-01..04).
//
// This package is the ONLY home for policy-file I/O. The pure internal/policy
// engine is never modified. The loader parses policies/*.json into typed rules,
// validates them (via ValidateSchema), and converts them to engine inputs that
// policy.Evaluate already accepts.
//
// Architecture constraint: internal/policy must stay a pure function library
// with no I/O. All policy-file I/O lives here. policyloader imports internal/policy;
// the reverse is never true.
package policyloader

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PolicyFile is the typed in-memory representation of a loaded policies/*.json
// file. It is parsed OUTSIDE the pure engine and its rules are converted to
// engine inputs before Evaluate is called. No I/O or side effects in this struct.
type PolicyFile struct {
	SchemaVersion string       `json:"schema_version"`
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	Rules         []PolicyRule `json:"rules"`
}

// PolicyRule is one entry in PolicyFile.Rules. The rule_type field determines
// which engine inputs the rule modifies. Unknown fields are rejected at parse
// time (DisallowUnknownFields) so a smuggled "url" or "exec" key produces a
// parse error with field context (T-09-01).
type PolicyRule struct {
	ID           string   `json:"id"`
	RuleType     string   `json:"rule_type"`               // "release_age" | "package_allowlist" | "sensitive_path" | "lifecycle_script_allowlist" | "corroboration_threshold"
	Ecosystems   []string `json:"ecosystems,omitempty"`    // for multi-ecosystem rules
	Ecosystem    string   `json:"ecosystem,omitempty"`     // for single-ecosystem rules
	Packages     []string `json:"packages,omitempty"`      // for allowlist/lifecycle rules
	PathPatterns []string `json:"path_patterns,omitempty"` // for sensitive_path rules
	MinAgeHours  int      `json:"min_age_hours,omitempty"` // for release_age rules
	Action       string   `json:"action,omitempty"`        // "block" | "warn" | "allow"
	WarnAt       int      `json:"warn_at,omitempty"`       // for corroboration_threshold rules
	BlockAt      int      `json:"block_at,omitempty"`      // for corroboration_threshold rules
	QuarantineAt int      `json:"quarantine_at,omitempty"` // for corroboration_threshold rules
	Note         string   `json:"note,omitempty"`
}

// PolicyFileSummary is a lightweight summary of a loaded policy file, used by
// ListPolicyFiles to report rule counts without fully parsing and validating
// every file.
type PolicyFileSummary struct {
	Path      string
	Name      string
	RuleCount int
}

// LoadPolicyFile reads path, parses with DisallowUnknownFields (so smuggled
// "url" / "exec" keys produce a parse error — T-09-01), validates the schema
// via ValidateSchema, and returns the PolicyFile. All validation errors are
// returned together (not just the first) with file + field context for
// `policy validate` output.
//
// A missing file is treated as an error (unlike config.Load where absence means
// defaults). An explicit policy path must refer to an existing file.
func LoadPolicyFile(path string) (PolicyFile, []error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PolicyFile{}, []error{fmt.Errorf("policy file %q not found", path)}
		}
		return PolicyFile{}, []error{fmt.Errorf("read policy file %q: %w", path, err)}
	}

	// Use json.Decoder with DisallowUnknownFields so that any smuggled field
	// ("url", "exec", or any other unknown key) causes a parse error immediately.
	// This is the primary guard for T-09-01 (adversarial policy smuggling).
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var pf PolicyFile
	if err := dec.Decode(&pf); err != nil {
		return PolicyFile{}, []error{fmt.Errorf("parse policy file %q: %w", path, err)}
	}

	// Validate schema: enum-check rule_types, schema_version, action values.
	// ValidateSchema returns ALL errors (not just the first) so the user gets a
	// complete picture of what needs to be fixed (T-09-02).
	if errs := ValidateSchema(pf); len(errs) > 0 {
		// Wrap each error with the file path for context.
		wrapped := make([]error, len(errs))
		for i, e := range errs {
			wrapped[i] = fmt.Errorf("policy file %q: %w", path, e)
		}
		return PolicyFile{}, wrapped
	}

	return pf, nil
}

// ListPolicyFiles scans dir for *.json files and returns a summary of each
// (path, name, rule count). A missing directory is treated as empty — NOT an
// error (Pitfall 3: beekeeper init may not have created ~/.beekeeper/policies/ yet).
func ListPolicyFiles(dir string) ([]PolicyFileSummary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Missing policies/ directory is normal — return empty list, not error.
			return nil, nil
		}
		return nil, fmt.Errorf("list policy files in %q: %w", dir, err)
	}

	var summaries []PolicyFileSummary
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		pf, errs := LoadPolicyFile(fullPath)
		if len(errs) > 0 {
			// Skip invalid files; the caller can use policy validate for details.
			continue
		}
		summaries = append(summaries, PolicyFileSummary{
			Path:      fullPath,
			Name:      pf.Name,
			RuleCount: len(pf.Rules),
		})
	}

	return summaries, nil
}
