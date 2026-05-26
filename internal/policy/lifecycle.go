package policy

import "strings"

// ruleLifecycleScript is the rule ID for the lifecycle-script policy (PLCY-03).
const ruleLifecycleScript = "lifecycle-script-policy"

// LifecycleInput carries the caller-resolved script fields for a package.
// The I/O adapter (internal/catalog/registry.go) fetches registry metadata;
// the policy function receives pre-resolved booleans. This keeps EvaluateLifecycle
// pure: no I/O, no network, no time.
//
// For npm, ScriptsPresent is the subset of {"preinstall", "install", "postinstall"}
// keys present in the package.json scripts object. For other ecosystems,
// the I/O adapter sets RegistryCheckFailed=true (causing a fail-closed block)
// because lifecycle script inspection is not yet supported outside npm —
// the allowlist in ~/.beekeeper/policies/lifecycle.json is the escape hatch.
type LifecycleInput struct {
	Ecosystem           string
	Package             string
	ScriptsPresent      []string // e.g. ["preinstall", "postinstall"]
	RegistryCheckFailed bool     // true when registry fetch failed (fail closed)
}

// EvaluateLifecycle is a pure function: given caller-resolved lifecycle script
// data, it returns a Decision without any I/O, goroutines, globals mutation, or
// wall-clock access.
//
// Decision logic (PLCY-03):
//  1. If RegistryCheckFailed → block (fail-closed).
//  2. If len(ScriptsPresent) == 0 → allow ("no lifecycle scripts").
//  3. If input.Package is in allowlist → allow ("lifecycle allowlisted").
//  4. Block: reason names each script and advises adding to allowlist.
//
// allowlist is the caller-supplied per-package allowlist from
// ~/.beekeeper/policies/lifecycle.json (or project-level equivalent).
//
// EvaluateLifecycle is pure: imports only "strings" (no time, net, os, io,
// sync, context).
func EvaluateLifecycle(input LifecycleInput, allowlist []string) Decision {
	// 1. Fail closed: registry check failure.
	if input.RegistryCheckFailed {
		return Decision{
			Allow:   false,
			Level:   "block",
			Reason:  "lifecycle script check unavailable (fail-closed)",
			RuleIDs: []string{ruleLifecycleScript},
		}
	}

	// 2. No scripts present → safe.
	if len(input.ScriptsPresent) == 0 {
		return Decision{
			Allow:   true,
			Level:   "allow",
			Reason:  "no lifecycle scripts",
			RuleIDs: []string{ruleLifecycleScript},
		}
	}

	// 3. Package is in the allowlist → exempt.
	for _, allowed := range allowlist {
		if allowed == input.Package {
			return Decision{
				Allow:   true,
				Level:   "allow",
				Reason:  "lifecycle allowlisted",
				RuleIDs: []string{ruleLifecycleScript},
			}
		}
	}

	// 4. Block: lifecycle scripts present and package not allowlisted.
	scriptsJoined := strings.Join(input.ScriptsPresent, ",")
	return Decision{
		Allow:   false,
		Level:   "block",
		Reason:  "lifecycle scripts present (" + scriptsJoined + "); add package to allowlist",
		RuleIDs: []string{ruleLifecycleScript},
	}
}
