// Package config — layered config merge (CODE-05, Phase 9).
//
// LoadLayered merges five configuration layers in ascending precedence order:
//
//  1. System:  /etc/beekeeper/config.json  (optional — skipped if absent or empty path)
//  2. User:    ~/.beekeeper/config.json    (required — error if file exists but is corrupt)
//  3. Project: <project>/.beekeeper/config.json (optional — skipped if absent or empty path)
//  4. Env:     BEEKEEPER_* environment variables
//  5. Flags:   CLI flag overrides
//
// Each later layer wins over the previous one. A zero-value field in a higher
// layer does NOT reset a non-zero field inherited from a lower layer (Pitfall 5 in
// 09-RESEARCH.md). The single-file Load is reused per layer; no duplicate parse
// logic exists here.
//
// Security note: BEEKEEPER_* env vars are mapped only to a hardcoded known field
// set; unknown env vars are silently ignored (T-09-05: no reflective application).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LayerOpts controls which configuration layers LoadLayered merges and in what
// order. All file-path fields accept an empty string to mean "skip this layer".
type LayerOpts struct {
	// SystemPath is the system-wide config file path, typically
	// /etc/beekeeper/config.json. Empty or absent → layer skipped silently.
	SystemPath string

	// UserPath is the user-level config file path, typically
	// ~/.beekeeper/config.json. A missing file is treated as defaults (same as
	// Load); a corrupt file returns an error.
	UserPath string

	// ProjectPath is the project-level config file path, typically
	// <project>/.beekeeper/config.json. Empty or absent → layer skipped silently.
	ProjectPath string

	// Environ is the environment variable slice to apply as the fourth layer.
	// Pass os.Environ() in production; pass a custom slice in tests.
	Environ []string

	// FlagOverrides is a map of logical key → string value applied as the fifth
	// and highest-precedence layer (CLI flags win over env vars).
	// Keys use the same names as BEEKEEPER_* suffixes lower-cased:
	// "fail_mode", "socket_api_token", "llamafirewall_enabled",
	// "audit_sinks", "self_catalog_url".
	FlagOverrides map[string]string
}

// LoadLayered merges the five configuration layers and returns the final Config.
//
// Layer order (lowest to highest precedence):
//
//	baseline defaults → system → user → project → env vars → CLI flags
//
// Missing optional layers (system, project) are silently skipped. The user
// layer follows Load's contract: absent file → defaults, corrupt file → error.
// The merged FailMode is validated; an invalid merged value returns an error
// rather than silently defaulting to a less-secure mode (T-09-08).
func LoadLayered(opts LayerOpts) (Config, error) {
	// Baseline: fail-closed default.
	cfg := Config{FailMode: FailModeClosed}

	// Layer 1: system (optional).
	// We call loadIfPresent to distinguish "file absent" (skip silently) from
	// "file present but corrupt" (skip with intent for system layer — a bad
	// system file should not brick individual developers).
	if opts.SystemPath != "" {
		if sys, ok, _ := loadIfPresent(opts.SystemPath); ok {
			cfg = merge(cfg, sys)
		}
	}

	// Layer 2: user (required source of defaults).
	// Load's contract: absent file → Config{FailMode:"closed"}, nil.
	// Corrupt file → error.
	user, err := Load(opts.UserPath)
	if err != nil {
		return Config{}, fmt.Errorf("load user config: %w", err)
	}
	cfg = merge(cfg, user)

	// Layer 3: project (optional) — LOW-TRUST: security-relaxing levers refused.
	if opts.ProjectPath != "" {
		if proj, ok, _ := loadIfPresent(opts.ProjectPath); ok {
			cfg = mergeUntrusted(cfg, proj, "project")
		}
	}

	// Layer 4: BEEKEEPER_* environment variables — LOW-TRUST: security-relaxing
	// levers refused (attacker-influenceable on shared/CI systems).
	cfg = applyEnvVarsUntrusted(cfg, opts.Environ)

	// Layer 5: CLI flag overrides (highest precedence).
	cfg = applyFlagOverrides(cfg, opts.FlagOverrides)

	// CSYNC: guarantee a non-nil, validated CatalogSync at the layered root.
	// Absent across every layer → default; a merged block is validated fail-closed
	// so an invalid project interval is rejected here rather than silently clamped.
	if cfg.CatalogSync == nil {
		cs := DefaultCatalogSyncConfig()
		cfg.CatalogSync = &cs
	} else if err := ValidateCatalogSyncConfig(*cfg.CatalogSync); err != nil {
		return Config{}, fmt.Errorf("invalid merged catalog_sync config: %w", err)
	}

	// IPOVR-03: a merged Posture block is validated fail-closed. nil is fine (the
	// accessor resolves nil to warn); a non-nil merged block with a bogus action is
	// rejected here rather than silently honored. Mirrors the CatalogSync guard.
	if cfg.Posture != nil {
		if err := ValidatePostureConfig(*cfg.Posture); err != nil {
			return Config{}, fmt.Errorf("invalid merged posture config: %w", err)
		}
	}

	// Final validation: reject an invalid merged FailMode rather than silently
	// using an insecure default (mitigates T-09-08).
	return validate(cfg)
}

