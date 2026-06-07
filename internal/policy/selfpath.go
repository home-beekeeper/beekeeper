package policy

import "strings"

// ruleSelfPath is the rule ID returned when a tool call is blocked because it
// targets Beekeeper's own protected files (state dir, binary).
const ruleSelfPath = "beekeeper-self-protection"

// selfProtectReason is surfaced to the agent on a self-protection block. It names
// the human-only channels so the developer knows the sanctioned way to make the
// change (none of which pass through the agent tool-call hook).
const selfProtectReason = "beekeeper self-protection: agent access to Beekeeper's own files is blocked " +
	"(human-only — edit it directly, run `beekeeper` yourself, or use `beekeeper dashboard --admin`)"

// SelfProtectConfig declares the absolute path prefixes Beekeeper protects from
// the agent. Prefixes are forward-slash-canonical; matching is case-insensitive
// (the caller passes already-resolved ToSlash paths, and these are absolute
// machine paths whose case can vary on Windows).
//
//   - ReadWritePrefixes block ANY operation (read or write) under the prefix —
//     used for Beekeeper's state directory (config, policies, audit, …), which is
//     treated as a secret.
//   - WriteOnlyPrefixes block only writes under the prefix — used for the running
//     binary, where reads are harmless but an overwrite would neuter the guard.
type SelfProtectConfig struct {
	ReadWritePrefixes []string
	WriteOnlyPrefixes []string
}

// EvaluateSelfPath reports whether resolvedPath (already canonicalized to forward
// slashes by the caller) falls under a protected prefix. isWrite distinguishes a
// write/delete operation from a read. It is a pure function: imports only
// "strings", performs no I/O, and has no side effects.
func EvaluateSelfPath(resolvedPath string, isWrite bool, cfg SelfProtectConfig) Decision {
	if resolvedPath == "" {
		return Decision{Allow: true, Level: "allow", Reason: "no self-path match"}
	}
	lower := strings.ToLower(resolvedPath)

	for _, p := range cfg.ReadWritePrefixes {
		if pathHasPrefix(lower, strings.ToLower(p)) {
			return selfBlock()
		}
	}
	if isWrite {
		for _, p := range cfg.WriteOnlyPrefixes {
			if pathHasPrefix(lower, strings.ToLower(p)) {
				return selfBlock()
			}
		}
	}
	return Decision{Allow: true, Level: "allow", Reason: "no self-path match"}
}

func selfBlock() Decision {
	return Decision{
		Allow:   false,
		Level:   "block",
		Reason:  selfProtectReason,
		RuleIDs: []string{ruleSelfPath},
	}
}

// pathHasPrefix reports whether path is prefix itself or a descendant of it,
// matching only at a path boundary so a prefix of ".../beekeeper" does not match
// a sibling ".../beekeeper-notes". Both arguments must already be lowercased and
// forward-slash-canonical.
func pathHasPrefix(path, prefix string) bool {
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return false
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}
