// posture_override.go - Cobra wiring for the scoped posture override CLI
// (IPOVR-01/02, Plan 29-02): `beekeeper posture allow` and `beekeeper posture
// enforce`. These are the actionable surface for the graduated response a posture
// warn/block offers: allow once, allow always (with a recorded reason), or raise
// a rule to block. Each override writes a DISTINCT `posture_override` audit record.
//
// Architecture constraint: this file is thin wiring. The standing-exception
// machinery lives in internal/config (PostureAllow / AddPostureAllow) and the
// one-shot store lives in internal/check (AddAllowOnce); the audit record is built
// and written here at the live audit path (mirroring writeConfigChangeRecord).
//
// SECURITY: `allow --always` appends to the POSTURE-SCOPED config.Posture.Allow
// list, NOT the general package_allowlist policy rule. A posture allow-always
// therefore silences a posture warn but never downgrades a catalog/corroboration
// malware block (T-09-31). See internal/config.PostureAllow.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/check"
	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// Posture override action discriminators recorded in the audit trail. These are
// the distinct `posture_override_action` values the verifier and the audit viewer
// key on.
const (
	postureOverrideAllowOnce    = "allow_once"
	postureOverrideAllowAlways  = "allow_always"
	postureOverrideEnforceBlock = "enforce_block"
	postureOverrideEnforceWarn  = "enforce_warn"
)

// addPostureAllowFn is the seam for the one-shot store write so tests can drive
// the CLI against a temp state dir without resolving the real platform.StateDir.
// Production assigns check.AddAllowOnce.
var addPostureAllowOnceFn = check.AddAllowOnce

// postureStateDirFn resolves the state directory for the allow-once store. Tests
// swap it to a temp dir; production resolves platform.StateDir.
var postureStateDirFn = platform.StateDir

// addPostureOverrideCommands registers `allow` and `enforce` under the existing
// read-only `posture` command. The parent posture command stays read-only; these
// two subcommands are the actionable override surface.
func addPostureOverrideCommands(postureCmd *cobra.Command) {
	postureCmd.AddCommand(newPostureAllowCmd())
	postureCmd.AddCommand(newPostureEnforceCmd())
}

