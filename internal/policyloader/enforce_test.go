package policyloader

import (
	"testing"

	"github.com/mzansi-agentive/beekeeper/internal/policy"
)

// baseAllow is a convenient allow base decision for overlay tests.
var baseAllow = policy.Decision{Allow: true, Level: "allow", Reason: "no catalog match"}

// baseWarn is a convenient warn base decision for overlay tests.
var baseWarn = policy.Decision{Allow: true, Level: "warn", Reason: "single-source catalog match: bumblebee"}

// baseBlock is a convenient block base decision for overlay tests.
var baseBlock = policy.Decision{Allow: false, Level: "block", Reason: "corroborated catalog match: bumblebee,osv"}

// packageAllowlistAllowFile returns a PolicyFile with one package_allowlist rule
// whose action is "allow" for "react" in the "npm" ecosystem.
func packageAllowlistAllowFile() PolicyFile {
	return PolicyFile{
		SchemaVersion: "1",
		Name:          "allow-react",
		Rules: []PolicyRule{
			{
				ID:        "allow-react-rule",
				RuleType:  "package_allowlist",
				Ecosystem: "npm",
				Packages:  []string{"react", "react-dom"},
				Action:    "allow",
			},
		},
	}
}

// packageAllowlistBlockFile returns a PolicyFile with one package_allowlist rule
// whose action is "block" for "evil-pkg" in the "npm" ecosystem.
func packageAllowlistBlockFile() PolicyFile {
	return PolicyFile{
		SchemaVersion: "1",
		Name:          "block-evil",
		Rules: []PolicyRule{
			{
				ID:        "block-evil-rule",
				RuleType:  "package_allowlist",
				Ecosystem: "npm",
				Packages:  []string{"evil-pkg"},
				Action:    "block",
			},
		},
	}
}

// sensitivePathBlockFile returns a PolicyFile with one sensitive_path block rule
// matching files in /home/user/.ssh/.
func sensitivePathBlockFile() PolicyFile {
	return PolicyFile{
		SchemaVersion: "1",
		Name:          "block-ssh",
		Rules: []PolicyRule{
			{
				ID:           "block-ssh-rule",
				RuleType:     "sensitive_path",
				PathPatterns: []string{"/.ssh/"},
				Action:       "block",
			},
		},
	}
}

// corroborationThresholdFile returns a PolicyFile with only a
// corroboration_threshold rule — the overlay must NOT double-apply it.
func corroborationThresholdFile() PolicyFile {
	return PolicyFile{
		SchemaVersion: "1",
		Name:          "tight-threshold",
		Rules: []PolicyRule{
			{
				ID:           "tight",
				RuleType:     "corroboration_threshold",
				WarnAt:       1,
				BlockAt:      1,
				QuarantineAt: 2,
			},
		},
	}
}

// npmTC returns a ToolCall for an npm install of the given package.
func npmTC(pkg string) policy.ToolCall {
	return policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "Bash",
		ToolInput: map[string]any{
			"ecosystem": "npm",
			"package":   pkg,
		},
	}
}

// pathTC returns a ToolCall that targets a filesystem path.
func pathTC(path string) policy.ToolCall {
	return policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "WriteFile",
		ToolInput: map[string]any{
			"path": path,
		},
	}
}

// TestApplyPolicyOverlay_AllowRuleDowngradesWarn verifies that a
// package_allowlist "allow" rule downgrades a warn base to allow for the exact
// matching package. This is the allowlist escape-hatch (T-09-31).
func TestApplyPolicyOverlay_AllowRuleDowngradesWarn(t *testing.T) {
	files := []PolicyFile{packageAllowlistAllowFile()}
	tc := npmTC("react")

	got := ApplyPolicyOverlay(files, tc, baseWarn)

	if !got.Allow {
		t.Errorf("Allow = false, want true (allowlist allow rule must downgrade warn to allow)")
	}
	if got.Level != "allow" {
		t.Errorf("Level = %q, want %q", got.Level, "allow")
	}
	if got.Reason == "" {
		t.Error("Reason must not be empty")
	}
}

// TestApplyPolicyOverlay_AllowRuleDowngradesBlock verifies that a
// package_allowlist "allow" rule downgrades a block base to allow for the exact
// matching package (version-controlled user-trust override).
func TestApplyPolicyOverlay_AllowRuleDowngradesBlock(t *testing.T) {
	files := []PolicyFile{packageAllowlistAllowFile()}
	tc := npmTC("react")

	got := ApplyPolicyOverlay(files, tc, baseBlock)

	if !got.Allow {
		t.Errorf("Allow = false, want true (allowlist allow rule must downgrade block to allow)")
	}
	if got.Level != "allow" {
		t.Errorf("Level = %q, want %q", got.Level, "allow")
	}
}

