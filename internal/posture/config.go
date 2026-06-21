package posture

// PURE — imports only stdlib (none currently). This is the read-only
// package-manager-posture configuration: the subset of knobs the detection
// readers (detect.go / scanners.go) need to report each PM's installed version
// and hardening state side-by-side with Beekeeper's enforced posture.
//
// History: this package is the relocated, steering-free remnant of the former
// internal/nudge feature (removed in v1.1.0). Only the read-only detection of
// installed PM versions and their config files (minimumReleaseAge, the bun
// socket scanner, pnpm-workspace hardening) survives here to power the Layer-2
// `beekeeper posture` view. None of the steer-to-pnpm/bun decision logic moved.

// minimumReleaseAgeWeaknessBaseline is the pnpm 11 default minimumReleaseAge of
// 1440 minutes (1 day) [pnpm.io/blog/releases/11.0, pnpm.io/settings]. A value
// materially below 1440 is the weakness signal in hardening-weakness detection
// (scanners.go). Beekeeper does NOT itself set minimumReleaseAge — this baseline
// is only used for the read-only hardening-weakness comparison.
const minimumReleaseAgeWeaknessBaseline = 1440

// VersionFloors holds the minimum version floors for each supported package
// manager, used only to compute the read-only "hardened" flag in the posture
// view (pnpm version meets floor → pnpm_hardened candidate).
type VersionFloors struct {
	// Pnpm is the minimum acceptable pnpm version, e.g. "11.0.0".
	Pnpm string
	// Bun is the minimum acceptable bun version, e.g. "1.3.0".
	Bun string
	// Node is the minimum Node.js version required for pnpm 11, e.g. "22.0.0".
	Node string
}

// Config is the pure read-only posture-detection configuration. It carries no
// I/O state — every DetectState call receives it as a value. It is the trimmed
// remnant of the former nudge.Config: only the fields the detection readers
// consume survive (no Mode / RequireHardened / Preferred / drift steering knobs).
type Config struct {
	// CheckSocketScanner controls whether bun is only "hardened" when
	// @socketsecurity/bun-security-scanner is present in bunfig.toml.
	CheckSocketScanner bool
	// VersionFloors are the minimum acceptable versions for pnpm/bun/node, used
	// to compute the read-only hardened flag.
	VersionFloors VersionFloors
}

// DefaultConfig returns the default posture-detection configuration.
//
// VersionFloors: pnpm 11.0.0, bun 1.3.0, node 22.0.0 (pnpm 11 requires Node 22+).
func DefaultConfig() Config {
	return Config{
		CheckSocketScanner: true,
		VersionFloors: VersionFloors{
			Pnpm: "11.0.0",
			Bun:  "1.3.0",
			Node: "22.0.0",
		},
	}
}
