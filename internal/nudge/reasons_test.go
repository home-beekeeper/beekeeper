package nudge

import "testing"

// TestReasonsClosedEnum verifies all expected reason codes are valid and an
// unknown string is not.
func TestReasonsClosedEnum(t *testing.T) {
	valid := []string{
		ReasonPnpmAvailableSoft,
		ReasonPnpmHardRewrite,
		ReasonBunAvailableNoScanner,
		ReasonBunAvailableSoft,
		ReasonNoHardenedPM,
		ReasonNodeIncompatiblePnpm11,
		ReasonSudoPassthrough,
		ReasonNoArgInstallSoft,
		ReasonNotApplicable,
	}
	for _, r := range valid {
		if !IsValidReason(r) {
			t.Errorf("IsValidReason(%q) = false, want true", r)
		}
	}

	invalid := []string{"", "unknown", "block", "advise", "not-a-reason"}
	for _, r := range invalid {
		if IsValidReason(r) {
			t.Errorf("IsValidReason(%q) = true, want false", r)
		}
	}
}

// TestReasonStringValues verifies the reason code string values match PRD §9.
func TestReasonStringValues(t *testing.T) {
	checks := map[string]string{
		"pnpm-available-soft":           ReasonPnpmAvailableSoft,
		"pnpm-hard-rewrite":             ReasonPnpmHardRewrite,
		"bun-available-no-scanner":      ReasonBunAvailableNoScanner,
		"bun-available-soft":            ReasonBunAvailableSoft,
		"no-hardened-pm":                ReasonNoHardenedPM,
		"node-incompatible-with-pnpm-11": ReasonNodeIncompatiblePnpm11,
		"sudo-passthrough":              ReasonSudoPassthrough,
		"no-arg-install-soft":           ReasonNoArgInstallSoft,
		"not-applicable":                ReasonNotApplicable,
	}
	for want, got := range checks {
		if got != want {
			t.Errorf("reason const = %q, want %q", got, want)
		}
	}
}
