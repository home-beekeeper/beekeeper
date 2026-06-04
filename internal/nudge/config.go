package nudge

// PURE — imports only stdlib (no internal/config to avoid import cycle).
// internal/config defines NudgeConfig (config struct with JSON tags) and
// internal/nudge defines Config (pure decision config). They are deliberately
// separate structs. ConfigFrom is the single mapper that callers (Plans 06/08)
// use to convert the loose primitive values destructured from config.NudgeConfig
// into a nudge.Config — this is the single-mapper rule (Flag 4, BLOCKER 1).

// minimumReleaseAgeWeaknessBaseline is the Flag 5 correction (NOT 60).
// pnpm 11 ships minimumReleaseAge: 1440 (1 day) by default [VERIFIED:
// pnpm.io/blog/releases/11.0, pnpm.io/settings]. A value materially below
// 1440 minutes is the weakness signal in hardening-weakness detection
// (detect.go / scanners.go). Beekeeper does NOT itself set minimumReleaseAge
// — this baseline is only used for the hardening-weakness comparison.
const minimumReleaseAgeWeaknessBaseline = 1440

// DriftCheckConfig holds the periodic major-version drift check settings.
type DriftCheckConfig struct {
	// Enabled controls whether the weekly pnpm/bun major-version drift check runs.
	Enabled bool
	// Interval is the time between drift checks as a Go duration string (e.g. "168h").
	Interval string
}

// VersionFloors holds the minimum version floors for each supported package manager.
type VersionFloors struct {
	// Pnpm is the minimum acceptable pnpm version, e.g. "11.0.0".
	Pnpm string
	// Bun is the minimum acceptable bun version, e.g. "1.3.0".
	Bun string
	// Node is the minimum Node.js version required for pnpm 11, e.g. "22.0.0".
	// Note (Flag 5): Node 24 is the current Active LTS (Node 22 is Maintenance
	// LTS through 2027-04, still supported). The floor remains 22.0.0 because
	// pnpm 11 requires Node 22+; the recommended target for new setups is Node 24.
	Node string
}

// Config is the pure nudge decision configuration. It carries no I/O state —
// every Evaluate call receives it as a value. It mirrors the shape of
// policy.ReleaseAgeConfig: a plain struct, no methods.
//
// To convert a config.NudgeConfig into a nudge.Config, use ConfigFrom.
// Do NOT import internal/config here (import cycle: config imports nudge in
// Plan 06/08 consumers).
type Config struct {
	// Enabled controls whether nudge evaluation runs at all (NUDGE-08 layered
	// disable). When false, Evaluate immediately returns Proceed/not-applicable.
	Enabled bool
	// Mode is "soft" (advise + proceed, default) or "hard" (rewrite the command).
	Mode string
	// RequireHardened, when true, blocks npm install when no hardened PM is
	// installed. Default false.
	RequireHardened bool
	// Preferred is the preferred hardened PM when both pnpm and bun are
	// available: "pnpm" (default) or "bun".
	Preferred string
	// CheckSocketScanner controls whether bun is only "hardened" when
	// @socketsecurity/bun-security-scanner is present in bunfig.toml.
	CheckSocketScanner bool
	// MajorDriftCheck holds the periodic drift check config (PRD §7.1).
	MajorDriftCheck DriftCheckConfig
	// VersionFloors are the minimum acceptable versions for pnpm/bun/node.
	VersionFloors VersionFloors
}

// DefaultConfig returns the PRD §5.1 default nudge configuration.
// Mirrors DefaultReleaseAgeConfig() in internal/policy/release_age.go.
//
// Flag 5 corrections baked in:
//   - VersionFloors: pnpm 11.0.0, bun 1.3.0, node 22.0.0 (floors unchanged)
//   - Node 24 is the recommended target for new setups (comment above in
//     VersionFloors.Node); the floor stays 22.0.0 (pnpm 11 requires Node 22+)
func DefaultConfig() Config {
	return Config{
		Enabled:            true,
		Mode:               "soft",
		RequireHardened:    false,
		Preferred:          "pnpm",
		CheckSocketScanner: true,
		MajorDriftCheck: DriftCheckConfig{
			Enabled:  true,
			Interval: "168h",
		},
		VersionFloors: VersionFloors{
			Pnpm: "11.0.0",
			Bun:  "1.3.0",
			Node: "22.0.0",
		},
	}
}

// ConfigFrom is the SINGLE config→nudge.Config mapper consumed by Plans 06 and
// 08 (check/gateway/shim/CLI). It takes the primitive field values that the
// caller destructures out of config.NudgeConfig, so internal/nudge does NOT
// import internal/config (no cycle — BLOCKER 1 closed).
//
// The single-mapper rule (Flag 4): all consumers call ONE mapper; there is no
// per-consumer copy that could drift.
//
// Fallback rules (T-08-09c — ConfigFrom never returns an empty floor that
// would break meetsFloor):
//   - empty mode → DefaultConfig().Mode ("soft")
//   - empty preferred → DefaultConfig().Preferred ("pnpm")
//   - empty floor strings → the corresponding DefaultConfig() floors
//   - empty driftInterval → DefaultConfig().MajorDriftCheck.Interval ("168h")
//
// The enabled, checkScanner, and driftEnabled booleans are passed through
// verbatim — a layered disable (enabled=false, NUDGE-08) maps faithfully.
func ConfigFrom(
	enabled bool,
	mode, preferred string,
	checkScanner bool,
	floorPnpm, floorBun, floorNode string,
	driftEnabled bool,
	driftInterval string,
) Config {
	def := DefaultConfig()

	// Apply documented fallbacks for empty strings.
	if mode == "" {
		mode = def.Mode
	}
	if preferred == "" {
		preferred = def.Preferred
	}
	if floorPnpm == "" {
		floorPnpm = def.VersionFloors.Pnpm
	}
	if floorBun == "" {
		floorBun = def.VersionFloors.Bun
	}
	if floorNode == "" {
		floorNode = def.VersionFloors.Node
	}
	if driftInterval == "" {
		driftInterval = def.MajorDriftCheck.Interval
	}

	return Config{
		Enabled:            enabled,
		Mode:               mode,
		RequireHardened:    def.RequireHardened, // not exposed as a primitive arg; callers override via Config literal when needed
		Preferred:          preferred,
		CheckSocketScanner: checkScanner,
		MajorDriftCheck: DriftCheckConfig{
			Enabled:  driftEnabled,
			Interval: driftInterval,
		},
		VersionFloors: VersionFloors{
			Pnpm: floorPnpm,
			Bun:  floorBun,
			Node: floorNode,
		},
	}
}
