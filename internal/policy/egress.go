package policy

import (
	"fmt"
	"net/url"
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
	AllowHosts      []string         // exact suffix matches for allowed hosts
	DenyHosts       []string         // exact suffix matches for blocked hosts
	MaxPayloadBytes int64            // default max payload size in bytes
	PerToolMaxBytes map[string]int64 // per-tool size overrides (key: ToolName)
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
	host, hostAmbiguous := extractHostChecked(input.TargetURL)

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

	// 2. Check deny list. Match requires a label boundary (host == entry or a
	// "."+entry suffix) so an attacker-registrable lookalike like
	// "notpastebin.com" does NOT satisfy a deny entry of "pastebin.com" — while
	// legitimate subdomains ("raw.pastebin.com") still match.
	//
	// Fail toward deny (TM): when host extraction was ambiguous/failed (a URL
	// crafted to defeat url.Parse), ALSO run the deny-boundary check against the
	// raw URL string so a deny target wrapped to evade parsing still blocks. A
	// block must never silently collapse to a warn just because the host could
	// not be cleanly extracted.
	for _, denyEntry := range cfg.DenyHosts {
		if hostMatchesEntry(host, denyEntry) ||
			(hostAmbiguous && rawURLContainsDenyHost(input.TargetURL, denyEntry)) {
			return Decision{
				Allow:   false,
				Level:   "block",
				Reason:  "egress to denied host: " + host,
				RuleIDs: []string{ruleNetworkEgress},
			}
		}
	}

	// 3. Check allow list. Same label-boundary requirement as the deny list so a
	// lookalike host ("evilpypi.org") does NOT match an allow entry ("pypi.org")
	// while a real subdomain ("sub.pypi.org") does. The allow check never runs
	// against the raw URL — only the deny side fails toward block.
	for _, allowEntry := range cfg.AllowHosts {
		if hostMatchesEntry(host, allowEntry) {
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

// hostMatchesEntry reports whether host matches a deny/allow entry under a label
// boundary: an exact match, or host is a subdomain of entry (host ends with
// "."+entry). A bare strings.HasSuffix(host, entry) is INSUFFICIENT because it
// matches attacker-registrable lookalikes — "evilpypi.org" suffix-matches
// "pypi.org" and "notpastebin.com" suffix-matches "pastebin.com". Requiring the
// "."+entry boundary closes that bypass while still covering real subdomains
// ("sub.pypi.org", "raw.pastebin.com").
//
// Comparison is case-insensitive (DNS labels are case-insensitive). An empty
// entry never matches.
func hostMatchesEntry(host, entry string) bool {
	if entry == "" {
		return false
	}
	h := strings.ToLower(host)
	e := strings.ToLower(entry)
	return h == e || strings.HasSuffix(h, "."+e)
}

// rawURLContainsDenyHost is the fail-toward-deny fallback used ONLY when host
// extraction was ambiguous/failed. It lowercases the raw URL and looks for the
// deny entry as a host appearing at a label boundary — i.e. preceded by a
// scheme/userinfo/"//"/"@" delimiter (or the start of the string) and followed
// by an end-of-host delimiter (":", "/", "?", "#", "\", or end). This catches a
// deny target wrapped to defeat url.Parse (e.g. "pastebin.com\@x" or the
// scheme-relative "//pastebin.com/raw/x") without matching an unrelated
// substring like "notpastebin.com". Pure: only string scanning, no I/O.
func rawURLContainsDenyHost(rawURL, entry string) bool {
	if entry == "" {
		return false
	}
	s := strings.ToLower(rawURL)
	e := strings.ToLower(entry)
	from := 0
	for {
		idx := strings.Index(s[from:], e)
		if idx < 0 {
			return false
		}
		abs := from + idx
		// Left boundary: start-of-string, or a host-delimiter byte immediately
		// before the match, or the match is preceded by "."+e checked via the
		// preceding byte being a label dot (subdomain of a deny host).
		leftOK := abs == 0
		if !leftOK {
			switch s[abs-1] {
			case '/', '@', ':', '.', '\\':
				leftOK = true
			}
		}
		// Right boundary: end-of-string or a byte that terminates the host.
		end := abs + len(e)
		rightOK := end == len(s)
		if !rightOK {
			switch s[end] {
			case ':', '/', '?', '#', '\\', '@':
				rightOK = true
			}
		}
		if leftOK && rightOK {
			return true
		}
		from = abs + 1
	}
}

// extractHost extracts the hostname (without port) from a URL. Thin wrapper over
// extractHostChecked for callers that do not need the ambiguity flag.
func extractHost(rawURL string) string {
	host, _ := extractHostChecked(rawURL)
	return host
}

// extractHostChecked extracts the hostname (without port) from a URL and reports
// whether the extraction was ambiguous. ambiguous is true when url.Parse failed
// or produced an empty Host, meaning the returned host string was reconstructed
// by the manual fallback and may not faithfully represent the real connect
// target. Callers use ambiguous to fail toward deny on the egress path.
//
// Uses url.Parse + u.Hostname() which correctly handles IPv6 bracket notation,
// userinfo (@), and port stripping. Pure function: net/url performs no I/O, no
// DNS, no network access.
func extractHostChecked(rawURL string) (host string, ambiguous bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		// Fallback: strip scheme manually for bare host strings without "//".
		s := rawURL
		for _, prefix := range []string{"https://", "http://"} {
			if strings.HasPrefix(s, prefix) {
				s = s[len(prefix):]
				break
			}
		}
		// Strip a leading "//" (scheme-relative URL).
		s = strings.TrimPrefix(s, "//")
		// Drop userinfo if present (everything up to and including the last "@"
		// before the first path/query/fragment separator).
		if idx := strings.IndexAny(s, "/?#"); idx >= 0 {
			s = s[:idx]
		}
		if at := strings.LastIndex(s, "@"); at >= 0 {
			s = s[at+1:]
		}
		// Drop a trailing :port.
		if idx := strings.IndexByte(s, ':'); idx >= 0 {
			s = s[:idx]
		}
		return s, true
	}
	// u.Hostname() strips port and brackets from IPv6 addresses.
	return u.Hostname(), false
}
