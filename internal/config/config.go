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
// History: a Phase 8 NudgeConfig block (package-manager nudge / NUDGE-08) lived
// here. The nudge feature was removed in v1.1.0; its config block, validator, and
// layered-merge handling are gone. config imports only stdlib.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
// absent block (nil → inherit lower layer) from an explicit disable.
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

// ValidateCatalogSyncConfig checks csc fail-closed.
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
// CatalogSync.
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
	// AutoQuarantineThresholdMin and AutoQuarantineThresholdMax are the EXPORTED
	// inclusive bounds for AutoQuarantineConfig.Threshold ([1, 3]). Consumers (the
	// TUI settings panel) clamp the cursor to the same range the validator accepts
	// instead of duplicating the literals, so the UI band cannot drift out of sync
	// with ValidateAutoQuarantineConfig.
	AutoQuarantineThresholdMin = 1
	AutoQuarantineThresholdMax = 3

	autoQuarantineThresholdMin     = AutoQuarantineThresholdMin
	autoQuarantineThresholdMax     = AutoQuarantineThresholdMax
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

// CorpusDownstreamCleanDaysMax is a generous sanity ceiling (10 years) for the
// corpus downstream-clean rolling window. A value above it is almost certainly a
// typo and is rejected fail-closed rather than silently accepted. Zero means
// "use the 30-day default" (see CorpusDownstreamCleanDays).
const CorpusDownstreamCleanDaysMax = 3650

