package nudge

import "testing"

// TestDefaultConfig verifies PRD §5.1 defaults are correct.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("DefaultConfig().Enabled = false, want true")
	}
	if cfg.Mode != "soft" {
		t.Errorf("DefaultConfig().Mode = %q, want %q", cfg.Mode, "soft")
	}
	if cfg.RequireHardened {
		t.Error("DefaultConfig().RequireHardened = true, want false")
	}
	if cfg.Preferred != "pnpm" {
		t.Errorf("DefaultConfig().Preferred = %q, want %q", cfg.Preferred, "pnpm")
	}
	if !cfg.CheckSocketScanner {
		t.Error("DefaultConfig().CheckSocketScanner = false, want true")
	}
	if !cfg.MajorDriftCheck.Enabled {
		t.Error("DefaultConfig().MajorDriftCheck.Enabled = false, want true")
	}
	if cfg.MajorDriftCheck.Interval != "168h" {
		t.Errorf("DefaultConfig().MajorDriftCheck.Interval = %q, want %q", cfg.MajorDriftCheck.Interval, "168h")
	}
	if cfg.VersionFloors.Pnpm != "11.0.0" {
		t.Errorf("DefaultConfig().VersionFloors.Pnpm = %q, want %q", cfg.VersionFloors.Pnpm, "11.0.0")
	}
	if cfg.VersionFloors.Bun != "1.3.0" {
		t.Errorf("DefaultConfig().VersionFloors.Bun = %q, want %q", cfg.VersionFloors.Bun, "1.3.0")
	}
	if cfg.VersionFloors.Node != "22.0.0" {
		t.Errorf("DefaultConfig().VersionFloors.Node = %q, want %q", cfg.VersionFloors.Node, "22.0.0")
	}
}

// TestMinimumReleaseAgeWeaknessBaselineConst verifies the Flag 5 correction:
// the hardening-weakness baseline must be 1440, NOT 60.
func TestMinimumReleaseAgeWeaknessBaselineConst(t *testing.T) {
	if minimumReleaseAgeWeaknessBaseline != 1440 {
		t.Errorf("minimumReleaseAgeWeaknessBaseline = %d, want 1440 (Flag 5 correction)", minimumReleaseAgeWeaknessBaseline)
	}
}

// TestConfigFrom covers the full config mapper (Task 4 below, but stub here for Task 1).
func TestConfigFrom(t *testing.T) {
	cfg := ConfigFrom(true, "soft", "pnpm", true, "11.0.0", "1.3.0", "22.0.0", true, "168h")

	if !cfg.Enabled {
		t.Error("ConfigFrom: Enabled = false, want true")
	}
	if cfg.Mode != "soft" {
		t.Errorf("ConfigFrom: Mode = %q, want %q", cfg.Mode, "soft")
	}
	if cfg.Preferred != "pnpm" {
		t.Errorf("ConfigFrom: Preferred = %q, want %q", cfg.Preferred, "pnpm")
	}
	if !cfg.CheckSocketScanner {
		t.Error("ConfigFrom: CheckSocketScanner = false, want true")
	}
	if cfg.VersionFloors.Pnpm != "11.0.0" {
		t.Errorf("ConfigFrom: VersionFloors.Pnpm = %q, want %q", cfg.VersionFloors.Pnpm, "11.0.0")
	}
	if cfg.VersionFloors.Bun != "1.3.0" {
		t.Errorf("ConfigFrom: VersionFloors.Bun = %q, want %q", cfg.VersionFloors.Bun, "1.3.0")
	}
	if cfg.VersionFloors.Node != "22.0.0" {
		t.Errorf("ConfigFrom: VersionFloors.Node = %q, want %q", cfg.VersionFloors.Node, "22.0.0")
	}
	if !cfg.MajorDriftCheck.Enabled {
		t.Error("ConfigFrom: MajorDriftCheck.Enabled = false, want true")
	}
	if cfg.MajorDriftCheck.Interval != "168h" {
		t.Errorf("ConfigFrom: MajorDriftCheck.Interval = %q, want %q", cfg.MajorDriftCheck.Interval, "168h")
	}

	// Layered disable passes through faithfully (NUDGE-08).
	cfgOff := ConfigFrom(false, "soft", "pnpm", true, "11.0.0", "1.3.0", "22.0.0", false, "168h")
	if cfgOff.Enabled {
		t.Error("ConfigFrom(enabled=false): Enabled = true, want false (layered disable must pass through)")
	}
	if cfgOff.MajorDriftCheck.Enabled {
		t.Error("ConfigFrom(driftEnabled=false): MajorDriftCheck.Enabled = true, want false")
	}

	// Empty floors fall back to DefaultConfig() floors.
	cfgEmpty := ConfigFrom(true, "", "", true, "", "", "", true, "")
	def := DefaultConfig()
	if cfgEmpty.VersionFloors.Pnpm != def.VersionFloors.Pnpm {
		t.Errorf("ConfigFrom(emptyFloors): Pnpm = %q, want %q (fallback to default)", cfgEmpty.VersionFloors.Pnpm, def.VersionFloors.Pnpm)
	}
	if cfgEmpty.VersionFloors.Bun != def.VersionFloors.Bun {
		t.Errorf("ConfigFrom(emptyFloors): Bun = %q, want %q", cfgEmpty.VersionFloors.Bun, def.VersionFloors.Bun)
	}
	if cfgEmpty.VersionFloors.Node != def.VersionFloors.Node {
		t.Errorf("ConfigFrom(emptyFloors): Node = %q, want %q", cfgEmpty.VersionFloors.Node, def.VersionFloors.Node)
	}
	if cfgEmpty.Mode != def.Mode {
		t.Errorf("ConfigFrom(emptyMode): Mode = %q, want %q", cfgEmpty.Mode, def.Mode)
	}
	if cfgEmpty.Preferred != def.Preferred {
		t.Errorf("ConfigFrom(emptyPreferred): Preferred = %q, want %q", cfgEmpty.Preferred, def.Preferred)
	}
	if cfgEmpty.MajorDriftCheck.Interval != def.MajorDriftCheck.Interval {
		t.Errorf("ConfigFrom(emptyInterval): Interval = %q, want %q", cfgEmpty.MajorDriftCheck.Interval, def.MajorDriftCheck.Interval)
	}
}