// TestApplyPolicyOverlay_AllowRuleDoesNotAffectAlreadyAllow verifies that an
// "allow" rule on an already-allow base does not change the base (step 4).
func TestApplyPolicyOverlay_AllowRuleDoesNotAffectAlreadyAllow(t *testing.T) {
	files := []PolicyFile{packageAllowlistAllowFile()}
	tc := npmTC("react")

	got := ApplyPolicyOverlay(files, tc, baseAllow)

	// base is already allow — the rule can't downgrade further.
	if got.Level != "allow" {
		t.Errorf("Level = %q, want %q", got.Level, "allow")
	}
}

// TestApplyPolicyOverlay_BlockRuleForcesBlock verifies that a package_allowlist
// "block" rule forces a block decision even when the base engine result is allow.
func TestApplyPolicyOverlay_BlockRuleForcesBlock(t *testing.T) {
	files := []PolicyFile{packageAllowlistBlockFile()}
	tc := npmTC("evil-pkg")

	got := ApplyPolicyOverlay(files, tc, baseAllow)

	if got.Allow {
		t.Errorf("Allow = true, want false (package_allowlist block rule must force block)")
	}
	if got.Level != "block" {
		t.Errorf("Level = %q, want %q", got.Level, "block")
	}
}

// TestApplyPolicyOverlay_SensitivePathBlocksMatchingPath verifies that a
// sensitive_path block rule blocks a tool call targeting a matching path.
func TestApplyPolicyOverlay_SensitivePathBlocksMatchingPath(t *testing.T) {
	files := []PolicyFile{sensitivePathBlockFile()}
	tc := pathTC("/home/user/.ssh/id_rsa")

	got := ApplyPolicyOverlay(files, tc, baseAllow)

	if got.Allow {
		t.Errorf("Allow = true, want false (sensitive_path block rule must block matching path)")
	}
	if got.Level != "block" {
		t.Errorf("Level = %q, want %q", got.Level, "block")
	}
}

// TestApplyPolicyOverlay_SensitivePathDoesNotBlockNonMatchingPath verifies that
// a sensitive_path rule does not affect tool calls targeting non-matching paths.
func TestApplyPolicyOverlay_SensitivePathDoesNotBlockNonMatchingPath(t *testing.T) {
	files := []PolicyFile{sensitivePathBlockFile()}
	tc := pathTC("/home/user/projects/myapp/main.go")

	got := ApplyPolicyOverlay(files, tc, baseAllow)

	if !got.Allow {
		t.Errorf("Allow = false, want true (non-matching path must not be blocked)")
	}
}

// TestApplyPolicyOverlay_NonMatchingRulesLeaveBaseUnchanged verifies that rules
// for different packages or ecosystems leave the base decision unchanged.
func TestApplyPolicyOverlay_NonMatchingRulesLeaveBaseUnchanged(t *testing.T) {
	files := []PolicyFile{packageAllowlistBlockFile()} // blocks "evil-pkg"
	tc := npmTC("innocent-pkg")                        // different package

	got := ApplyPolicyOverlay(files, tc, baseAllow)

	if !got.Allow {
		t.Errorf("Allow = false, want true (non-matching rule must not affect base)")
	}
	if got.Level != "allow" {
		t.Errorf("Level = %q, want %q (base unchanged)", got.Level, "allow")
	}
	if got.Reason != baseAllow.Reason {
		t.Errorf("Reason = %q, want %q (base reason unchanged)", got.Reason, baseAllow.Reason)
	}
}

// TestApplyPolicyOverlay_CorroborationThresholdNotDoubleApplied verifies that
// corroboration_threshold rules in policy files are NOT re-applied by the
// overlay (they are already handled by thresholdsFromPolicyFile in the engine).
// An allow base with only a corroboration_threshold rule must remain allow.
func TestApplyPolicyOverlay_CorroborationThresholdNotDoubleApplied(t *testing.T) {
	files := []PolicyFile{corroborationThresholdFile()}
	tc := npmTC("some-package")

	// The engine returned allow (no catalog match). The overlay must NOT
	// re-evaluate the corroboration threshold and produce a different result.
	got := ApplyPolicyOverlay(files, tc, baseAllow)

	if !got.Allow {
		t.Errorf("Allow = false, want true (corroboration_threshold must not be re-applied by overlay)")
	}
	if got.Level != "allow" {
		t.Errorf("Level = %q, want %q", got.Level, "allow")
	}
}

// TestApplyPolicyOverlay_EmptyFilesReturnsBase verifies that an empty policy
// file list returns the base decision unchanged.
func TestApplyPolicyOverlay_EmptyFilesReturnsBase(t *testing.T) {
	got := ApplyPolicyOverlay(nil, npmTC("any-pkg"), baseBlock)

	if got.Level != "block" {
		t.Errorf("Level = %q, want %q (empty files must return base unchanged)", got.Level, "block")
	}
	if got.Reason != baseBlock.Reason {
		t.Errorf("Reason changed with empty files: %q", got.Reason)
	}
}

