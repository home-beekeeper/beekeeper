// Package config provides Beekeeper's user-level configuration loader.
//
// Phase 1 scope is intentionally minimal: a single user-level config file with
// the fail mode that governs how the hook handler behaves when it cannot reach
// a decision (crash, timeout, oversized input, missing catalog index). The full
// layered system→user→project→env→flag merge (CODE-05) lands in Phase 9 and is
// out of scope here.
//
// Phase 2 addition: Socket API token (socket.api_token) for the Socket PURL
// catalog source. All other Phase 2 catalog source config is wired in Plan 08.
// Full layered config remains Phase 9.
//
// Phase 8 addition: NudgeConfig block (NUDGE-08) + EXPORTED ValidateNudgeConfig.
// config imports only stdlib — it MUST NOT import internal/nudge (cycle).
// cmd/beekeeper (package main, Plan 08) calls ValidateNudgeConfig directly for
// the §10-17 config-set rejection test; Load delegates to the same exported
// function so there is exactly ONE validator.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Fail-mode values. "closed" is the secure default: any failure to reach a
// decision blocks the tool call. "open" and "warn" are explicit, documented
// opt-outs that allow on failure and therefore reduce security.
const (
	FailModeClosed = "closed"
	FailModeOpen   = "open"
	FailModeWarn   = "warn"
)

// SocketConfig holds optional Socket.dev API credentials.
//
// If APIToken is empty, the Socket PURL catalog source is disabled gracefully —
// this is not an error. Users must register at socket.dev and configure the
// token to enable the third corroboration source (CTLG-03).
type SocketConfig struct {
	APIToken string `json:"api_token"`
}

// WatchSettings holds file-watcher configuration (Phase 3, EDXT-06).
type WatchSettings struct {
	Directories []string `json:"directories,omitempty"`
}

// AuditConfig holds Phase 6 audit log configuration (AUDT-03, AUDT-04).
type AuditConfig struct {
	// Sinks lists the active output sinks. Valid values: "file" (always active),
	// "syslog", "otlp", "https". Unknown values are silently ignored.
	Sinks []string `json:"sinks,omitempty"`
	// SyslogAddress is the syslog destination in the form "proto:host:port" or
	// "host:port" (UDP default). Required when "syslog" is in Sinks.
	SyslogAddress string `json:"syslog_address,omitempty"`
	// OTLPEndpoint is the OTLP collector HTTP endpoint, e.g.
	// "https://collector:4318/v1/logs". Required when "otlp" is in Sinks.
	OTLPEndpoint string `json:"otlp_endpoint,omitempty"`
	// HTTPSEndpoint is an arbitrary HTTPS POST URL. Required when "https" is in
	// Sinks.
	HTTPSEndpoint string `json:"https_endpoint,omitempty"`
	// RetentionDays is how many days archived log files are kept. Default 30.
	RetentionDays int `json:"retention_days,omitempty"`
	// MaxSizeBytes is the rotation threshold in bytes. Default 10 MB.
	MaxSizeBytes int64 `json:"max_size_bytes,omitempty"`
}

// LlamaFirewallConfig holds Phase 6 LlamaFirewall sidecar configuration
// (LLMF-01–06).
type LlamaFirewallConfig struct {
	// Enabled controls whether the LlamaFirewall sidecar is started.
	Enabled bool `json:"enabled"`
	// SampleRate is the fraction of tool calls forwarded to LlamaFirewall
	// (0.0–1.0). Default 1.0 (scan all).
	SampleRate float64 `json:"sample_rate,omitempty"`
	// FailMode governs sidecar-crash behaviour: "closed" (block), "open"
	// (allow), or "warn" (allow + surface warning). Default "closed".
	FailMode string `json:"fail_mode,omitempty"`
	// CodeShield enables the LlamaFirewall CodeShield scanner. Default true
	// when LlamaFirewall is enabled.
	CodeShield bool `json:"codeshield,omitempty"`
	// CodeShieldAction controls what happens on a CodeShield hit: "warn" or
	// "block". Default "warn".
	CodeShieldAction string `json:"codeshield_action,omitempty"`
	// PythonPath is the path to the Python interpreter used to launch the
	// sidecar. Default "python3".
	PythonPath string `json:"python_path,omitempty"`
}