// loadIfPresent loads the config at path only when the file exists, using a
// raw parse that does NOT apply FailMode defaults. This is intentional: for
// layered merge, we must distinguish "field absent" (zero value = do not
// override lower layer) from "field set" (non-zero = override). The default-
// filling in Load is correct for single-file use; here we want the raw values.
//
// Returns (cfg, true, nil) when the file exists and parses successfully.
// Returns (zero, false, nil) when the file does not exist (absent = skip).
// Returns (zero, false, err) when the file exists but cannot be read or parsed.
func loadIfPresent(path string) (Config, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, false, nil // absent → skip silently
		}
		return Config{}, false, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	if err := unmarshalConfig(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("parse config %q: %w", path, err)
	}
	// Do NOT apply FailMode default here — absent FailMode must stay "" so
	// merge() knows not to override the lower layer's value.
	return cfg, true, nil
}

// unmarshalConfig is a thin wrapper around json.Unmarshal so the parse logic
// lives in one place and tests can call it directly if needed.
func unmarshalConfig(data []byte, cfg *Config) error {
	return json.Unmarshal(data, cfg)
}

// failModeStrictness maps a FailMode string to an integer where higher means
// more restrictive. Used by mergeUntrusted to allow only tightening changes
// from low-trust layers (TM-D-01).
//
//	closed(2) > warn(1) > open(0)   — "" treated as closed (fail-closed default)
func failModeStrictness(m string) int {
	switch m {
	case FailModeClosed, "":
		return 2
	case FailModeWarn:
		return 1
	case FailModeOpen:
		return 0
	default:
		return 2 // unknown → treat as most strict (fail-closed)
	}
}

// merge applies src fields over dst only where src fields are non-zero.
// This implements the "src wins if non-zero" rule that prevents a partial
// higher layer from resetting lower-layer values to zero (Pitfall 5).
//
// Use merge for TRUSTED layers (system, user, flag); use mergeUntrusted for
// low-trust layers (project, env) to enforce security-relaxation guards
// (TM-D-01, TM-D-02).
//
// Rules per field type:
//   - string: src wins if src != "".
//   - bool:   src's bool value is applied only when the parallel rawBool sentinel
//     says src explicitly set the field — but LlamaFirewallConfig.Enabled is a
//     plain bool. Since encoding/json sets absent bools to false, we cannot
//     distinguish "absent" from "explicitly false" from the struct alone.
//     The approach taken here: for LlamaFirewall.Enabled, we apply src if
//     either (a) src.LlamaFirewall.Enabled == true (explicit enable wins), or
//     (b) the rest of LlamaFirewallConfig is non-zero (the user configured the
//     sidecar section, so trust their Enabled field). This is documented
//     intentional behaviour; a project file that sets ONLY Enabled=false cannot
//     disable a user-enabled sidecar without also setting another sidecar field.
//     This limitation is acceptable: a project that wants to disable the sidecar
//     should set a different sidecar field (e.g. fail_mode or sample_rate) too.
//   - []string: src replaces dst only when src is non-nil and non-empty.
//   - sub-structs: delegated to field-by-field src-wins-if-nonzero logic.
func merge(dst, src Config) Config {
	if src.FailMode != "" {
		dst.FailMode = src.FailMode
	}

	// SocketConfig
	if src.Socket.APIToken != "" {
		dst.Socket.APIToken = src.Socket.APIToken
	}

	// WatchSettings — pointer field; src wins if non-nil.
	if src.Watch != nil {
		if dst.Watch == nil {
			dst.Watch = &WatchSettings{}
		}
		if len(src.Watch.Directories) > 0 {
			dst.Watch.Directories = src.Watch.Directories
		}
	}

	// RedactPatterns — src replaces if non-empty.
	if len(src.RedactPatterns) > 0 {
		dst.RedactPatterns = src.RedactPatterns
	}

	// AuditConfig
	dst.Audit = mergeAudit(dst.Audit, src.Audit)

	// LlamaFirewallConfig
	dst.LlamaFirewall = mergeLlamaFirewall(dst.LlamaFirewall, src.LlamaFirewall)

	// SelfCatalogConfig
	if src.SelfCatalog.URL != "" {
		dst.SelfCatalog.URL = src.SelfCatalog.URL
	}
	if src.SelfCatalog.PubKey != "" {
		dst.SelfCatalog.PubKey = src.SelfCatalog.PubKey
	}

	// CatalogSyncConfig — pointer field (CSYNC). Trusted-layer merge: src wins
	// where set, including a disable (a trusted user/global layer MAY opt out).
	dst.CatalogSync = mergeCatalogSync(dst.CatalogSync, src.CatalogSync)

	// CorpusConfig — trusted layer: src.Corpus.Enabled=true activates corpus
	// processing; false cannot distinguish "absent" from "disabled", so a true
	// value always wins (enabling from a higher layer is always allowed on the
	// trusted path). Path and other string fields override only when non-empty.
	dst.Corpus = mergeCorpus(dst.Corpus, src.Corpus)

	// AutoQuarantineConfig — trusted layer: src wins where non-nil and non-zero.
	if src.AutoQuarantine != nil {
		if dst.AutoQuarantine == nil {
			dst.AutoQuarantine = &AutoQuarantineConfig{}
		}
		merged := mergeAutoQuarantine(*dst.AutoQuarantine, *src.AutoQuarantine)
		dst.AutoQuarantine = &merged
	}

	// PostureConfig (IPOVR-03) -- pointer field. Trusted-layer merge: a per-rule
	// action set in src wins (a trusted user/global layer MAY raise OR lower a rule).
	// Merged in BOTH merge() and mergeUntrusted() so a posture block can never be
	// silently zero-valued (do NOT repeat the v1.4.0 FRB-05 missing-merge bug).
	dst.Posture = mergePosture(dst.Posture, src.Posture)

	return dst
}

