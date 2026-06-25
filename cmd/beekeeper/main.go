// Command beekeeper is the real-time safety harness for autonomous coding
// agents. This file contains ONLY Cobra command wiring per the project
// architecture constraint — all business logic lives in internal/ packages.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/home-beekeeper/beekeeper/internal/audit"
	"github.com/home-beekeeper/beekeeper/internal/catalog"
	"github.com/home-beekeeper/beekeeper/internal/check"
	"github.com/home-beekeeper/beekeeper/internal/config"
	"github.com/home-beekeeper/beekeeper/internal/editorinit"
	"github.com/home-beekeeper/beekeeper/internal/gateway"
	"github.com/home-beekeeper/beekeeper/internal/hooks"
	"github.com/home-beekeeper/beekeeper/internal/llamafirewall"
	"github.com/home-beekeeper/beekeeper/internal/notify"
	"github.com/home-beekeeper/beekeeper/internal/platform"
	"github.com/home-beekeeper/beekeeper/internal/policy"
	"github.com/home-beekeeper/beekeeper/internal/quarantine"
	"github.com/home-beekeeper/beekeeper/internal/scan"
	"github.com/home-beekeeper/beekeeper/internal/shim"
	tui "github.com/home-beekeeper/beekeeper/internal/tui"
	"github.com/home-beekeeper/beekeeper/internal/version"
	"github.com/home-beekeeper/beekeeper/internal/watch"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// hookFailClosedExit renders a fail-closed deny for the given harness and exits
// the process — it never returns. SEC (remediation 260615): in --hook mode a
// pre-decision failure (self-quarantine, unresolvable state dir, unreadable
// config) MUST reach the harness as a BLOCK via exit 2 + the harness-specific
// deny form, NOT as the bare exit 1 that several harnesses treat as a hook error
// and IGNORE (silently allowing the tool — the exact class the exit-1→exit-2
// work fixed for the decision path). In default (non-hook) mode it exits 1, the
// established block signal for the shim/gateway/test callers.
func hookFailClosedExit(hook, reason string) {
	if hook == "" {
		fmt.Fprintln(os.Stderr, "beekeeper: "+reason)
		os.Exit(1)
	}
	d := policy.Decision{
		Allow:   false,
		Level:   "block",
		Reason:  reason,
		RuleIDs: []string{"self-protect"},
	}
	out := check.RenderDeny(check.HarnessID(hook), d)
	if len(out.Stdout) > 0 {
		fmt.Fprint(os.Stdout, string(out.Stdout))
	}
	if len(out.Stderr) > 0 {
		fmt.Fprint(os.Stderr, string(out.Stderr))
	}
	os.Exit(out.ExitCode)
}

// scanOnDeltaFn is the function called by the catalogs watch onDelta callback
// to trigger an extension rescan after a catalog delta. It is a package-level
// var so tests can replace it with a mock without needing a live scan binary.
// Production code leaves this as the default (scan.Scan).
var scanOnDeltaFn = func(ctx context.Context, cfg scan.Config, out io.Writer) error {
	return scan.Scan(ctx, cfg, out)
}

// runFirstResponderFn is the function called by the catalogs watch onDelta
// callback to cross-reference installed packages against the updated catalog
// and optionally auto-quarantine high-corroboration hits. It is a package-level
// var so tests can replace it without spawning a real pollen process.
// Production code leaves this as the default (watch.RunFirstResponder). The
// watch onDelta path does not use the returned counts (it is not the sync-summary
// surface), so the seam discards the FirstResponderResult and keeps the
// error-only contract its caller expects.
var runFirstResponderFn = func(ctx context.Context, cfg watch.FirstResponderConfig) error {
	_, err := watch.RunFirstResponder(ctx, cfg)
	return err
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "beekeeper",
		Short:        "Real-time safety harness for autonomous coding agents",
		Long:         "Beekeeper intercepts agent tool calls before they execute and evaluates them against unified threat intelligence.",
		SilenceUsage: true,
		// Bare `beekeeper` (no subcommand) greets with a branded banner, then the
		// usual help. This is the first Beekeeper-authored output a user sees after
		// install: `go install` and the install scripts only get the binary onto
		// the machine; the Go toolchain owns the download output, so the welcome
		// lives here, on first run. Every subcommand is unaffected.
		Run: func(cmd *cobra.Command, _ []string) {
			printWelcome(cmd.OutOrStdout())
			_ = cmd.Help()
		},
	}

	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newCheckCmd(),
		newCatalogsCmd(),
		newAuditCmd(),
		newSelftestCmd(),
		newWatchCmd(),
		newScanCmd(),
		newQuarantineCmd(),
		newHooksCmd(),
		newGatewayCmd(),
		newShimCmd(),
		// Phase 28: read-only install-posture view (IPVIEW-01/02, IPBND-01).
		newPostureCmd(),
		newAuditRecordCmd(),
		newProtectCmd(),
		newSentryCmd(),
		newLlamaFirewallCmd(),
		newDashboardCmd(),
		// Phase 9: policy-as-code (CODE-02/03/04) and diagnostics (CODE-06).
		newPolicyCmd(),
		newDiagCmd(),
		// config set with audit logging.
		newConfigCmd(),
	)

	return root
}

// printWelcome writes the branded first-run banner: a small honeycomb cell, the
// build version, and the one-line purpose. Plain ASCII (no ANSI) so it renders in
// any terminal and stays stable under output capture in tests.
func printWelcome(w io.Writer) {
	fmt.Fprintf(w,
		"\n   __\n  /  \\   BEEKEEPER  %s\n  \\__/   Real-time safety harness for autonomous coding agents\n\n",
		version.Version,
	)
}

// newVersionCmd prints the build metadata injected via ldflags. Fully
// implemented in Phase 1, Plan 01.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "version: %s\n", version.Version)
			fmt.Fprintf(out, "commit:  %s\n", version.Commit)
			fmt.Fprintf(out, "date:    %s\n", version.Date)
			return nil
		},
	}
}