// NudgeMajorDriftCheck holds the periodic major-version drift check settings.
type NudgeMajorDriftCheck struct {
	// Enabled controls whether the weekly pnpm/bun major-version drift check runs.
	Enabled bool `json:"enabled"`
	// Interval is the time between drift checks as a Go duration string (e.g. "168h").
	// Default "168h" (7 days). Must be parseable by time.ParseDuration.
	Interval string `json:"interval,omitempty"`
}

// NudgeVersionFloors holds the minimum version floors for each supported package manager.
type NudgeVersionFloors struct {
	// Pnpm is the minimum acceptable pnpm version, e.g. "11.0.0".
	Pnpm string `json:"pnpm,omitempty"`
	// Bun is the minimum acceptable bun version, e.g. "1.3.0".
	Bun string `json:"bun,omitempty"`
	// Node is the minimum Node.js version for pnpm 11 compatibility, e.g. "22.0.0".
	// Note: Node 24 is the current Active LTS (Node 22 is Maintenance LTS through 2027-04);
	// the floor remains 22 because pnpm 11 requires Node 22+.
	Node string `json:"node,omitempty"`
}

// NudgeConfig holds Phase 8 package-manager nudge configuration (NUDGE-08).
//
// This struct is defined in internal/config to avoid an import cycle: config is
// imported by many packages; internal/nudge imports config, so config must not
// import internal/nudge.
//
// PRD §5 defaults:
//   - Enabled: true (nudge is on out of the box)
//   - Mode: "soft" (advise + proceed; respects agent agency)
//   - Preferred: "pnpm" (pnpm defaults are on automatically; bun requires the Socket scanner)
//   - CheckSocketScanner: true (verify @socketsecurity/bun-security-scanner for bun)
//   - MajorDriftCheck: enabled, interval "168h" (weekly)
//   - VersionFloors: pnpm 11.0.0, bun 1.3.0, node 22.0.0
//
// Project-level .beekeeper.json nudge.enabled:false wins over user config
// (layered merge, NUDGE-08 / PRD §11).
type NudgeConfig struct {
	// Enabled controls whether the nudge feature runs at all. Default true.
	// Set to false in project .beekeeper.json to opt out project-wide.
	Enabled bool `json:"enabled"`
	// Mode is the nudge aggressiveness:
	//   "soft"  (advise + proceed, default) — warn but allow the npm install.
	//   "hard"  (rewrite the command to its pnpm/bun equivalent, advisory; allow).
	//   "block" (DENY npm/yarn installs when a hardened PM is available, telling
	//           the agent to use pnpm/bun instead — supply-chain enforcement).
	// All other values are rejected by ValidateNudgeConfig (fail-closed).
	Mode string `json:"mode,omitempty"`
	// RequireHardened, when true, blocks npm install when no hardened PM is
	// installed. Default false (npm calls proceed with advisory).
	RequireHardened bool `json:"require_hardened,omitempty"`
	// Preferred selects the preferred hardened PM when both pnpm and bun are
	// available. Must be "pnpm" (default) or "bun". Other values are rejected.
	Preferred string `json:"preferred,omitempty"`
	// CheckSocketScanner controls whether bun is only considered "hardened"
	// when @socketsecurity/bun-security-scanner is present in bunfig.toml.
	// Default true.
	CheckSocketScanner bool `json:"check_socket_scanner,omitempty"`
	// MajorDriftCheck holds the periodic pnpm/bun major-version drift check
	// settings (PRD §7.1).
	MajorDriftCheck NudgeMajorDriftCheck `json:"major_drift_check,omitempty"`
	// VersionFloors holds the minimum acceptable versions for each PM.
	VersionFloors NudgeVersionFloors `json:"version_floors,omitempty"`
}

// DefaultNudgeConfig returns the PRD §5.1 default nudge configuration.
// A missing "nudge" block in config.json resolves to this value.
func DefaultNudgeConfig() NudgeConfig {
	return NudgeConfig{
		Enabled:            true,
		Mode:               "soft",
		RequireHardened:    false,
		Preferred:          "pnpm",
		CheckSocketScanner: true,
		MajorDriftCheck: NudgeMajorDriftCheck{
			Enabled:  true,
			Interval: "168h",
		},
		VersionFloors: NudgeVersionFloors{
			Pnpm: "11.0.0",
			Bun:  "1.3.0",
			Node: "22.0.0",
		},
	}
}

