package corpus

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/home-beekeeper/beekeeper/internal/platform"
)

// LoadOrCreateSigningKey loads the Ed25519 private key from keyPath if it exists,
// or generates a new one and persists it to keyPath with 0600 / owner-DACL permissions.
//
// Key format: the 64-byte Ed25519 private key seed (first 32 bytes = private seed,
// last 32 bytes = public key) written as a raw binary file. This is the minimal
// portable format — no PEM wrapping in v1 since the key is local-only (no transport).
//
// keyPath must be an absolute path under StateDir (e.g. StateDir()/corpus-signing.key).
// The caller resolves the path; corpus does not call platform.StateDir internally.
//
// Generate-once guarantee: if keyPath exists and contains a valid 64-byte key,
// it is returned as-is (idempotent). The key is never rotated automatically.
//
// Security: generated with crypto/rand; written with O_CREATE|O_WRONLY|O_TRUNC at
// 0600; platform.SetOwnerOnly enforced after write (mirrors audit.Writer pattern).
func LoadOrCreateSigningKey(keyPath string) (ed25519.PrivateKey, error) {
	// Try to load an existing key.
	if data, err := os.ReadFile(keyPath); err == nil {
		// Validate: Ed25519 private key is 64 bytes.
		if len(data) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(data), nil
		}
		return nil, fmt.Errorf("corpus: signing key at %q is %d bytes, want %d (Ed25519 private key size); delete and restart to regenerate",
			keyPath, len(data), ed25519.PrivateKeySize)
	}

	// Generate a new Ed25519 key pair.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("corpus: generate Ed25519 signing key: %w", err)
	}

	// Ensure the parent directory exists with 0700.
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("corpus: create signing key directory: %w", err)
	}

	// Write the raw private key bytes with O_CREATE|O_WRONLY|O_TRUNC at 0600.
	f, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("corpus: create signing key file %q: %w", keyPath, err)
	}
	if _, err := f.Write([]byte(priv)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("corpus: write signing key: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("corpus: close signing key file: %w", err)
	}

	// Enforce owner-only permissions after write (mirrors audit.Writer, T-23-07).
	if err := platform.SetOwnerOnly(keyPath); err != nil {
		return nil, fmt.Errorf("corpus: enforce owner-only on signing key: %w", err)
	}

	return priv, nil
}

// canonicalSigningInput constructs the stable byte sequence that is signed by
// SignEnvelope and verified by ed25519.Verify. The sequence is the JSON-marshalled
// struct of the envelope fields that carry provenance:
//
//   - Signature (EnvelopeSignature): package ID, version, behavior hash, IOCs
//   - TrueLabel
//   - ConfidenceTier
//   - SourceCount
//   - Scope
//   - ActionHint
//
// Signing.Nonce/IssuedAt are NOT included in the input (they are output fields).
// The canonical struct is marshalled with sorted keys (encoding/json uses struct
// field declaration order, which is stable across runs).
func canonicalSigningInput(env PushEnvelope) ([]byte, error) {
	// Signing input: a stable subset of PushEnvelope fields.
	input := struct {
		Signature      EnvelopeSignature `json:"signature"`
		TrueLabel      string            `json:"true_label"`
		ConfidenceTier string            `json:"confidence_tier"`
		SourceCount    int               `json:"source_count"`
		Scope          string            `json:"scope"`
		ActionHint     string            `json:"action_hint"`
	}{
		Signature:      env.Signature,
		TrueLabel:      env.TrueLabel,
		ConfidenceTier: env.ConfidenceTier,
		SourceCount:    env.SourceCount,
		Scope:          string(env.Scope),
		ActionHint:     string(env.ActionHint),
	}
	return json.Marshal(input)
}

// SignEnvelope signs the push envelope with the Ed25519 key at keyPath and
// returns a populated SigningBlock.
//
// The signing key is loaded (or generated once) by LoadOrCreateSigningKey.
// The canonical signing input is deterministic: same envelope + same key → same
// signature. The Nonce is a fresh 16-byte CSPRNG value per call to prevent replay.
//
// v1 contract: SigningBlock is populated but NOT transported (no transport in v1).
// The block freezes the wire format so v1.1+ transport is wiring, not migration
// (STORE-04 alignment, T-23-07).
//
// keyPath is typically StateDir()/corpus-signing.key; the caller resolves it.
//
// The returned SigningBlock.Signature is a lowercase hex Ed25519 signature (128
// hex chars = 64 bytes).
func SignEnvelope(env PushEnvelope, keyPath string) (SigningBlock, error) {
	priv, err := LoadOrCreateSigningKey(keyPath)
	if err != nil {
		return SigningBlock{}, fmt.Errorf("corpus: SignEnvelope: load signing key: %w", err)
	}

	// Compute the canonical signing input.
	msg, err := canonicalSigningInput(env)
	if err != nil {
		return SigningBlock{}, fmt.Errorf("corpus: SignEnvelope: marshal signing input: %w", err)
	}

	// Sign with Ed25519 (stdlib; zero new deps).
	sig := ed25519.Sign(priv, msg)

	// Generate a fresh nonce (16 bytes) to prevent replay.
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return SigningBlock{}, fmt.Errorf("corpus: SignEnvelope: generate nonce: %w", err)
	}

	return SigningBlock{
		Issuer:    "local",
		Signature: hex.EncodeToString(sig),
		IssuedAt:  time.Now().UTC().Format(time.RFC3339),
		Nonce:     hex.EncodeToString(nonceBytes),
	}, nil
}