// mergeUntrusted applies src fields over dst from a LOW-TRUST layer (project
// config file, environment variables). It behaves identically to merge for
// all fields EXCEPT the four security-relaxing levers gated by TM-D-01 and
// TM-D-02:
//
//   - FailMode: tightening (closed > warn > open) is always allowed; a
//     relaxation (e.g. closed → open) is silently ignored with a stderr warning.
//   - SelfCatalog.URL / SelfCatalog.PubKey: always ignored from low-trust layers.
//   - LlamaFirewall.Enabled == false: a disable is ignored (enable is allowed).
//
// All other fields (socket token, audit sinks, redact patterns, watch dirs, etc.)
// are merged unchanged — only the security levers are gated here.
//
// The layerName parameter ("project" or "env") is included in stderr warnings
// so operators can locate the source of the refused relaxation.
func mergeUntrusted(dst, src Config, layerName string) Config {
	// --- FailMode: allow only tightening from low-trust layers (TM-D-01) ---
	if src.FailMode != "" {
		if failModeStrictness(src.FailMode) >= failModeStrictness(dst.FailMode) {
			// Equal or stricter → apply (tightening is always safe).
			dst.FailMode = src.FailMode
		} else {
			// Relaxation refused.
			fmt.Fprintf(os.Stderr,
				"beekeeper: ignoring fail_mode relaxation %q→%q from %s config layer (security)\n",
				dst.FailMode, src.FailMode, layerName)
		}
	}

	// SocketConfig — not a security-relaxing lever; apply unconditionally.
	if src.Socket.APIToken != "" {
		dst.Socket.APIToken = src.Socket.APIToken
	}

	// WatchSettings — not a security-relaxing lever; apply unconditionally.
	if src.Watch != nil {
		if dst.Watch == nil {
			dst.Watch = &WatchSettings{}
		}
		if len(src.Watch.Directories) > 0 {
			dst.Watch.Directories = src.Watch.Directories
		}
	}

	// RedactPatterns — not a security-relaxing lever; apply unconditionally.
	if len(src.RedactPatterns) > 0 {
		dst.RedactPatterns = src.RedactPatterns
	}

	// AuditConfig — the LOCAL fields (file sink, rotation size, retention) are not
	// security-relaxing, but the REMOTE-sink fields are: a poisoned project config
	// could point the audit stream at an attacker endpoint (exfil / SSRF) or add a
	// remote sink. Use the low-trust variant which refuses those levers (finding
	// #12) while still honoring local/safe audit fields.
	dst.Audit = mergeAuditUntrusted(dst.Audit, src.Audit, layerName)

	// LlamaFirewallConfig — use the low-trust merge variant (TM-D-02).
	dst.LlamaFirewall = mergeLlamaFirewallUntrusted(dst.LlamaFirewall, src.LlamaFirewall, layerName)

	// SelfCatalogConfig — always ignored from low-trust layers (TM-D-02).
	if src.SelfCatalog.URL != "" {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring self_catalog.url override from %s config layer (security)\n",
			layerName)
	}
	if src.SelfCatalog.PubKey != "" {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring self_catalog.pub_key override from %s config layer (security)\n",
			layerName)
	}

	// CatalogSyncConfig — use the low-trust variant (CSYNC-04): a disable or an
	// interval-loosening from a project/env layer is refused.
	dst.CatalogSync = mergeCatalogSyncUntrusted(dst.CatalogSync, src.CatalogSync, layerName)

	// CorpusConfig — enabling monitoring is security-enhancing, so an ENABLE from a
	// low-trust layer (project/env) is allowed (it tightens posture). A DISABLE is
	// refused: a project file must not turn off corpus monitoring.
	//
	// Corpus.Path is a TRUSTED-ONLY lever (finding #12): a poisoned repo config
	// could redirect the corpus NDJSON to an attacker-chosen file (e.g. overwriting
	// an unrelated file via the append-only store, or hiding adjudications by
	// pointing reads at an empty file). Refuse it from the untrusted layer, same
	// class as self_catalog.url and the remote audit sinks above.
	if src.Corpus.Enabled {
		dst.Corpus.Enabled = true
	}
	if src.Corpus.Path != "" {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring corpus.path override from %s config layer (security)\n",
			layerName)
	}
	// DownstreamCleanDays is a benign tuning knob (adjudication window); apply it.
	if src.Corpus.DownstreamCleanDays > 0 {
		dst.Corpus.DownstreamCleanDays = src.Corpus.DownstreamCleanDays
	}

	// AutoQuarantineConfig — Enabled=true is always allowed from low-trust layers
	// (activating quarantine tightens posture); disable is refused.
	if src.AutoQuarantine != nil && src.AutoQuarantine.Enabled {
		if dst.AutoQuarantine == nil {
			dst.AutoQuarantine = &AutoQuarantineConfig{}
		}
		dst.AutoQuarantine.Enabled = true
	}

	// PostureConfig (IPOVR-03) -- TIGHTEN-ONLY from a low-trust layer (mirrors
	// failModeStrictness / the LlamaFirewall.FailMode gate). An untrusted layer may
	// raise a rule warn->block (a tightening); a block->warn loosening is refused.
	// Merged in BOTH merge() and mergeUntrusted() (FRB-05 missing-merge guard).
	dst.Posture = mergePostureUntrusted(dst.Posture, src.Posture, layerName)

	return dst
}