// newInitCmd creates the Beekeeper state directory tree and, on first run,
// detects installed editors and gates configuration changes on explicit consent.
// EDXT-06: extends Phase 1 dir creation with quarantine/marketplace-cache dirs
// plus per-editor consent for auto-update disable and watch-dir registration.
func newInitCmd() *cobra.Command {
	var yes, noEditors bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the Beekeeper state directory and configure editor protection",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			// Phase 1: create state, catalogs, audit directories.
			// Phase 9 (CODE-04): also create policies/ so `policy list` works on a fresh install.
			for _, dir := range []string{stateDir, catalogDir, auditDir, filepath.Join(stateDir, "policies")} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					return fmt.Errorf("create directory %q: %w", dir, err)
				}
			}

			// Phase 3: create quarantine/extensions and marketplace-cache.
			for _, dir := range []string{
				filepath.Join(stateDir, "quarantine", "extensions"),
				filepath.Join(catalogDir, "marketplace-cache"),
			} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					return fmt.Errorf("create directory %q: %w", dir, err)
				}
			}

			// Phase 4: create shims directory (INTG-06).
			shimsDir := filepath.Join(stateDir, "shims")
			if err := os.MkdirAll(shimsDir, 0700); err != nil {
				return fmt.Errorf("create directory %q: %w", shimsDir, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Created shim directory: %s\n", shimsDir)

			fmt.Fprintf(cmd.OutOrStdout(), "Initialized Beekeeper state directory at %s\n", stateDir)

			if noEditors {
				return nil
			}

			// EDXT-06: detect editors and gate configuration on consent.
			editors, err := editorinit.DetectEditors()
			if err != nil || len(editors) == 0 {
				return nil
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			out := cmd.OutOrStdout()
			in := cmd.InOrStdin()
			configDirty := false

			for _, editor := range editors {
				if editor.SettingsPath != "" {
					var consent bool
					if yes {
						consent = true
					} else {
						fmt.Fprintf(out, "Disable extension auto-update for %s? NOTE: comments in settings.json will be removed. [y/N] ", editor.Name)
						var resp string
						fmt.Fscanln(in, &resp)
						consent = resp == "y" || resp == "Y"
					}
					if consent {
						if err := editorinit.DisableExtensionAutoUpdate(editor.SettingsPath); err != nil {
							fmt.Fprintf(out, "  Warning: could not disable auto-update for %s: %v\n", editor.Name, err)
						} else {
							fmt.Fprintf(out, "  Disabled extension auto-update for %s\n", editor.Name)
						}
					}
				}

				if editor.ExtensionDir != "" {
					var consent bool
					if yes {
						consent = true
					} else {
						fmt.Fprintf(out, "Enable Beekeeper file-watcher for %s (%s)? [y/N] ", editor.Name, editor.ExtensionDir)
						var resp string
						fmt.Fscanln(in, &resp)
						consent = resp == "y" || resp == "Y"
					}
					if consent {
						cfg.AddWatchDirectory(editor.ExtensionDir)
						configDirty = true
						fmt.Fprintf(out, "  Registered watch directory: %s\n", editor.ExtensionDir)
					}
				}
			}

			if configDirty {
				if err := config.Save(configPath, cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Auto-consent to all editor configuration prompts")
	cmd.Flags().BoolVar(&noEditors, "no-editors", false, "Skip editor detection (scripted installs)")
	return cmd
}

// newCheckCmd is the hook handler entry point. It reads a tool call from stdin,
// evaluates it against the mmap catalog index under hard caps, writes an audit
// record, and exits 0 (allow) or non-zero (block) — failing CLOSED on any
// crash, timeout, oversized input, malformed JSON, or missing index.
//
// When --tool is provided (shim invocation path), the tool call JSON is
// constructed from the flag values using json.Marshal (injection-safe) and
// passed as stdin to RunCheck. This avoids shell-level JSON construction and
// the injection risks of embedding $* in heredoc strings (CR-03, CR-04).
func newCheckCmd() *cobra.Command {
	var toolName string
	var toolArgs []string
	var hookTarget string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Evaluate a tool call read from stdin (allow=0, block!=0)",
		// ArbitraryArgs allows positional arguments so that shim invocations like
		//   beekeeper check --tool npm --args install left-pad react
		// work correctly. The shim emits: --args <first-arg> [extra positional args…]
		// and we append the extra positional args to toolArgs below (TM-A-04).
		//
		// Without --tool the check command reads a tool call from stdin and
		// positional args are disallowed in practice (they are unused). Allowing
		// them does not widen the attack surface because the positional args are
		// only used when --tool is set (shim path), and only appended to toolArgs
		// which feeds json.Marshal (injection-safe).
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// SEC (remediation 260615): in --hook mode, any pre-decision failure
			// must reach the harness as a fail-closed BLOCK (exit 2 + the deny
			// form), not the bare exit-1 that several harnesses treat as a hook
			// error and IGNORE (silently allowing the tool). failClosed routes such
			// errors through hookFailClosedExit when --hook is set; in default mode
			// it returns the error for the normal exit-1 path.
			failClosed := func(err error) error {
				if hookTarget != "" {
					hookFailClosedExit(hookTarget, err.Error())
				}
				return err
			}

			// Phase 9 (CTLG-04/SFDF-06): self-quarantine guard — runs before any
			// enforcement logic. Refuses to continue if the running binary version
			// appears in the beekeeper-self compromised-version list. A
			// self-quarantine (or integrity) failure under --hook must BLOCK the
			// tool call (exit 2), never exit 1 — this is the highest-severity case.
			if err := enforceSelfQuarantine(cmd); err != nil {
				return failClosed(err)
			}

			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return failClosed(fmt.Errorf("resolve catalog directory: %w", err))
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return failClosed(fmt.Errorf("resolve audit directory: %w", err))
			}

			indexPath := filepath.Join(catalogDir, "bumblebee.idx")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			// CODE-05 SC2: use layered resolver so system→user→project→BEEKEEPER_*
			// all apply to enforcement decisions. resolveConfig is defined in
			// config_resolve.go (shared by check/gateway/watch/scan).
			cfg, err := resolveConfig(cmd)
			if err != nil {
				return failClosed(fmt.Errorf("load config: %w", err))
			}

			// Shim invocation: build ToolCall JSON from flags using json.Marshal.
			// This is injection-safe — no shell string embedding of user arguments.
			//
			// TM-A-04: shim scripts emit --args for the first positional arg, then
			// the remaining package-manager args fall through as cobra positional
			// args (because the shell expansion of "$@" / %* is not individually
			// wrapped in --args flags). Append those positional args here so that
			//   beekeeper check --tool npm --args install left-pad react
			// produces "args": ["install","left-pad","react"] — not ["install"]
			// with "left-pad react" silently dropped.
			//
			// Only apply when --tool is set (shim path). On the stdin path (no
			// --tool), extra positional args are an error in the user's invocation
			// and are ignored silently (they do not feed any policy decision).
			var stdin io.Reader = os.Stdin
			if toolName != "" {
				// Merge flag-supplied toolArgs with any remaining positional args.
				// Order: flag args first (preserves the original shim arg order),
				// then positional args (which are the overflow from the shell's
				// "$@" / %* expansion).
				allArgs := make([]string, 0, len(toolArgs)+len(args))
				allArgs = append(allArgs, toolArgs...)
				allArgs = append(allArgs, args...)
				tc := buildShimToolCall(toolName, allArgs)
				data, merr := json.Marshal(tc)
				if merr != nil {
					return failClosed(fmt.Errorf("marshal shim tool call: %w", merr))
				}
				stdin = strings.NewReader(string(data))
			}

			// --hook mode: suppress the raw Decision JSON so the harness sees ONLY
			// its own deny form on stdout. Hermes is fail-open on exit codes and
			// parses the FIRST JSON object on stdout as the decision — a leading
			// {"Allow":false,...} would cause Hermes to silently allow the block.
			// io.Discard suppresses the raw Decision JSON; the harness-specific deny
			// form is written below by RenderDeny.
			//
			// Default path (no --hook): raw Decision JSON to stdout via RunCheck,
			// exit 0 (allow) or exit 1 (block). Shim/gateway/tests rely on exit 1.
			var result check.Result
			if hookTarget != "" {
				result = check.RunCheckTo(cmd.Context(), stdin, cfg, indexPath, auditPath, catalogDir, io.Discard)
			} else {
				result = check.RunCheck(cmd.Context(), stdin, cfg, indexPath, auditPath, catalogDir)
			}

			// --hook adapter: emits the per-harness deny signal ONLY on block.
			// On allow/warn, fall through to the default exit path (exit 0) —
			// never emit permissionDecision:"allow" (CONTEXT decision 3, T-10-02).
			if hookTarget != "" && !result.Decision.Allow {
				out := check.RenderDeny(check.HarnessID(hookTarget), result.Decision)
				if len(out.Stdout) > 0 {
					fmt.Fprint(os.Stdout, string(out.Stdout))
				}
				if len(out.Stderr) > 0 {
					fmt.Fprint(os.Stderr, string(out.Stderr))
				}
				os.Exit(out.ExitCode)
				return nil // unreachable
			}

			os.Exit(result.ExitCode)
			return nil // unreachable; os.Exit above is the real return path
		},
	}
	cmd.Flags().StringVar(&toolName, "tool", "", "Tool name for shim invocations (builds ToolCall JSON from flags)")
	cmd.Flags().StringArrayVar(&toolArgs, "args", nil, "Arguments for shim invocations (used with --tool)")
	cmd.Flags().StringVar(&hookTarget, "hook", "", "Harness name for hook invocations (emits exit 2 + harness-specific deny JSON on block; default mode unchanged)")
	return cmd
}

// buildShimToolCall constructs the tool call JSON for a shim invocation
// (beekeeper check --tool <tool> --args <args...>). It reconstructs the full
// shell command and shapes it as a Bash tool call, because the shim intercepts a
// shell invocation of a package manager and Bash is the shape the catalog engine
// (policy extract) and the install-posture adapter (evaluatePosture) parse. This
// is what makes the shim actually enforce: without the reconstruction, command is
// the bare tool name ("npm") and no install is identified, so beekeeper check
// would allow it. Injection-safe: the command is a JSON string value built by
// json.Marshal and parsed by the pure pkgparse, never executed by a shell.
func buildShimToolCall(tool string, args []string) map[string]any {
	command := tool
	if len(args) > 0 {
		command = tool + " " + strings.Join(args, " ")
	}
	return map[string]any{
		"tool_name":  "Bash",
		"agent_name": "shim",
		"tool_input": map[string]any{
			"command": command,
		},
	}
}

// newCatalogsCmd groups catalog-management subcommands.
func newCatalogsCmd() *cobra.Command {
	catalogs := &cobra.Command{
		Use:   "catalogs",
		Short: "Manage cached threat-intel catalogs",
	}
	var syncForce, syncBackground bool
	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch and cache catalogs, then build the mmap index (interval-gated; --force to override)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// --background is the OS-scheduler entry point: hide the console
			// (Windows; no-op elsewhere) so the hourly heartbeat does not flash a
			// blank window, and tee output to <state>/logs/sync.log so a scheduled
			// run is observable. Both are best-effort — a failure to hide or open
			// the log never blocks the sync (fail-open on VISIBILITY only; the
			// sync's own fail-closed semantics are unchanged).
			if syncBackground {
				HideConsoleWindow()
				if lf, err := openSyncLog(); err == nil {
					defer lf.Close()
					cmd.SetOut(teeWriter(cmd.OutOrStdout(), lf))
					cmd.SetErr(teeWriter(cmd.ErrOrStderr(), lf))
				}
			}
			// Interval gate + ETag-conditional sync + freshness tracking live in
			// runCatalogsSync so the OS-scheduled daemon and manual invocation
			// share one implementation (Phase 20, CSYNC).
			return runCatalogsSync(cmd, syncForce)
		},
	}
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Sync now even if the configured interval has not elapsed (manual/TUI use)")
	syncCmd.Flags().BoolVar(&syncBackground, "background", false, "Scheduled-daemon mode: hide the console (Windows) and log to <state>/logs/sync.log")
	catalogs.AddCommand(syncCmd)

	// catalogs daemon — unprivileged user-level background sync scheduler (CSYNC-04/05).
	catalogs.AddCommand(newCatalogsDaemonCmd())

	// catalogs status — report the last sync result, next-due time, daemon
	// registration, and the background log path. Read-only: it reports, it does
	// not itself fetch or block.
	catalogs.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show the last catalog-sync result, next-due time, and daemon registration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			st, err := catalog.LoadState(filepath.Join(stateDir, "state.json"))
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}
			cfg, err := resolveConfig(cmd)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.CatalogSyncEnabled() {
				fmt.Fprintf(out, "Catalog sync: enabled (interval %s)\n", cfg.CatalogSyncInterval())
			} else {
				fmt.Fprintln(out, "Catalog sync: disabled (catalog_sync.enabled=false)")
			}

			if s := st.LastSync; s != nil {
				fmt.Fprintf(out, "Last run:     %s (%s)\n", s.At.Local().Format(time.RFC3339), s.Result)
				fmt.Fprintf(out, "  entries:    %d\n", s.Entries)
				fmt.Fprintf(out, "  scan hits:  %d (quarantined %d, pending %d, would-quarantine %d)\n",
					s.ScanHits, s.Quarantined, s.Pending, s.WouldQuarantine)
				if s.LastError != "" {
					fmt.Fprintf(out, "  error:      %s\n", s.LastError)
				}
				if !s.NextDue.IsZero() {
					fmt.Fprintf(out, "  next due:   %s\n", s.NextDue.Local().Format(time.RFC3339))
				}
			} else {
				fmt.Fprintln(out, "Last run:     never synced")
			}

			installed, detail, _ := catalogDaemonStatus()
			if installed {
				fmt.Fprintf(out, "Daemon:       installed — %s\n", detail)
			} else {
				fmt.Fprintf(out, "Daemon:       not installed (%s)\n", detail)
			}

			if lp, lerr := syncLogPath(); lerr == nil {
				fmt.Fprintf(out, "Log:          %s\n", lp)
			}
			return nil
		},
	})

	// catalogs watch — foreground catalog watch daemon.
	catalogs.AddCommand(&cobra.Command{
		Use:   "watch",
		Short: "Poll catalog sources and trigger re-scans on delta (Ctrl+C to stop)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			stateFile := filepath.Join(stateDir, "state.json")

			// CODE-05 SC2: use layered resolver for watch commands.
			beekeeperCfg, cfgErr := resolveConfig(cmd)
			if cfgErr != nil {
				return fmt.Errorf("load config: %w", cfgErr)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			client := &http.Client{Timeout: 30 * time.Second}
			cfg := catalog.WatchConfig{
				PollInterval: time.Hour,
				CatalogDir:   catalogDir,
				StateFile:    stateFile,
				Client:       client,
				Sanity:       catalog.DefaultSanityConfig(),
			}

			// Resolve extension dirs for post-delta scan (same logic as newScanCmd).
			var extDirs []string
			if configDirs := beekeeperCfg.WatchDirectories(); len(configDirs) > 0 {
				extDirs = configDirs
			} else {
				editors, _ := editorinit.DetectEditors()
				for _, e := range editors {
					if e.ExtensionDir != "" {
						extDirs = append(extDirs, e.ExtensionDir)
					}
				}
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Starting catalog watch daemon (interval: %s, Ctrl+C to stop)...\n", cfg.PollInterval)

			return catalog.Watch(ctx, cfg, func(delta catalog.CatalogDelta, sanity catalog.SanityResult) {
				if sanity.Block {
					fmt.Fprintf(out, "catalog delta [HARD-BLOCK sanity breach]: source=%s prev=%d new=%d delta=%d — %s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount, sanity.Reason)
					// Hard-block sanity breach: do NOT trigger re-scan on potentially
					// poisoned catalog data. Log and await operator intervention.
					return
				} else if sanity.Alert {
					fmt.Fprintf(out, "catalog delta [ALERT sanity breach]: source=%s prev=%d new=%d delta=%d — %s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount, sanity.Reason)
				} else if delta.HasChanges() {
					fmt.Fprintf(out, "catalog delta: source=%s prev_count=%d new_count=%d delta=%d prev_hash=%s new_hash=%s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount,
						delta.PrevHash, delta.NewHash)
				}

				// CTLG-06: on a real delta (non-block), refresh the catalog via Sync
				// then trigger an immediate scan of installed extensions so newly-
				// published threat_intel entries are acted on without waiting for the
				// next user-triggered scan.
				if delta.HasChanges() || sanity.Alert {
					fmt.Fprintf(out, "catalog delta: triggering catalog sync + extension rescan...\n")

					// Re-sync the catalog so the mmap index is up-to-date.
					if _, syncErr := catalog.Sync(ctx, client, catalogDir); syncErr != nil {
						fmt.Fprintf(os.Stderr, "beekeeper watch: catalog sync on delta failed: %v\n", syncErr)
						// Non-fatal: continue to scan with existing index.
					}

					// Run a fresh extension scan against the updated index.
					scanCfg := scan.Config{
						ExtensionDirs: extDirs,
						IndexPath:     filepath.Join(catalogDir, "bumblebee.idx"),
						CacheDir:      catalogDir,
						AuditPath:     filepath.Join(auditDir, "beekeeper.ndjson"),
						SocketToken:   beekeeperCfg.SocketAPIToken(),
						HTTPClient:    &http.Client{Timeout: 4 * time.Second},
						Now:           func() time.Time { return time.Now().UTC() },
					}
					if scanErr := scanOnDeltaFn(ctx, scanCfg, out); scanErr != nil {
						fmt.Fprintf(os.Stderr, "beekeeper watch: extension scan on delta failed: %v\n", scanErr)
						// Non-fatal: daemon keeps running; next delta will retry.
					}

					// First-responder: cross-reference installed packages against the
					// updated catalog and auto-quarantine high-corroboration hits when
					// configured (auto_quarantine.enabled=true in config). Runs AFTER
					// the extension scan so the audit log records are ordered correctly.
					// Fail-closed semantics are inside RunFirstResponder (any move error
					// leaves the artifact in place and logs a quarantine_error record).
					frCfg := watch.FirstResponderConfig{
						Enabled:           beekeeperCfg.AutoQuarantineEnabled(),
						DryRun:            beekeeperCfg.AutoQuarantineDryRun(),
						Threshold:         beekeeperCfg.AutoQuarantineThreshold(),
						QuarantineDir:     filepath.Join(stateDir, "quarantine"),
						AuditPath:         filepath.Join(auditDir, "beekeeper.ndjson"),
						IndexPath:         filepath.Join(catalogDir, "bumblebee.idx"),
						CacheDir:          catalogDir,
						SocketToken:       beekeeperCfg.SocketAPIToken(),
						SentryTargetsPath: filepath.Join(stateDir, "sentry-targets.json"),
					}
					if frErr := runFirstResponderFn(ctx, frCfg); frErr != nil {
						fmt.Fprintf(os.Stderr, "beekeeper watch: first-responder on delta failed: %v\n", frErr)
						// Non-fatal: daemon keeps running; next delta will retry.
					}
				}
			})
		},
	})

	// catalogs verify — clear degraded mode for a named catalog source (CTLG-08).
	var verifySource string
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Clear degraded mode for a catalog source after operator review",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if verifySource == "" {
				return fmt.Errorf("--source is required")
			}

			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			stateFile := filepath.Join(stateDir, "state.json")

			st, err := catalog.LoadState(stateFile)
			if err != nil {
				return fmt.Errorf("load state %q: %w", stateFile, err)
			}

			ss, ok := st.Sources[verifySource]
			if !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "Source %q not found in state; nothing to clear.\n", verifySource)
				return nil
			}

			if !ss.Degraded {
				fmt.Fprintf(cmd.OutOrStdout(), "Source %q is not degraded; nothing to clear.\n", verifySource)
				return nil
			}

			ss.Degraded = false
			ss.DegradedReason = ""
			st.Sources[verifySource] = ss

			if err := catalog.SaveState(stateFile, st); err != nil {
				return fmt.Errorf("save state %q: %w", stateFile, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cleared degraded mode for source %q.\n", verifySource)
			return nil
		},
	}
	verifyCmd.Flags().StringVar(&verifySource, "source", "", "catalog source name to clear (e.g. bumblebee)")
	catalogs.AddCommand(verifyCmd)

	// catalogs diff — show per-source delta between cached state and current on-disk snapshot (PRD §10).
	catalogs.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Show per-source delta between last-synced state and current on-disk catalog snapshot",
		Long: `Compare the cached (last-synced) catalog state against the current on-disk snapshot.

For each source, prints:
  - entry count delta (added/removed)
  - whether the content hash changed
  - degraded status

This command is read-only — no catalog mutation, no enforcement side effects.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			stateFile := filepath.Join(stateDir, "state.json")

			results, err := catalog.Diff(cmd.Context(), stateFile, catalogDir, &http.Client{Timeout: 10 * time.Second})
			if err != nil {
				return fmt.Errorf("catalog diff: %w", err)
			}

			out := cmd.OutOrStdout()
			if len(results) == 0 {
				fmt.Fprintln(out, "No catalog state found — run 'beekeeper catalogs sync' first.")
				return nil
			}

			anyChange := false
			for _, r := range results {
				if r.HasChanges() {
					anyChange = true
				}
			}
			if !anyChange {
				fmt.Fprintln(out, "Catalogs are up-to-date (no delta detected).")
			}

			for _, r := range results {
				status := "up-to-date"
				if r.HasChanges() {
					status = "CHANGED"
				}
				fmt.Fprintf(out, "%-16s  %s  prev=%d current=%d  added=+%d removed=-%d  degraded=%v\n",
					r.Source, status, r.PrevCount, r.CurrentCount, r.Added, r.Removed, r.Degraded)
				if r.Changed {
					fmt.Fprintf(out, "  hash changed: %s → %s\n", truncHash(r.PrevHash), truncHash(r.CurrentHash))
				}
			}
			return nil
		},
	})

	return catalogs
}

// truncHash returns the first 12 chars of a hash string for display (or the full
// string if shorter). Empty strings become "(none)".
func truncHash(h string) string {
	if h == "" {
		return "(none)"
	}
	if len(h) > 12 {
		return h[:12] + "..."
	}
	return h
}

// newWatchCmd starts the foreground extension file-watcher daemon.
// It detects watch directories from config or DetectEditors, then runs
// watch.Watch until SIGINT/SIGTERM (Ctrl+C).
func newWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch extension directories for new installations (Ctrl+C to stop)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Phase 9 (CTLG-04/SFDF-06): self-quarantine guard.
			if err := enforceSelfQuarantine(cmd); err != nil {
				return err
			}

			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}

			// CODE-05 SC2: layered resolver so project config overrides user config.
			cfg, err := resolveConfig(cmd)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			indexPath := filepath.Join(catalogDir, "bumblebee.idx")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")
			quarantineDir := filepath.Join(stateDir, "quarantine")

			// Resolve watch directories: config > DetectEditors.
			var dirs []string
			if configDirs := cfg.WatchDirectories(); len(configDirs) > 0 {
				dirs = configDirs
			} else {
				editors, _ := editorinit.DetectEditors()
				for _, e := range editors {
					if e.ExtensionDir != "" {
						dirs = append(dirs, e.ExtensionDir)
					}
				}
			}

			handler := watch.NewHandler(
				indexPath,
				catalogDir,
				quarantineDir,
				auditPath,
				notify.Config{Enabled: true},
				cfg.SocketAPIToken(),
				&http.Client{Timeout: 4 * time.Second},
				func() time.Time { return time.Now().UTC() },
				dirs,
			)

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Starting extension watch (Ctrl+C to stop)...\n")
			if len(dirs) == 0 {
				fmt.Fprintf(out, "Warning: no watch directories configured. Run 'beekeeper init' to detect editors.\n")
			}
			for _, d := range dirs {
				fmt.Fprintf(out, "Watching: %s\n", d)
			}

			return watch.Watch(ctx, dirs, watch.WatchConfig{}, handler)
		},
	}
}

// newScanCmd scans installed extensions using the Bumblebee CLI (when present)
// and the Beekeeper-own per-extension catalog/release-age engine.
func newScanCmd() *cobra.Command {
	var deep bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan installed extensions against catalog and release-age policy",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}

			// CODE-05 SC2: layered resolver so project config overrides user config.
			cfg, err := resolveConfig(cmd)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			var dirs []string
			if configDirs := cfg.WatchDirectories(); len(configDirs) > 0 {
				dirs = configDirs
			} else {
				editors, _ := editorinit.DetectEditors()
				for _, e := range editors {
					if e.ExtensionDir != "" {
						dirs = append(dirs, e.ExtensionDir)
					}
				}
			}

			scanCfg := scan.Config{
				Deep:          deep,
				ExtensionDirs: dirs,
				IndexPath:     filepath.Join(catalogDir, "bumblebee.idx"),
				CacheDir:      catalogDir,
				AuditPath:     filepath.Join(auditDir, "beekeeper.ndjson"),
				SocketToken:   cfg.SocketAPIToken(),
				HTTPClient:    &http.Client{Timeout: 4 * time.Second},
				Now:           func() time.Time { return time.Now().UTC() },
			}

			return scan.Scan(cmd.Context(), scanCfg, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&deep, "deep", false, "Run a deep scan (passes --profile deep --root <home> to Pollen)")
	return cmd
}

// newQuarantineCmd groups quarantine management subcommands.
func newQuarantineCmd() *cobra.Command {
	qCmd := &cobra.Command{
		Use:   "quarantine",
		Short: "Manage quarantined extensions",
	}

	// list
	qCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List quarantined extensions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			qDir := filepath.Join(stateDir, "quarantine")
			manifests, err := quarantine.List(qDir)
			if err != nil {
				return fmt.Errorf("list quarantined extensions: %w", err)
			}
			out := cmd.OutOrStdout()
			if len(manifests) == 0 {
				fmt.Fprintln(out, "no quarantined items")
				return nil
			}
			fmt.Fprintf(out, "%-40s %-24s %-12s %-22s %s\n", "ID", "publisher.name", "version", "quarantined_at", "reason")
			for _, m := range manifests {
				fmt.Fprintf(out, "%-40s %-24s %-12s %-22s %s\n",
					m.ID,
					m.Publisher+"."+m.Name,
					m.Version,
					m.QuarantinedAt.Format(time.RFC3339),
					m.Reason,
				)
			}
			return nil
		},
	})

	// restore
	qCmd.AddCommand(&cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a quarantined extension to its original location",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			qDir := filepath.Join(stateDir, "quarantine")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			if err := quarantine.Restore(qDir, args[0]); err != nil {
				return fmt.Errorf("restore %q: %w", args[0], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored %q\n", args[0])

			// Emit audit record for restore.
			rec := audit.AuditRecord{
				RecordType:       "quarantine_restore",
				RecordID:         args[0],
				Timestamp:        time.Now().UTC().Format(time.RFC3339),
				ScannerName:      "beekeeper",
				Decision:         "allow",
				Reason:           "operator restore",
				RuleIDs:          []string{"EDXT-05"},
				CatalogMatches:   []audit.CatalogProvenance{},
				SourcesAgreed:    []string{},
				SourcesDissented: []string{},
				Endpoint:         "quarantine",
			}
			if w, werr := audit.NewWriter(auditPath); werr == nil {
				_ = w.Write(rec)
				w.Close()
			}
			return nil
		},
	})

	// purge
	var purgeYes bool
	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Remove ALL quarantined extensions (prompts for confirmation unless --yes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			qDir := filepath.Join(stateDir, "quarantine")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			if !purgeYes {
				fmt.Fprint(cmd.OutOrStdout(), "Remove ALL quarantined items? [y/N] ")
				var resp string
				fmt.Fscanln(cmd.InOrStdin(), &resp)
				if resp != "y" && resp != "Y" {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}

			purged, err := quarantine.Purge(qDir)
			if err != nil {
				return fmt.Errorf("purge quarantine: %w", err)
			}

			if len(purged) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No items to purge.")
				return nil
			}

			if w, werr := audit.NewWriter(auditPath); werr == nil {
				for _, id := range purged {
					rec := audit.AuditRecord{
						RecordType:       "quarantine_purge",
						RecordID:         id,
						Timestamp:        time.Now().UTC().Format(time.RFC3339),
						ScannerName:      "beekeeper",
						Decision:         "allow",
						Reason:           "operator purge",
						RuleIDs:          []string{"EDXT-05"},
						CatalogMatches:   []audit.CatalogProvenance{},
						SourcesAgreed:    []string{},
						SourcesDissented: []string{},
						Endpoint:         "quarantine",
					}
					_ = w.Write(rec)
				}
				w.Close()
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Purged %d item(s).\n", len(purged))
			return nil
		},
	}
	purgeCmd.Flags().BoolVar(&purgeYes, "yes", false, "Skip confirmation prompt")
	qCmd.AddCommand(purgeCmd)

	return qCmd
}

// newAuditCmd groups audit-log subcommands.
func newAuditCmd() *cobra.Command {
	auditCmd := &cobra.Command{
		Use:   "audit",
		Short: "Inspect the Beekeeper audit log",
	}

	// --- tail subcommand ---
	var noFollow bool
	tailCmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream the live audit log to the terminal",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			logPath := filepath.Join(auditDir, "beekeeper.ndjson")
			if noFollow {
				return tailAuditLogOnce(cmd.OutOrStdout(), logPath)
			}
			return tailAuditLog(cmd.Context(), cmd.OutOrStdout(), logPath)
		},
	}
	tailCmd.Flags().BoolVar(&noFollow, "no-follow", false, "dump existing records and exit without following")
	auditCmd.AddCommand(tailCmd)

	// --- query subcommand ---
	var (
		qSince    string
		qAgent    string
		qTool     string
		qDecision string
		qLimit    int
	)
	queryCmd := &cobra.Command{
		Use:   "query",
		Short: "Filter audit log records by time, agent, tool, or decision",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			logPath := filepath.Join(auditDir, "beekeeper.ndjson")

			f, err := os.Open(logPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("audit log %q does not exist yet (run a beekeeper check first)", logPath)
				}
				return fmt.Errorf("open audit log: %w", err)
			}
			defer f.Close()

			opts := audit.QueryOpts{
				Agent:    qAgent,
				Tool:     qTool,
				Decision: qDecision,
				Limit:    qLimit,
			}
			if qSince != "" {
				if dur, derr := time.ParseDuration(qSince); derr == nil {
					opts.Since = time.Now().Add(-dur)
				} else if ts, terr := time.Parse(time.RFC3339, qSince); terr == nil {
					opts.Since = ts
				} else {
					return fmt.Errorf("--since %q: expected duration (e.g. 24h) or RFC3339 timestamp", qSince)
				}
			}

			return audit.Query(cmd.Context(), f, opts, cmd.OutOrStdout())
		},
	}
	queryCmd.Flags().StringVar(&qSince, "since", "", "Only show records after this duration (e.g. 24h) or RFC3339 timestamp")
	queryCmd.Flags().StringVar(&qAgent, "agent", "", "Filter by agent name")
	queryCmd.Flags().StringVar(&qTool, "tool", "", "Filter by tool name")
	queryCmd.Flags().StringVar(&qDecision, "decision", "", "Filter by decision (allow|warn|block)")
	queryCmd.Flags().IntVar(&qLimit, "limit", 0, "Maximum number of records to return (0 = no limit)")
	auditCmd.AddCommand(queryCmd)

	// --- export subcommand ---
	var (
		eFormat   string
		eSince    string
		eAgent    string
		eTool     string
		eDecision string
	)
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export audit log records in the requested format (ndjson, csv, otlp)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			logPath := filepath.Join(auditDir, "beekeeper.ndjson")

			f, err := os.Open(logPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("audit log %q does not exist yet (run a beekeeper check first)", logPath)
				}
				return fmt.Errorf("open audit log: %w", err)
			}
			defer f.Close()

			opts := audit.ExportOpts{
				Format: eFormat,
				QueryOpts: audit.QueryOpts{
					Agent:    eAgent,
					Tool:     eTool,
					Decision: eDecision,
				},
			}
			if eSince != "" {
				if dur, derr := time.ParseDuration(eSince); derr == nil {
					opts.Since = time.Now().Add(-dur)
				} else if ts, terr := time.Parse(time.RFC3339, eSince); terr == nil {
					opts.Since = ts
				} else {
					return fmt.Errorf("--since %q: expected duration (e.g. 24h) or RFC3339 timestamp", eSince)
				}
			}

			return audit.Export(cmd.Context(), f, opts, cmd.OutOrStdout())
		},
	}
	exportCmd.Flags().StringVar(&eFormat, "format", "", "Output format: ndjson, csv, or otlp (required)")
	exportCmd.Flags().StringVar(&eSince, "since", "", "Only export records after this duration (e.g. 24h) or RFC3339 timestamp")
	exportCmd.Flags().StringVar(&eAgent, "agent", "", "Filter by agent name")
	exportCmd.Flags().StringVar(&eTool, "tool", "", "Filter by tool name")
	exportCmd.Flags().StringVar(&eDecision, "decision", "", "Filter by decision (allow|warn|block)")
	_ = exportCmd.MarkFlagRequired("format")
	auditCmd.AddCommand(exportCmd)

	return auditCmd
}

// tailAuditLogOnce dumps all existing audit log content to out and returns.
// It does not follow for new bytes. Used by `beekeeper audit tail --no-follow`.
func tailAuditLogOnce(out io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("audit log %q does not exist yet (run a beekeeper check first)", path)
		}
		return fmt.Errorf("open audit log %q: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(out, f); err != nil {
		return fmt.Errorf("read audit log: %w", err)
	}
	return nil
}

// tailAuditLog streams the NDJSON audit log to out: it prints all existing
// content, then follows the file by polling for newly-appended bytes every
// ~500ms until the context is cancelled (Ctrl+C). Stdlib only — no external
// dependencies. The log is opened read-only; tailing never mutates it.
func tailAuditLog(ctx context.Context, out io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("audit log %q does not exist yet (run a beekeeper check first)", path)
		}
		return fmt.Errorf("open audit log %q: %w", path, err)
	}
	defer f.Close()

	offset, err := io.Copy(out, f)
	if err != nil {
		return fmt.Errorf("read audit log: %w", err)
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for {
				n, readErr := f.ReadAt(buf, offset)
				if n > 0 {
					if _, werr := out.Write(buf[:n]); werr != nil {
						return fmt.Errorf("write audit log to output: %w", werr)
					}
					offset += int64(n)
				}
				if readErr == io.EOF || n == 0 {
					break
				}
				if readErr != nil {
					return fmt.Errorf("follow audit log: %w", readErr)
				}
			}
		}
	}
}

// newSelftestCmd runs the embedded adversarial corpus and exits non-zero if any
// fixture produces an unexpected decision.
func newSelftestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "selftest",
		Short: "Run embedded adversarial fixtures as a sanity check",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			passed, failed, err := check.RunSelftest()
			fmt.Fprintf(cmd.OutOrStdout(), "PASS: %d, FAIL: %d\n", passed, failed)
			if err != nil {
				return fmt.Errorf("selftest error: %w", err)
			}
			if failed > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
}

// newHooksCmd groups hook installer subcommands for writing Beekeeper
// PreToolUse/PostToolUse hooks to agent CLIs (INTG-01, INTG-02).
func newHooksCmd() *cobra.Command {
	hooksCmd := &cobra.Command{
		Use:   "hooks",
		Short: "Install or uninstall Beekeeper hooks for agent CLIs",
	}

	// hooks install
	var target string
	var dryRun, force bool
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install Beekeeper PreToolUse/PostToolUse hooks for the given agent CLI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := hooks.Install(target, dryRun, force); err != nil {
				return fmt.Errorf("hooks install: %w", err)
			}
			if !dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Beekeeper hooks installed for target %q.\n", target)
				// CSYNC-06: populate the threat-intel index now (best-effort) and
				// offer to register the unprivileged background sync daemon.
				offerCatalogSyncDaemon(cmd)
			}
			return nil
		},
	}
	installCmd.Flags().StringVar(&target, "target", "", "Agent CLI target (claude-code, cursor, codex, augment, codebuddy, qwen, continue, opencode, openclaw)")
	installCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be written without modifying files")
	installCmd.Flags().BoolVar(&force, "force", false, "Overwrite existing hooks without prompting")
	_ = installCmd.MarkFlagRequired("target")
	hooksCmd.AddCommand(installCmd)

	// hooks uninstall
	var uninstallTarget string
	var uninstallDryRun bool
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Beekeeper hooks for the given agent CLI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := hooks.Uninstall(uninstallTarget, uninstallDryRun); err != nil {
				return fmt.Errorf("hooks uninstall: %w", err)
			}
			if !uninstallDryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Beekeeper hooks uninstalled for target %q.\n", uninstallTarget)
			}
			return nil
		},
	}
	uninstallCmd.Flags().StringVar(&uninstallTarget, "target", "", "Agent CLI target (claude-code, cursor, codex, augment, codebuddy, qwen, continue, opencode, openclaw)")
	uninstallCmd.Flags().BoolVar(&uninstallDryRun, "dry-run", false, "Print what would be removed without modifying files")
	_ = uninstallCmd.MarkFlagRequired("target")
	hooksCmd.AddCommand(uninstallCmd)

	return hooksCmd
}

// newGatewayCmd manages the Beekeeper MCP gateway daemon (INTG-03, INTG-04).
// The root `beekeeper gateway` subcommand is the foreground daemon.
// Subcommands `token` and `status` read state.json without starting the daemon.
func newGatewayCmd() *cobra.Command {
	var port int
	var upstream, bind string
	var allowRemote bool
	gatewayCmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the Beekeeper MCP gateway daemon",
		Long: `Start the Beekeeper MCP gateway daemon (foreground, Ctrl+C to stop).

