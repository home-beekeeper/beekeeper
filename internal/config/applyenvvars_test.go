package config

import (
	"os"
	"testing"
)

// applyEnvVars (layered.go:573) is the TRUSTED env-application helper. It is NOT
// reached through LoadLayered — LoadLayered's env layer is low-trust and routes
// through applyEnvVarsUntrusted instead. applyEnvVars is retained for callers
// that need unrestricted env application (per its doc comment), so these tests
// exercise it directly (white-box, package config) to (a) get coverage credit
// and (b) pin the trusted-path behaviour that distinguishes it from the
// untrusted variant: it applies BEEKEEPER_FAIL_MODE unconditionally, including a
// security RELAXATION that applyEnvVarsUntrusted would refuse.
//
// Env vars are set via t.Setenv and read back through os.Environ() so the test
// drives the real KEY=VALUE parsing path (parseEnvSlice) rather than a synthetic
// slice — this exercises applyEnvVars exactly as a production caller would.

// TestApplyEnvVars_AllKnownVars sets every BEEKEEPER_* var applyEnvVars handles
// and asserts each maps to the right Config field.
func TestApplyEnvVars_AllKnownVars(t *testing.T) {
	t.Setenv("BEEKEEPER_FAIL_MODE", "warn")
	t.Setenv("BEEKEEPER_SOCKET_API_TOKEN", "tok_env_trusted")
	t.Setenv("BEEKEEPER_LLAMAFIREWALL_ENABLED", "true")
	t.Setenv("BEEKEEPER_AUDIT_SINKS", "file, syslog ,otlp")
	t.Setenv("BEEKEEPER_SELF_CATALOG_URL", "https://trusted.example.com/self.json")

	// Start from a fail-closed baseline (the LoadLayered baseline).
	base := Config{FailMode: FailModeClosed}
	got := applyEnvVars(base, os.Environ())

	if got.FailMode != FailModeWarn {
		t.Errorf("FailMode = %q, want %q (BEEKEEPER_FAIL_MODE applied)", got.FailMode, FailModeWarn)
	}
	if got.Socket.APIToken != "tok_env_trusted" {
		t.Errorf("Socket.APIToken = %q, want tok_env_trusted", got.Socket.APIToken)
	}
	if !got.LlamaFirewall.Enabled {
		t.Error("LlamaFirewall.Enabled = false, want true (BEEKEEPER_LLAMAFIREWALL_ENABLED=true)")
	}
	// Audit sinks are comma-split and trimmed; empty fragments dropped.
	wantSinks := []string{"file", "syslog", "otlp"}
	if len(got.Audit.Sinks) != len(wantSinks) {
		t.Fatalf("Audit.Sinks = %v, want %v", got.Audit.Sinks, wantSinks)
	}
	for i, s := range wantSinks {
		if got.Audit.Sinks[i] != s {
			t.Errorf("Audit.Sinks[%d] = %q, want %q (trimmed)", i, got.Audit.Sinks[i], s)
		}
	}
	if got.SelfCatalog.URL != "https://trusted.example.com/self.json" {
		t.Errorf("SelfCatalog.URL = %q, want trusted URL (trusted path applies it)", got.SelfCatalog.URL)
	}
}

// TestApplyEnvVars_AppliesFailModeRelaxation pins the key behavioural difference
// between applyEnvVars (trusted) and applyEnvVarsUntrusted (low-trust): the
// trusted variant applies a fail_mode RELAXATION (closed→open) unconditionally,
// whereas the untrusted variant refuses it (TM-D-01). This is the security-
// relevant contract — a caller using applyEnvVars is asserting the env is
// trusted.
func TestApplyEnvVars_AppliesFailModeRelaxation(t *testing.T) {
	t.Setenv("BEEKEEPER_FAIL_MODE", "open")

	got := applyEnvVars(Config{FailMode: FailModeClosed}, os.Environ())
	if got.FailMode != FailModeOpen {
		t.Errorf("FailMode = %q, want %q — applyEnvVars (trusted) applies the relaxation unconditionally",
			got.FailMode, FailModeOpen)
	}
}

// TestApplyEnvVars_LlamaFirewallDisable verifies the false/0/no parsing path
// disables the sidecar (trusted variant honours an explicit disable, unlike the
// untrusted variant which refuses a disable of an enabled sidecar).
func TestApplyEnvVars_LlamaFirewallDisable(t *testing.T) {
	t.Setenv("BEEKEEPER_LLAMAFIREWALL_ENABLED", "false")

	got := applyEnvVars(Config{LlamaFirewall: LlamaFirewallConfig{Enabled: true}}, os.Environ())
	if got.LlamaFirewall.Enabled {
		t.Error("LlamaFirewall.Enabled = true, want false (BEEKEEPER_LLAMAFIREWALL_ENABLED=false honoured by trusted path)")
	}
}

// TestApplyEnvVars_UnknownAndUnsetIgnored verifies that absent BEEKEEPER_* vars
// leave the corresponding fields untouched and unknown BEEKEEPER_* / unrelated
// vars are ignored (no reflective application, T-09-05).
func TestApplyEnvVars_UnknownAndUnsetIgnored(t *testing.T) {
	// Only an unknown key and an unrelated key are set — no known var.
	t.Setenv("BEEKEEPER_NOT_A_REAL_KEY", "x")
	t.Setenv("SOME_UNRELATED_VAR", "y")

	base := Config{
		FailMode:      FailModeWarn,
		Socket:        SocketConfig{APIToken: "preexisting"},
		LlamaFirewall: LlamaFirewallConfig{Enabled: true},
	}
	got := applyEnvVars(base, os.Environ())

	if got.FailMode != FailModeWarn {
		t.Errorf("FailMode = %q, want %q — unknown env must not alter it", got.FailMode, FailModeWarn)
	}
	if got.Socket.APIToken != "preexisting" {
		t.Errorf("Socket.APIToken = %q, want preexisting — unknown env must not alter it", got.Socket.APIToken)
	}
	if !got.LlamaFirewall.Enabled {
		t.Error("LlamaFirewall.Enabled = false, want true — unknown env must not alter it")
	}
}
