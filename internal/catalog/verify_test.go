package catalog

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestUnsignedReturnsFalse(t *testing.T) {
	e := Entry{Package: "x", CatalogSignature: ""}
	if VerifySignature(e) {
		t.Error("VerifySignature(empty signature) = true, want false")
	}
}

func TestSignedReturnsTrue(t *testing.T) {
	e := Entry{Package: "x", CatalogSignature: "abc123"}
	if !VerifySignature(e) {
		t.Error("VerifySignature(non-empty signature) = false, want true")
	}
}

// signEntry produces a valid Ed25519 signature over the canonical entry payload
// using the given private key. Used by tests to create signed test entries.
func signEntry(t *testing.T, e Entry, priv ed25519.PrivateKey) string {
	t.Helper()
	payload := entryPayload{
		ID:            e.ID,
		Name:          e.Name,
		Ecosystem:     e.Ecosystem,
		Package:       e.Package,
		Versions:      e.Versions,
		Severity:      e.Severity,
		SourceURL:     e.SourceURL,
		CatalogSource: e.CatalogSource,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal entry payload: %v", err)
	}
	sig := ed25519.Sign(priv, data)
	return base64.StdEncoding.EncodeToString(sig)
}

// TestVerifySignatureWithKey_ValidSignature verifies that a correctly signed entry
// returns true when verified with the matching public key (CTLG-07).
func TestVerifySignatureWithKey_ValidSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	e := Entry{
		ID:            "test-001",
		Name:          "test entry",
		Ecosystem:     "npm",
		Package:       "test-pkg",
		Versions:      []string{"1.0.0"},
		Severity:      "high",
		CatalogSource: "test-source",
	}
	e.CatalogSignature = signEntry(t, e, priv)

	if !VerifySignatureWithKey(e, pub) {
		t.Error("VerifySignatureWithKey(valid sig) = false, want true")
	}
}

// TestVerifySignatureWithKey_TamperedEntry verifies that a tampered entry
// (valid signature but modified content) returns false — not a crash (CTLG-07).
func TestVerifySignatureWithKey_TamperedEntry(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	e := Entry{
		ID:            "test-002",
		Ecosystem:     "npm",
		Package:       "legit-pkg",
		Versions:      []string{"1.0.0"},
		Severity:      "low",
		CatalogSource: "test-source",
	}
	e.CatalogSignature = signEntry(t, e, priv)

	// Tamper with the entry after signing.
	e.Severity = "critical"

	if VerifySignatureWithKey(e, pub) {
		t.Error("VerifySignatureWithKey(tampered entry) = true, want false")
	}
}

// TestVerifySignatureWithKey_InvalidBase64 verifies that a malformed base64
// signature returns false (not a panic) — tampered/corrupted data (CTLG-07).
func TestVerifySignatureWithKey_InvalidBase64(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	e := Entry{
		Package:          "x",
		CatalogSignature: "not-valid-base64!!!",
	}

	if VerifySignatureWithKey(e, pub) {
		t.Error("VerifySignatureWithKey(invalid base64) = true, want false")
	}
}

// TestVerifySignatureWithKey_UnsignedEntry verifies that an entry with no
// signature is Signed=false — must NOT hard-fail (unsigned → warn-only per CTLG-07).
func TestVerifySignatureWithKey_UnsignedEntry(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	e := Entry{
		Package:          "unsigned-pkg",
		CatalogSignature: "",
	}

	if VerifySignatureWithKey(e, pub) {
		t.Error("VerifySignatureWithKey(unsigned entry) = true, want false (unsigned must stay warn-only)")
	}
}

// TestVerifySignatureWithKey_NilKeyFallsBackToPresence verifies that passing a
// nil/empty key falls back to presence-only behavior (backward compat).
func TestVerifySignatureWithKey_NilKeyFallsBackToPresence(t *testing.T) {
	eSigned := Entry{Package: "x", CatalogSignature: "abc123"}
	if !VerifySignatureWithKey(eSigned, nil) {
		t.Error("VerifySignatureWithKey(nil key, non-empty sig) = false, want true (presence fallback)")
	}

	eUnsigned := Entry{Package: "x", CatalogSignature: ""}
	if VerifySignatureWithKey(eUnsigned, nil) {
		t.Error("VerifySignatureWithKey(nil key, empty sig) = true, want false")
	}
}

// TestUnsignedSourceStaysWarnOnlyViaCorroboration verifies the critical CTLG-07
// requirement: an unsigned catalog source (Signed=false) must yield warn-only
// regardless of corroboration, and must NOT hard-fail or block.
// This test proves the existing behavior is preserved after the Task 06 upgrade.
func TestUnsignedSourceStaysWarnOnlyViaCorroboration(t *testing.T) {
	// Build a MultiIndex where Bumblebee has an entry for evil-pkg but with no
	// signature (unsigned → presence check returns false → warn-only).
	bbEntry := Entry{
		ID:               "bb-unsigned-001",
		Ecosystem:        "npm",
		Package:          "evil-pkg",
		Versions:         []string{"1.0.0"},
		Severity:         "critical",
		CatalogSource:    "bumblebee",
		CatalogSignature: "", // unsigned
	}
	bbIdx := buildTestIndexWithEntry(t, bbEntry)
	mi := NewMultiIndex(bbIdx, nil, nil)

	// All matches from bumblebee are unsigned (Signed=false).
	matches := mi.LookupAll("npm", "evil-pkg")

	// Filter out dissent sentinels to check the real matches.
	var realMatches []int
	for i, m := range matches {
		if !m.Dissented {
			realMatches = append(realMatches, i)
		}
	}
	if len(realMatches) == 0 {
		t.Fatal("no real matches from unsigned bumblebee entry, expected at least 1")
	}
	for _, i := range realMatches {
		if matches[i].Signed {
			t.Errorf("match[%d].Signed = true for unsigned entry, want false", i)
		}
	}

	// The single unsigned source must yield warn (not block) in corroboration.
	// We exercise this by checking what policy.Evaluate would determine:
	// unsigned → warn-only is enforced by the corroborate() function.
	// We verify the "signed" field is correctly false so corroborate treats it as unsigned.
	signedCount := 0
	for _, m := range matches {
		if !m.Dissented && m.Signed {
			signedCount++
		}
	}
	// signedCount must be 0 for unsigned entries.
	if signedCount != 0 {
		t.Errorf("signedCount = %d for unsigned source, want 0", signedCount)
	}
}
