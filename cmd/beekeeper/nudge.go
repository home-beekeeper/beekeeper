// nudge.go — beekeeper nudge subcommands (NUDGE-07 / SC5, Phase 8 Plan 08).
//
// Provides three subcommands grouped under `beekeeper nudge`:
//
//	nudge status                     — human-readable current PM state + active config
//	nudge check "<command>"          — dry-run: parse + detect + evaluate, print decision
//	nudge audit [--since=<duration>] — query audit log filtered to record_type:"nudge"
//
// All business logic lives in internal/nudge, internal/pkgparse, internal/audit.
// This file is thin Cobra wiring per the project architecture constraint.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/config"
	"github.com/bantuson/beekeeper/internal/nudge"
	"github.com/bantuson/beekeeper/internal/pkgparse"
	"github.com/bantuson/beekeeper/internal/platform"
)

// newNudgeCmd groups the three nudge-as-code subcommands.
// Pattern: newPolicyCmd() grouped idiom (policy.go lines 27-45).
func newNudgeCmd() *cobra.Command {
	nudgeCmd := &cobra.Command{
		Use:   "nudge",
		Short: "Inspect and test the package-manager nudge feature",
		Long: `Work with the Beekeeper package-manager nudge feature.

The nudge feature steers npm install commands toward pnpm (>=11.0) or bun
(>=1.3) when either is available, because both ship structural supply-chain
defenses that npm does not (minimumReleaseAge, blockExoticSubdeps, lifecycle
script allowlists, Socket security scanner API).

Soft mode (default): advise the operator and proceed with the original command.
Hard mode (opt-in):  rewrite the command to the pnpm/bun equivalent.`,
	}
	nudgeCmd.AddCommand(
		newNudgeStatusCmd(),
		newNudgeCheckCmd(),
		newNudgeAuditCmd(),
	)
	return nudgeCmd
}

// newNudgeStatusCmd implements `beekeeper nudge status`.
// It prints a human-readable summary of the detected PM state and the active
// nudge configuration (PRD §13 — not NDJSON, operator-readable).
func newNudgeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current PM state and active nudge configuration",
		Long: `Detect locally installed package managers and print a human-readable
summary of the current PM state alongside the active Beekeeper nudge
configuration.

This is not NDJSON — it is plain-text operator output (PRD §13).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := resolveConfig(cmd)
			if err != nil {
				return fmt.Errorf("nudge status: load config: %w", err)
			}

			// Build nudge.Config from layered cfg.Nudge via the single mapper.
			// CLEAN-02: LoadLayered now always populates a non-nil, validated
			// cfg.Nudge (mergeNudge + the LoadLayered defaulting guard), so the
			// nil branch below is no longer load-bearing for the resolveConfig path.
			// It is retained as DEFENSE-IN-DEPTH: a direct zero-Config construction
			// (e.g. in a future test or caller that bypasses LoadLayered) still gets
			// defaults rather than a nil-pointer deref. Mirrors Load's defaulting.
			nc := cfg.Nudge
			if nc == nil {
				d := defaultNudgeConfigHelper()
				nc = &d
			}
			nudgeCfg := nudge.ConfigFrom(
				nc.Enabled,
				nc.Mode,
				nc.Preferred,
				nc.CheckSocketScanner,
				nc.VersionFloors.Pnpm,
				nc.VersionFloors.Bun,
				nc.VersionFloors.Node,
				nc.MajorDriftCheck.Enabled,
				nc.MajorDriftCheck.Interval,
			)

			// Detect current PM state.
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			state := nudge.DetectStateFn(ctx, nudgeCfg)

			out := cmd.OutOrStdout()

			// --- Detected PM state ---
			fmt.Fprintln(out, "=== Package Manager State ===")
			if state.PnpmInstalled {
				hardened := "no"
				if state.PnpmHardened {
					hardened = "yes"
				}
				fmt.Fprintf(out, "  pnpm:  %s (hardened: %s)\n", state.PnpmVersion, hardened)
			} else {
				fmt.Fprintln(out, "  pnpm:  not installed")
			}
			if state.BunInstalled {
				scanner := "no"
				if state.BunScannerOK {
					scanner = "yes"
				}
				fmt.Fprintf(out, "  bun:   %s (socket scanner: %s)\n", state.BunVersion, scanner)
			} else {
				fmt.Fprintln(out, "  bun:   not installed")
			}
			if state.NodeVersion != "" {
				fmt.Fprintf(out, "  node:  %s\n", state.NodeVersion)
			} else {
				fmt.Fprintln(out, "  node:  not detected")
			}

			// --- Active nudge configuration ---
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "=== Active Nudge Configuration ===")
			fmt.Fprintf(out, "  enabled:             %v\n", nudgeCfg.Enabled)
			fmt.Fprintf(out, "  mode:                %s\n", nudgeCfg.Mode)
			fmt.Fprintf(out, "  preferred:           %s\n", nudgeCfg.Preferred)
			fmt.Fprintf(out, "  require_hardened:    %v\n", nudgeCfg.RequireHardened)
			fmt.Fprintf(out, "  check_socket_scanner: %v\n", nudgeCfg.CheckSocketScanner)
			fmt.Fprintf(out, "  drift_check_enabled: %v\n", nudgeCfg.MajorDriftCheck.Enabled)
			fmt.Fprintf(out, "  drift_check_interval: %s\n", nudgeCfg.MajorDriftCheck.Interval)
			fmt.Fprintln(out, "  version_floors:")
			fmt.Fprintf(out, "    pnpm: %s\n", nudgeCfg.VersionFloors.Pnpm)
			fmt.Fprintf(out, "    bun:  %s\n", nudgeCfg.VersionFloors.Bun)
			fmt.Fprintf(out, "    node: %s\n", nudgeCfg.VersionFloors.Node)
			return nil
		},
	}
}

// newNudgeCheckCmd implements `beekeeper nudge check "<command>"`.
// It dry-runs pkgparse.Parse → nudge.DetectStateFn → nudge.Evaluate and prints
// the decision/reason/rewritten. The operator's command string is NEVER passed
// to a shell — it is only parsed (T-08-27).
func newNudgeCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <command>",
		Short: "Dry-run: show what Beekeeper would do with a given install command",
		Long: `Parse <command>, detect the local PM state, run nudge.Evaluate, and