func mergeAudit(dst, src AuditConfig) AuditConfig {
	if len(src.Sinks) > 0 {
		dst.Sinks = src.Sinks
	}
	if src.SyslogAddress != "" {
		dst.SyslogAddress = src.SyslogAddress
	}
	if src.OTLPEndpoint != "" {
		dst.OTLPEndpoint = src.OTLPEndpoint
	}
	if src.HTTPSEndpoint != "" {
		dst.HTTPSEndpoint = src.HTTPSEndpoint
	}
	if src.RetentionDays > 0 {
		dst.RetentionDays = src.RetentionDays
	}
	if src.MaxSizeBytes > 0 {
		dst.MaxSizeBytes = src.MaxSizeBytes
	}
	return dst
}

// remoteAuditSinks is the set of audit sink names that ship records OFF the local
// machine. These are trusted-only: an untrusted (project/env) layer must not be
// able to add a remote sink, because the matching endpoint field is itself
// trusted-only (an attacker endpoint = exfil/SSRF). "file" (and any non-remote
// sink) is local and safe.
var remoteAuditSinks = map[string]bool{
	"syslog": true,
	"otlp":   true,
	"http":   true,
	"https":  true,
}

// mergeAuditUntrusted is the low-trust variant of mergeAudit (finding #12). The
// remote-egress levers are refused from a project/env layer; local/safe fields
// (the file sink, rotation size, retention) are still honored:
//
//   - OTLPEndpoint / HTTPSEndpoint / SyslogAddress: always refused — a poisoned
//     repo config must not be able to point the audit stream at an attacker host.
//     These are the same class as self_catalog.url.
//   - Sinks: a remote sink name (otlp/http/https/syslog) added by the untrusted
//     layer is stripped; local sinks (e.g. "file") pass through. Without an
//     accompanying endpoint a remote sink is inert, but stripping it keeps the
//     refusal explicit and defends against any future endpoint default.
//   - RetentionDays / MaxSizeBytes: local rotation knobs; applied unconditionally.
func mergeAuditUntrusted(dst, src AuditConfig, layerName string) AuditConfig {
	if len(src.Sinks) > 0 {
		kept := make([]string, 0, len(src.Sinks))
		var refused []string
		for _, s := range src.Sinks {
			if remoteAuditSinks[strings.ToLower(strings.TrimSpace(s))] {
				refused = append(refused, s)
				continue
			}
			kept = append(kept, s)
		}
		if len(refused) > 0 {
			fmt.Fprintf(os.Stderr,
				"beekeeper: ignoring remote audit sink(s) %v from %s config layer (security)\n",
				refused, layerName)
		}
		if len(kept) > 0 {
			dst.Sinks = kept
		}
	}

	// Remote endpoint fields are trusted-only — refuse them from this layer.
	if src.SyslogAddress != "" {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring audit.syslog_address override from %s config layer (security)\n",
			layerName)
	}
	if src.OTLPEndpoint != "" {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring audit.otlp_endpoint override from %s config layer (security)\n",
			layerName)
	}
	if src.HTTPSEndpoint != "" {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring audit.https_endpoint override from %s config layer (security)\n",
			layerName)
	}

	// Local rotation/retention knobs are safe to honor.
	if src.RetentionDays > 0 {
		dst.RetentionDays = src.RetentionDays
	}
	if src.MaxSizeBytes > 0 {
		dst.MaxSizeBytes = src.MaxSizeBytes
	}
	return dst
}

func mergeLlamaFirewall(dst, src LlamaFirewallConfig) LlamaFirewallConfig {
	// Apply src LlamaFirewall.Enabled if true OR if src has any other non-zero
	// sidecar field (indicating the user configured the sidecar section).
	srcHasOtherFields := src.SampleRate > 0 || src.FailMode != "" ||
		src.CodeShield ||
		src.CodeShieldAction != "" || src.PythonPath != ""

	if src.Enabled || srcHasOtherFields {
		dst.Enabled = src.Enabled
	}
	if src.SampleRate > 0 {
		dst.SampleRate = src.SampleRate
	}
	if src.FailMode != "" {
		dst.FailMode = src.FailMode
	}
	if src.CodeShield {
		dst.CodeShield = src.CodeShield
	}
	if src.CodeShieldAction != "" {
		dst.CodeShieldAction = src.CodeShieldAction
	}
	if src.PythonPath != "" {
		dst.PythonPath = src.PythonPath
	}
	return dst
}

