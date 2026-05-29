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
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	tui "github.com/mzansi-agentive/beekeeper/internal/tui"
	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/check"
	"github.com/mzansi-agentive/beekeeper/internal/config"
	"github.com/mzansi-agentive/beekeeper/internal/editorinit"
	"github.com/mzansi-agentive/beekeeper/internal/gateway"
	"github.com/mzansi-agentive/beekeeper/internal/hooks"
	"github.com/mzansi-agentive/beekeeper/internal/notify"
	"github.com/mzansi-agentive/beekeeper/internal/platform"
	"github.com/mzansi-agentive/beekeeper/internal/quarantine"
	"github.com/mzansi-agentive/beekeeper/internal/scan"
	"github.com/mzansi-agentive/beekeeper/internal/shim"
	"github.com/mzansi-agentive/beekeeper/internal/version"
	"github.com/mzansi-agentive/beekeeper/internal/watch"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "beekeeper",
		Short:        "Real-time safety harness for autonomous coding agents",
		Long:         "Beekeeper intercepts agent tool calls before they execute and evaluates them against unified threat intelligence.",
		SilenceUsage: true,
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
		newAuditRecordCmd(),
		newProtectCmd(),
		newSentryCmd(),
		newLlamaFirewallCmd(),
		newDashboardCmd(),
	)

	return root
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
			for _, dir := range []string{stateDir, catalogDir, auditDir} {
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

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Evaluate a tool call read from stdin (allow=0, block!=0)",
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
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			indexPath := filepath.Join(catalogDir, "bumblebee.idx")
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Shim invocation: build ToolCall JSON from flags using json.Marshal.
			// This is injection-safe — no shell string embedding of user arguments.
			var stdin io.Reader = os.Stdin
			if toolName != "" {
				tc := map[string]any{
					"tool_name":  "execute",
					"agent_name": "shim",
					"tool_input": map[string]any{
						"command": toolName,
						"args":    toolArgs,
					},
				}
				data, merr := json.Marshal(tc)
				if merr != nil {
					os.Exit(1)
					return nil
				}
				stdin = strings.NewReader(string(data))
			}

			result := check.RunCheck(cmd.Context(), stdin, cfg, indexPath, auditPath, catalogDir)
			os.Exit(result.ExitCode)
			return nil // unreachable; os.Exit above is the real return path
		},
	}
	cmd.Flags().StringVar(&toolName, "tool", "", "Tool name for shim invocations (builds ToolCall JSON from flags)")
	cmd.Flags().StringArrayVar(&toolArgs, "args", nil, "Arguments for shim invocations (used with --tool)")
	return cmd
}