// legalNudgeModes is the complete enum of valid nudge Mode values.
// Any value not in this set is rejected by ValidateNudgeConfig (fail-closed,
// mirrors legalRuleTypes / legalActions in internal/policyloader/validate.go).
var legalNudgeModes = map[string]bool{
	"soft":  true,
	"hard":  true,
	"block": true,
}

// legalNudgePreferred is the complete enum of valid Preferred values.
var legalNudgePreferred = map[string]bool{
	"pnpm": true,
	"bun":  true,
}

// parseVersionFloor checks that a non-empty version floor is a syntactically
// valid "major.minor[.patch]" string (all integer components). It returns an
// error when the string is non-empty but malformed.
//
// Full semver range validation (pre-release labels, build metadata) is not
// required: the floors are simple major.minor gates (11.0 / 1.3 / 22) per
// research Anti-Patterns ("No new dep for simple floor checks; major.minor
// int compare suffices"). A local pure check keeps config free of imports.
func parseVersionFloor(v string) error {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("version floor %q must be major.minor or major.minor.patch", v)
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return fmt.Errorf("version floor %q: component %q is not an integer", v, p)
		}
	}
	return nil
}

// ValidateNudgeConfig checks nc for correctness using fail-closed bounds
// validation (mirrors validateCorroborationThresholds / CORR-02 discipline).
//
// This function is EXPORTED because cmd/beekeeper (package main, Plan 08)
// calls it directly for the §10-17 config-set rejection test — an unexported
// function in internal/config cannot be called from package main.
// Load delegates to this same exported function; there is exactly ONE validator.
//
// Rejects:
//   - Mode not in {"soft", "hard", "block"} (e.g. "aggressive" is not a valid mode)
//   - Preferred not in {"pnpm", "bun"}
//   - Malformed version floor (non-empty string that is not major.minor[.patch])
//   - Malformed MajorDriftCheck.Interval (non-empty string that time.ParseDuration rejects)
func ValidateNudgeConfig(nc NudgeConfig) error {
	// Validate Mode (closed enum).
	if nc.Mode != "" && !legalNudgeModes[nc.Mode] {
		return fmt.Errorf("invalid nudge mode %q (want %q, %q, or %q)", nc.Mode, "soft", "hard", "block")
	}

	// Validate Preferred (closed enum).
	if nc.Preferred != "" && !legalNudgePreferred[nc.Preferred] {
		return fmt.Errorf("invalid nudge preferred %q (want %q or %q)", nc.Preferred, "pnpm", "bun")
	}

	// Validate version floors (must be parseable major.minor[.patch]).
	if err := parseVersionFloor(nc.VersionFloors.Pnpm); err != nil {
		return fmt.Errorf("nudge version_floors.pnpm: %w", err)
	}
	if err := parseVersionFloor(nc.VersionFloors.Bun); err != nil {
		return fmt.Errorf("nudge version_floors.bun: %w", err)
	}
	if err := parseVersionFloor(nc.VersionFloors.Node); err != nil {
		return fmt.Errorf("nudge version_floors.node: %w", err)
	}

	// Validate MajorDriftCheck.Interval (must be a valid Go duration when non-empty).
	if nc.MajorDriftCheck.Interval != "" {
		if _, err := time.ParseDuration(nc.MajorDriftCheck.Interval); err != nil {
			return fmt.Errorf("nudge major_drift_check.interval %q: %w", nc.MajorDriftCheck.Interval, err)
		}
	}

	return nil
}

