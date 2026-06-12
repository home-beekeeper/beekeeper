package scan

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/bantuson/beekeeper/internal/audit"
	"github.com/bantuson/beekeeper/internal/catalog"
	"github.com/bantuson/beekeeper/internal/policy"
	"github.com/bantuson/beekeeper/internal/policyloader"
)

// ScanHit is a single finding produced by the cross-reference scanner: an
// installed language package that matches the threat catalog.
//
// Honesty invariant: CrossReference NEVER removes, disables, or edits a
// package. It only reads the pollen inventory and reads the catalog index.
type ScanHit struct {
	// Ecosystem is the package ecosystem (npm, pip, cargo, etc.).
	Ecosystem string
	// Package is the package name as reported by pollen.
	Package string
	// Version is the installed version.
	Version string
	// InstalledPath is the best-effort resolved on-disk path from the pollen
	// record (project_path field). Empty when the path cannot be resolved.
	InstalledPath string
	// PathResolved reports whether InstalledPath was populated from the scan record.
	// When false, the consumer should emit a pending-quarantine incident rather
	// than guessing a path.
	PathResolved bool
	// Decision is the policy decision from policy.Evaluate.
	Decision policy.Decision
	// CorroborationCount is the number of distinct SIGNED catalog sources that
	// matched. Consumed by the first-responder to gate auto-quarantine.
	CorroborationCount int
	// RuleIDs from the decision.
	RuleIDs []string
}

// CrossRefConfig holds parameters for a CrossReference call.
type CrossRefConfig struct {
	// IndexPath is the path to the bumblebee mmap index (beekeeper.idx).
	IndexPath string
	// CacheDir is used for policy files and OSV/Socket cache.
	CacheDir string
	// AuditPath is the NDJSON audit log path. May be empty (no audit writes).
	AuditPath string
	// SocketToken is optional; enables the Socket PURL catalog source.
	SocketToken string
}

// pollenPackageRecord is the minimal structure we need from a pollen "package"
// NDJSON record to build a policy.ToolCall for cross-referencing.
type pollenPackageRecord struct {
	RecordType     string `json:"record_type"`
	Ecosystem      string `json:"ecosystem"`
	NormalizedName string `json:"normalized_name"`
	Version        string `json:"version"`
	ProjectPath    string `json:"project_path,omitempty"`
}

// CrossReference reads the Pollen/Bumblebee "package" inventory records
// (via the injectable runPollenFn) and, for each installed package, evaluates
// it against the threat catalog via policy.Evaluate. Each match that reaches
// at least the warn tier emits a ScanHit.
//
// Honesty invariants:
//   - READ-ONLY: this function never removes, disables, or edits a package.
//   - Audit writes are append-only to the audit log (not the package itself).
//   - When the on-disk path is not resolvable from the scan record,
//     InstalledPath is "" and PathResolved is false.
func CrossReference(ctx context.Context, cfg CrossRefConfig) ([]ScanHit, error) {
	// Collect all "package" type records from the pollen stream.
	var pkgRecords []pollenPackageRecord

	ch, ok := runPollenFn(ctx, false /* shallow scan; no --root needed for inventory */)
	if ok {
		for line := range ch {
			if len(line) == 0 {
				continue
			}
			// Filter to "package" record_type only.
			var probe struct {
				RecordType string `json:"record_type"`
			}
			if err := json.Unmarshal(line, &probe); err != nil {
				continue
			}
			if probe.RecordType != "package" {
				continue
			}
			var rec pollenPackageRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				continue
			}
			if rec.NormalizedName == "" || rec.Ecosystem == "" {
				continue
			}
			pkgRecords = append(pkgRecords, rec)
		}
	}
	// If pollen is unavailable, pkgRecords is empty — return no hits (graceful degrade).

	if len(pkgRecords) == 0 {
		return nil, nil
	}

	// Open the mmap index; if unavailable, proceed with nil (OSV/Socket only).
	var bbIdx *catalog.Index
	if cfg.IndexPath != "" {
		if idx, err := catalog.OpenIndex(cfg.IndexPath); err == nil {
			bbIdx = idx
			defer bbIdx.Close()
		}
		// Index unavailable is non-fatal; we continue with OSV/Socket only.
	}

	// Load policy overlay files and derive corroboration thresholds.
	var policyFiles []policyloader.PolicyFile
	if cfg.CacheDir != "" {
		policiesDir := filepath.Join(filepath.Dir(cfg.CacheDir), "policies")
		policyFiles, _ = policyloader.LoadPolicyDir(policiesDir)
	}
	thresholds := policyloader.ThresholdsFromPolicyFiles(policyFiles)
	thresholds.CatalogHealthy = resolveCatalogHealthy(cfg.CacheDir)

	var hits []ScanHit

	for _, rec := range pkgRecords {
		// Build adapters (no socket for cross-reference; can be extended later).
		multiIdx := catalog.NewMultiIndex(bbIdx, nil, nil)

		tc := policy.ToolCall{
			ToolName: "scan",
			ToolInput: map[string]any{
				"ecosystem": rec.Ecosystem,
				"package":   rec.NormalizedName,
				"version":   rec.Version,
			},
		}
		decision := policy.Evaluate(tc, multiIdx, thresholds, policy.AgentContext{})

		// Apply policy overlay.
		if len(policyFiles) > 0 {
			decision = policyloader.ApplyPolicyOverlay(policyFiles, tc, decision)
		}

		// Only emit a ScanHit for packages that have at least one catalog match
		// (warn or block). Allow decisions with 0 corroboration are skipped.
		if decision.Allow && decision.CorroborationCount == 0 {
			continue
		}

		// Resolve path best-effort from the project_path field.
		installedPath := ""
		pathResolved := false
		if rec.ProjectPath != "" {
			installedPath = rec.ProjectPath
			pathResolved = true
		}

		hit := ScanHit{
			Ecosystem:          rec.Ecosystem,
			Package:            rec.NormalizedName,
			Version:            rec.Version,
			InstalledPath:      installedPath,
			PathResolved:       pathResolved,
			Decision:           decision,
			CorroborationCount: decision.CorroborationCount,
			RuleIDs:            decision.RuleIDs,
		}
		hits = append(hits, hit)

		// Audit the finding as a "finding" record — read-only, no package mutation.
		if cfg.AuditPath != "" {
			auditRec := audit.FromDecision(tc, decision, generateScanID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
			auditRec.RecordType = "finding"
			if w, err := audit.NewWriter(cfg.AuditPath); err == nil {
				_ = w.Write(auditRec)
				w.Close()
			}
		}
	}

	return hits, nil
}
