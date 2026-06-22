package policy

import "fmt"

// ruleRemoteSource is the rule ID for the git/remote-URL install-posture rule.
const ruleRemoteSource = "remote-source-policy"

// RemoteSourceInput carries the caller-resolved source classification for an
// install. Kind is the pkgparse.ParsedCommand.RemoteSource value: "" for a
// normal registry install, or one of "git"/"github"/"url"/"tarball"/"file".
// EvaluateRemoteSource is pure: the (string-only) classification is done by the
// caller via pkgparse; this function receives the already-resolved Kind.
type RemoteSourceInput struct {
	Ecosystem string
	Package   string // the install spec (URL/ref/path); may be "" for a bare remote spec
	Kind      string // pkgparse RemoteSource: "" = registry install
}

// RemoteSourceConfig holds an allowlist of install specs/packages that are
// exempt from the remote-source flag (always allowed). Many projects legitimately
// vendor a git or file dependency; the allowlist makes those deliberate.
type RemoteSourceConfig struct {
	Exclude []string
}

// DefaultRemoteSourceConfig returns the default: no allowlist. Git and remote-URL
// installs are flagged and WARNED on first encounter (PRD Layer 1 default
// posture), not blocked. Raising the action to block per rule/ecosystem is a
// roadmap item (deep per-rule policy editing), not part of the default posture.
func DefaultRemoteSourceConfig() RemoteSourceConfig {
	return RemoteSourceConfig{Exclude: nil}
}

// EvaluateRemoteSource is a pure function: given a caller-resolved source kind
// and an allowlist, it returns a Decision without any I/O, goroutines, globals
// mutation, or wall-clock access.
//
// Decision logic (install-posture, git/remote-URL rule):
//  1. Kind == "" (normal registry install) → allow ("no remote source").
//  2. input.Package in cfg.Exclude → allow ("remote-source allowlisted").
//  3. Otherwise → WARN (Allow:true). The install pulls from a non-registry
//     source; surface the kind so the choice is visible and deliberate.
//
// The default action is WARN, not block: a git/file/URL dependency is legitimate
// in many projects, and the default posture makes it visible rather than
// forbidden (PRD). Enforcement boundary: this rule is evaluated at the pre-exec
// hook for hooked (Tier-1) harnesses only and inherits their tier caveats;
// installs run outside a hooked tool call are observed/audited by Sentry, not
// prevented here.
//
// EvaluateRemoteSource is pure: imports only "fmt" (no time, net, os, io, sync,
// context).
func EvaluateRemoteSource(input RemoteSourceInput, cfg RemoteSourceConfig) Decision {
	// 1. Normal registry install — nothing to flag.
	if input.Kind == "" {
		return Decision{
			Allow:   true,
			Level:   "allow",
			Reason:  "no remote source",
			RuleIDs: []string{ruleRemoteSource},
		}
	}

	// 2. Allowlist exemption.
	for _, excluded := range cfg.Exclude {
		if excluded == input.Package {
			return Decision{
				Allow:   true,
				Level:   "allow",
				Reason:  "remote-source allowlisted",
				RuleIDs: []string{ruleRemoteSource},
			}
		}
	}

	// 3. Flag the non-registry source (warn, does not block).
	return Decision{
		Allow:   true,
		Level:   "warn",
		Reason:  fmt.Sprintf("install pulls from a %s source (%s); flagged for review", input.Kind, remoteSpecForReason(input.Package)),
		RuleIDs: []string{ruleRemoteSource},
	}
}

// remoteSpecForReason returns a non-empty placeholder when the resolved spec is
// empty so the reason string stays readable for a bare remote spec.
func remoteSpecForReason(pkg string) string {
	if pkg == "" {
		return "non-registry spec"
	}
	return pkg
}
