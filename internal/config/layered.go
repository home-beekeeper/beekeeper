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

	// CLEAN-02: guarantee a non-nil, validated Nudge at the layered root, mirroring
	// Load's single-file contract. After all layers merge, an absent nudge across
	// every layer resolves to DefaultNudgeConfig(); a present (merged) block is
	// validated fail-closed via the EXPORTED ValidateNudgeConfig so an invalid
	// project nudge (e.g. mode:"aggressive") is rejected here rather than silently
	// degrading. This makes the consumer-side nil-guards defense-in-depth, not
	// load-bearing (T-09-07).
	if cfg.Nudge == nil {
		d := DefaultNudgeConfig()
		cfg.Nudge = &d
	} else if err := ValidateNudgeConfig(*cfg.Nudge); err != nil {
		return Config{}, fmt.Errorf("invalid merged nudge config: %w", err)
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

	// NudgeConfig — pointer field (CLEAN-02). Without this, a project-layer
	// nudge.* override was silently dropped and any LoadLayered consumer reading
	// cfg.Nudge without a nil-check got nil. mergeNudge carries the block through.
	dst.Nudge = mergeNudge(dst.Nudge, src.Nudge)

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
//   - Nudge.Enabled == false: a disable is ignored (enable is allowed) when the
//     nudge block contains ONLY the disable (srcHasOtherSignal == false) — the
//     project-disable path defined in NUDGE-08/§11. When the nudge block carries
//     other fields, the standard mergeNudge logic applies and an absent
//     (false) Enabled does not clobber the lower layer.
//
// All other fields (socket token, audit sinks, redact patterns, watch dirs, etc.)
// are merged unchanged — only the four security levers are gated here.
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

	// AuditConfig — not a security-relaxing lever; apply unconditionally.
	dst.Audit = mergeAudit(dst.Audit, src.Audit)

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

	// NudgeConfig — use the low-trust nudge merge variant (TM-D-02).
	dst.Nudge = mergeNudgeUntrusted(dst.Nudge, src.Nudge, layerName)

	return dst
}