// mergeLlamaFirewallUntrusted is the low-trust variant of mergeLlamaFirewall
// (TM-D-02). It refuses the THREE security-relaxing sidecar levers from a
// project/env layer; all other fields are merged unconditionally:
//
//   - Enabled: a disable (src.Enabled == false) is refused when the lower layer
//     has the sidecar enabled; an explicit enable is always allowed.
//   - FailMode: a RELAXATION (closed/warn → open, i.e. a lower strictness) is
//     refused. An equal-or-stricter value is honored. Uses the same
//     failModeStrictness ordering as the top-level fail_mode gate (TM-D-01) so a
//     project config cannot flip the sidecar to fail-open while it stays
//     "enabled" (finding #4).
//   - SampleRate: a REDUCTION (a lower fraction of tool calls forwarded to the
//     sidecar) is refused. A lower sample rate means fewer tool calls are
//     scanned, which is a relaxation; only an equal-or-higher rate is honored.
//     The effective lower-layer rate is resolved via LlamaFirewallSampleRate so
//     an unset (zero) dst is compared against its real default of 1.0 — a project
//     setting sample_rate:0.0001 against an unset user layer is therefore
//     correctly refused (finding #4).
func mergeLlamaFirewallUntrusted(dst, src LlamaFirewallConfig, layerName string) LlamaFirewallConfig {
	srcHasOtherFields := src.SampleRate > 0 || src.FailMode != "" ||
		src.CodeShield ||
		src.CodeShieldAction != "" || src.PythonPath != ""

	if src.Enabled {
		// Explicit enable from low-trust layer: always honored.
		dst.Enabled = true
	} else if srcHasOtherFields && !src.Enabled && dst.Enabled {
		// Low-trust layer has other sidecar fields but sets Enabled=false while
		// the lower layer has it enabled → refuse the disable.
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring llamafirewall.enabled:false from %s config layer (security)\n",
			layerName)
	}

	// SampleRate: refuse a reduction (relaxation). Compare against the EFFECTIVE
	// lower-layer rate (LlamaFirewallSampleRate resolves an unset 0 to the 1.0
	// default) so a project sample_rate:0.0001 cannot neuter an enabled sidecar
	// that left sample_rate at its default.
	if src.SampleRate > 0 {
		dstEffective := Config{LlamaFirewall: dst}.LlamaFirewallSampleRate()
		if src.SampleRate >= dstEffective {
			dst.SampleRate = src.SampleRate
		} else {
			fmt.Fprintf(os.Stderr,
				"beekeeper: ignoring llamafirewall.sample_rate reduction %g→%g from %s config layer (security)\n",
				dstEffective, src.SampleRate, layerName)
		}
	}

	// FailMode: allow only an equal-or-stricter value (TM-D-01 ordering). A
	// relaxation (e.g. closed/warn → open) is refused so an untrusted layer cannot
	// turn the sidecar fail-open while it remains enabled.
	if src.FailMode != "" {
		if failModeStrictness(src.FailMode) >= failModeStrictness(dst.FailMode) {
			dst.FailMode = src.FailMode
		} else {
			fmt.Fprintf(os.Stderr,
				"beekeeper: ignoring llamafirewall.fail_mode relaxation %q→%q from %s config layer (security)\n",
				dst.FailMode, src.FailMode, layerName)
		}
	}

	if src.CodeShield {
		dst.CodeShield = src.CodeShield
	}
	if src.CodeShieldAction != "" {
		dst.CodeShieldAction = src.CodeShieldAction
	}
	if src.PythonPath != "" {
		dst.PythonPath = src.PythonPath
	}
	return dst
}

// mergeCatalogSync merges the src CatalogSync pointer over dst following the
// layered "src wins if set" discipline. This is the TRUSTED-layer variant: a
// trusted user/global layer MAY disable sync or set any in-range interval.
//
// Semantics:
//   - src == nil: absent in this layer → lower layer authoritative, dst returned.
//   - dst == nil, src != nil: adopt a copy of src.
//   - both non-nil: Enabled follows the bool convention (a bare /
//     interval-less object asserts Enabled; with an interval present only an
//     explicit enable applies so a partial override never silently disables);
//     Interval is src-wins-if-non-empty.
func mergeCatalogSync(dst, src *CatalogSyncConfig) *CatalogSyncConfig {
	if src == nil {
		return dst
	}
	if dst == nil {
		cp := *src
		return &cp
	}
	out := *dst

	srcHasOtherSignal := src.Interval != ""
	if !srcHasOtherSignal || src.Enabled {
		out.Enabled = src.Enabled
	}
	if src.Interval != "" {
		out.Interval = src.Interval
	}
	return &out
}