// newPostureAllowCmd implements `beekeeper posture allow <pkg> [--ecosystem E]
// [--rule release-age|lifecycle|git-remote] (--once | --always) --reason "..."`.
func newPostureAllowCmd() *cobra.Command {
	var (
		ecosystem string
		rule      string
		once      bool
		always    bool
		reason    string
	)
	cmd := &cobra.Command{
		Use:   "allow <package>",
		Short: "Allow a package past the install posture (once, or always with a recorded reason)",
		Long: `Grant a scoped, audited exception to the install posture for a package.

  --once    Allow the NEXT matching install of this package, then warn again.
            A one-shot token is recorded and consumed on the next install.
  --always  Record a standing exception so this package stops firing the posture
            rule(s). --reason is required (the justification is recorded).

The exception is POSTURE-SCOPED: it silences a posture warn for this package but
NEVER downgrades a catalog/corroboration malware block for the same package. Use
--rule to scope the exception to one rule; omit it to exempt all posture rules.

Each invocation writes a distinct posture_override audit record.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := strings.TrimSpace(args[0])
			if pkg == "" {
				return fmt.Errorf("posture allow: package is required")
			}
			if once == always {
				return fmt.Errorf("posture allow: exactly one of --once or --always is required")
			}
			rule = strings.TrimSpace(rule)
			if !validPostureRuleFlag(rule) {
				return fmt.Errorf("posture allow: invalid --rule %q (want %q, %q, %q, or empty for all rules)",
					rule, config.PostureRuleReleaseAge, config.PostureRuleLifecycle, config.PostureRuleRemoteSource)
			}
			reason = strings.TrimSpace(reason)

			if always {
				if reason == "" {
					return fmt.Errorf("posture allow --always: --reason is required (the justification is recorded)")
				}
				return runPostureAllowAlways(cmd, ecosystem, pkg, rule, reason)
			}
			return runPostureAllowOnce(cmd, ecosystem, pkg, rule, reason)
		},
	}
	cmd.Flags().StringVar(&ecosystem, "ecosystem", "", "Scope the exception to one ecosystem (e.g. npm); empty matches any")
	cmd.Flags().StringVar(&rule, "rule", "", "Scope to one rule: release-age|lifecycle|git-remote; empty = all posture rules")
	cmd.Flags().BoolVar(&once, "once", false, "Allow the next matching install once, then warn again")
	cmd.Flags().BoolVar(&always, "always", false, "Record a standing exception (requires --reason)")
	cmd.Flags().StringVar(&reason, "reason", "", "Recorded justification (required for --always)")
	return cmd
}

// runPostureAllowAlways appends a posture-scoped standing exception to the user
// config (load -> AddPostureAllow -> save) and writes an allow_always audit record.
func runPostureAllowAlways(cmd *cobra.Command, ecosystem, pkg, rule, reason string) error {
	cfgPath, err := platform.ConfigPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cfg.AddPostureAllow(config.PostureAllow{
		Ecosystem: ecosystem,
		Package:   pkg,
		Rule:      rule,
		Reason:    reason,
	})
	// Validate fail-closed before persisting so a bad rule/empty package never lands.
	if cfg.Posture != nil {
		if verr := config.ValidatePostureConfig(*cfg.Posture); verr != nil {
			return fmt.Errorf("posture allow --always: %w", verr)
		}
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if err := writePostureOverrideRecord(postureOverrideAllowAlways, ecosystem, pkg, rule, reason); err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"Recorded a standing posture exception for %s (%s). It silences the posture rule(s) but never a catalog malware block.\n",
		pkg, allowScopeLabel(rule))
	return nil
}

// runPostureAllowOnce records a one-shot token in the owner-only allow-once store
// and writes an allow_once audit record.
func runPostureAllowOnce(cmd *cobra.Command, ecosystem, pkg, rule, reason string) error {
	stateDir, err := postureStateDirFn()
	if err != nil {
		return fmt.Errorf("resolve state dir: %w", err)
	}
	if err := addPostureAllowOnceFn(stateDir, ecosystem, pkg, reason); err != nil {
		return fmt.Errorf("record allow-once token: %w", err)
	}
	if err := writePostureOverrideRecord(postureOverrideAllowOnce, ecosystem, pkg, rule, reason); err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(),
		"Recorded a one-shot posture allow for %s. The next matching install proceeds; the one after warns again.\n",
		pkg)
	return nil
}

// newPostureEnforceCmd implements `beekeeper posture enforce <rule> (--block |
// --warn)`: the IPOVR-03 opt-up CLI surface. It sets the per-rule action in the
// user config and writes an enforce_block / enforce_warn audit record.
func newPostureEnforceCmd() *cobra.Command {
	var (
		block bool
		warn  bool
	)
	cmd := &cobra.Command{
		Use:   "enforce <rule>",
		Short: "Set a posture rule's action: --block (opt up) or --warn (lower it back)",
		Long: `Set the enforced action for a posture rule (IPOVR-03).

  <rule>   One of: release-age | lifecycle | git-remote
  --block  Block a DEFINITE violation of this rule (opt the rule UP from warn).
  --warn   Lower the rule back to the default warn.

This writes to your user config (a trusted layer may raise OR lower its own
setting; only untrusted project/env layers are tighten-only). The unknown/
fail-soft path always warns regardless: a registry outage never blocks an install
even under --block. Each invocation writes a distinct posture_override record.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rule := strings.TrimSpace(args[0])
			if !isNamedPostureRule(rule) {
				return fmt.Errorf("posture enforce: invalid rule %q (want %q, %q, or %q)",
					rule, config.PostureRuleReleaseAge, config.PostureRuleLifecycle, config.PostureRuleRemoteSource)
			}
			if block == warn {
				return fmt.Errorf("posture enforce: exactly one of --block or --warn is required")
			}
			action := config.PostureActionWarn
			overrideAction := postureOverrideEnforceWarn
			if block {
				action = config.PostureActionBlock
				overrideAction = postureOverrideEnforceBlock
			}
			return runPostureEnforce(cmd, rule, action, overrideAction)
		},
	}
	cmd.Flags().BoolVar(&block, "block", false, "Block a definite violation of this rule")
	cmd.Flags().BoolVar(&warn, "warn", false, "Lower the rule back to the default warn")
	return cmd
}

// runPostureEnforce sets the named rule's action in the user config and writes the
// enforce override audit record.
func runPostureEnforce(cmd *cobra.Command, rule, action, overrideAction string) error {
	cfgPath, err := platform.ConfigPath()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Posture == nil {
		cfg.Posture = &config.PostureConfig{}
	}
	switch rule {
	case config.PostureRuleReleaseAge:
		cfg.Posture.ReleaseAge.Action = action
	case config.PostureRuleLifecycle:
		cfg.Posture.Lifecycle.Action = action
	case config.PostureRuleRemoteSource:
		cfg.Posture.RemoteSource.Action = action
	}
	if verr := config.ValidatePostureConfig(*cfg.Posture); verr != nil {
		return fmt.Errorf("posture enforce: %w", verr)
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	if err := writePostureOverrideRecord(overrideAction, "", "", rule, ""); err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Set posture rule %q to %s.\n", rule, action)
	return nil
}

// writePostureOverrideRecord constructs and writes a distinct posture_override
// audit record at the live audit path. The reason flows through Reason (redacted
// like other reason fields); the action discriminator and the package/rule scope
// are recorded in the dedicated posture_* fields (IPOVR-02).
func writePostureOverrideRecord(overrideAction, ecosystem, pkg, rule, reason string) error {
	auditPath, err := configAuditPath()
	if err != nil {
		return err
	}
	w, err := audit.NewWriter(auditPath)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer w.Close()

	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		copy(raw[:], []byte(fmt.Sprintf("%016x", time.Now().UnixNano())))
	}
	recordID := hex.EncodeToString(raw[:])

	rec := audit.AuditRecord{
		RecordType:            "posture_override",
		RecordID:              recordID,
		Timestamp:             time.Now().UTC().Format(time.RFC3339),
		ScannerName:           "beekeeper",
		Endpoint:              "posture",
		PostureOverrideAction: overrideAction,
		PostureRule:           rule,
		PostureEcosystem:      ecosystem,
		PosturePackage:        pkg,
		Reason:                reason,
	}

	// Redact the record (the operator-supplied reason / package may embed a token)
	// at the single chokepoint before writing to disk, consistent with the hook
	// and gateway audit paths.
	rec = audit.RedactRecordWithDefaults(rec)
	return w.Write(rec)
}

// validPostureRuleFlag reports whether r is an accepted --rule flag value for the
// allow command: empty (all rules) or one of the three named rules.
func validPostureRuleFlag(r string) bool {
	return r == "" || isNamedPostureRule(r)
}

// isNamedPostureRule reports whether r is exactly one of the three named posture
// rules (no empty / all-rules wildcard).
func isNamedPostureRule(r string) bool {
	switch r {
	case config.PostureRuleReleaseAge, config.PostureRuleLifecycle, config.PostureRuleRemoteSource:
		return true
	default:
		return false
	}
}

// allowScopeLabel renders the rule scope for the success message.
func allowScopeLabel(rule string) string {
	if rule == "" {
		return "all posture rules"
	}
	return "rule " + rule
}
