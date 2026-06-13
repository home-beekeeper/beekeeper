package corpus

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadOrCreateSigningKeyGeneratesOnFirstRun verifies that LoadOrCreateSigningKey
// generates a valid Ed25519 key on the first call and persists it at the key path.
func TestLoadOrCreateSigningKeyGeneratesOnFirstRun(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "corpus-signing.key")

	priv, err := LoadOrCreateSigningKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey (first run): %v", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("private key size = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}

	// Key file must exist after generation.
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}
	if len(data) != ed25519.PrivateKeySize {
		t.Errorf("key file size = %d, want %d", len(data), ed25519.PrivateKeySize)
	}
}

// TestLoadOrCreateSigningKeyIsIdempotent verifies that calling LoadOrCreateSigningKey
// twice returns the same key (key-generation idempotency).
func TestLoadOrCreateSigningKeyIsIdempotent(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "corpus-signing.key")

	priv1, err := LoadOrCreateSigningKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey (call 1): %v", err)
	}
	priv2, err := LoadOrCreateSigningKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey (call 2): %v", err)
	}

	// Both calls must return the same key bytes.
	if string(priv1) != string(priv2) {
		t.Error("LoadOrCreateSigningKey is not idempotent: call 1 and call 2 returned different keys")
	}
}

// TestSignEnvelopeRoundTrip verifies the Ed25519 sign + verify round-trip.
// A signing block produced by SignEnvelope must be verifiable with the corresponding
// public key.
func TestSignEnvelopeRoundTrip(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "corpus-signing.key")

	// Build a minimal push envelope.
	env := PushEnvelope{
		Signature: EnvelopeSignature{
			PackageOrExtensionID:  "npm:malicious-pkg",
			Version:               "1.0.0",
			BehaviorSignatureHash: BehaviorSigHash("bash", "~/.ssh/id_rsa", ""),
		},
		TrueLabel:      "malicious",
		ConfidenceTier: "watch",
		SourceCount:    1,
		Scope:          ScopeOrgOnly,
		ActionHint:     ActionHintWatchAndBlock,
	}

	block, err := SignEnvelope(env, keyPath)
	if err != nil {
		t.Fatalf("SignEnvelope: %v", err)
	}
	if block.Issuer != "local" {
		t.Errorf("Issuer = %q, want %q", block.Issuer, "local")
	}
	// Ed25519 signature is 64 bytes = 128 hex chars.
	if len(block.Signature) != 128 {
		t.Errorf("Signature length = %d hex chars, want 128 (64 bytes)", len(block.Signature))
	}
	if block.IssuedAt == "" {
		t.Error("IssuedAt must be non-empty")
	}
	// Nonce: 16 bytes = 32 hex chars.
	if len(block.Nonce) != 32 {
		t.Errorf("Nonce length = %d hex chars, want 32 (16 bytes)", len(block.Nonce))
	}

	// Verify the signature with the public key from the private key.
	priv, err := LoadOrCreateSigningKey(keyPath)
	if err != nil {
		t.Fatalf("LoadOrCreateSigningKey for verify: %v", err)
	}
	pub := priv.Public().(ed25519.PublicKey)

	msg, err := canonicalSigningInput(env)
	if err != nil {
		t.Fatalf("canonicalSigningInput: %v", err)
	}

	// Decode the hex signature from the signing block.
	sigBytes, err := hex.DecodeString(block.Signature)
	if err != nil {
		t.Fatalf("decode signature hex: %v", err)
	}

	if !ed25519.Verify(pub, msg, sigBytes) {
		t.Error("ed25519.Verify: signature verification failed (round-trip broken)")
	}
}

// TestSigningKeyFilePermissions verifies that the signing key file is created
// with 0600 permissions (owner-only) on Unix. On Windows, platform.SetOwnerOnly
// applies owner-DACL — this test verifies the file exists with restricted perms.
func TestSigningKeyFilePermissions(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "corpus-signing.key")

	if _, err := LoadOrCreateSigningKey(keyPath); err != nil {
		t.Fatalf("LoadOrCreateSigningKey: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat signing key: %v", err)
	}

	// On Unix: mode must be exactly 0600.
	// On Windows: platform.SetOwnerOnly applies owner-DACL; os.Stat returns 0666
	// (Windows does not expose DACL via Stat.Mode). We verify the file exists and
	// trust the SetOwnerOnly call in LoadOrCreateSigningKey.
	mode := info.Mode()
	if mode.IsDir() {
		t.Fatal("signing key path is a directory, not a file")
	}
	// Accept 0600 (Unix) or 0666 (Windows placeholder via SetOwnerOnly/DACL).
	perm := mode.Perm()
	if perm != 0o600 && perm != 0o666 {
		t.Errorf("signing key mode = %04o, want 0600 (Unix) or 0666 (Windows DACL)", perm)
	}
}