// TestApplyPolicyOverlay_WarnRuleElevatesAllowBase verifies that a
// package_allowlist warn rule elevates an allow base to warn.
func TestApplyPolicyOverlay_WarnRuleElevatesAllowBase(t *testing.T) {
	warnFile := PolicyFile{
		SchemaVersion: "1",
		Name:          "warn-lodash",
		Rules: []PolicyRule{
			{
				ID:        "warn-lodash-rule",
				RuleType:  "package_allowlist",
				Ecosystem: "npm",
				Packages:  []string{"lodash"},
				Action:    "warn",
			},
		},
	}
	tc := npmTC("lodash")

	got := ApplyPolicyOverlay([]PolicyFile{warnFile}, tc, baseAllow)

	if !got.Allow {
		t.Errorf("Allow = false, want true (warn must not block)")
	}
	if got.Level != "warn" {
		t.Errorf("Level = %q, want %q", got.Level, "warn")
	}
}

// TestApplyPolicyOverlay_CommandShapeExtracted verifies that a package embedded
// in a command-shape ToolCall is correctly matched by a package_allowlist rule.
func TestApplyPolicyOverlay_CommandShapeExtracted(t *testing.T) {
	files := []PolicyFile{packageAllowlistBlockFile()}
	tc := policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "Bash",
		ToolInput: map[string]any{
			"command": "npm install evil-pkg",
		},
	}

	got := ApplyPolicyOverlay(files, tc, baseAllow)

	if got.Allow {
		t.Errorf("Allow = true, want false (command-shape package must match block rule)")
	}
	if got.Level != "block" {
		t.Errorf("Level = %q, want %q", got.Level, "block")
	}
}

// TestApplyPolicyOverlay_MultiEcosystemRule verifies that a rule with
// multiple ecosystems in the Ecosystems slice matches correctly.
func TestApplyPolicyOverlay_MultiEcosystemRule(t *testing.T) {
	multiFile := PolicyFile{
		SchemaVersion: "1",
		Name:          "multi-eco",
		Rules: []PolicyRule{
			{
				ID:         "block-multi-eco",
				RuleType:   "package_allowlist",
				Ecosystems: []string{"npm", "pypi"},
				Packages:   []string{"badpkg"},
				Action:     "block",
			},
		},
	}

	// Should block for npm.
	gotNPM := ApplyPolicyOverlay([]PolicyFile{multiFile}, npmTC("badpkg"), baseAllow)
	if gotNPM.Allow {
		t.Errorf("[npm] Allow = true, want false (multi-eco rule must match npm)")
	}

	// Should block for pypi.
	pypiTC := policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "Bash",
		ToolInput: map[string]any{
			"ecosystem": "pypi",
			"package":   "badpkg",
		},
	}
	gotPyPI := ApplyPolicyOverlay([]PolicyFile{multiFile}, pypiTC, baseAllow)
	if gotPyPI.Allow {
		t.Errorf("[pypi] Allow = true, want false (multi-eco rule must match pypi)")
	}

	// Should NOT block for go.
	goTC := policy.ToolCall{
		AgentName: "test-agent",
		ToolName:  "Bash",
		ToolInput: map[string]any{
			"ecosystem": "go",
			"package":   "badpkg",
		},
	}
	gotGo := ApplyPolicyOverlay([]PolicyFile{multiFile}, goTC, baseAllow)
	if !gotGo.Allow {
		t.Errorf("[go] Allow = false, want true (multi-eco rule must not match go)")
	}
}

// TestLoadPolicyDir_MissingDir verifies that a missing directory returns an
// empty slice and no error.
func TestLoadPolicyDir_MissingDir(t *testing.T) {
	files, err := LoadPolicyDir("testdata/nonexistent_dir_for_overlay_test")
	if err != nil {
		t.Fatalf("LoadPolicyDir(missing dir): unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("LoadPolicyDir(missing dir): expected empty slice, got %d files", len(files))
	}
}

// TestLoadPolicyDir_ValidDir verifies that a directory with valid policy files
// returns PolicyFile structs.
func TestLoadPolicyDir_ValidDir(t *testing.T) {
	files, err := LoadPolicyDir("testdata")
	if err != nil {
		t.Fatalf("LoadPolicyDir(testdata): unexpected error: %v", err)
	}
	// testdata has valid_allowlist.json and valid_release_age.json — at least 2 valid files.
	if len(files) < 2 {
		t.Errorf("LoadPolicyDir(testdata): expected at least 2 valid files, got %d", len(files))
	}
}
