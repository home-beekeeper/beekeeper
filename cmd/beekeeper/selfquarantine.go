// selfquarantine.go — startup self-quarantine guard (CTLG-04, SFDF-06, Phase 9).
//
// enforceSelfQuarantine runs the beekeeper-self catalog check and refuses to
// continue for enforcement commands when the running version appears in the
// compromised-version list. On a quarantine match it:
//   - writes a self_quarantine audit record (T-09-20 / V7)
//   - prints a prominent warning to stderr with the verification path
//   - returns a non-nil error so RunE exits non-zero
//
// On a fail-closed (integrity failure) result it does the same but with an
// integrity-failure message.
//
// On warn-continue (network error, no cache) it logs a warning to stderr and
// returns nil so the tool continues.
//
// On continue (nominal path) it returns nil silently.
//
// The guard is called at the TOP of every enforcement command's RunE
// (check, gateway, sentry, watch) and at the END of catalogs sync.
// It is NOT called from version, diag, selftest, or policy validate — those
// commands must remain usable for diagnosis even when enforcement is locked
// (T-09-21, Open Question 3).
//
// The checkSelfCatalogFn package variable allows tests to inject a stub without
// modifying the implementation.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mzansi-agentive/beekeeper/internal/audit"
	"github.com/mzansi-agentive/beekeeper/internal/catalog"
	"github.com/mzansi-agentive/beekeeper/internal/platform"
	"github.com/mzansi-agentive/beekeeper/internal/version"
)

// checkSelfCatalogFn is the injectable seam for catalog.CheckSelfCatalog.
// Production code uses catalog.CheckSelfCatalog; tests inject a stub.
var checkSelfCatalogFn = catalog.CheckSelfCatalog

// selfQuarantineVerificationMsg is the verification-path message printed to
// stderr on a quarantine or integrity-failure result. It points the developer
// to the documented verification commands so they can independently verify
// the binary integrity before re-enabling enforcement.
//
// The text is deliberately prominent (uses uppercase WARNING and multiple
// verification methods) to satisfy V7 (T-09-20) and the CLAUDE.md requirement:
// "write an audit record AND a prominent stderr warning pointing to the verification path".
const selfQuarantineVerificationMsg = `
To verify binary integrity and determine safe next steps, run:

  make verify-release VERSION=<your-version>
        - or -
  cosign verify --certificate-identity=https://github.com/mzansi-agentive/beekeeper/.github/workflows/release.yml@refs/heads/main \
                --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
                <binary-path>
        - or -
  slsa-verifier verify-artifact <binary-path> \
                --provenance-path beekeeper.intoto.jsonl \
                --source-uri github.com/mzansi-agentive/beekeeper

See SECURITY.md for the full verification path and disclosure process.
Diagnostic commands (beekeeper version, beekeeper diag, beekeeper selftest,
beekeeper policy validate) remain available while enforcement is blocked.
`

