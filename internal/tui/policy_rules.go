package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// PolicyRule is a single named policy rule with an enabled/disabled toggle.
// The TUI owns this file; the Phase 9 policy-as-code engine supersedes it.
type PolicyRule struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Detail  string `json:"detail"`
	Enabled bool   `json:"enabled"`
}

// defaultPolicyRules returns the 5 locked prototype policy rules (all enabled).
func defaultPolicyRules() []PolicyRule {
	return []PolicyRule{
		{
			ID:      "corroboration",
			Label:   "corroboration",
			Detail:  "single → warn  two → enforce  three → quarantine",
			Enabled: true,
		},
		{
			ID:      "release-age",
			Label:   "release-age",
			Detail:  "1440 min (24h) · npm pypi cargo gem composer",
			Enabled: true,
		},
		{
			ID:      "lifecycle",
			Label:   "lifecycle",
			Detail:  "deny by default · allowlist 3 pkgs",
			Enabled: true,
		},
		{
			ID:      "sentry-baseline",
			Label:   "sentry baseline",
			Detail:  "day 3 of 7 · audit-only until day 7",
			Enabled: true,
		},
		{
			ID:      "llamafirewall",
			Label:   "llamafirewall",
			Detail:  "enabled · sample 1.0",
			Enabled: true,
		},
	}
}

// rulesFilePath returns the path to the TUI-owned policy rules file.
func rulesFilePath(policiesDir string) string {
	return filepath.Join(policiesDir, "tui_rules.json")
}

// writeRules marshals rules to the policy file with owner-only permissions.
// The write is atomic: data is written to a temp file then renamed into place,
// so a crash mid-write cannot leave a truncated/partial rules file.
func writeRules(policiesDir string, rules []PolicyRule) error {
	if err := os.MkdirAll(policiesDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := rulesFilePath(policiesDir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, rulesFilePath(policiesDir))
}

// LoadPolicyRules reads the TUI policy rules from <policiesDir>/tui_rules.json.
//
// On first run (file genuinely absent) the function seeds the file with the
// 5 locked prototype rules, all enabled, and returns them. On any other read
// error (e.g. a transient sharing violation on Windows) the function returns
// defaults WITHOUT writing to disk — preserving any real user toggles that may
// exist in the file. Any unmarshal error likewise returns defaults without
// overwriting (fail-soft, consistent with config.Load tolerance).
func LoadPolicyRules(policiesDir string) []PolicyRule {
	data, err := os.ReadFile(rulesFilePath(policiesDir))
	if errors.Is(err, os.ErrNotExist) {
		// Genuine first run: seed defaults and persist them.
		defaults := defaultPolicyRules()
		_ = writeRules(policiesDir, defaults) // best-effort seed; ignore error
		return defaults
	}
	if err != nil {
		// Transient or permission error on an existing file — do NOT overwrite.
		return defaultPolicyRules()
	}

	var rules []PolicyRule
	if err := json.Unmarshal(data, &rules); err != nil {
		// Malformed file — return defaults without overwriting (preserve user data).
		return defaultPolicyRules()
	}
	return rules
}

// ToggleRule loads the current rules, sets the matching rule's Enabled to the
// given value, and writes the file back with 0600 permissions. An unknown id is
// a no-op. Returns any write error.
func ToggleRule(policiesDir, id string, enabled bool) error {
	rules := LoadPolicyRules(policiesDir)
	for i := range rules {
		if rules[i].ID == id {
			rules[i].Enabled = enabled
			return writeRules(policiesDir, rules)
		}
	}
	// Unknown id — no-op (no error for the TUI path).
	return nil
}