The gateway is a stateless per-request HTTP proxy that intercepts MCP tools/call
requests and evaluates them against the Beekeeper policy engine before forwarding
to the upstream MCP server.

Security: the gateway binds to 127.0.0.1 only by default. Binding a non-loopback
address (e.g. --bind 0.0.0.0) requires --allow-remote and is strongly discouraged
because the gateway is plain HTTP — the bearer token travels in cleartext over any
non-loopback network path. If you must expose the gateway on a LAN or external
interface, place it behind a TLS-terminating reverse proxy.

Note: --upstream is the URL of the upstream MCP server to proxy to (required).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Phase 9 (CTLG-04/SFDF-06): self-quarantine guard.
			if err := enforceSelfQuarantine(cmd); err != nil {
				return err
			}

			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			catalogDir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}

			// CODE-05 SC2: layered resolver so project config overrides user config.
			cfg, err := resolveConfig(cmd)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// Start LlamaFirewall supervisor when enabled (INT-BLOCK-1 / LLMF-01).
			// The gateway daemon is the lifecycle host for the long-lived sidecar.
			// With LLMF disabled (default) no supervisor is created and the gateway
			// runs without scanning — all existing behavior is unchanged.
			var llmfScanner gateway.GatewayScanner
			if cfg.LlamaFirewallEnabled() {
				llmfCfg := llamafirewall.LlamaFirewallConfig{
					Enabled:          true,
					SampleRate:       cfg.LlamaFirewallSampleRate(),
					FailMode:         cfg.LlamaFirewall.FailMode,
					CodeShield:       cfg.LlamaFirewall.CodeShield,
					CodeShieldAction: cfg.LlamaFirewall.CodeShieldAction,
					PythonPath:       cfg.LlamaFirewall.PythonPath,
				}
				failClosed := cfg.LlamaFirewall.FailMode == "" || cfg.LlamaFirewall.FailMode == "closed"
				// Materialize the embedded sidecar script under the StateDir (hash-skip
				// on a matching stamp). The venv + gated model are bootstrapped
				// separately by `beekeeper llamafirewall install`.
				sidecarPath, instErr := llamafirewall.InstallSidecar(stateDir)
				if instErr != nil {
					if failClosed {
						return fmt.Errorf("llamafirewall sidecar install failed (fail-closed): %w", instErr)
					}
					fmt.Fprintf(os.Stderr, "beekeeper gateway: llamafirewall sidecar install failed (fail-open): %v\n", instErr)
				} else {
					sup := llamafirewall.NewSupervisor(llmfCfg, sidecarPath)
					if err := sup.Start(ctx); err != nil {
						// Fail-closed: if sidecar fails to start and FailMode is closed, abort.
						if failClosed {
							return fmt.Errorf("llamafirewall sidecar failed to start (fail-closed): %w", err)
						}
						// fail-open: log and continue without scanning.
						fmt.Fprintf(os.Stderr, "beekeeper gateway: llamafirewall sidecar unavailable (fail-open): %v\n", err)
					} else {
						llmfScanner = sup
						defer sup.Stop() //nolint:errcheck
					}
				}
			}

			gatewayCfg := gateway.Config{
				UpstreamURL: upstream,
				BindAddr:    bind,
				Port:        port,
				AllowRemote: allowRemote,
				StateFile:   filepath.Join(stateDir, "state.json"),
				IndexPath:   filepath.Join(catalogDir, "bumblebee.idx"),
				CacheDir:    catalogDir,
				AuditPath:   filepath.Join(auditDir, "beekeeper.ndjson"),
				SocketToken: cfg.SocketAPIToken(),
				FailOpen:    !cfg.FailClosed(),
				Scanner:     llmfScanner,
			}

			bindAddr := bind
			if bindAddr == "" {
				bindAddr = "127.0.0.1"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Starting Beekeeper gateway on %s:%d (Ctrl+C to stop)...\n", bindAddr, port)

			return gateway.Start(ctx, gatewayCfg)
		},
	}
	gatewayCmd.Flags().IntVar(&port, "port", 7837, "TCP port to bind (default 7837; 0 = random)")
	gatewayCmd.Flags().StringVar(&upstream, "upstream", "", "Upstream MCP server URL (required at runtime)")
	gatewayCmd.Flags().StringVar(&bind, "bind", "127.0.0.1", "Bind address (default 127.0.0.1 — localhost only)")
	gatewayCmd.Flags().BoolVar(&allowRemote, "allow-remote", false, "Permit binding a non-loopback address (plain HTTP — place behind TLS proxy; TM-A-01)")

	// gateway token — print the current session token from state.json
	gatewayCmd.AddCommand(&cobra.Command{
		Use:   "token",
		Short: "Print the current gateway session token from state.json",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			st, err := gateway.LoadGatewayState(filepath.Join(stateDir, "state.json"))
			if err != nil {
				return fmt.Errorf("load gateway state: %w", err)
			}
			if st.GatewayToken == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Gateway not running (or state.json not found).")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), st.GatewayToken)
			return nil
		},
	})

	// gateway status — print running status, bound address, masked token, started time
	gatewayCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Print the gateway daemon running status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			st, err := gateway.LoadGatewayState(filepath.Join(stateDir, "state.json"))
			if err != nil {
				return fmt.Errorf("load gateway state: %w", err)
			}

			out := cmd.OutOrStdout()

			// Check whether the PID in state.json is still a live process.
			running := false
			if st.PID > 0 {
				proc, procErr := os.FindProcess(st.PID)
				if procErr == nil {
					// On Unix, FindProcess always succeeds; use Signal(0) to probe.
					if signalErr := proc.Signal(syscall.Signal(0)); signalErr == nil {
						running = true
					}
				}
			}

			status := "not running"
			if running {
				status = fmt.Sprintf("running (pid %d)", st.PID)
			}
			fmt.Fprintf(out, "Status:  %s\n", status)

			if st.BoundAddr != "" && st.BoundPort > 0 {
				fmt.Fprintf(out, "Address: %s:%d\n", st.BoundAddr, st.BoundPort)
			} else {
				fmt.Fprintf(out, "Address: (not bound)\n")
			}

			// Mask token: show first 8 chars + "..." for security (T-04-05-01).
			token := st.GatewayToken
			if len(token) >= 8 {
				fmt.Fprintf(out, "Token:   %s...\n", token[:8])
			} else if token != "" {
				fmt.Fprintf(out, "Token:   %s...\n", token)
			} else {
				fmt.Fprintf(out, "Token:   (none)\n")
			}

			if st.StartedAt != "" {
				fmt.Fprintf(out, "Started: %s\n", st.StartedAt)
			}
			return nil
		},
	})

	return gatewayCmd
}