// CatalogSyncConfig holds Phase 20 background catalog-sync configuration
// (CSYNC-01..06).
//
// Background sync keeps threat intel fresh for hook-only users who would
// otherwise never run `beekeeper catalogs sync` by hand. Disabling sync — or
// loosening its cadence — is a SECURITY-RELAXING lever: a stale catalog widens
// the window in which a hijacked or off-task agent can act unseen. Therefore an
// untrusted (project/.beekeeper.json or env) layer cannot set Enabled:false or
// loosen the interval (see mergeCatalogSyncUntrusted, CSYNC-04); only a trusted
// user/global layer may opt out.
//
// The field is a POINTER on Config so the layered merge can distinguish an
// absent block (nil → inherit lower layer) from an explicit disable, exactly
// like Nudge.
type CatalogSyncConfig struct {
	// Enabled controls whether background catalog sync runs. Default true.
	Enabled bool `json:"enabled"`
	// Interval is the minimum time between successful syncs as a Go duration
	// string in [2h, 24h]. Default "2h"; empty resolves to the default. The OS
	// scheduler fires on a frequent (hourly) heartbeat and `catalogs sync`
	// no-ops unless time.Since(LastSuccess) >= this interval (D-T1-interval).
	Interval string `json:"interval,omitempty"`
}

// Catalog-sync interval bounds. The interval is clamped to [2h, 24h]: shorter
// than 2h adds list-call pressure with little freshness benefit given the
// conditional ETag requests; longer than 24h is too stale to defend a
// long-running agent session.
const (
	catalogSyncMinInterval     = 2 * time.Hour
	catalogSyncMaxInterval     = 24 * time.Hour
	catalogSyncDefaultInterval = 2 * time.Hour
)

// DefaultCatalogSyncConfig returns the documented default: sync enabled on a
// 2h interval. A missing "catalog_sync" block resolves to this value.
func DefaultCatalogSyncConfig() CatalogSyncConfig {
	return CatalogSyncConfig{Enabled: true, Interval: "2h"}
}

// ValidateCatalogSyncConfig checks csc fail-closed (mirrors ValidateNudgeConfig).
// An empty Interval is allowed (resolves to the default). A non-empty Interval
// must be parseable by time.ParseDuration AND fall within [2h, 24h]; anything
// else is rejected so a typo or out-of-range value cannot silently degrade the
// sync cadence.
func ValidateCatalogSyncConfig(csc CatalogSyncConfig) error {
	if csc.Interval == "" {
		return nil
	}
	d, err := time.ParseDuration(csc.Interval)
	if err != nil {
		return fmt.Errorf("invalid catalog_sync interval %q: %w", csc.Interval, err)
	}
	if d < catalogSyncMinInterval || d > catalogSyncMaxInterval {
		return fmt.Errorf("catalog_sync interval %q out of range (want %s..%s)",
			csc.Interval, catalogSyncMinInterval, catalogSyncMaxInterval)
	}
	return nil
}

// parseClampCatalogSyncInterval parses s and defensively clamps the result to
// [2h, 24h], returning the 2h default for an empty or unparseable string. It
// never returns 0 and never panics — it is the load-bearing accessor relied on
// by the sync scheduler even if validation was somehow bypassed (mirrors the
// gateway drift.go parse-then-default idiom).
func parseClampCatalogSyncInterval(s string) time.Duration {
	if s == "" {
		return catalogSyncDefaultInterval
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return catalogSyncDefaultInterval
	}
	if d < catalogSyncMinInterval {
		return catalogSyncMinInterval
	}
	if d > catalogSyncMaxInterval {
		return catalogSyncMaxInterval
	}
	return d
}

// CatalogSyncInterval returns the effective sync interval, parsed and clamped to
// [2h, 24h] (default 2h on empty/invalid/nil). The OS-scheduled `catalogs sync`
// no-ops unless time.Since(LastSuccess) >= this value (D-T1-interval).
func (c Config) CatalogSyncInterval() time.Duration {
	if c.CatalogSync == nil {
		return catalogSyncDefaultInterval
	}
	return parseClampCatalogSyncInterval(c.CatalogSync.Interval)
}

// CatalogSyncEnabled reports whether background catalog sync is enabled. A nil
// block (absent config) defaults to enabled (DefaultCatalogSyncConfig).
func (c Config) CatalogSyncEnabled() bool {
	if c.CatalogSync == nil {
		return DefaultCatalogSyncConfig().Enabled
	}
	return c.CatalogSync.Enabled
}