// mergeCatalogSyncUntrusted is the low-trust variant of mergeCatalogSync
// (CSYNC-04). Both security-relaxing levers are gated:
//
//   - Enabled: a disable (src.Enabled == false) from a project/env layer is
//     refused when the lower layer has sync enabled; an explicit enable is
//     always honored (tightening is safe).
//   - Interval: a LOOSENING (a longer/less-frequent interval) is refused; a
//     tightening (shorter or equal) interval is honored. Disabling sync or
//     letting the catalog go stale widens the hijacked-agent window, so neither
//     can be set by an untrusted layer.
func mergeCatalogSyncUntrusted(dst, src *CatalogSyncConfig, layerName string) *CatalogSyncConfig {
	if src == nil {
		return dst
	}
	if dst == nil {
		// No trusted baseline to protect — adopt src as-is.
		cp := *src
		return &cp
	}
	out := *dst

	// Enabled: refuse a disable; honor an enable.
	if src.Enabled {
		out.Enabled = true
	} else if dst.Enabled {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring catalog_sync.enabled:false from %s config layer (security)\n",
			layerName)
		// out.Enabled stays dst.Enabled (true)
	}

	// Interval: refuse a loosening, honor a tightening.
	if src.Interval != "" {
		srcDur := parseClampCatalogSyncInterval(src.Interval)
		dstDur := parseClampCatalogSyncInterval(dst.Interval)
		if srcDur <= dstDur {
			out.Interval = src.Interval
		} else {
			fmt.Fprintf(os.Stderr,
				"beekeeper: ignoring catalog_sync.interval loosening %q→%q from %s config layer (security)\n",
				dst.Interval, src.Interval, layerName)
		}
	}

	return &out
}

// applyEnvVars applies BEEKEEPER_* environment variables as the fourth layer.
// Only the hardcoded known set of variables is mapped; unknown BEEKEEPER_* vars
// and all other env vars are silently ignored (T-09-05: no reflective application).
//
// Deprecated internal call path: use applyEnvVarsUntrusted for the env layer in
// LoadLayered (env vars are low-trust). This function is retained for callers that
// explicitly need unrestricted env application (e.g. flag layer helpers).
func applyEnvVars(cfg Config, environ []string) Config {
	env := parseEnvSlice(environ)

	if v, ok := env["BEEKEEPER_FAIL_MODE"]; ok {
		cfg.FailMode = v
	}
	if v, ok := env["BEEKEEPER_SOCKET_API_TOKEN"]; ok {
		cfg.Socket.APIToken = v
	}
	if v, ok := env["BEEKEEPER_LLAMAFIREWALL_ENABLED"]; ok {
		switch strings.ToLower(v) {
		case "true", "1", "yes":
			cfg.LlamaFirewall.Enabled = true
		case "false", "0", "no":
			cfg.LlamaFirewall.Enabled = false
		}
	}
	if v, ok := env["BEEKEEPER_AUDIT_SINKS"]; ok && v != "" {
		parts := strings.Split(v, ",")
		sinks := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				sinks = append(sinks, s)
			}
		}
		if len(sinks) > 0 {
			cfg.Audit.Sinks = sinks
		}
	}
	if v, ok := env["BEEKEEPER_SELF_CATALOG_URL"]; ok && v != "" {
		cfg.SelfCatalog.URL = v
	}

	return cfg
}

// applyEnvVarsUntrusted is the low-trust variant of applyEnvVars used by
// LoadLayered for the env layer (TM-D-01, TM-D-02). Security-relaxing levers
// (FailMode relaxation, SelfCatalog.URL, LlamaFirewall disable) are refused
// from the env layer; all other env vars are applied unconditionally.
func applyEnvVarsUntrusted(cfg Config, environ []string) Config {
	env := parseEnvSlice(environ)

	// FailMode: apply only if not relaxing (TM-D-01).
	if v, ok := env["BEEKEEPER_FAIL_MODE"]; ok && v != "" {
		if failModeStrictness(v) >= failModeStrictness(cfg.FailMode) {
			cfg.FailMode = v
		} else {
			fmt.Fprintf(os.Stderr,
				"beekeeper: ignoring fail_mode relaxation %q→%q from env config layer (security)\n",
				cfg.FailMode, v)
		}
	}

	// SocketAPIToken — not a security-relaxing lever; apply unconditionally.
	if v, ok := env["BEEKEEPER_SOCKET_API_TOKEN"]; ok {
		cfg.Socket.APIToken = v
	}

	// LlamaFirewall enable/disable — enable is always allowed; disable refused (TM-D-02).
	if v, ok := env["BEEKEEPER_LLAMAFIREWALL_ENABLED"]; ok {
		switch strings.ToLower(v) {
		case "true", "1", "yes":
			cfg.LlamaFirewall.Enabled = true
		case "false", "0", "no":
			if cfg.LlamaFirewall.Enabled {
				fmt.Fprintf(os.Stderr,
					"beekeeper: ignoring llamafirewall.enabled:false from env config layer (security)\n")
			} else {
				cfg.LlamaFirewall.Enabled = false
			}
		}
	}

	// AuditSinks — local sinks (e.g. "file") apply; REMOTE sinks (otlp/http/https/
	// syslog) are refused from the env layer (finding #12), mirroring the project-
	// layer mergeAuditUntrusted refusal. A remote sink's endpoint field is itself
	// trusted-only, so an env-set remote sink must not be honored.
	if v, ok := env["BEEKEEPER_AUDIT_SINKS"]; ok && v != "" {
		parts := strings.Split(v, ",")
		sinks := make([]string, 0, len(parts))
		var refused []string
		for _, p := range parts {
			s := strings.TrimSpace(p)
			if s == "" {
				continue
			}
			if remoteAuditSinks[strings.ToLower(s)] {
				refused = append(refused, s)
				continue
			}
			sinks = append(sinks, s)
		}
		if len(refused) > 0 {
			fmt.Fprintf(os.Stderr,
				"beekeeper: ignoring remote audit sink(s) %v from env config layer (security)\n",
				refused)
		}
		if len(sinks) > 0 {
			cfg.Audit.Sinks = sinks
		}
	}

	// SelfCatalog.URL — always refused from env layer (TM-D-02).
	if v, ok := env["BEEKEEPER_SELF_CATALOG_URL"]; ok && v != "" {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring self_catalog.url override from env config layer (security)\n")
		_ = v // refused
	}

	return cfg
}