// newShimCmd groups shim layer subcommands for managing PATH-prepended wrapper
// scripts for package managers and toolchains (INTG-06).
func newShimCmd() *cobra.Command {
	shimCmd := &cobra.Command{
		Use:   "shim",
		Short: "Manage PATH shims for package managers and toolchains",
		Long: `Install PATH-prepended wrapper scripts that run 'beekeeper check' before each
package-manager install, so catalog corroboration and install posture apply to
installs you run yourself in a terminal.

This is an experimental surface with real limits: it only covers tools invoked
through the shimmed PATH, it can be bypassed by calling a tool by its absolute
path, and it requires adding the shim directory to the front of your PATH. It is
not a complete machine-wide guarantee. For hooked agents the pre-exec hook is the
primary enforcement point; see the install-posture docs for the full boundary.`,
	}

	// shim install
	shimCmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Create shim scripts for package managers in ~/.beekeeper/shims/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			shimDir := filepath.Join(stateDir, "shims")
			return shim.Install(shimDir, shim.DefaultTools, cmd.OutOrStdout())
		},
	})

	// shim uninstall
	shimCmd.AddCommand(&cobra.Command{
		Use:   "uninstall",
		Short: "Remove all Beekeeper shim scripts from ~/.beekeeper/shims/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			shimDir := filepath.Join(stateDir, "shims")
			return shim.Uninstall(shimDir)
		},
	})

	// shim status
	shimCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "List which tools are shimmed and their real binary paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state directory: %w", err)
			}
			shimDir := filepath.Join(stateDir, "shims")
			return shim.Status(shimDir, shim.DefaultTools, cmd.OutOrStdout())
		},
	})

	return shimCmd
}

