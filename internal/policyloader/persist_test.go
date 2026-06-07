package policyloader

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultManagedPolicyIsValid verifies the seed file passes the full persist
// gate and carries the engine default corroboration thresholds.
func TestDefaultManagedPolicyIsValid(t *testing.T) {
	pf := DefaultManagedPolicy()
	if errs := ValidateForPersist(pf); len(errs) > 0 {
		t.Fatalf("default managed policy must be valid, got: %v", errs)
	}
	if pf.SchemaVersion != SupportedSchemaVersion {
		t.Errorf("schema_version = %q, want %q", pf.SchemaVersion, SupportedSchemaVersion)
	}
	if len(pf.Rules) != 1 || pf.Rules[0].RuleType != "corroboration_threshold" {
		t.Fatalf("expected one corroboration_threshold rule, got %+v", pf.Rules)
	}
	r := pf.Rules[0]
	if r.WarnAt != 1 || r.BlockAt != 2 || r.QuarantineAt != 3 {
		t.Errorf("seed thresholds = warn %d/block %d/quar %d, want 1/2/3", r.WarnAt, r.BlockAt, r.QuarantineAt)
	}
}

// TestSavePolicyFileRoundTrip verifies a valid file is written atomically and
// reloads cleanly via the same loader beekeeper check uses.
func TestSavePolicyFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := ManagedPolicyPath(dir)

	pf := DefaultManagedPolicy()
	pf.Rules[0].BlockAt = 1 // a real edit (tighten enforcement)
	if errs := SavePolicyFile(path, pf); len(errs) > 0 {
		t.Fatalf("SavePolicyFile(valid) returned errors: %v", errs)
	}

	loaded, errs := LoadPolicyFile(path)
	if len(errs) > 0 {
		t.Fatalf("LoadPolicyFile after save returned errors: %v", errs)
	}
	if loaded.Name != pf.Name || len(loaded.Rules) != 1 || loaded.Rules[0].BlockAt != 1 {
		t.Errorf("round-trip mismatch: got %+v", loaded)
	}

	// The change must be visible to the enforcement path's threshold derivation.
	th := ThresholdsFromPolicyFiles([]PolicyFile{loaded})
	if th.BlockAt != 1 {
		t.Errorf("ThresholdsFromPolicyFiles.BlockAt = %d, want 1 (edit not enforced)", th.BlockAt)
	}
}

// TestSavePolicyFileRejectsInvalidOrdering proves the last gate: an edit that
// violates threshold ordering is rejected and NOTHING is written.
func TestSavePolicyFileRejectsInvalidOrdering(t *testing.T) {
	dir := t.TempDir()
	path := ManagedPolicyPath(dir)

	pf := DefaultManagedPolicy()
	pf.Rules[0].WarnAt = 3 // warn(3) > block(2) — invalid

	errs := SavePolicyFile(path, pf)
	if len(errs) == 0 {
		t.Fatal("SavePolicyFile must reject warn_at > block_at")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("rejected save must not create the file; stat err = %v", err)
	}
}

// TestSavePolicyFileRejectsBadEnum proves ValidateSchema is part of the gate.
func TestSavePolicyFileRejectsBadEnum(t *testing.T) {
	dir := t.TempDir()
	path := ManagedPolicyPath(dir)

	pf := DefaultManagedPolicy()
	pf.Rules = append(pf.Rules, PolicyRule{ID: "bad", RuleType: "totally_bogus"})

	if errs := SavePolicyFile(path, pf); len(errs) == 0 {
		t.Fatal("SavePolicyFile must reject an unknown rule_type")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("rejected save must not create the file; stat err = %v", err)
	}
}

// TestSavePolicyFileLeavesExistingUnchanged proves a rejected edit can't corrupt
// or truncate an already-valid file on disk (the core "can't break anything").
func TestSavePolicyFileLeavesExistingUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := ManagedPolicyPath(dir)

	good := DefaultManagedPolicy()
	if errs := SavePolicyFile(path, good); len(errs) > 0 {
		t.Fatalf("seed save failed: %v", errs)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after seed: %v", err)
	}

	bad := DefaultManagedPolicy()
	bad.Rules[0].CriticalBlockAt = 99 // critical(99) > block(2) — invalid
	if errs := SavePolicyFile(path, bad); len(errs) == 0 {
		t.Fatal("expected rejection for critical_block_at > block_at")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after rejected save: %v", err)
	}
	if string(after) != string(before) {
		t.Error("rejected save must not modify the existing file on disk")
	}
}

// TestLoadOrSeedManagedPolicy seeds on first call and loads thereafter.
func TestLoadOrSeedManagedPolicy(t *testing.T) {
	dir := t.TempDir()
	path := ManagedPolicyPath(dir)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("managed file should not exist before first load")
	}

	pf, errs := LoadOrSeedManagedPolicy(dir)
	if len(errs) > 0 {
		t.Fatalf("first LoadOrSeed returned errors: %v", errs)
	}
	if len(pf.Rules) != 1 || pf.Rules[0].RuleType != "corroboration_threshold" {
		t.Fatalf("seeded file shape unexpected: %+v", pf.Rules)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("seed must persist the file: %v", err)
	}

	// Second call loads the persisted file (and it is a valid, enforceable file).
	again, errs := LoadOrSeedManagedPolicy(dir)
	if len(errs) > 0 {
		t.Fatalf("second LoadOrSeed returned errors: %v", errs)
	}
	if again.Name != pf.Name {
		t.Errorf("reload name = %q, want %q", again.Name, pf.Name)
	}
}

// TestSavePolicyFileRetiresLegacyFile proves the prototype tui_rules.json is
// removed once a real managed file is written (no more enforced-dir pollution).
func TestSavePolicyFileRetiresLegacyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, legacyTUIRulesName)
	if err := os.WriteFile(legacy, []byte(`[{"id":"x","enabled":true}]`), 0o600); err != nil {
		t.Fatal(err)
	}

	if errs := SavePolicyFile(ManagedPolicyPath(dir), DefaultManagedPolicy()); len(errs) > 0 {
		t.Fatalf("save failed: %v", errs)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy tui_rules.json should be retired after a managed write; stat err = %v", err)
	}
}
