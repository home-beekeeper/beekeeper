// policy.go — beekeeper policy subcommands (CODE-02, CODE-03, CODE-04, Phase 9).
//
// Provides three subcommands grouped under `beekeeper policy`:
//
//	policy validate <file>  — schema-check a policy file; exit non-zero on errors
//	policy test <file>      — dry-run a policy file against a sample tool-call JSON
//	policy list             — list loaded policy files with rule counts
//
// All business logic lives in internal/policyloader. This file is thin Cobra
// wiring per the project architecture constraint.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/home-beekeeper/beekeeper/internal/platform"
	"github.com/home-beekeeper/beekeeper/internal/policy"
	"github.com/home-beekeeper/beekeeper/internal/policyloader"
)

// newPolicyCmd groups the three policy-as-code subcommands.
// Pattern: newCatalogsCmd() / newQuarantineCmd() grouped idiom.
func newPolicyCmd() *cobra.Command {
	policyCmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage and test declarative policy files",
		Long: `Work with declarative JSON policy files stored in ~/.beekeeper/policies/.

Policy files let you version-control project-level rules that restrict or
override the default enforcement behaviour without modifying built-in engine
logic. All rules are pure data predicates — no code execution surface.`,
	}
	policyCmd.AddCommand(
		newPolicyValidateCmd(),
		newPolicyTestCmd(),
		newPolicyListCmd(),
	)
	return policyCmd
}

// newPolicyValidateCmd implements `beekeeper policy validate <file>`.
// It exits non-zero when the file has schema errors and exits 0 on a valid file.
func newPolicyValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a policy file schema; exit non-zero on errors",
		Long: `Load and schema-validate a declarative policy JSON file.

Validation checks:
  - schema_version must be "1"
  - each rule's rule_type must be one of the five legal values
  - each rule's action (if present) must be "block", "warn", or "allow"
  - unknown fields (e.g. "url", "exec") are rejected

All errors are reported together. Exit code 0 means the file is valid.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			_, errs := policyloader.LoadPolicyFile(path)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(cmd.ErrOrStderr(), e)
				}
				return fmt.Errorf("policy validate: %d error(s) in %q", len(errs), path)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK: %s\n", path)
			return nil
		},
	}
}

// newPolicyTestCmd implements `beekeeper policy test <file> --tool-call <path|->`.
// It dry-runs the policy file against the sample tool-call JSON (no live catalog)
// and prints the resulting Decision.
func newPolicyTestCmd() *cobra.Command {
	var toolCallPath string
	cmd := &cobra.Command{
		Use:   "test <file>",
		Short: "Dry-run a policy file against a sample tool-call JSON",
		Long: `Load and validate a policy file, then dry-run it against a sample tool-call JSON.

The dry-run uses NO live catalog (empty lookup) so results reflect only the
policy file's threshold overrides. This makes output deterministic. Use
--tool-call to provide the sample tool-call JSON as a file path or "-" for stdin.

Output: the decision the engine would produce (allow|warn|block) and the reason.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			policyPath := args[0]

			// Load and validate the policy file.
			pf, errs := policyloader.LoadPolicyFile(policyPath)
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(cmd.ErrOrStderr(), e)
				}
				return fmt.Errorf("policy test: %d validation error(s) in %q", len(errs), policyPath)
			}

			// Read the tool-call JSON from the flag path or stdin.
			var tcJSON []byte
			var readErr error
			if toolCallPath == "" || toolCallPath == "-" {
				tcJSON, readErr = io.ReadAll(cmd.InOrStdin())
			} else {
				tcJSON, readErr = os.ReadFile(toolCallPath)
			}
			if readErr != nil {
				return fmt.Errorf("policy test: read tool-call: %w", readErr)
			}

			var tc policy.ToolCall
			if err := json.Unmarshal(tcJSON, &tc); err != nil {
				return fmt.Errorf("policy test: parse tool-call JSON: %w", err)
			}

			decision := policyloader.RunPolicyTest(pf, tc, policy.AgentContext{})
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "decision: %s\n", decision.Level)
			fmt.Fprintf(out, "reason:   %s\n", decision.Reason)
			if len(decision.RuleIDs) > 0 {
				fmt.Fprintf(out, "rules:    %v\n", decision.RuleIDs)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&toolCallPath, "tool-call", "-",
		"Path to tool-call JSON file, or \"-\" to read from stdin")
	return cmd
}

// newPolicyListCmd implements `beekeeper policy list`.
// It resolves ~/.beekeeper/policies/, calls ListPolicyFiles, and prints each file
// with its rule count. An empty policies/ directory prints a friendly message
// rather than an error.
func newPolicyListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List loaded policy files with their rule counts",
		Long: `Scan ~/.beekeeper/policies/ for policy JSON files and print each with its rule count.

A missing policies/ directory (before beekeeper init has been run) prints a
friendly "no policy files" message and exits 0 — it is not an error.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stateDir, err := platform.StateDir()
			if err != nil {
				return fmt.Errorf("policy list: resolve state directory: %w", err)
			}
			policiesDir := filepath.Join(stateDir, "policies")

			summaries, err := policyloader.ListPolicyFiles(policiesDir)
			if err != nil {
				return fmt.Errorf("policy list: %w", err)
			}

			out := cmd.OutOrStdout()
			if len(summaries) == 0 {
				fmt.Fprintln(out, "no policy files (run 'beekeeper init' then add *.json files to ~/.beekeeper/policies/)")
				return nil
			}

			fmt.Fprintf(out, "%-6s  %-30s  %s\n", "rules", "name", "path")
			for _, s := range summaries {
				name := s.Name
				if name == "" {
					name = "(unnamed)"
				}
				fmt.Fprintf(out, "%-6d  %-30s  %s\n", s.RuleCount, name, s.Path)
			}
			return nil
		},
	}
}
