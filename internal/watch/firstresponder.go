package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/quarantine"
	"github.com/bantuson/beekeeper/internal/sentry"
)

// FirstResponderConfig holds all parameters for RunFirstResponder.
// The CrossRefFn field is injectable for tests; production code leaves it nil
// and RunFirstResponder substitutes scan.CrossReference.
type FirstResponderConfig struct {
	// Enabled mirrors AutoQuarantineConfig.Enabled.
	Enabled bool
	// DryRun: when true, audits findings without moving any artifact.
	DryRun bool
	// Threshold is the minimum CorroborationCount to trigger auto-quarantine.
	Threshold int
	// QuarantineDir is the root quarantine directory.
	QuarantineDir string
	// AuditPath is the NDJSON audit log path.
	AuditPath string
	// IndexPath is the beekeeper mmap catalog index (beekeeper.idx).
	IndexPath string
	// CacheDir is used for policy files and OSV/Socket cache.
	CacheDir string
	// SocketToken for Socket PURL catalog source (optional).
	SocketToken string
	// SentryTargetsPath is where sentry-targets.json is persisted.
	// May be empty (target-list recording is skipped when empty).
	SentryTargetsPath string
	// CrossRefFn is the injectable cross-reference function. Production callers
	// leave this nil and RunFirstResponder uses CrossReference (same package).
	CrossRefFn func(ctx context.Context, cfg CrossRefConfig) ([]ScanHit, error)
}

// firstResponderFn is the package-level injectable seam for cmd/beekeeper.
// Mirrors scanOnDeltaFn: production code leaves it as defaultFirstResponder;
// cmd tests can replace it with a no-op to avoid requiring a live scan binary.
var firstResponderFn = defaultFirstResponder

// defaultFirstResponder is the production implementation. It is separate from
// RunFirstResponder so the injectable var is a stable target.
func defaultFirstResponder(ctx context.Context, cfg FirstResponderConfig) error {
	return RunFirstResponder(ctx, cfg)
}

// RunFirstResponder orchestrates the scan-hit -> auto-quarantine loop:
//
//  1. Run CrossReference to get ScanHit values from the installed-package inventory.
//  2. For each hit where CorroborationCount >= Threshold AND Enabled:
//     - DryRun:          audit "would-quarantine", no move.
//     - PathResolved:    quarantine.MoveTyped the artifact, audit "catalog_quarantine".
//     - path unknown:    audit "pending-quarantine", no move.
//  3. Record each hit whose CorroborationCount >= Threshold into the Sentry
//     target list (F-4: a single-source warn-tier hit must not tighten Sentry).
//
// Fail-closed contract:
//   - A MoveTyped error logs and leaves the artifact in place (never half-deletes).
//   - A target-list save error logs and continues (best-effort; not a security gate).
//   - RunFirstResponder never returns an error from per-hit failures; errors from
//     the initial CrossReference call are propagated.
//
// Honesty invariants:
//   - Scan is READ-ONLY: never removes/disables/edits a package.
//   - Quarantine is a REVERSIBLE move (os.Rename + manifest). Purge stays human-gated.
//   - Sentry target-list is DETECTION-ONLY: no kill/isolate/network-cut.
func RunFirstResponder(ctx context.Context, cfg FirstResponderConfig) error {
	crossRef := cfg.CrossRefFn
	if crossRef == nil {
		crossRef = CrossReference
	}

	crossRefCfg := CrossRefConfig{
		IndexPath:   cfg.IndexPath,
		CacheDir:    cfg.CacheDir,
		AuditPath:   cfg.AuditPath,
		SocketToken: cfg.SocketToken,
	}

	hits, err := crossRef(ctx, crossRefCfg)
	if err != nil {
		// CrossReference failure is propagated — callers should log and continue
		// (fail-closed: a broken scan is not a license to skip quarantine checks).
		return fmt.Errorf("first-responder: cross-reference scan: %w", err)
	}

	// Load the Sentry target list (best-effort; missing file is an empty list).
	var targets *sentry.TargetList
	if cfg.SentryTargetsPath != "" {
		t, _ := sentry.LoadTargets(cfg.SentryTargetsPath)
		if t == nil {
			t = &sentry.TargetList{}
		}
		targets = t
	}

	threshold := cfg.Threshold
	if threshold <= 0 {
		threshold = 2 // hard-coded default as a safety net (config accessor enforces this)
	}

	for _, hit := range hits {
		// F-4: gate Sentry target-list recording on the same corroboration
		// discipline the move path uses. A single warn-tier (1-source) hit must
		// NOT tighten Sentry detection on a legitimate package — that would let
		// one compromised catalog source flood the victim with false-positive
		// credential alerts (the exact single-source threat corroboration is
		// meant to neutralize). Record a target ONLY when the hit is corroborated
		// to threshold, regardless of the Enabled/DryRun move gate below.
		if targets != nil && hit.CorroborationCount >= threshold {
			expectedProcess := ecosystemToProcess(hit.Ecosystem)
			targets.AddTarget(hit.Package, hit.InstalledPath, expectedProcess)
		}

		// Only act if enabled and corroboration meets threshold.
		if !cfg.Enabled || hit.CorroborationCount < threshold {
			continue
		}

		if cfg.DryRun {
			// Dry-run: audit "would-quarantine" without moving.
			writeFirstResponderAudit(cfg.AuditPath, "would-quarantine", hit)
			continue
		}

		if !hit.PathResolved || hit.InstalledPath == "" {
			// Path unknown: emit pending-quarantine audit record.
			writeFirstResponderAudit(cfg.AuditPath, "pending-quarantine", hit)
			continue
		}

		// Real quarantine: move the artifact.
		artifactType := quarantine.ArtifactTypeLanguagePackage
		if hit.Ecosystem == "editor-extension" {
			artifactType = quarantine.ArtifactTypeEditorExtension
		}

		m := quarantine.Manifest{
			Publisher:    hit.Ecosystem,
			Name:         hit.Package,
			Version:      hit.Version,
			OriginalPath: hit.InstalledPath,
			ArtifactType: artifactType,
			Reason:       fmt.Sprintf("catalog match: %d sources corroborated", hit.CorroborationCount),
			RuleIDs:      []string{"FRSP-01"},
		}

		_, moveErr := quarantine.MoveTyped(cfg.QuarantineDir, hit.InstalledPath, m)
		if moveErr != nil {
			// Fail-closed: log the error, leave artifact in place, still audit.
			log.Printf("beekeeper first-responder: quarantine move failed for %s/%s: %v (artifact left in place)", hit.Ecosystem, hit.Package, moveErr)
			writeFirstResponderAudit(cfg.AuditPath, "quarantine_error", hit)
			continue
		}

		writeFirstResponderAudit(cfg.AuditPath, "catalog_quarantine", hit)
	}

	// Persist the updated target list (best-effort).
	if targets != nil && cfg.SentryTargetsPath != "" {
		if saveErr := sentry.SaveTargets(cfg.SentryTargetsPath, targets); saveErr != nil {
			log.Printf("beekeeper first-responder: save sentry targets failed: %v", saveErr)
		}
	}

	return nil
}

