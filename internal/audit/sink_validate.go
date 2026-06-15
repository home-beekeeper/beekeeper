package audit

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Finding #12 (MEDIUM): the remote audit sink constructors POST the full record
// JSON to any URL with no validation. A misconfigured (or attacker-influenced)
// endpoint can turn the audit pipeline into an SSRF / credential-exfil channel —
// e.g. an http:// endpoint sends records in cleartext, and a link-local target
// like http://169.254.169.254/ reaches a cloud instance-metadata service.
//
// ValidateRemoteSinkEndpoint fails CLOSED at construction so a misconfigured sink
// is rejected before any record leaves the host, rather than silently
// exfiltrating. It is a pure string check — it performs NO DNS resolution at
// construction (a hostname that only resolves to a private range at runtime is
// out of scope by design; we reject obvious SSRF targets by literal host).
//
// Rejected:
//   - non-https:// schemes (the remote sinks must use TLS in transit)
//   - empty / whitespace-only hosts
//   - loopback literals: localhost, 127.0.0.0/8, ::1
//   - link-local literals: 169.254.0.0/16 (esp. the 169.254.169.254 metadata IP)
//   - RFC 1918 private literals: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
//
// requireHTTPS lets a future caller relax the TLS requirement; the OTLP and HTTPS
// audit sinks always pass true.
func ValidateRemoteSinkEndpoint(endpoint string, requireHTTPS bool) error {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return fmt.Errorf("audit sink endpoint is empty")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("audit sink endpoint %q: invalid URL: %w", endpoint, err)
	}

	scheme := strings.ToLower(u.Scheme)
	if requireHTTPS {
		if scheme != "https" {
			return fmt.Errorf("audit sink endpoint %q: scheme %q rejected — only https:// is permitted (audit data must be encrypted in transit)", endpoint, u.Scheme)
		}
	} else if scheme != "http" && scheme != "https" {
		return fmt.Errorf("audit sink endpoint %q: scheme %q rejected — only http/https are permitted", endpoint, u.Scheme)
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return fmt.Errorf("audit sink endpoint %q: missing or empty host", endpoint)
	}

	if isSSRFTargetHost(host) {
		return fmt.Errorf("audit sink endpoint %q: host %q resolves to a loopback/link-local/private-range literal and is rejected as an SSRF target", endpoint, host)
	}

	return nil
}

// isSSRFTargetHost reports whether host is a literal loopback, link-local, or
// RFC 1918 private address (or the localhost name). It is a literal-host check
// only — no DNS resolution is performed, by design.
func isSSRFTargetHost(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]")) // strip IPv6 brackets if present

	// Name-based loopback.
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}

	ip := net.ParseIP(h)
	if ip == nil {
		// Not an IP literal and not a known loopback name — allowed.
		// (A hostname that only resolves to a private range at runtime is out of
		// scope: we do not resolve DNS at construction.)
		return false
	}

	// Loopback: 127.0.0.0/8 and ::1.
	if ip.IsLoopback() {
		return true
	}
	// Link-local unicast: 169.254.0.0/16 (esp. 169.254.169.254 metadata) and fe80::/10.
	if ip.IsLinkLocalUnicast() {
		return true
	}
	// RFC 1918 private ranges (10/8, 172.16/12, 192.168/16) + unique-local IPv6 (fc00::/7).
	if ip.IsPrivate() {
		return true
	}

	return false
}