// ValidateCorpusConfig checks cc fail-closed (mirrors ValidateAutoQuarantineConfig).
// CorpusConfig is a VALUE block (no pointer-absence distinction), so unlike
// AutoQuarantine there is no "absent → skip" path: it is validated on every Load.
// A DownstreamCleanDays of 0 is allowed (resolves to the 30-day default); a
// negative or out-of-sanity-range window, or a Scope outside the known set, is
// rejected so a hand-edited or poisoned config cannot silently degrade the
// corpus loop. "community_shareable" is accepted here (it is a legal stored
// value) even though promotion to it has no runtime effect this release.
func ValidateCorpusConfig(cc CorpusConfig) error {
	if cc.DownstreamCleanDays < 0 || cc.DownstreamCleanDays > CorpusDownstreamCleanDaysMax {
		return fmt.Errorf("invalid corpus downstream_clean_days %d (want 0 or 1..%d)",
			cc.DownstreamCleanDays, CorpusDownstreamCleanDaysMax)
	}
	switch cc.Scope {
	case "", "org_only", "community_shareable":
		// valid
	default:
		return fmt.Errorf("invalid corpus scope %q (want %q or %q)",
			cc.Scope, "org_only", "community_shareable")
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

// Posture rule action values. "" is treated as the default (warn). "warn" is the
// shipped default posture (surface, do not block). "block" opts the rule UP to a
// hard block on a DEFINITE violation (IPOVR-03). The unknown/fail-soft path in the
// check adapter always warns regardless of this value (a registry outage cannot
// turn into a blocked install even when a rule is set to block).
const (
	PostureActionWarn  = "warn"
	PostureActionBlock = "block"
)

// Posture rule names. These are the stable, user-facing keys used in config and by
// the accessor PostureRuleAction. They are intentionally distinct from the internal
// policy rule IDs (release-age-policy / lifecycle-script-policy / remote-source-policy)
// so the config surface stays decoupled from the pure evaluator rule IDs.
const (
	PostureRuleReleaseAge   = "release-age"
	PostureRuleLifecycle    = "lifecycle"
	PostureRuleRemoteSource = "git-remote"
)

// PostureRuleConfig holds the per-rule install-posture severity override (IPOVR-03).
//
// Action is "" (treated as warn) | "warn" | "block". A user opts a rule UP to block
// by setting Action:"block"; that blocks a DEFINITE violation (release age below
// threshold, lifecycle scripts present, remote source present). The unknown path
// (missing timestamp / registry error / fetch timeout) STAYS fail-soft warn even
// under block -- that mapping lives in the check adapter, not here.
type PostureRuleConfig struct {
	Action string `json:"action,omitempty"`
}

// PostureConfig holds the per-rule install-posture severity overrides (IPOVR-03).
//
// Each rule defaults to warn (the shipped Phase 27 default). A user may opt an
// individual rule UP to block via a trusted layer; an untrusted (project/env) layer
// may only TIGHTEN a rule warn->block, never loosen block->warn (mergePostureUntrusted).
//
// The field is a POINTER on Config so the layered merge can distinguish an absent
// block (nil -> all rules default to warn) from an explicit override, exactly like
// CatalogSync and AutoQuarantine. The scoped allow list is added in Plan 29-02.
type PostureConfig struct {
	// ReleaseAge overrides the action for the release-age rule (package younger
	// than the configured minimum).
	ReleaseAge PostureRuleConfig `json:"release_age,omitempty"`
	// Lifecycle overrides the action for the lifecycle-script rule (package carries
	// install lifecycle scripts).
	Lifecycle PostureRuleConfig `json:"lifecycle,omitempty"`
	// RemoteSource overrides the action for the git/remote-source rule (install
	// from a git/url/file spec rather than a registry package).
	RemoteSource PostureRuleConfig `json:"remote_source,omitempty"`
}

// DefaultPostureConfig returns the documented default: every rule warns. A missing
// "posture" block resolves to this value (all-warn, fail-soft -- the Phase 27 default).
func DefaultPostureConfig() PostureConfig {
	return PostureConfig{
		ReleaseAge:   PostureRuleConfig{Action: PostureActionWarn},
		Lifecycle:    PostureRuleConfig{Action: PostureActionWarn},
		RemoteSource: PostureRuleConfig{Action: PostureActionWarn},
	}
}

// validPostureAction reports whether a is an accepted posture action.
// "" is accepted (resolves to warn in the accessor); "warn" and "block" are the
// only non-empty legal values. Anything else is rejected fail-closed at load time.
func validPostureAction(a string) bool {
	switch a {
	case "", PostureActionWarn, PostureActionBlock:
		return true
	default:
		return false
	}
}

// ValidatePostureConfig checks pc fail-closed (mirrors ValidateAutoQuarantineConfig).
// Any rule Action outside {"", "warn", "block"} is rejected so a typo or a bogus
// value cannot silently land somewhere undefined -- the load fails loudly instead.
func ValidatePostureConfig(pc PostureConfig) error {
	for rule, action := range map[string]string{
		PostureRuleReleaseAge:   pc.ReleaseAge.Action,
		PostureRuleLifecycle:    pc.Lifecycle.Action,
		PostureRuleRemoteSource: pc.RemoteSource.Action,
	} {
		if !validPostureAction(action) {
			return fmt.Errorf("invalid posture %s action %q (want %q or %q)",
				rule, action, PostureActionWarn, PostureActionBlock)
		}
	}
	return nil
}

// PostureRuleAction returns the effective action ("warn" by default) for the given
// posture rule name (PostureRuleReleaseAge / PostureRuleLifecycle /
// PostureRuleRemoteSource). It is nil-safe: a nil Posture block, an unknown rule
// name, or an empty Action all resolve to "warn" (the shipped default). The check
// adapter calls this per rule and applies block only when this returns "block" AND
// the rule fired on a DEFINITE violation.
func (c Config) PostureRuleAction(rule string) string {
	resolve := func(action string) string {
		if action == PostureActionBlock {
			return PostureActionBlock
		}
		return PostureActionWarn
	}
	if c.Posture == nil {
		return PostureActionWarn
	}
	switch rule {
	case PostureRuleReleaseAge:
		return resolve(c.Posture.ReleaseAge.Action)
	case PostureRuleLifecycle:
		return resolve(c.Posture.Lifecycle.Action)
	case PostureRuleRemoteSource:
		return resolve(c.Posture.RemoteSource.Action)
	default:
		return PostureActionWarn
	}
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

	// Posture holds the per-rule install-posture severity overrides (IPOVR-03,
	// Plan 29-01). A nil pointer means the block was absent; the accessor
	// PostureRuleAction resolves nil (and any unset rule) to "warn" -- the shipped
	// Phase 27 default. An explicit block is validated fail-closed via
	// ValidatePostureConfig so a bogus action is rejected at load time. An untrusted
	// (project/env) layer may only TIGHTEN a rule warn->block, never loosen
	// (mergePostureUntrusted) -- the IPOVR-03 self-defense invariant.
	Posture *PostureConfig `json:"posture,omitempty"`
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
			// Missing file = use defaults. Apply DefaultCatalogSyncConfig so callers
			// always get a fully-populated block (mirrors FailMode default).
			cs := DefaultCatalogSyncConfig()
			return Config{FailMode: FailModeClosed, CatalogSync: &cs}, nil
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

	// Phase 20 (CSYNC): resolve the CatalogSync block —
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

	// Corpus is a value block (always present), so validate it unconditionally —
	// a hand-edited negative window or an unknown scope is rejected here rather
	// than silently accepted (symmetry with the AutoQuarantine check above).
	if err := ValidateCorpusConfig(cfg.Corpus); err != nil {
		return Config{}, fmt.Errorf("invalid corpus config: %w", err)
	}

	// IPOVR-03: resolve the Posture block -- absent (nil) leaves nil (the accessor
	// PostureRuleAction resolves nil to "warn"); present -> validated fail-closed via
	// ValidatePostureConfig so a bogus per-rule action is rejected here rather than
	// silently landing somewhere undefined.
	if cfg.Posture != nil {
		if err := ValidatePostureConfig(*cfg.Posture); err != nil {
			return Config{}, fmt.Errorf("invalid posture config: %w", err)
		}
	}

	return cfg, nil
}

// Save writes cfg to path as indented JSON with 0600 permissions.
//
// The write is ATOMIC: cfg is marshaled to a sibling temp file (fsynced, 0600),
// then renamed over the target. A crash mid-write, or a torn read by a concurrent
// reader (the catalog-sync daemon or a `beekeeper check` hook reading config.json
// with no lock), can therefore only ever observe the old complete file or the new
// complete file — never a truncated one. os.Rename is atomic on POSIX and a
// replace-existing move on Windows (same directory), so the previous
// truncate-then-write race (which a frequent TUI editor could hit per keystroke)
// is closed for every caller, including `beekeeper config set`.
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp config in %q: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we bail before the rename succeeds. After a
	// successful rename the temp no longer exists and Remove is a harmless no-op.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp config over %q: %w", path, err)
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