// writeFirstResponderAudit appends a FRSP-01 audit record to the audit log.
// The record_type is one of: "would-quarantine", "catalog_quarantine",
// "pending-quarantine", "quarantine_error".
// Errors are logged but do not interrupt the first-responder loop.
func writeFirstResponderAudit(auditPath, recordType string, hit ScanHit) {
	if auditPath == "" {
		return
	}

	tc := policy.ToolCall{
		ToolName: hit.Package,
		ToolInput: map[string]any{
			"ecosystem": hit.Ecosystem,
			"package":   hit.Package,
			"version":   hit.Version,
		},
	}

	rec := audit.FromDecision(tc, hit.Decision, generateRecordID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
	rec.RecordType = recordType
	if !containsRuleID(rec.RuleIDs, "FRSP-01") {
		rec.RuleIDs = append([]string{"FRSP-01"}, rec.RuleIDs...)
	}

	// F-1 (TM-D-03): redact before write, matching the check + watch handler
	// discipline. hit.Decision.Reason and CatalogMatches[].Package/EntryID carry
	// attacker-influenced strings that must not reach the audit log verbatim.
	rec = audit.RedactRecord(rec, audit.DefaultRedactPatterns())

	if w, wErr := audit.NewWriter(auditPath); wErr == nil {
		if err := w.Write(rec); err != nil {
			log.Printf("beekeeper first-responder: write audit record failed: %v", err)
		}
		w.Close()
	}
}

// ecosystemToProcess returns a best-effort expected process name for a given
// package ecosystem. This is used to populate the Sentry target list so the
// correlation engine can tighten thresholds on processes that might execute
// the flagged package.
func ecosystemToProcess(ecosystem string) string {
	switch strings.ToLower(ecosystem) {
	case "npm", "yarn", "pnpm":
		return "node"
	case "pip", "pypi":
		return "python"
	case "cargo":
		return "cargo"
	case "go":
		return "go"
	case "rubygems":
		return "ruby"
	case "packagist":
		return "php"
	default:
		return ""
	}
}

// firstResponderTargetListJSON is a minimal JSON serialisation helper used by
// tests that verify the target-list file contents.
type firstResponderTargetListJSON struct {
	Targets []struct {
		Name            string `json:"name"`
		Path            string `json:"path"`
		ExpectedProcess string `json:"expected_process"`
	} `json:"targets"`
}

// marshalTargetListJSON is exposed for test assertions.
// Not used in the hot path — callers use sentry.LoadTargets/SaveTargets.
func marshalTargetListJSON(tl *sentry.TargetList) ([]byte, error) {
	return json.MarshalIndent(tl, "", "  ")
}

// Ensure the helpers compile (avoids unused import warning).
var _ = marshalTargetListJSON
var _ = firstResponderTargetListJSON{}

// Suppress unused import warnings for os and filepath used only in production
// paths exercised at runtime, not in tests.
var _ = filepath.Join
var _ = os.Stat
