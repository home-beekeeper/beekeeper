package nudge

// Reason codes are a closed enum. Every reason code returned by Evaluate must
// be one of these constants. IsValidReason enforces this at test time and can
// be used in audit record validation.
//
// PRD §9 reason_code enum. New reason codes require updating both this file
// and the tests in reasons_test.go.
const (
	// ReasonPnpmAvailableSoft is returned when pnpm >= floor is installed,
	// mode is soft, and the command is a normal (non-no-arg) npm install.
	// Action: Advise.
	ReasonPnpmAvailableSoft = "pnpm-available-soft"

	// ReasonPnpmHardRewrite is returned when pnpm >= floor is installed and
	// mode is hard — the command is rewritten to its pnpm equivalent.
	// Action: Rewrite.
	ReasonPnpmHardRewrite = "pnpm-hard-rewrite"

	// ReasonBunAvailableNoScanner is returned when bun >= floor is installed but
	// @socketsecurity/bun-security-scanner is absent from bunfig.toml.
	// Action: Advise (recommend installing the scanner; proceed with npm).
	ReasonBunAvailableNoScanner = "bun-available-no-scanner"

	// ReasonBunAvailableSoft is returned when bun >= floor is installed with
	// the Socket scanner present, and mode is soft.
	// Action: Advise.
	ReasonBunAvailableSoft = "bun-available-soft"

	// ReasonNoHardenedPM is returned when no hardened PM (pnpm or bun meeting
	// the floor) is installed. Action depends on cfg.RequireHardened: Proceed
	// when false (§10-3), Block when true (§10-4).
	ReasonNoHardenedPM = "no-hardened-pm"

	// ReasonNodeIncompatiblePnpm11 is returned when pnpm 11 is installed but
	// the active Node.js version is below the 22.0.0 floor (§10-6).
	// Action: Advise.
	ReasonNodeIncompatiblePnpm11 = "node-incompatible-with-pnpm-11"

	// ReasonSudoPassthrough is returned when the install command was prefixed
	// with "sudo". Beekeeper parses and logs it but NEVER rewrites privileged
	// commands (§10-10, T-08-07).
	// Action: Advise.
	ReasonSudoPassthrough = "sudo-passthrough"

	// ReasonNoArgInstallSoft is the softer reason code for "npm install" with
	// no package argument (§10-8). A lockfile-based install already pins
	// versions, so the advisory is softer than for an explicit package install.
	// Action: Advise.
	ReasonNoArgInstallSoft = "no-arg-install-soft"

	// ReasonNotApplicable is returned when the nudge feature is disabled
	// (cfg.Enabled false) or the command is not an install-class verb
	// (cmd.IsInstall false).
	// Action: Proceed.
	ReasonNotApplicable = "not-applicable"
)

// validReasons is the complete set of legal reason codes. Use IsValidReason to
// check whether a reason code is in this set.
var validReasons = map[string]bool{
	ReasonPnpmAvailableSoft:      true,
	ReasonPnpmHardRewrite:        true,
	ReasonBunAvailableNoScanner:  true,
	ReasonBunAvailableSoft:       true,
	ReasonNoHardenedPM:           true,
	ReasonNodeIncompatiblePnpm11: true,
	ReasonSudoPassthrough:        true,
	ReasonNoArgInstallSoft:       true,
	ReasonNotApplicable:          true,
}

// IsValidReason reports whether code is one of the defined closed-enum reason
// codes. It mirrors the legalRuleTypes / legalActions pattern from
// internal/policyloader/validate.go.
func IsValidReason(code string) bool {
	return validReasons[code]
}