// enforceSelfQuarantine runs the beekeeper-self catalog check and returns a
// non-nil error when the running binary must be refused. The error is returned
// to the caller's RunE and causes a non-zero exit code.
//
// It always resolves platform paths itself (no coupling to caller state) so it
// can be added to any RunE body with a single call.
func enforceSelfQuarantine(cmd *cobra.Command) error {
	stateDir, err := platform.StateDir()
	if err != nil {
		// Cannot resolve state dir — fail closed (cannot check, cannot allow).
		return fmt.Errorf("enforce self-quarantine: resolve state directory: %w", err)
	}
	catalogDir, err := platform.CatalogDir()
	if err != nil {
		return fmt.Errorf("enforce self-quarantine: resolve catalog directory: %w", err)
	}

	cfg, cfgErr := resolveConfig(cmd)
	feedURL := ""
	var pubKeyOverride ed25519.PublicKey
	if cfgErr == nil {
		feedURL = cfg.SelfCatalog.URL

		// CR-01: If the operator has configured a self-hosted public key, decode
		// it and use it as the PubKeyOverride. A key that is present but cannot
		// be decoded into a valid Ed25519 public key MUST fail closed — it cannot
		// silently fall back to the embedded key (T-09-32 / CR-01).
		if cfg.SelfCatalog.PubKey != "" {
			keyBytes, decErr := hex.DecodeString(cfg.SelfCatalog.PubKey)
			if decErr != nil {
				return fmt.Errorf("enforce self-quarantine: configured self_catalog.pub_key is not valid hex: %w", decErr)
			}
			if len(keyBytes) != ed25519.PublicKeySize {
				return fmt.Errorf("enforce self-quarantine: configured self_catalog.pub_key has wrong length %d (want %d bytes for Ed25519 public key) — fail closed rather than silently use embedded key",
					len(keyBytes), ed25519.PublicKeySize)
			}
			pubKeyOverride = ed25519.PublicKey(keyBytes)
		}
	}

	opts := catalog.SelfCatalogOpts{
		FeedURL:        feedURL,
		CacheDir:       catalogDir,
		StatePath:      filepath.Join(stateDir, "state.json"),
		Version:        version.Version,
		Client:         &http.Client{Timeout: 10 * time.Second},
		PubKeyOverride: pubKeyOverride, // nil when not configured (uses embedded key)
	}

	result := checkSelfCatalogFn(opts)

	switch result.Outcome {
	case catalog.SelfCatalogContinue:
		// Normal operation.
		return nil

	case catalog.SelfCatalogWarnContinue:
		// Network error with no usable cache — warn but continue (Pitfall 2).
		fmt.Fprintf(cmd.ErrOrStderr(),
			"WARNING: beekeeper-self catalog check failed (network error): %v\n"+
				"Continuing in degraded mode — could not verify binary integrity.\n",
			result.Err)
		return nil

	case catalog.SelfCatalogQuarantine:
		// Version match — refuse to run.
		entry := result.MatchedEntry
		entryID := ""
		reason := "unknown"
		if entry != nil {
			entryID = entry.ID
			reason = entry.Name
		}

		writeQuarantineAuditRecord(stateDir, entryID, reason)

		fmt.Fprintf(cmd.ErrOrStderr(),
			"\n*** BEEKEEPER SELF-QUARANTINE ACTIVATED ***\n\n"+
				"The running beekeeper binary (version %s) matches a known-compromised\n"+
				"version in the beekeeper-self catalog (entry: %s).\n\n"+
				"Reason: %s\n\n"+
				"Enforcement commands (check, gateway, sentry, watch, catalogs sync)\n"+
				"are BLOCKED to protect your development environment.%s",
			version.Version, entryID, reason, selfQuarantineVerificationMsg)

		return fmt.Errorf("self-quarantine: beekeeper %s is in the compromised-version list (entry: %s); enforcement blocked — see verification path above",
			version.Version, entryID)

	case catalog.SelfCatalogFailClosed:
		// Integrity failure — refuse to run.
		writeQuarantineAuditRecord(stateDir, "integrity-failure", "beekeeper-self feed signature invalid")

		fmt.Fprintf(cmd.ErrOrStderr(),
			"\n*** BEEKEEPER INTEGRITY FAILURE — FAIL CLOSED ***\n\n"+
				"The beekeeper-self catalog feed signature is INVALID.\n"+
				"This is a proven integrity failure (T-09-10), not a network error.\n\n"+
				"Error: %v\n\n"+
				"Enforcement commands are BLOCKED.%s",
			result.Err, selfQuarantineVerificationMsg)

		return fmt.Errorf("self-quarantine: beekeeper-self feed integrity failure; enforcement blocked — %w", result.Err)

	default:
		// Unknown outcome — fail closed by default (CLAUDE.md: fail closed on any
		// unexpected condition).
		return fmt.Errorf("self-quarantine: unknown outcome %v from self-catalog check; failing closed", result.Outcome)
	}
}

// writeQuarantineAuditRecord writes a self_quarantine audit record to the audit
// log. A write failure is silently ignored — the quarantine decision has already
// been made; failure only affects the forensic record (V7 best-effort).
func writeQuarantineAuditRecord(stateDir, entryID, reason string) {
	auditDir := filepath.Join(stateDir, "audit")
	auditPath := filepath.Join(auditDir, "beekeeper.ndjson")

	w, err := audit.NewWriter(auditPath)
	if err != nil {
		// Non-fatal: the quarantine is still enforced even if we cannot write.
		fmt.Fprintf(os.Stderr, "beekeeper: warning: could not open audit log for self-quarantine record: %v\n", err)
		return
	}
	defer w.Close()

	rec := audit.AuditRecord{
		RecordType:       "self_quarantine",
		RecordID:         entryID,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		ScannerName:      "beekeeper",
		Decision:         "block",
		Reason:           reason,
		RuleIDs:          []string{"CTLG-04", "SFDF-06"},
		CatalogMatches:   []audit.CatalogProvenance{},
		SourcesAgreed:    []string{},
		SourcesDissented: []string{},
		Endpoint:         "startup",
	}
	if wErr := w.Write(rec); wErr != nil {
		fmt.Fprintf(os.Stderr, "beekeeper: warning: could not write self-quarantine audit record: %v\n", wErr)
	}
}