print the resulting decision/reason/rewritten.

The command string is NEVER executed — it is parsed for the dry-run decision
only (T-08-27). Detection runs pnpm/bun/node --version with a 2s timeout.

Output mirrors 'beekeeper policy test':
  decision: <level>
  reason:   <reason_code>
  action:   <nudge_action>
  rewritten: <cmd or ->`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			command := args[0]

			cfg, err := resolveConfig(cmd)
			if err != nil {
				return fmt.Errorf("nudge check: load config: %w", err)
			}

			// CLEAN-02: LoadLayered now always populates a non-nil, validated
			// cfg.Nudge, so this nil branch is DEFENSE-IN-DEPTH (belt-and-suspenders
			// for a direct zero-Config construction that bypasses LoadLayered), not
			// the load-bearing defaulting it once was.
			nc := cfg.Nudge
			if nc == nil {
				d := defaultNudgeConfigHelper()
				nc = &d
			}
			nudgeCfg := nudge.ConfigFrom(
				nc.Enabled,
				nc.Mode,
				nc.Preferred,
				nc.CheckSocketScanner,
				nc.VersionFloors.Pnpm,
				nc.VersionFloors.Bun,
				nc.VersionFloors.Node,
				nc.MajorDriftCheck.Enabled,
				nc.MajorDriftCheck.Interval,
			)

			// Parse the command string — NEVER execute it (T-08-27).
			parsed, ok := pkgparse.Parse(command)
			if !ok {
				return fmt.Errorf("nudge check: %q is not a recognised install/exec command (nudge does not apply)", command)
			}

			// Detect current PM state (fail-open by design).
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			state := nudge.DetectStateFn(ctx, nudgeCfg)

			// Evaluate the nudge decision (pure).
			decision := nudge.Evaluate(parsed, state, nudgeCfg)

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "decision:  %s\n", decision.Level)
			fmt.Fprintf(out, "reason:    %s\n", decision.Reason)
			fmt.Fprintf(out, "action:    %s\n", nudge.ActionString(decision.Action))
			rewritten := decision.Rewritten
			if rewritten == "" {
				rewritten = "-"
			}
			fmt.Fprintf(out, "rewritten: %s\n", rewritten)
			return nil
		},
	}
}

// newNudgeAuditCmd implements `beekeeper nudge audit [--since=...]`.
// It queries the audit log filtered to record_type:"nudge" using audit.Query +
// audit.QueryOpts{Since}, mirroring the main.go audit query block (~line 912).
func newNudgeAuditCmd() *cobra.Command {
	var qSince string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Query nudge decisions from the audit log",
		Long: `Filter the Beekeeper audit log to nudge decision records