// newProtectCmd groups the Sentry daemon lifecycle subcommands.
// Linux (systemd), macOS (launchd/eslogger), and Windows (Service/ETW) are
// supported via build-tagged implementations (protect_{linux,darwin,windows}.go);
// only genuinely unsupported platforms print a not-supported message (protect_other.go).
func newProtectCmd() *cobra.Command {
	protect := &cobra.Command{
		Use:   "protect",
		Short: "Manage the Beekeeper Sentry runtime-monitor daemon",
	}
	protect.AddCommand(
		newProtectInstallCmd(),
		newProtectUninstallCmd(),
		newProtectStatusCmd(),
	)
	return protect
}

func newProtectInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and start the Sentry daemon as an OS service (systemd / launchd / Windows Service)",
		Args:  cobra.NoArgs,
		RunE:  runProtectInstall,
	}
}

func newProtectUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the Sentry daemon",
		Args:  cobra.NoArgs,
		RunE:  runProtectUninstall,
	}
}

func newProtectStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Sentry daemon status",
		Args:  cobra.NoArgs,
		RunE:  runProtectStatus,
	}
}

// newSentryCmd is the Sentry daemon subcommand. When invoked directly it runs
// the daemon (the ExecStart target of the OS service: the systemd unit on Linux,
// launchd on macOS, the Windows Service on Windows). The rules subcommand group
// provides live rule management via IPC.
func newSentryCmd() *cobra.Command {
	daemon := &cobra.Command{
		Use:   "sentry",
		Short: "Sentry daemon (invoked by the OS service manager)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Phase 9 (CTLG-04/SFDF-06): self-quarantine guard.
			if err := enforceSelfQuarantine(cmd); err != nil {
				return err
			}
			return runSentryDaemon(cmd, args)
		},
	}

	rulesCmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage Sentry correlation rules at runtime",
	}
	rulesCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List active rules and their enabled state",
			Args:  cobra.NoArgs,
			RunE:  runSentryRulesList,
		},
		&cobra.Command{
			Use:   "enable <id>",
			Short: "Enable a Sentry rule by ID",
			Args:  cobra.ExactArgs(1),
			RunE:  runSentryRulesEnable,
		},
		&cobra.Command{
			Use:   "disable <id>",
			Short: "Disable a Sentry rule by ID",
			Args:  cobra.ExactArgs(1),
			RunE:  runSentryRulesDisable,
		},
	)
	daemon.AddCommand(rulesCmd)
	return daemon
}