// mergeNudge merges the src Nudge pointer over dst following the layered "src
// wins if set" discipline, and is the CLEAN-02 root-cause fix for the dropped
// Nudge pointer in merge().
//
// Semantics:
//   - src == nil: lower layer (dst) is authoritative — an ABSENT nudge key in a
//     higher layer never zeroes a lower-layer block. Returns dst unchanged.
//     (loadIfPresent does NOT default-fill, so a project file WITHOUT a "nudge"
//     key yields src==nil and the lower layer survives intact — Pitfall 5.)
//   - dst == nil, src != nil: start from a copy of *src (no lower layer to merge).
//   - both non-nil: field-level override, src wins where set (non-empty / non-zero).
//
// Bool fields (Enabled, RequireHardened, CheckSocketScanner) share the
// "encoding/json cannot distinguish absent from false" limitation already
// documented on mergeLlamaFirewall. We resolve it to satisfy BOTH the
// NUDGE-08 / PRD §11 project-disable (`nudge.enabled:false` must win) and the
// partial-override case (a `mode`-only project layer must NOT silently disable
// the nudge):
//
//   - srcHasOtherSignal == false (a disable-only / bare object such as
//     `{"nudge":{"enabled":false}}` — the ONLY way Enabled is the meaningful
//     field): the project object IS the Enabled assertion → src.Enabled wins.
//     This lands the §11 project-disable.
//   - srcHasOtherSignal == true (e.g. `{"nudge":{"mode":"hard"}}`): the project
//     configured some OTHER field; its absent `enabled` (Go zero false) must NOT
//     clobber the lower-layer Enabled. So src.Enabled is applied only when it is
//     explicitly true (an explicit enable still wins); otherwise Enabled is
//     inherited. A project that has other nudge fields AND wants to disable must
//     set `enabled:false` explicitly together with them — the same accepted
//     limitation as LlamaFirewall.Enabled, documented here.
//
// RequireHardened / CheckSocketScanner follow the LlamaFirewall convention:
// applied only when src carries some other non-zero nudge signal.
//
// Strings (Mode, Preferred) and the nested VersionFloors / MajorDriftCheck
// fields are src-wins-if-non-empty / non-zero, so a partial project override
// never clobbers lower-layer non-zero values.
func mergeNudge(dst, src *NudgeConfig) *NudgeConfig {
	if src == nil {
		// Absent in this layer → do not touch the lower layer.
		return dst
	}
	if dst == nil {
		// No lower layer to merge against — adopt a copy of src.
		cp := *src
		return &cp
	}

	out := *dst // copy so we never mutate the caller's struct

	srcHasOtherSignal := src.Mode != "" || src.Preferred != "" ||
		src.RequireHardened || src.CheckSocketScanner ||
		src.MajorDriftCheck != (NudgeMajorDriftCheck{}) ||
		src.VersionFloors != (NudgeVersionFloors{})

	// Enabled resolution (see doc comment): a disable-only object is the Enabled
	// assertion (§11 project-disable wins); when other fields are present, only an
	// explicit enable applies so a partial override never silently disables.
	if !srcHasOtherSignal || src.Enabled {
		out.Enabled = src.Enabled
	}

	// The other bools follow the LlamaFirewall convention: apply only when src
	// carries another non-zero nudge signal (so a bare project disable object
	// cannot silently flip them).
	if srcHasOtherSignal {
		out.RequireHardened = src.RequireHardened
		out.CheckSocketScanner = src.CheckSocketScanner
	}

	// Strings: src wins if non-empty.
	if src.Mode != "" {
		out.Mode = src.Mode
	}
	if src.Preferred != "" {
		out.Preferred = src.Preferred
	}

	// MajorDriftCheck — field-level src-wins-if-set.
	if src.MajorDriftCheck.Enabled {
		out.MajorDriftCheck.Enabled = true
	}
	if src.MajorDriftCheck.Interval != "" {
		out.MajorDriftCheck.Interval = src.MajorDriftCheck.Interval
	}

	// VersionFloors — field-level src-wins-if-non-empty (no zeroing).
	if src.VersionFloors.Pnpm != "" {
		out.VersionFloors.Pnpm = src.VersionFloors.Pnpm
	}
	if src.VersionFloors.Bun != "" {
		out.VersionFloors.Bun = src.VersionFloors.Bun
	}
	if src.VersionFloors.Node != "" {
		out.VersionFloors.Node = src.VersionFloors.Node
	}

	return &out
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

func mergeLlamaFirewall(dst, src LlamaFirewallConfig) LlamaFirewallConfig {
	// Apply src LlamaFirewall.Enabled if true OR if src has any other non-zero
	// sidecar field (indicating the user configured the sidecar section).
	srcHasOtherFields := src.SampleRate > 0 || src.FailMode != "" ||
		src.CodeShield || src.AlignmentCheck ||
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
	if src.AlignmentCheck {
		dst.AlignmentCheck = src.AlignmentCheck
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
// (TM-D-02). A disable (src.Enabled == false) from a low-trust layer is refused
// when the lower layer has LlamaFirewall enabled; an explicit enable is always
// allowed. All other non-security fields are merged unconditionally.
func mergeLlamaFirewallUntrusted(dst, src LlamaFirewallConfig, layerName string) LlamaFirewallConfig {
	srcHasOtherFields := src.SampleRate > 0 || src.FailMode != "" ||
		src.CodeShield || src.AlignmentCheck ||
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

	if src.SampleRate > 0 {
		dst.SampleRate = src.SampleRate
	}
	if src.FailMode != "" {
		dst.FailMode = src.FailMode
	}
	if src.CodeShield {
		dst.CodeShield = src.CodeShield
	}
	if src.AlignmentCheck {
		dst.AlignmentCheck = src.AlignmentCheck
	}
	if src.CodeShieldAction != "" {
		dst.CodeShieldAction = src.CodeShieldAction
	}
	if src.PythonPath != "" {
		dst.PythonPath = src.PythonPath
	}
	return dst
}

// mergeNudgeUntrusted is the low-trust variant of mergeNudge (TM-D-02).
// A project/env layer's nudge.enabled:false is refused when the lower layer
// has Nudge enabled; enables from low-trust layers are always allowed. All
// other nudge sub-fields follow the standard mergeNudge logic.
func mergeNudgeUntrusted(dst, src *NudgeConfig, layerName string) *NudgeConfig {
	if src == nil {
		return dst
	}
	if dst == nil {
		// No lower layer to merge — adopt src, but gate the Enabled field.
		cp := *src
		// If the src has only Enabled:false and nothing else, treat as default.
		// (No lower layer means we cannot "refuse" a relax — start from src as-is.)
		return &cp
	}

	// Determine whether src carries signal beyond Enabled.
	srcHasOtherSignal := src.Mode != "" || src.Preferred != "" ||
		src.RequireHardened || src.CheckSocketScanner ||
		src.MajorDriftCheck != (NudgeMajorDriftCheck{}) ||
		src.VersionFloors != (NudgeVersionFloors{})

	// Gate: refuse a disable from a low-trust layer when the lower layer has nudge enabled.
	// A disable-only object (!srcHasOtherSignal && !src.Enabled) from low-trust is refused.
	if !srcHasOtherSignal && !src.Enabled && dst.Enabled {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring nudge.enabled:false from %s config layer (security)\n",
			layerName)
		// Run the standard merge with Enabled forced to true so all other fields
		// are preserved but the disable is suppressed. Since src has no other
		// signal, this effectively keeps dst unchanged.
		srcCopy := *src
		srcCopy.Enabled = dst.Enabled
		return mergeNudge(dst, &srcCopy)
	}

	// For all other cases (enable, or disable accompanied by other nudge fields,
	// or disable when lower layer already has it disabled) use standard merge —
	// but if it's a disable with other fields and the lower layer is enabled, warn.
	if srcHasOtherSignal && !src.Enabled && dst.Enabled {
		fmt.Fprintf(os.Stderr,
			"beekeeper: ignoring nudge.enabled:false from %s config layer (security)\n",
			layerName)
		srcCopy := *src
		srcCopy.Enabled = dst.Enabled
		return mergeNudge(dst, &srcCopy)
	}

	return mergeNudge(dst, src)
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

	// AuditSinks — not a security-relaxing lever; apply unconditionally.
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