(record_type: "nudge") and stream matching NDJSON lines to stdout.

--since accepts a Go duration string (e.g. "1h", "24h", "7d") or an
RFC3339 timestamp. When omitted all nudge records in the log are returned.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("nudge audit: resolve audit directory: %w", err)
			}
			logPath := filepath.Join(auditDir, "beekeeper.ndjson")

			f, err := os.Open(logPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("audit log %q does not exist yet (run a beekeeper check first)", logPath)
				}
				return fmt.Errorf("nudge audit: open audit log: %w", err)
			}
			defer f.Close()

			opts := audit.QueryOpts{}
			if qSince != "" {
				if dur, derr := time.ParseDuration(qSince); derr == nil {
					opts.Since = time.Now().Add(-dur)
				} else if ts, terr := time.Parse(time.RFC3339, qSince); terr == nil {
					opts.Since = ts
				} else {
					return fmt.Errorf("--since %q: expected duration (e.g. 1h) or RFC3339 timestamp", qSince)
				}
			}

			// Wrap the output writer to filter to record_type:"nudge" lines.
			// audit.Query does not have a native RecordType filter, so we apply it
			// with a pre-filter reader that only passes through nudge records.
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			return queryNudgeRecords(ctx, f, opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&qSince, "since", "", "Only show records after this duration (e.g. 1h) or RFC3339 timestamp")
	return cmd
}

// defaultNudgeConfigHelper returns the default NudgeConfig for the defense-in-depth
// nil-guards above. As of CLEAN-02, config.LoadLayered always returns a non-nil,
// validated cfg.Nudge (mergeNudge + the LoadLayered defaulting/validation guard),
// so this helper is no longer reached on the normal resolveConfig path. It is kept
// as belt-and-suspenders for any caller that constructs a zero Config directly and
// bypasses LoadLayered. Mirrors config.Load's defaulting when the config file is absent.
func defaultNudgeConfigHelper() config.NudgeConfig {
	return config.DefaultNudgeConfig()
}

// queryNudgeRecords streams NDJSON lines from r, applies the time filter in opts,
// additionally filters to record_type:"nudge", and writes matching lines to out.
//
// audit.QueryOpts has no RecordType filter field, so this function implements
// the nudge-specific record_type filter directly (mirrors audit.Query structure).
// Malformed lines are silently skipped. Context cancellation is respected.
func queryNudgeRecords(ctx context.Context, r io.Reader, opts audit.QueryOpts, out io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	skipped := 0
	written := 0
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		if lineNum%100 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		rawLine := scanner.Bytes()

		// Decode the record to apply filters.
		var rec audit.AuditRecord
		if err := json.Unmarshal(rawLine, &rec); err != nil {
			skipped++
			continue
		}

		// Filter to nudge records only.
		if rec.RecordType != "nudge" {
			continue
		}

		// Apply time filter.
		if !opts.Since.IsZero() {
			ts, err := time.Parse(time.RFC3339, rec.Timestamp)
			if err != nil || ts.Before(opts.Since) {
				continue
			}
		}

		// Write the raw line verbatim (no re-marshal loss).
		if _, err := out.Write(rawLine); err != nil {
			return fmt.Errorf("write nudge audit record: %w", err)
		}
		if _, err := out.Write([]byte{'\n'}); err != nil {
			return fmt.Errorf("write nudge audit record newline: %w", err)
		}
		written++

		if opts.Limit > 0 && written >= opts.Limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan audit log: %w", err)
	}

	if skipped > 0 {
		fmt.Fprintf(out, "# %d malformed line(s) skipped\n", skipped)
	}

	return nil
}
