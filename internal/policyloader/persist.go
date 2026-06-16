// Package policyloader — persist.go
//
// Persistence primitives for policy files edited interactively (e.g. by the
// Beekeeper TUI dashboard). All policy-file I/O lives in this package
// (architecture constraint); the TUI never writes policy files directly.
//
// The central guarantee is the "last gate": SavePolicyFile validates a file via
// ValidateForPersist BEFORE writing, and writes NOTHING when validation fails.
// An interactive editor that persists exclusively through SavePolicyFile can
// therefore never put a policy file on disk that beekeeper check would later
// reject or clamp — the TUI becomes a true source of truth, not a cosmetic one.
package policyloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/home-beekeeper/beekeeper/internal/policy"
)

// ManagedPolicyName is the filename of the policy file owned and edited by the
// Beekeeper dashboard. It is a valid typed PolicyFile (schema_version "1"), so
// LoadPolicyDir loads and enforces it exactly like any other policy file —
// unlike the retired prototype tui_rules.json, whose foreign schema the engine
// silently skipped (making TUI edits cosmetic).
const ManagedPolicyName = "beekeeper-tui.json"

// legacyTUIRulesName is the retired prototype file. SavePolicyFile best-effort
// removes it once a real managed file is written, so it no longer sits in the
// enforced policies/ dir triggering "skipping invalid policy file" warnings.
const legacyTUIRulesName = "tui_rules.json"

// ManagedPolicyPath returns the path of the dashboard-managed policy file inside
// policiesDir.
func ManagedPolicyPath(policiesDir string) string {
	return filepath.Join(policiesDir, ManagedPolicyName)
}

// DefaultManagedPolicy builds the seed managed policy file: a single
// corroboration_threshold rule populated from policy.DefaultCorroborationThresholds().
// Seeding with the real engine defaults means the dashboard displays accurate,
// enforced values and a fresh install behaves identically with or without the file.
func DefaultManagedPolicy() PolicyFile {
	d := policy.DefaultCorroborationThresholds()
	critical := 0
	if ov, ok := d.SeverityOverrides["critical"]; ok {
		critical = ov.BlockAt
	}
	return PolicyFile{
		SchemaVersion: SupportedSchemaVersion,
		Name:          "beekeeper-tui",
		Description:   "Policy rules managed by the Beekeeper dashboard (beekeeper dashboard --admin).",
		Rules: []PolicyRule{
			{
				ID:              "tui-corroboration",
				RuleType:        "corroboration_threshold",
				WarnAt:          d.WarnAt,
				BlockAt:         d.BlockAt,
				QuarantineAt:    d.QuarantineAt,
				CriticalBlockAt: critical,
			},
		},
	}
}

// ValidateForPersist is the complete pre-write validation gate for a policy file
// edited interactively. It runs ValidateSchema (enum/structural checks) AND the
// corroboration threshold-ordering checks that ValidateSchema defers to eval time
// (warn_at <= block_at < quarantine_at, plus per-severity bounds). This closes
// the gap where a file passes ValidateSchema but would be rejected/clamped by the
// engine's validateCorroborationThresholds at evaluation. Returns all errors;
// empty means the file is safe to persist.
func ValidateForPersist(pf PolicyFile) []error {
	errs := ValidateSchema(pf)
	// Derive the effective thresholds the engine would compute, then bound-check
	// the ordering the schema validator could not (it lacks the merged view).
	t := ThresholdsFromPolicyFiles([]PolicyFile{pf})
	errs = append(errs, validateThresholdOrdering(t)...)
	return errs
}

// validateThresholdOrdering enforces the corroboration ordering invariants on the
// derived thresholds, mirroring the engine's eval-time sanity bounds (the
// SeverityThreshold doc contract in internal/policy/types.go).
func validateThresholdOrdering(t policy.CorroborationThresholds) []error {
	var errs []error
	if t.WarnAt < 1 {
		errs = append(errs, fmt.Errorf("corroboration warn_at (%d) must be >= 1", t.WarnAt))
	}
	if t.BlockAt < t.WarnAt {
		errs = append(errs, fmt.Errorf("corroboration block_at (%d) must be >= warn_at (%d)", t.BlockAt, t.WarnAt))
	}
	if t.QuarantineAt <= t.BlockAt {
		errs = append(errs, fmt.Errorf("corroboration quarantine_at (%d) must be > block_at (%d)", t.QuarantineAt, t.BlockAt))
	}
	for sev, ov := range t.SeverityOverrides {
		if ov.BlockAt < 1 {
			errs = append(errs, fmt.Errorf("corroboration %s block_at (%d) must be >= 1", sev, ov.BlockAt))
		}
		if ov.BlockAt > t.BlockAt {
			errs = append(errs, fmt.Errorf("corroboration %s block_at (%d) must be <= global block_at (%d)", sev, ov.BlockAt, t.BlockAt))
		}
		if ov.QuarantineAt < ov.BlockAt {
			errs = append(errs, fmt.Errorf("corroboration %s quarantine_at (%d) must be >= block_at (%d)", sev, ov.QuarantineAt, ov.BlockAt))
		}
	}
	return errs
}

// SavePolicyFile validates pf via ValidateForPersist and, only if valid, writes
// it atomically to path (MkdirAll 0700, temp file + rename, 0600). On any
// validation error it returns the errors and leaves the existing file on disk
// UNCHANGED. This is the last gate: a caller can never persist a file the engine
// would reject.
func SavePolicyFile(path string, pf PolicyFile) []error {
	if errs := ValidateForPersist(pf); len(errs) > 0 {
		return errs
	}
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return []error{fmt.Errorf("marshal policy file: %w", err)}
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return []error{fmt.Errorf("create policy dir: %w", err)}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return []error{fmt.Errorf("write temp policy file: %w", err)}
	}
	if err := os.Rename(tmp, path); err != nil {
		// Clean up the orphaned temp file; ignore removal error.
		_ = os.Remove(tmp)
		return []error{fmt.Errorf("rename policy file: %w", err)}
	}
	// Best-effort retire the prototype file so it stops polluting the enforced dir.
	_ = os.Remove(filepath.Join(filepath.Dir(path), legacyTUIRulesName))
	return nil
}

// LoadOrSeedManagedPolicy returns the dashboard-managed policy file from
// policiesDir. If absent it seeds a default (DefaultManagedPolicy) and persists
// it, so the dashboard always shows real enforced values. Returns the
// loaded/seeded file plus any errors: a load error for a present-but-invalid
// managed file, or a seed-write error (in which case the in-memory default is
// still returned for display — fail-soft, mirroring the engine's tolerance).
func LoadOrSeedManagedPolicy(policiesDir string) (PolicyFile, []error) {
	path := ManagedPolicyPath(policiesDir)
	if _, err := os.Stat(path); err == nil {
		pf, errs := LoadPolicyFile(path)
		if len(errs) > 0 {
			return PolicyFile{}, errs
		}
		return pf, nil
	}
	pf := DefaultManagedPolicy()
	if errs := SavePolicyFile(path, pf); len(errs) > 0 {
		return pf, errs
	}
	return pf, nil
}
