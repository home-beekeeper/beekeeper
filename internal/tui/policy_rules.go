package tui

import (
	"encoding/json"
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
func writeRules(policiesDir string, rules []PolicyRule) error {
	if err := os.MkdirAll(policiesDir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(rulesFilePath(policiesDir), data, 0600)
}

// LoadPolicyRules reads the TUI policy rules from <policiesDir>/tui_rules.json.
//
// On first run (file or directory absent) the function seeds the file with the
// 5 locked prototype rules, all enabled, and returns them. Any read or unmarshal
// error also returns the seeded defaults — the dashboard never panics on bad
// policy data (fail-soft, consistent with config.Load tolerance).
func LoadPolicyRules(policiesDir string) []PolicyRule {
	data, err := os.ReadFile(rulesFilePath(policiesDir))
	if err != nil {
		// File absent or unreadable — seed defaults and return them.
		defaults := defaultPolicyRules()
		_ = writeRules(policiesDir, defaults) // best-effort seed; ignore error
		return defaults
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