// llamafirewallConfigPath resolves the beekeeper config.json path via the
// platform resolver (honors %APPDATA% on Windows / BEEKEEPER_HOME on all OSes),
// returning "" if it cannot be determined so the caller's config.Load surfaces
// the error. Replaces the old $HOME/.beekeeper hardcode (Phase 20, LLMF — the
// Windows StateDir bug).
func llamafirewallConfigPath() string {
	p, err := platform.ConfigPath()
	if err != nil {
		return ""
	}
	return p
}

// llamafirewallStatePath resolves <StateDir>/state.json via the platform
// resolver, returning "" if the state dir cannot be determined.
func llamafirewallStatePath() string {
	dir, err := platform.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "state.json")
}

// newLlamaFirewallCmd groups the LlamaFirewall prompt-injection sidecar subcommands.
// It provides enable/disable/status management (LLMF-01, LLMF-06).
func newLlamaFirewallCmd() *cobra.Command {
	lfCmd := &cobra.Command{
		Use:   "llamafirewall",
		Short: "Manage the LlamaFirewall prompt-injection sidecar",
	}

	lfCmd.AddCommand(&cobra.Command{
		Use:   "enable",
		Short: "Enable LlamaFirewall sidecar scanning",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath := llamafirewallConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			cfg.LlamaFirewall.Enabled = true
			if err := config.Save(cfgPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall enabled. Run 'beekeeper llamafirewall status' to check sidecar state.")
			return nil
		},
	})

	lfCmd.AddCommand(&cobra.Command{
		Use:   "disable",
		Short: "Disable LlamaFirewall sidecar scanning",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath := llamafirewallConfigPath()
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			cfg.LlamaFirewall.Enabled = false
			if err := config.Save(cfgPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall disabled.")
			return nil
		},
	})

	lfCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show LlamaFirewall sidecar status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Read state.json for PID + started_at written by the supervisor.
			statePath := llamafirewallStatePath()
			data, err := os.ReadFile(statePath)
			if err != nil || data == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running")
				return nil
			}
			var state map[string]any
			if json.Unmarshal(data, &state) != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running")
				return nil
			}
			lfState, ok := state["llamafirewall"].(map[string]any)
			if !ok {
				fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running")
				return nil
			}
			pidF, okPID := lfState["pid"].(float64)
			startedAt, okStarted := lfState["started_at"].(string)
			if !okPID || !okStarted {
				fmt.Fprintln(cmd.OutOrStdout(), "LlamaFirewall Sidecar — Not running")
				return nil
			}
			pid := int(pidF)

			// Check if the process is still alive using Signal(0).
			proc, _ := os.FindProcess(pid)
			alive := proc != nil && proc.Signal(syscall.Signal(0)) == nil

			cfgPath := llamafirewallConfigPath()
			cfg, _ := config.Load(cfgPath)
			sampleRate := cfg.LlamaFirewallSampleRate()
			failMode := cfg.LlamaFirewall.FailMode
			if failMode == "" {
				failMode = "closed"
			}

			out := cmd.OutOrStdout()
			if alive {
				t, _ := time.Parse(time.RFC3339, startedAt)
				uptime := time.Since(t).Round(time.Second)
				fmt.Fprintf(out, "LlamaFirewall Sidecar — Active (PID %d, uptime %s)\n", pid, uptime)
			} else {
				fmt.Fprintln(out, "LlamaFirewall Sidecar — Not running (stale PID in state.json)")
			}
			fmt.Fprintf(out, "Sample Rate: %.2f\n", sampleRate)
			fmt.Fprintf(out, "Fail Mode:  %s\n", failMode)
			fmt.Fprintf(out, "P95 Latency: N/A (use beekeeper diag when running)\n")
			fmt.Fprintf(out, "Degraded:   %v\n", !alive)
			return nil
		},
	})

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Bootstrap the LlamaFirewall venv, pinned CPU deps, and gated model",
		Long: `Bootstrap the opt-in LlamaFirewall runtime under the beekeeper StateDir:
materialize the sidecar, create a Python venv, install pinned dependencies from
the CPU torch index (NOT the multi-GB CUDA wheels), and pre-pull the GATED
Llama-Prompt-Guard-2 model into a pinned HF_HOME.

The model is GATED: accept its license at
https://huggingface.co/meta-llama/Llama-Prompt-Guard-2-22M and run
'huggingface-cli login' first, or the download step will fail.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("resolve state dir: %w", err)
			}
			model, _ := cmd.Flags().GetString("model")
			out := cmd.OutOrStdout()

			// Materialize the embedded sidecar script + requirements.txt.
			scriptPath, err := llamafirewall.InstallSidecar(stateDir)
			if err != nil {
				return fmt.Errorf("install sidecar assets: %w", err)
			}
			sidecarDir := filepath.Dir(scriptPath)
			reqPath := filepath.Join(sidecarDir, "requirements.txt")
			venvDir := llamafirewall.VenvDir(stateDir)
			hfHome := llamafirewall.HFHome(stateDir)

			// Surface the GATED-model requirement up front — accepting the license
			// is a human-only web action automation cannot perform.
			fmt.Fprintf(out, "LlamaFirewall is opt-in and uses the GATED model %s.\n", model)
			fmt.Fprintln(out, "You MUST accept its license and authenticate first:")
			fmt.Fprintf(out, "  1. Accept https://huggingface.co/%s\n", model)
			fmt.Fprintln(out, "  2. huggingface-cli login   (HF token that accepted the license)")
			fmt.Fprintln(out)

			// Choose the interpreter that creates the venv (config override wins).
			basePython := "python3"
			if runtime.GOOS == "windows" {
				basePython = "python"
			}
			if cfg, lerr := config.Load(llamafirewallConfigPath()); lerr == nil && cfg.LlamaFirewall.PythonPath != "" {
				basePython = cfg.LlamaFirewall.PythonPath
			}

			fmt.Fprintf(out, "Creating venv at %s ...\n", venvDir)
			if err := runLlamafirewallCmd(cmd, basePython, "-m", "venv", venvDir); err != nil {
				return fmt.Errorf("create venv: %w", err)
			}
			venvPython := llamafirewall.VenvPython(venvDir)

			fmt.Fprintln(out, "Installing pinned dependencies (CPU torch index, no CUDA wheels) ...")
			if err := runLlamafirewallCmd(cmd, venvPython, "-m", "pip", "install", "--upgrade", "pip"); err != nil {
				return fmt.Errorf("upgrade pip: %w", err)
			}
			if err := runLlamafirewallCmd(cmd, venvPython, "-m", "pip", "install", "-r", reqPath,
				"--extra-index-url", "https://download.pytorch.org/whl/cpu"); err != nil {
				return fmt.Errorf("pip install: %w", err)
			}

			// Pre-pull the gated model into a pinned HF_HOME under the StateDir.
			if err := os.MkdirAll(hfHome, 0o700); err != nil {
				return fmt.Errorf("create HF cache dir: %w", err)
			}
			fmt.Fprintf(out, "Pre-pulling gated model %s into %s ...\n", model, hfHome)
			dl := exec.Command(venvPython, "-c",
				"import sys; from huggingface_hub import snapshot_download; snapshot_download(sys.argv[1])", model)
			dl.Env = append(os.Environ(), "HF_HOME="+hfHome)
			dl.Stdout = out
			dl.Stderr = cmd.ErrOrStderr()
			if err := dl.Run(); err != nil {
				return fmt.Errorf("pre-pull gated model %s failed — did you accept the Llama license and run 'huggingface-cli login'? %w", model, err)
			}

			fmt.Fprintln(out, "LlamaFirewall install complete. Enable scanning with: beekeeper llamafirewall enable")
			return nil
		},
	}
	installCmd.Flags().String("model", llamafirewall.DefaultPromptGuardModel, "Gated PromptGuard model to pre-pull into HF_HOME")
	lfCmd.AddCommand(installCmd)

	return lfCmd
}

// runLlamafirewallCmd runs name+args, streaming stdout/stderr to the command's
// output so venv/pip progress is visible. Used by `beekeeper llamafirewall
// install`.
func runLlamafirewallCmd(cmd *cobra.Command, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

// newAuditRecordCmd is the PostToolUse hook handler. It reads PostToolUse JSON
// from stdin, writes a tool_result audit record, and exits 0 always unless LLMF
// is enabled and a sidecar scan returns a fail-closed block decision.
//
// With LLMF enabled, the command connects to an already-running sidecar socket
// (started by the gateway daemon or a standalone llamafirewall serve command)
// and routes to RunAuditRecordWithLLMF. If the sidecar is unreachable, the
// fail_mode governs: fail-closed = block (return 1); fail-open = allow (return 0).
func newAuditRecordCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit-record",
		Short: "Record a PostToolUse hook event to the audit log (exit 0 always)",
		Long: `Read a PostToolUse hook event from stdin and write a tool_result audit record.

This command is registered as the PostToolUse hook command:
  {"type": "command", "command": "beekeeper audit-record"}

Without LlamaFirewall enabled it always exits 0 — PostToolUse hook failures
must not disrupt the agent. With LlamaFirewall enabled, a fail-closed sidecar
unreachability can exit 1 (block).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auditDir, err := platform.AuditDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "beekeeper audit-record: resolve audit directory: %v\n", err)
				return nil
			}
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			// Load config to check if LLMF is enabled.
			cfgPath, cfgErr := platform.ConfigPath()
			if cfgErr != nil {
				_ = check.RunAuditRecord(os.Stdin, auditPath)
				return nil
			}
			cfg, cfgErr := config.Load(cfgPath)
			if cfgErr != nil || !cfg.LlamaFirewallEnabled() {
				// LLMF disabled or config unreadable → plain audit-record (exit 0).
				_ = check.RunAuditRecord(os.Stdin, auditPath)
				return nil
			}

			// LLMF enabled: connect to a running sidecar socket (fail-closed on
			// unreachability per fail_mode — INT-BLOCK-1 / LLMF-05).
			stateDir, sdErr := platform.StateDir()
			if sdErr != nil {
				_ = check.RunAuditRecord(os.Stdin, auditPath)
				return nil
			}
			// Read the loopback port + per-launch bearer token the gateway's
			// supervisor persisted to state.json (Phase 20, LLMF — IPC is loopback
			// TCP + bearer token; one-shot commands connect to the running sidecar).
			port, token, epErr := readLlamafirewallEndpoint(stateDir)
			if epErr != nil {
				// No running sidecar endpoint recorded → treat as unreachable.
				if cfg.LlamaFirewall.FailMode == "" || cfg.LlamaFirewall.FailMode == "closed" {
					fmt.Fprintf(os.Stderr, "beekeeper audit-record: LLMF sidecar endpoint unknown (fail-closed): %v\n", epErr)
					os.Exit(1) // block PostToolUse
				}
				// fail-open: continue without scanning.
				_ = check.RunAuditRecord(os.Stdin, auditPath)
				return nil
			}
			client, dialErr := llamafirewall.Dial(net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), token, 2*time.Second)
			if dialErr != nil {
				// Sidecar unreachable.
				if cfg.LlamaFirewall.FailMode == "" || cfg.LlamaFirewall.FailMode == "closed" {
					fmt.Fprintf(os.Stderr, "beekeeper audit-record: LLMF sidecar unreachable (fail-closed): %v\n", dialErr)
					os.Exit(1) // block PostToolUse
				}
				// fail-open: continue without scanning.
				_ = check.RunAuditRecord(os.Stdin, auditPath)
				return nil
			}
			defer client.Close() //nolint:errcheck

			// clientScanner adapts *llamafirewall.Client to check.Scannable so
			// RunAuditRecordWithLLMF can use the connected client.
			scanner := &llmfClientScanner{client: client}
			exitCode := check.RunAuditRecordWithLLMF(os.Stdin, auditPath, cfg, scanner)
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}
}

// llmfClientScanner adapts *llamafirewall.Client to the check.Scannable interface.
// It wraps the raw client for use by RunAuditRecordWithLLMF in one-shot commands.
// The sidecar is long-lived (started by the gateway daemon); one-shot commands
// connect to the running socket rather than spawning their own.
type llmfClientScanner struct {
	client *llamafirewall.Client
}

func (s *llmfClientScanner) Scan(ctx context.Context, req llamafirewall.ScanRequest) (llamafirewall.ScanResponse, error) {
	return s.client.Scan(req)
}

func (s *llmfClientScanner) IsDegraded() bool { return false }

// readLlamafirewallEndpoint reads the loopback port + per-launch bearer token the
// LlamaFirewall supervisor persisted to <stateDir>/state.json. One-shot commands
// (audit-record) use it to reach the long-lived sidecar started by the gateway
// daemon (Phase 20, LLMF — IPC is loopback TCP + bearer token). It returns an
// error when no complete endpoint is recorded, which the caller treats per the
// configured fail-mode (fail-closed = block).
func readLlamafirewallEndpoint(stateDir string) (int, string, error) {
	statePath := filepath.Join(stateDir, "state.json")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return 0, "", err
	}
	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, "", err
	}
	lf, ok := state["llamafirewall"].(map[string]any)
	if !ok {
		return 0, "", fmt.Errorf("no llamafirewall endpoint recorded in state.json")
	}
	portF, okPort := lf["port"].(float64) // JSON numbers decode to float64
	token, okToken := lf["token"].(string)
	if !okPort || !okToken || int(portF) == 0 || token == "" {
		return 0, "", fmt.Errorf("incomplete llamafirewall endpoint in state.json")
	}
	return int(portF), token, nil
}

// newDashboardCmd opens the real-time Bubble Tea TUI dashboard.
func newDashboardCmd() *cobra.Command {
	var adminMode bool
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Open the real-time TUI dashboard",
		Long: `Open a single-screen terminal dashboard showing live tool call decisions,
Sentry alerts, catalog freshness, scan status, active policies, quarantine,
and system health. Use --admin to enable policy toggle and quarantine actions.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tui.Run(cmd.Context(), adminMode)
		},
	}
	cmd.Flags().BoolVar(&adminMode, "admin", false,
		"Enable admin mode (policy toggle, quarantine restore/purge, scan trigger)")
	return cmd
}
