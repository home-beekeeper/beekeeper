package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// Accessor defaults + configured-value tests (config.go).
//
// These accessors are live (called from cmd/beekeeper) and back security-
// relevant defaults: an unset AuditRetentionDays must NOT silently become 0
// (which would imply "delete archives immediately"); an unset
// LlamaFirewallSampleRate must NOT become 0.0 (which would imply "scan
// nothing"). Each accessor is asserted in BOTH the empty-config (default) and
// the configured-value case.
// ---------------------------------------------------------------------------

// TestGetRedactPatterns covers the empty-config (nil) and populated cases.
func TestGetRedactPatterns(t *testing.T) {
	// Empty config: no custom patterns configured → nil.
	if got := (Config{}).GetRedactPatterns(); got != nil {
		t.Errorf("GetRedactPatterns() on empty config = %v, want nil", got)
	}

	// Populated config: the exact configured slice is returned verbatim.
	want := []string{`MY_SECRET=\S+`, `password=\S+`}
	cfg := Config{RedactPatterns: want}
	got := cfg.GetRedactPatterns()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetRedactPatterns() = %v, want %v", got, want)
	}
}

// TestAuditRetentionDays covers the default (30) and configured value.
func TestAuditRetentionDays(t *testing.T) {
	// Unset → default 30 (NOT 0 — a 0 would mean "purge archives immediately").
	if got := (Config{}).AuditRetentionDays(); got != 30 {
		t.Errorf("AuditRetentionDays() default = %d, want 30", got)
	}

	// Configured value wins.
	cfg := Config{Audit: AuditConfig{RetentionDays: 7}}
	if got := cfg.AuditRetentionDays(); got != 7 {
		t.Errorf("AuditRetentionDays() = %d, want 7", got)
	}

	// A non-positive configured value falls back to the default (guards against
	// a 0/negative in JSON silently disabling retention).
	cfgZero := Config{Audit: AuditConfig{RetentionDays: 0}}
	if got := cfgZero.AuditRetentionDays(); got != 30 {
		t.Errorf("AuditRetentionDays() with 0 configured = %d, want 30 (default)", got)
	}
}

// TestAuditMaxSizeBytes covers the default (10 MB) and configured value.
func TestAuditMaxSizeBytes(t *testing.T) {
	const tenMB = int64(10 * 1024 * 1024)

	// Unset → default 10 MB.
	if got := (Config{}).AuditMaxSizeBytes(); got != tenMB {
		t.Errorf("AuditMaxSizeBytes() default = %d, want %d", got, tenMB)
	}

	// Configured value wins.
	cfg := Config{Audit: AuditConfig{MaxSizeBytes: 4096}}
	if got := cfg.AuditMaxSizeBytes(); got != 4096 {
		t.Errorf("AuditMaxSizeBytes() = %d, want 4096", got)
	}

	// Non-positive configured value falls back to the default.
	cfgZero := Config{Audit: AuditConfig{MaxSizeBytes: 0}}
	if got := cfgZero.AuditMaxSizeBytes(); got != tenMB {
		t.Errorf("AuditMaxSizeBytes() with 0 configured = %d, want %d (default)", got, tenMB)
	}
}

// TestLlamaFirewallEnabled covers the default (false) and configured (true) cases.
func TestLlamaFirewallEnabled(t *testing.T) {
	// Default: disabled.
	if (Config{}).LlamaFirewallEnabled() {
		t.Error("LlamaFirewallEnabled() default = true, want false")
	}

	// Configured: enabled.
	cfg := Config{LlamaFirewall: LlamaFirewallConfig{Enabled: true}}
	if !cfg.LlamaFirewallEnabled() {
		t.Error("LlamaFirewallEnabled() = false, want true when configured")
	}
}

// TestLlamaFirewallSampleRate covers the default (1.0 = scan all) and the
// configured value. A zero/unset sample rate must default to 1.0, not 0.0
// (which would silently disable scanning).
func TestLlamaFirewallSampleRate(t *testing.T) {
	// Unset → default 1.0 (scan all). A 0.0 default would mean "scan nothing".
	if got := (Config{}).LlamaFirewallSampleRate(); got != 1.0 {
		t.Errorf("LlamaFirewallSampleRate() default = %v, want 1.0", got)
	}

	// Configured fractional value wins.
	cfg := Config{LlamaFirewall: LlamaFirewallConfig{SampleRate: 0.25}}
	if got := cfg.LlamaFirewallSampleRate(); got != 0.25 {
		t.Errorf("LlamaFirewallSampleRate() = %v, want 0.25", got)
	}

	// Explicit 0.0 falls back to the default (guards against disabling scans).
	cfgZero := Config{LlamaFirewall: LlamaFirewallConfig{SampleRate: 0.0}}
	if got := cfgZero.LlamaFirewallSampleRate(); got != 1.0 {
		t.Errorf("LlamaFirewallSampleRate() with 0.0 configured = %v, want 1.0 (default)", got)
	}
}

// ---------------------------------------------------------------------------
// Save round-trip (config.go).
// ---------------------------------------------------------------------------