// AutoQuarantineConfig holds the auto-quarantine knob for the first-responder
// scan-hit -> reversible quarantine path (FRSP-01, Task C1).
//
// Auto-quarantine is REVERSIBLE (os.Rename + manifest); the DESTRUCTIVE purge
// is NEVER automatic and stays human-gated via the existing CLI/TUI purge path.
//
// The field is a POINTER on Config so the layered merge can distinguish an
// absent block (nil -> use defaults) from an explicit disable, exactly like
// Nudge and CatalogSync.
type AutoQuarantineConfig struct {
	// Enabled controls whether auto-quarantine fires at all. Default false (opt-in).
	// Set to true to activate the first-responder reversible-move at >= Threshold.
	Enabled bool `json:"enabled"`
	// DryRun, when true, audits what WOULD be quarantined without moving anything.
	// Default true: new deployments observe findings before committing to moves.
	DryRun bool `json:"dry_run"`
	// Threshold is the minimum CorroborationCount (distinct signed sources) needed
	// to trigger auto reversible-quarantine. Default 2 (the "block" tier).
	// Clamped to [1, 3]: 1 = warn-tier opt-in; 3 = engine quarantine tier.
	// A zero or absent value resolves to 2 (NOT to the clamp floor 1).
	Threshold int `json:"threshold,omitempty"`
}

// DefaultAutoQuarantineConfig returns the documented default: opt-in disabled,
// dry-run enabled, threshold 2. A missing "auto_quarantine" block resolves here.
func DefaultAutoQuarantineConfig() AutoQuarantineConfig {
	return AutoQuarantineConfig{
		Enabled:   false,
		DryRun:    true,
		Threshold: 2,
	}
}

// autoQuarantineThresholdMin and autoQuarantineThresholdMax define the valid
// range [1, 3] for AutoQuarantineConfig.Threshold. These match the three
// corroboration tiers: 1=warn, 2=block, 3=block+quarantine.
const (
	autoQuarantineThresholdMin     = 1
	autoQuarantineThresholdMax     = 3
	autoQuarantineThresholdDefault = 2
)

// ValidateAutoQuarantineConfig checks ac fail-closed (mirrors ValidateCatalogSyncConfig).
// A Threshold of 0 is allowed (resolves to the default 2 in the accessor).
// A non-zero Threshold outside [1, 3] is rejected so out-of-range values never
// silently degrade the security posture.
func ValidateAutoQuarantineConfig(ac AutoQuarantineConfig) error {
	if ac.Threshold != 0 && (ac.Threshold < autoQuarantineThresholdMin || ac.Threshold > autoQuarantineThresholdMax) {
		return fmt.Errorf("invalid auto_quarantine threshold %d (want 0 or %d..%d)",
			ac.Threshold, autoQuarantineThresholdMin, autoQuarantineThresholdMax)
	}
	return nil
}

// parseClampAutoQuarantineThreshold parses t and defensively applies:
//   - zero -> default 2 (NOT clamp floor 1 — "absent -> default" path is distinct).
//   - < 1  -> clamped to 1 (clamp floor).
//   - > 3  -> clamped to 3 (clamp ceiling).
//
// This mirrors parseClampCatalogSyncInterval and avoids the default-vs-clamp-floor
// bug class where a zero value becomes the floor instead of the documented default.
func parseClampAutoQuarantineThreshold(t int) int {
	if t == 0 {
		return autoQuarantineThresholdDefault
	}
	if t < autoQuarantineThresholdMin {
		return autoQuarantineThresholdMin
	}
	if t > autoQuarantineThresholdMax {
		return autoQuarantineThresholdMax
	}
	return t
}

// AutoQuarantineEnabled returns whether auto-quarantine is enabled.
// A nil block (absent config) defaults to false (opt-in).
func (c Config) AutoQuarantineEnabled() bool {
	if c.AutoQuarantine == nil {
		return false
	}
	return c.AutoQuarantine.Enabled
}

// AutoQuarantineDryRun returns whether dry-run mode is enabled.
// A nil block (absent config) defaults to true (safe default: observe before move).
func (c Config) AutoQuarantineDryRun() bool {
	if c.AutoQuarantine == nil {
		return true
	}
	return c.AutoQuarantine.DryRun
}

// AutoQuarantineThreshold returns the effective corroboration threshold,
// applying the zero-to-default and out-of-range-to-clamp logic. Never returns 0.
func (c Config) AutoQuarantineThreshold() int {
	if c.AutoQuarantine == nil {
		return autoQuarantineThresholdDefault
	}
	return parseClampAutoQuarantineThreshold(c.AutoQuarantine.Threshold)
}

