package policy

import (
	"fmt"
	"strings"
)

// ruleNetworkEgress is the rule ID for network egress policy decisions.
const ruleNetworkEgress = "network-egress-policy"

// EgressInput carries the caller-resolved outbound request attributes.
// All fields must be pre-resolved by the caller; this function performs no I/O.
type EgressInput struct {
	ToolName    string
	TargetURL   string // full URL e.g. "https://pastebin.com/raw/abc"
	PayloadSize int64  // bytes
}

// EgressConfig holds the allowlist, blocklist, and size limits for egress decisions.
type EgressConfig struct {
	AllowHosts      []string          // exact suffix matches for allowed hosts
	DenyHosts       []string          // exact suffix matches for blocked hosts
	MaxPayloadBytes int64             // default max payload size in bytes
	PerToolMaxBytes map[string]int64  // per-tool size overrides (key: ToolName)
}

// DefaultEgressConfig returns the default egress policy configuration.
// AllowHosts are common package registries and documentation domains.
// DenyHosts are paste sites, generic webhooks, and known exfil destinations.
// MaxPayloadBytes defaults to 10MB.
func DefaultEgressConfig() EgressConfig {
	return EgressConfig{
		AllowHosts: []string{
			"registry.npmjs.org",
			"pypi.org",
			"files.pythonhosted.org",
			"crates.io",
			"static.crates.io",
			"rubygems.org",
			"pkg.go.dev",
			"proxy.golang.org",
			"repo.packagist.org",
			"docs.anthropic.com",
		},
		DenyHosts: []string{
			"pastebin.com",
			"hastebin.com",
			"ghostbin.com",
			"webhook.site",
			"requestbin.com",
			"ngrok.io",
		},
		MaxPayloadBytes: 10 << 20, // 10MB
		PerToolMaxBytes: nil,
	}
}

// EvaluateEgress checks the target URL against the egress allowlist/blocklist and
// payload size limits. Pure function; no net.Dial, no DNS, no I/O.
//
// Decision order:
//  1. If PayloadSize > effective size limit → block
//  2. If host suffix-matches a DenyHost → block
//  3. If host suffix-matches an AllowHost → allow
//  4. Otherwise → warn (unknown egress is warn, not silent allow)
func EvaluateEgress(input EgressInput, cfg EgressConfig) Decision {
	host := extractHost(input.TargetURL)

	// Determine the effective size limit for this tool.
	limit := cfg.MaxPayloadBytes
	if cfg.PerToolMaxBytes != nil {
		if toolLimit, ok := cfg.PerToolMaxBytes[input.ToolName]; ok {
			limit = toolLimit
		}
	}

	// 1. Check payload size first.
	if input.PayloadSize > limit {
		return Decision{
			Allow:   false,
			Level:   "block",
			Reason:  fmt.Sprintf("egress payload %d exceeds limit %d", input.PayloadSize, limit),
			RuleIDs: []string{ruleNetworkEgress},
		}
	}

	// 2. Check deny list (suffix match so subdomains are covered).
	for _, denyEntry := range cfg.DenyHosts {
		if strings.HasSuffix(host, denyEntry) {
			return Decision{
				Allow:   false,
				Level:   "block",
				Reason:  "egress to denied host: " + host,
				RuleIDs: []string{ruleNetworkEgress},
			}
		}
	}

	// 3. Check allow list (suffix match so subdomains of allowed registries pass).
	for _, allowEntry := range cfg.AllowHosts {
		if strings.HasSuffix(host, allowEntry) {
			return Decision{
				Allow:   true,
				Level:   "allow",
				Reason:  "",
				RuleIDs: []string{ruleNetworkEgress},
			}
		}
	}

	// 4. Unknown host → warn (non-blocking but surfaced).
	return Decision{
		Allow:   true,
		Level:   "warn",
		Reason:  "egress to unrecognized host: " + host,
		RuleIDs: []string{ruleNetworkEgress},
	}
}

// extractHost extracts the hostname from a URL using only strings operations.
// No net/url import — uses pure string manipulation.
// Strips scheme (http:// or https://) then takes the segment before the first "/".
func extractHost(rawURL string) string {
	s := rawURL
	// Strip scheme.
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	// Take the segment before the first "/".
	if idx := strings.Index(s, "/"); idx >= 0 {
		s = s[:idx]
	}
	// Strip port if present.
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		// Only strip port if what follows looks like a number (not IPv6 without brackets).
		rest := s[idx+1:]
		if rest != "" && !strings.Contains(rest, ":") {
			s = s[:idx]
		}
	}
	return s
}