// applyFlagOverrides applies CLI flag values as the fifth and highest-precedence
// layer. Keys use the logical name (same as the BEEKEEPER_* suffix, lower-cased):
// "fail_mode", "socket_api_token", "llamafirewall_enabled",
// "audit_sinks", "self_catalog_url".
func applyFlagOverrides(cfg Config, overrides map[string]string) Config {
	if overrides == nil {
		return cfg
	}
	if v, ok := overrides["fail_mode"]; ok && v != "" {
		cfg.FailMode = v
	}
	if v, ok := overrides["socket_api_token"]; ok && v != "" {
		cfg.Socket.APIToken = v
	}
	if v, ok := overrides["llamafirewall_enabled"]; ok {
		switch strings.ToLower(v) {
		case "true", "1", "yes":
			cfg.LlamaFirewall.Enabled = true
		case "false", "0", "no":
			cfg.LlamaFirewall.Enabled = false
		}
	}
	if v, ok := overrides["audit_sinks"]; ok && v != "" {
		parts := strings.Split(v, ",")
		sinks := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				sinks = append(sinks, s)
			}
		}
		if len(sinks) > 0 {
			cfg.Audit.Sinks = sinks
		}
	}
	if v, ok := overrides["self_catalog_url"]; ok && v != "" {
		cfg.SelfCatalog.URL = v
	}
	return cfg
}

// validate checks the merged Config for correctness and returns an error if
// invalid (same FailMode rule as Load). Reuses the Load FailMode switch pattern
// so the validation behaviour is identical regardless of which load path was used.
func validate(cfg Config) (Config, error) {
	if cfg.FailMode == "" {
		cfg.FailMode = FailModeClosed
	}
	switch cfg.FailMode {
	case FailModeClosed, FailModeOpen, FailModeWarn:
		// valid
	default:
		return Config{}, fmt.Errorf("invalid fail_mode %q after merge (want %q, %q, or %q)",
			cfg.FailMode, FailModeClosed, FailModeOpen, FailModeWarn)
	}
	return cfg, nil
}

// parseEnvSlice converts a KEY=VALUE slice (e.g. os.Environ()) into a map.
// Entries without "=" are skipped. Only the first "=" is treated as delimiter
// so values that contain "=" are handled correctly.
func parseEnvSlice(environ []string) map[string]string {
	m := make(map[string]string, len(environ))
	for _, e := range environ {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			continue
		}
		m[e[:idx]] = e[idx+1:]
	}
	return m
}

// mergeCorpus merges a CorpusConfig src over dst for the trusted layer.
// Enabling corpus (src.Enabled=true) always wins. Disable cannot be
// distinguished from "absent" (Go zero bool), so false never clobbers a
// lower-layer true. String fields override when non-empty.
func mergeCorpus(dst, src CorpusConfig) CorpusConfig {
	if src.Enabled {
		dst.Enabled = true
	}
	if src.Path != "" {
		dst.Path = src.Path
	}
	if src.DownstreamCleanDays > 0 {
		dst.DownstreamCleanDays = src.DownstreamCleanDays
	}
	return dst
}

// mergeAutoQuarantine merges an AutoQuarantineConfig src over dst for the
// trusted layer. Enabling auto-quarantine (src.Enabled=true) always wins;
// false cannot be distinguished from absent so it never clobbers a lower-layer
// true. Threshold and DryRun override when non-zero/true.
func mergeAutoQuarantine(dst, src AutoQuarantineConfig) AutoQuarantineConfig {
	if src.Enabled {
		dst.Enabled = true
	}
	if src.DryRun {
		dst.DryRun = true
	}
	if src.Threshold != 0 {
		dst.Threshold = src.Threshold
	}
	return dst
}

// postureActionStrictness maps a posture action to an integer where higher means
// more restrictive. Used by mergePostureUntrusted to allow only tightening changes
// from low-trust layers (IPOVR-03), mirroring failModeStrictness (TM-D-01).
//
//	block(1) > warn(0)   -- "" (unset) is treated as warn (the shipped default)
func postureActionStrictness(action string) int {
	if action == PostureActionBlock {
		return 1
	}
	return 0 // "" and "warn" are equally the warn-default
}