// CorpusConfig holds Phase 22+ corpus configuration (SCHEMA-01/02/05).
//
// Follows the same pattern as AuditConfig. This block is additive and backward-
// compatible: a missing "corpus" key in config.json leaves Corpus at its zero
// value (Enabled:false, empty Path/Scope), which is the safe default until
// Phase 23 wires the store.
type CorpusConfig struct {
	// Enabled controls whether the corpus store is active. Default false until
	// Phase 23 wires the append-only NDJSON store. Setting true before Phase 23
	// has no effect — the store is not yet implemented.
	Enabled bool `json:"enabled"`
	// Path overrides the default corpus file location.
	// Default (when empty): StateDir()/corpus/beekeeper-corpus.ndjson
	Path string `json:"path,omitempty"`
	// Scope is the default scope for new records.
	// Valid values: "org_only" (default) or "community_shareable".
	// "community_shareable" is reserved for v2.0 — setting it in v1 has no effect
	// (PromoteScope always returns an error until anonymization is implemented).
	Scope string `json:"scope,omitempty"`
	// DownstreamCleanDays is the rolling window (in days) used by the
	// adjudication engine to classify a record as "downstream_clean": if no
	// follow-on alert with the same ClusterID appears within this window, the
	// record is adjudicated as benign. Default 30 (OQ-1: "30 days, configurable").
	DownstreamCleanDays int `json:"downstream_clean_days,omitempty"`
}

// SelfCatalogConfig holds configuration for the beekeeper-self catalog source
// (Phase 9, CTLG-04/SFDF-06). The self-catalog is a separately-hosted feed
// verified against a distinct public key embedded in the binary. It is checked
// on every startup and every catalogs sync to detect compromised Beekeeper releases.
//
// Both fields are optional overrides; sensible defaults are compiled in.
type SelfCatalogConfig struct {
	// URL is the HTTPS endpoint for the beekeeper-self catalog feed.
	// Defaults to the official endpoint compiled into the binary.
	URL string `json:"url,omitempty"`
	// PubKey is a base64-encoded Ed25519 public key that overrides the
	// compiled-in public key for signature verification. Leave empty to use
	// the compiled-in key.
	PubKey string `json:"pub_key,omitempty"`
}

