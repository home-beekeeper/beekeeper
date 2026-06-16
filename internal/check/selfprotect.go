package check

// Package check — selfprotect.go
//
// Builds the SelfProtectConfig (Beekeeper's own protected path prefixes) consumed
// by policy.EvaluateSelfPath in the check handler. Resolution is BEST-EFFORT: any
// prefix that cannot be resolved is omitted, never aborting the check — a resolver
// hiccup must not fail-closed into a developer lockout.

import (
	"os"

	"github.com/home-beekeeper/beekeeper/internal/platform"
	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// buildSelfProtectConfig resolves the canonical, absolute prefixes Beekeeper
// protects from the agent:
//   - ReadWritePrefixes: the state directory (config.json, policies/, audit/,
//     catalogs/, quarantine/, baselines/, state.json) — treated as a secret.
//   - WriteOnlyPrefixes: the running binary file — overwrite would neuter the
//     guard; reads are harmless (`go install` carries no path token, so it is
//     unaffected).
func buildSelfProtectConfig() policy.SelfProtectConfig {
	var cfg policy.SelfProtectConfig

	if dir, err := platform.StateDir(); err == nil && dir != "" {
		if c := canonicalizePath(dir); c != "" {
			cfg.ReadWritePrefixes = append(cfg.ReadWritePrefixes, c)
		}
	}

	if exe, err := os.Executable(); err == nil && exe != "" {
		// Protect the exact binary file (not its whole directory, which would
		// over-block sibling binaries in e.g. ~/go/bin).
		if c := canonicalizePath(exe); c != "" {
			cfg.WriteOnlyPrefixes = append(cfg.WriteOnlyPrefixes, c)
		}
	}

	return cfg
}