// mergePosture merges the src Posture pointer over dst for the TRUSTED layer.
// A trusted user/global layer MAY raise OR lower a per-rule action, so a non-empty
// src action wins for that rule; an empty src action leaves the lower layer's value.
//
//   - src == nil: absent in this layer -> lower layer authoritative, dst returned.
//   - dst == nil, src != nil: adopt a copy of src.
//   - both non-nil: per rule, src.Action wins when non-empty.
func mergePosture(dst, src *PostureConfig) *PostureConfig {
	if src == nil {
		return dst
	}
	if dst == nil {
		cp := *src
		return &cp
	}
	out := *dst
	if src.ReleaseAge.Action != "" {
		out.ReleaseAge.Action = src.ReleaseAge.Action
	}
	if src.Lifecycle.Action != "" {
		out.Lifecycle.Action = src.Lifecycle.Action
	}
	if src.RemoteSource.Action != "" {
		out.RemoteSource.Action = src.RemoteSource.Action
	}
	// Allow (IPOVR-01/02): the scoped allow-always list is additive. From a TRUSTED
	// layer, src entries are appended to the lower layer's entries (a higher trusted
	// layer can add standing exemptions). Untrusted layers DROP Allow entirely (see
	// mergePostureUntrusted) because adding an allow LOOSENS the posture.
	if len(src.Allow) > 0 {
		out.Allow = append(append([]PostureAllow(nil), dst.Allow...), src.Allow...)
	}
	return &out
}

// mergePostureUntrusted is the low-trust variant of mergePosture (IPOVR-03). It is
// TIGHTEN-ONLY: per rule, the src action is applied ONLY when it is equal-or-stricter
// than the lower layer's effective action (postureActionStrictness(src) >=
// postureActionStrictness(dst)). A loosening (block -> warn, or block -> "") from a
// project/env layer is refused with a stderr warning, mirroring the FailMode and
// LlamaFirewall.FailMode untrusted gates. This is the IPOVR-03 self-defense
// invariant: a poisoned repo config can raise a rule to block but can never opt a
// rule that the user blocked back down to warn.
func mergePostureUntrusted(dst, src *PostureConfig, layerName string) *PostureConfig {
	if src == nil {
		return dst
	}
	// Allow (IPOVR-01/02): an untrusted layer may NOT add a scoped allow-always
	// entry. Adding an allow LOOSENS the posture (it silences a rule for a package),
	// so a poisoned project/env config must never introduce an exemption. Drop
	// src.Allow with a warning whenever the untrusted layer tries to add one.
	if len(src.Allow) > 0 {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring %d posture.allow entr(ies) from %s config layer (security: an untrusted layer cannot add an allow)\n",
			len(src.Allow), layerName)
	}
	if dst == nil {
		// No trusted baseline to protect -- adopt src's per-rule ACTIONS as-is (they
		// can only tighten relative to the warn default). Allow is DROPPED: an
		// untrusted layer never contributes a standing exemption.
		out := PostureConfig{
			ReleaseAge:   src.ReleaseAge,
			Lifecycle:    src.Lifecycle,
			RemoteSource: src.RemoteSource,
			// Allow intentionally omitted (dropped from the untrusted layer).
		}
		return &out
	}
	out := *dst
	// Preserve the trusted lower layer's Allow list unchanged; never append src's.
	out.Allow = append([]PostureAllow(nil), dst.Allow...)
	out.ReleaseAge.Action = tightenPostureAction(dst.ReleaseAge.Action, src.ReleaseAge.Action, PostureRuleReleaseAge, layerName)
	out.Lifecycle.Action = tightenPostureAction(dst.Lifecycle.Action, src.Lifecycle.Action, PostureRuleLifecycle, layerName)
	out.RemoteSource.Action = tightenPostureAction(dst.RemoteSource.Action, src.RemoteSource.Action, PostureRuleRemoteSource, layerName)
	return &out
}

// tightenPostureAction returns the effective action for one rule under the
// tighten-only untrusted rule. An empty src means "absent in this layer" -> keep
// dst. A non-empty src is applied only when it is equal-or-stricter than dst; a
// loosening is refused (kept at dst) with a stderr warning naming the rule and layer.
func tightenPostureAction(dstAction, srcAction, rule, layerName string) string {
	if srcAction == "" {
		return dstAction // absent in this layer -> lower layer authoritative
	}
	if postureActionStrictness(srcAction) >= postureActionStrictness(dstAction) {
		return srcAction // equal or stricter -> tightening is always safe
	}
	fmt.Fprintf(os.Stderr,
		"beekeeper: ignoring posture.%s action relaxation %q->%q from %s config layer (security)\n",
		rule, effectivePostureAction(dstAction), srcAction, layerName)
	return dstAction
}

// effectivePostureAction renders an action for a log message, mapping the unset
// "" to its effective "warn" so the warning reads sensibly.
func effectivePostureAction(action string) string {
	if action == "" {
		return PostureActionWarn
	}
	return action
}