// Config is the user-level Beekeeper configuration.
type Config struct {
	// FailMode controls behavior when the hook handler cannot produce a real
	// policy decision (crash, timeout, oversized stdin, missing/corrupt index):
	//   "closed" (default) — failures BLOCK (fail-closed; secure default).
	//   "open"             — failures ALLOW. "open" reduces security: failures
	//                        allow instead of block.
	//   "warn"             — failures ALLOW but are surfaced as a warning.
	// Empty is treated as "closed".
	FailMode string `json:"fail_mode"`

	// Socket holds optional Socket.dev API credentials (Phase 2).
	// Absent or empty api_token disables the Socket catalog source gracefully.
	Socket SocketConfig `json:"socket"`

	// Watch holds Phase 3 file-watcher configuration.
	// Absent or nil means no watch directories are configured.
	Watch *WatchSettings `json:"watch,omitempty"`

	// RedactPatterns is an optional list of additional regex patterns used for
	// sensitive-field redaction in audit records (Phase 4, INTG-07 / T-04-05-02).
	// Each element is a regex pattern string. On match, the entire match is
	// replaced with "[REDACTED]". The default patterns (Bearer tokens, JWT tokens,
	// common API key prefixes) are always applied regardless of this field.
	// This field is forward compatibility for custom redaction rules; the Phase 4
	// implementation always applies the default patterns.
	RedactPatterns []string `json:"redact_patterns,omitempty"`

	// Audit holds Phase 6 audit log configuration (rotation, sinks).
	Audit AuditConfig `json:"audit,omitempty"`

	// LlamaFirewall holds Phase 6 LlamaFirewall sidecar configuration.
	LlamaFirewall LlamaFirewallConfig `json:"llamafirewall,omitempty"`

	// SelfCatalog holds Phase 9 beekeeper-self catalog overrides (CTLG-04/SFDF-06).
	// Consumers (Plans 03 and 05) read URL and PubKey to locate and verify the feed.
	// Leave both fields empty to use the compiled-in defaults.
	SelfCatalog SelfCatalogConfig `json:"self_catalog,omitempty"`

	// Nudge holds Phase 8 package-manager nudge configuration (NUDGE-08).
	// A nil pointer means the nudge block was absent from the config file;
	// Load replaces nil with DefaultNudgeConfig() so callers always see a
	// fully-populated struct. An explicit nudge.enabled:false in a project
	// .beekeeper.json is preserved verbatim (the pointer is non-nil, Enabled
	// is false) — project config wins over user config (layered merge, §11).
	Nudge *NudgeConfig `json:"nudge,omitempty"`

	// CatalogSync holds Phase 20 background catalog-sync configuration
	// (CSYNC-01..06). A nil pointer means the block was absent; Load resolves
	// nil → DefaultCatalogSyncConfig() so callers always see a populated struct.
	// An untrusted (project/env) layer cannot disable sync or loosen the
	// interval (mergeCatalogSyncUntrusted) — disabling sync reduces security.
	CatalogSync *CatalogSyncConfig `json:"catalog_sync,omitempty"`

	// AutoQuarantine holds the first-responder auto-reversible-quarantine knob
	// (FRSP-01, Task C1). A nil pointer means the block was absent; accessors
	// resolve nil to the documented defaults (opt-in disabled, dry_run=true,
	// threshold=2). An explicit block is validated fail-closed via
	// ValidateAutoQuarantineConfig so an out-of-range threshold is rejected
	// at load time rather than silently clamped.
	AutoQuarantine *AutoQuarantineConfig `json:"auto_quarantine,omitempty"`

	// Corpus holds Phase 22+ corpus configuration (SCHEMA-01/02/05).
	// Absent or zero-value means corpus is disabled (Enabled:false), which is the
	// safe default until Phase 23 wires the append-only store. The corpus block
	// defines the type and config shape only; no decision behavior is changed in
	// Phase 22 (store wiring is Phase 23, T-22-04).
	Corpus CorpusConfig `json:"corpus,omitempty"`
}

// CorpusDownstreamCleanDays returns the downstream_clean rolling window in days.
// If DownstreamCleanDays is unset (zero), the default of 30 days is returned
// (OQ-1: "30 days, configurable"). The adjudication engine reads this value to
// decide when a record with no follow-on alerts may be classified as benign.
func (c Config) CorpusDownstreamCleanDays() int {
	if c.Corpus.DownstreamCleanDays > 0 {
		return c.Corpus.DownstreamCleanDays
	}
	return 30
}

// SocketAPIToken returns the Socket API token, or "" if not configured.
// An empty token disables the Socket PURL source without error (CTLG-03).
func (c Config) SocketAPIToken() string {
	return c.Socket.APIToken
}

// WatchDirectories returns the configured watch directories, or nil if none.
func (c Config) WatchDirectories() []string {
	if c.Watch == nil {
		return nil
	}
	return c.Watch.Directories
}

// AddWatchDirectory appends dir to Watch.Directories idempotently.
func (c *Config) AddWatchDirectory(dir string) {
	if c.Watch == nil {
		c.Watch = &WatchSettings{}
	}
	for _, d := range c.Watch.Directories {
		if d == dir {
			return
		}
	}
	c.Watch.Directories = append(c.Watch.Directories, dir)
}