// newCatalogsCmd groups catalog-management subcommands.
func newCatalogsCmd() *cobra.Command {
	catalogs := &cobra.Command{
		Use:   "catalogs",
		Short: "Manage cached threat-intel catalogs",
	}
	catalogs.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Fetch and cache catalogs, then build the mmap index",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := platform.CatalogDir()
			if err != nil {
				return fmt.Errorf("resolve catalog directory: %w", err)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create catalog directory %q: %w", dir, err)
			}

			client := &http.Client{Timeout: 30 * time.Second}
			n, err := catalog.Sync(cmd.Context(), client, dir)
			if err != nil {
				return fmt.Errorf("catalog sync failed: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Synced %d catalog entries\n", n)
			fmt.Fprintf(out, "Index: %s\n", filepath.Join(dir, "bumblebee.idx"))
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
			stateFile := filepath.Join(stateDir, "state.json")

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

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Starting catalog watch daemon (interval: %s, Ctrl+C to stop)...\n", cfg.PollInterval)

			return catalog.Watch(ctx, cfg, func(delta catalog.CatalogDelta, sanity catalog.SanityResult) {
				if sanity.Block {
					fmt.Fprintf(out, "catalog delta [HARD-BLOCK sanity breach]: source=%s prev=%d new=%d delta=%d — %s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount, sanity.Reason)
				} else if sanity.Alert {
					fmt.Fprintf(out, "catalog delta [ALERT sanity breach]: source=%s prev=%d new=%d delta=%d — %s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount, sanity.Reason)
				} else if delta.HasChanges() {
					fmt.Fprintf(out, "catalog delta: source=%s prev_count=%d new_count=%d delta=%d prev_hash=%s new_hash=%s\n",
						delta.Source, delta.PrevCount, delta.NewCount, delta.DeltaCount,
						delta.PrevHash, delta.NewHash)
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

	return catalogs
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
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			cfg, err := config.Load(configPath)
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
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			cfg, err := config.Load(configPath)
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
	cmd.Flags().BoolVar(&deep, "deep", false, "Run a deep scan (passes --profile deep to Bumblebee)")
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
			}
			return nil
		},
	}
	installCmd.Flags().StringVar(&target, "target", "", "Agent CLI target (claude-code, cursor, codex, continue, opencode, openclaw)")
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
	uninstallCmd.Flags().StringVar(&uninstallTarget, "target", "", "Agent CLI target (claude-code, cursor, codex, continue, opencode, openclaw)")
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
	gatewayCmd := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the Beekeeper MCP gateway daemon",
		Long: `Start the Beekeeper MCP gateway daemon (foreground, Ctrl+C to stop).

The gateway is a stateless per-request HTTP proxy that intercepts MCP tools/call
requests and evaluates them against the Beekeeper policy engine before forwarding
to the upstream MCP server.

Security: the gateway binds to 127.0.0.1 only by default. Exposing it on a
public interface (--bind 0.0.0.0) requires allow_remote_gateway:true in config
and is documented as reducing security.

Note: --upstream is the URL of the upstream MCP server to proxy to (required).`,
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
			auditDir, err := platform.AuditDir()
			if err != nil {
				return fmt.Errorf("resolve audit directory: %w", err)
			}
			configPath, err := platform.ConfigPath()
			if err != nil {
				return fmt.Errorf("resolve config path: %w", err)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			gatewayCfg := gateway.Config{
				UpstreamURL: upstream,
				BindAddr:    bind,
				Port:        port,
				StateFile:   filepath.Join(stateDir, "state.json"),
				IndexPath:   filepath.Join(catalogDir, "bumblebee.idx"),
				CacheDir:    catalogDir,
				AuditPath:   filepath.Join(auditDir, "beekeeper.ndjson"),
				SocketToken: cfg.SocketAPIToken(),
				FailOpen:    !cfg.FailClosed(),
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

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
// On non-Linux platforms each subcommand prints a not-supported message (see protect_other.go).
func newProtectCmd() *cobra.Command {
	protect := &cobra.Command{
		Use:   "protect",
		Short: "Manage the Beekeeper Sentry daemon (Linux only)",
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
		Short: "Install and start the Sentry daemon via systemd",
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
// the daemon (this is the ExecStart target in the systemd unit). The rules
// subcommand group provides live rule management via IPC.
func newSentryCmd() *cobra.Command {
	daemon := &cobra.Command{
		Use:   "sentry",
		Short: "Sentry daemon (invoked by systemd; Linux only)",
		Args:  cobra.NoArgs,
		RunE:  runSentryDaemon,
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
			cfgPath := filepath.Join(os.ExpandEnv("$HOME"), ".beekeeper", "config.json")
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
			cfgPath := filepath.Join(os.ExpandEnv("$HOME"), ".beekeeper", "config.json")
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
			statePath := filepath.Join(os.ExpandEnv("$HOME"), ".beekeeper", "state.json")
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
			pid := int(lfState["pid"].(float64))
			startedAt := lfState["started_at"].(string)

			// Check if the process is still alive using Signal(0).
			proc, _ := os.FindProcess(pid)
			alive := proc != nil && proc.Signal(syscall.Signal(0)) == nil

			cfgPath := filepath.Join(os.ExpandEnv("$HOME"), ".beekeeper", "config.json")
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

	return lfCmd
}

// newAuditRecordCmd is the PostToolUse hook handler. It reads PostToolUse JSON
// from stdin, writes a tool_result audit record, and exits 0 always — PostToolUse
// hook failures must not disrupt the running agent (INTG-07 / T-04-05-04).
func newAuditRecordCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit-record",
		Short: "Record a PostToolUse hook event to the audit log (exit 0 always)",
		Long: `Read a PostToolUse hook event from stdin and write a tool_result audit record.

This command is registered as the PostToolUse hook command:
  {"type": "command", "command": "beekeeper audit-record"}

It always exits 0 — PostToolUse hook failures must not disrupt the agent.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			auditDir, err := platform.AuditDir()
			if err != nil {
				// Cannot resolve audit directory — exit 0 anyway (T-04-05-04).
				fmt.Fprintf(os.Stderr, "beekeeper audit-record: resolve audit directory: %v\n", err)
				return nil
			}
			auditPath := filepath.Join(auditDir, "beekeeper.ndjson")
			_ = check.RunAuditRecord(os.Stdin, auditPath)
			// Always return nil (exit 0) regardless of RunAuditRecord result.
			return nil
		},
	}
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