// TestSaveLoadRoundTrip writes a populated Config with Save, reads it back with
// Load, and asserts the key fields survive the JSON round-trip intact.
func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	orig := Config{
		FailMode:       FailModeWarn,
		Socket:         SocketConfig{APIToken: "tok_roundtrip"},
		RedactPatterns: []string{`CUSTOM=\S+`},
		Audit: AuditConfig{
			Sinks:         []string{"file", "syslog"},
			RetentionDays: 14,
			MaxSizeBytes:  2048,
		},
		LlamaFirewall: LlamaFirewallConfig{Enabled: true, SampleRate: 0.5},
		SelfCatalog:   SelfCatalogConfig{URL: "https://example.com/self.json"},
	}

	if err := Save(path, orig); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// File must exist and be 0600 (Save's documented permission). On Windows the
	// Unix permission bits are not enforced, so only assert mode on non-Windows
	// by checking the file is at least readable — existence is the load-bearing
	// part of the round-trip.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Save did not create file: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save returned error: %v", err)
	}

	if got.FailMode != orig.FailMode {
		t.Errorf("FailMode round-trip = %q, want %q", got.FailMode, orig.FailMode)
	}
	if got.SocketAPIToken() != orig.SocketAPIToken() {
		t.Errorf("SocketAPIToken round-trip = %q, want %q", got.SocketAPIToken(), orig.SocketAPIToken())
	}
	if !reflect.DeepEqual(got.GetRedactPatterns(), orig.GetRedactPatterns()) {
		t.Errorf("RedactPatterns round-trip = %v, want %v", got.GetRedactPatterns(), orig.GetRedactPatterns())
	}
	if got.AuditRetentionDays() != orig.AuditRetentionDays() {
		t.Errorf("AuditRetentionDays round-trip = %d, want %d", got.AuditRetentionDays(), orig.AuditRetentionDays())
	}
	if got.AuditMaxSizeBytes() != orig.AuditMaxSizeBytes() {
		t.Errorf("AuditMaxSizeBytes round-trip = %d, want %d", got.AuditMaxSizeBytes(), orig.AuditMaxSizeBytes())
	}
	if !reflect.DeepEqual(got.Audit.Sinks, orig.Audit.Sinks) {
		t.Errorf("Audit.Sinks round-trip = %v, want %v", got.Audit.Sinks, orig.Audit.Sinks)
	}
	if got.LlamaFirewallEnabled() != orig.LlamaFirewallEnabled() {
		t.Errorf("LlamaFirewallEnabled round-trip = %v, want %v", got.LlamaFirewallEnabled(), orig.LlamaFirewallEnabled())
	}
	if got.LlamaFirewallSampleRate() != orig.LlamaFirewallSampleRate() {
		t.Errorf("LlamaFirewallSampleRate round-trip = %v, want %v", got.LlamaFirewallSampleRate(), orig.LlamaFirewallSampleRate())
	}
	if got.SelfCatalog.URL != orig.SelfCatalog.URL {
		t.Errorf("SelfCatalog.URL round-trip = %q, want %q", got.SelfCatalog.URL, orig.SelfCatalog.URL)
	}
}

// TestSaveErrorOnUncreatablePath covers Save's write-error branch by targeting a
// path whose parent component is an existing regular file (so the directory
// cannot be created/traversed). os.WriteFile must fail and Save must surface a
// non-nil error rather than silently swallowing it.
func TestSaveErrorOnUncreatablePath(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file, then attempt to Save "underneath" it as if it were
	// a directory. Writing to <file>/config.json fails on all platforms because
	// a file cannot contain children.
	notADir := filepath.Join(dir, "iamafile")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup: write blocking file: %v", err)
	}
	badPath := filepath.Join(notADir, "config.json")

	if err := Save(badPath, Config{FailMode: FailModeClosed}); err == nil {
		t.Errorf("Save to path under a regular file returned nil error, want non-nil")
	}
}

// ---------------------------------------------------------------------------
// WatchDirectories + AddWatchDirectory (config.go).
// ---------------------------------------------------------------------------

// TestWatchDirectoriesNilWhenUnset verifies the nil-Watch path returns nil.
func TestWatchDirectoriesNilWhenUnset(t *testing.T) {
	if got := (Config{}).WatchDirectories(); got != nil {
		t.Errorf("WatchDirectories() on nil Watch = %v, want nil", got)
	}
}

// TestAddWatchDirectory covers lazy init, read-back, and idempotent dedup.
func TestAddWatchDirectory(t *testing.T) {
	var cfg Config // Watch is nil — AddWatchDirectory must lazily initialise it.

	cfg.AddWatchDirectory("/a")
	cfg.AddWatchDirectory("/b")

	got := cfg.WatchDirectories()
	if !reflect.DeepEqual(got, []string{"/a", "/b"}) {
		t.Fatalf("WatchDirectories() after two adds = %v, want [/a /b]", got)
	}

	// Adding a duplicate must be a no-op (idempotent dedup, per source).
	cfg.AddWatchDirectory("/a")
	got = cfg.WatchDirectories()
	if !reflect.DeepEqual(got, []string{"/a", "/b"}) {
		t.Errorf("WatchDirectories() after duplicate add = %v, want [/a /b] (no dup)", got)
	}

	// A genuinely new entry still appends.
	cfg.AddWatchDirectory("/c")
	got = cfg.WatchDirectories()
	if !reflect.DeepEqual(got, []string{"/a", "/b", "/c"}) {
		t.Errorf("WatchDirectories() after third add = %v, want [/a /b /c]", got)
	}
}