// Load reads the config at path.
//
// A missing file is normal — absence means "use defaults" — so it returns
// Config{FailMode: "closed"} with a nil error. If the file exists it is read and
// unmarshaled; an empty fail_mode defaults to "closed", and any value other than
// "closed"/"open"/"warn" is rejected with a non-nil error so a typo cannot
// silently degrade to a less-secure mode.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Missing file = use defaults. Apply DefaultNudgeConfig +
			// DefaultCatalogSyncConfig so callers always get fully-populated
			// blocks (mirrors FailMode default).
			d := DefaultNudgeConfig()
			cs := DefaultCatalogSyncConfig()
			return Config{FailMode: FailModeClosed, Nudge: &d, CatalogSync: &cs}, nil
		}
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	if cfg.FailMode == "" {
		cfg.FailMode = FailModeClosed
	}

	switch cfg.FailMode {
	case FailModeClosed, FailModeOpen, FailModeWarn:
		// valid
	default:
		return Config{}, fmt.Errorf("invalid fail_mode %q (want %q, %q, or %q)",
			cfg.FailMode, FailModeClosed, FailModeOpen, FailModeWarn)
	}

	// Phase 8 (NUDGE-08): resolve the Nudge block.
	// A missing key (nil pointer) resolves to documented defaults (PRD §5.1) —
	// mirrors the FailMode=="" → FailModeClosed defaulting idiom.
	// An explicit block (non-nil) is validated fail-closed via ValidateNudgeConfig —
	// a typo or out-of-range value is rejected here so it can never silently degrade.
	// Load delegates to the EXPORTED ValidateNudgeConfig so there is one validator.
	if cfg.Nudge == nil {
		d := DefaultNudgeConfig()
		cfg.Nudge = &d
	} else {
		if err := ValidateNudgeConfig(*cfg.Nudge); err != nil {
			return Config{}, fmt.Errorf("invalid nudge config: %w", err)
		}
	}

	// Phase 20 (CSYNC): resolve the CatalogSync block the same way as Nudge —
	// absent (nil) → DefaultCatalogSyncConfig(); present → validated fail-closed
	// via the EXPORTED ValidateCatalogSyncConfig so an out-of-range or malformed
	// interval is rejected here rather than silently clamped.
	if cfg.CatalogSync == nil {
		cs := DefaultCatalogSyncConfig()
		cfg.CatalogSync = &cs
	} else if err := ValidateCatalogSyncConfig(*cfg.CatalogSync); err != nil {
		return Config{}, fmt.Errorf("invalid catalog_sync config: %w", err)
	}

	// FRSP-01: resolve the AutoQuarantine block — absent (nil) leaves nil (accessors
	// handle nil safely); present → validated fail-closed via ValidateAutoQuarantineConfig
	// so an out-of-range threshold is rejected here rather than silently clamped.
	if cfg.AutoQuarantine != nil {
		if err := ValidateAutoQuarantineConfig(*cfg.AutoQuarantine); err != nil {
			return Config{}, fmt.Errorf("invalid auto_quarantine config: %w", err)
		}
	}

	return cfg, nil
}

// Save writes cfg to path as indented JSON with 0600 permissions.
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

// FailClosed reports whether failures should block. It returns true unless
// FailMode is explicitly "open" or "warn", so an empty or unrecognized mode is
// treated as fail-closed (the secure default).
func (c Config) FailClosed() bool {
	return c.FailMode != FailModeOpen && c.FailMode != FailModeWarn
}

// GetRedactPatterns returns the configured custom redaction pattern strings, or
// nil when none are configured. The caller (typically writeAuditWithAC) uses this
// alongside defaultRedactPatterns() — the default patterns are always applied;
// this returns any additional user-configured patterns.
//
// Phase 4 note: custom patterns are returned for forward compatibility but are not
// yet compiled or applied in the Phase 4 implementation. The default patterns cover
// the three critical cases (Bearer/JWT/API-key prefixes). Custom pattern compilation
// is a Phase 6 audit enhancement.
func (c Config) GetRedactPatterns() []string {
	return c.RedactPatterns
}

// AuditRetentionDays returns the configured retention in days, defaulting to 30.
func (c Config) AuditRetentionDays() int {
	if c.Audit.RetentionDays > 0 {
		return c.Audit.RetentionDays
	}
	return 30
}

// AuditMaxSizeBytes returns the configured max size, defaulting to 10 MB.
func (c Config) AuditMaxSizeBytes() int64 {
	if c.Audit.MaxSizeBytes > 0 {
		return c.Audit.MaxSizeBytes
	}
	return 10 * 1024 * 1024
}

// LlamaFirewallEnabled returns whether the LlamaFirewall sidecar is enabled.
func (c Config) LlamaFirewallEnabled() bool { return c.LlamaFirewall.Enabled }

// LlamaFirewallSampleRate returns the configured sample rate, defaulting to 1.0
// (scan all tool calls).
func (c Config) LlamaFirewallSampleRate() float64 {
	if c.LlamaFirewall.SampleRate > 0 {
		return c.LlamaFirewall.SampleRate
	}
	return 1.0
}
