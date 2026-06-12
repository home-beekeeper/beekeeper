package watch

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os/exec"
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

// crossRefPollenFn is the injectable pollen runner for the cross-reference path.
// It mirrors scan.runPollenFn and is replaced in tests.
// Returns (channel, true) when the scanner is available, (nil, false) otherwise.
var crossRefPollenFn = func(ctx context.Context, deep bool) (<-chan []byte, bool) {
	return defaultRunPollenForCrossRef(ctx, deep)
}

// defaultRunPollenForCrossRef is the production pollen runner for cross-reference.
// It resolves the scanner binary exactly as scan.defaultRunPollen does, but is
// defined here to avoid an import of internal/scan (which imports internal/watch,
// creating a cycle).
func defaultRunPollenForCrossRef(ctx context.Context, deep bool) (<-chan []byte, bool) {
	bin, _, err := resolveCrossRefScanner()
	if err != nil {
		return nil, false
	}
	args := []string{"scan"}
	if deep {
		args = append(args, "--profile", "deep")
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, false
	}
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	ch := make(chan []byte, 64)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			line := sc.Bytes()
			out := make([]byte, len(line))
			copy(out, line)
			ch <- out
		}
		_ = cmd.Wait()
	}()
	return ch, true
}

// resolveCrossRefScanner resolves the inventory scanner binary (bumblebee preferred,
// pollen fallback). Mirrors scan.resolveScannerBinary.
func resolveCrossRefScanner() (path, name string, err error) {
	for _, candidate := range []string{"bumblebee", "pollen"} {
		if p, lerr := exec.LookPath(candidate); lerr == nil {
			return p, candidate, nil
		}
	}
	return "", "", exec.ErrNotFound
}

// CrossReference reads the Pollen/Bumblebee "package" inventory records
// (via the injectable crossRefPollenFn) and, for each installed package, evaluates
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

	ch, ok := crossRefPollenFn(ctx, false /* shallow scan for inventory */)
	if ok {
		for line := range ch {
			if len(line) == 0 {
				continue
			}
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
	// Pollen unavailable: pkgRecords is empty — return no hits (graceful degrade).

	if len(pkgRecords) == 0 {
		return nil, nil
	}

	// Open the mmap index; if unavailable, proceed with nil.
	var bbIdx *catalog.Index
	if cfg.IndexPath != "" {
		if idx, err := catalog.OpenIndex(cfg.IndexPath); err == nil {
			bbIdx = idx
			defer bbIdx.Close()
		}
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

	// F-7: construct the MultiIndex ONCE per pass — it was previously rebuilt
	// identically on every iteration (bbIdx/nil/nil never change within a pass),
	// which is pure allocation churn on the catalog-watch goroutine.
	multiIdx := catalog.NewMultiIndex(bbIdx, nil, nil)

	// F-7: open a single audit Writer for the whole pass (deferred Close) instead
	// of open/close per finding. A nil writer (open failed or AuditPath empty)
	// disables audit writes without affecting hit detection.
	var auditWriter *audit.Writer
	if cfg.AuditPath != "" {
		if w, werr := audit.NewWriter(cfg.AuditPath); werr == nil {
			auditWriter = w
			defer auditWriter.Close()
		}
	}

	// F-7: cap the number of findings audited per pass so a catalog delta that
	// matches a very large inventory cannot stall the watch loop with unbounded
	// file writes. Detection (the returned hits slice) is NOT capped; only the
	// per-finding audit write is. Truncation is logged (no silent cap).
	const maxAuditedFindings = 1000
	auditedFindings := 0
	auditTruncated := false

	for _, rec := range pkgRecords {
		tc := policy.ToolCall{
			ToolName: "scan",
			ToolInput: map[string]any{
				"ecosystem": rec.Ecosystem,
				"package":   rec.NormalizedName,
				"version":   rec.Version,
			},
		}
		decision := policy.Evaluate(tc, multiIdx, thresholds, policy.AgentContext{})

		if len(policyFiles) > 0 {
			decision = policyloader.ApplyPolicyOverlay(policyFiles, tc, decision)
		}

		// Only emit a ScanHit for packages with at least one catalog match.
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
		if auditWriter != nil && auditedFindings < maxAuditedFindings {
			auditRec := audit.FromDecision(tc, decision, generateRecordID(), time.Now().UTC().Format(time.RFC3339), policy.AgentContext{})
			auditRec.RecordType = "finding"
			// F-1 (TM-D-03): route the record through the redaction chokepoint
			// before writing, exactly as the check + watch handlers do. The
			// finding record carries attacker-influenced strings (decision.Reason,
			// CatalogMatches[].Package/EntryID) that must never reach the on-disk
			// or remote-sink audit log verbatim.
			auditRec = audit.RedactRecord(auditRec, audit.DefaultRedactPatterns())
			_ = auditWriter.Write(auditRec)
			auditedFindings++
		} else if auditWriter != nil && auditedFindings >= maxAuditedFindings {
			auditTruncated = true
		}
	}

	// F-7: surface the cap so a truncated audit is never silent (no-silent-cap rule).
	if auditTruncated {
		log.Printf("beekeeper cross-reference: audited findings capped at %d this pass; remaining matches detected but not written to the audit log", maxAuditedFindings)
	}

	return hits, nil
}
