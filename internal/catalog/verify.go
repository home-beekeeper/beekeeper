package catalog

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
)

// VerifySignature reports whether a catalog entry carries a non-empty
// catalog_signature. This is the presence-only check used when no trusted
// Ed25519 key is configured for the source (unsigned → warn-only per CTLG-07).
// Downstream consumers that have a configured trusted key should use
// VerifySignatureWithKey instead.
func VerifySignature(e Entry) bool {
	return e.CatalogSignature != ""
}

// entryPayload is the canonical representation of an Entry used for signature
// verification. The CatalogSignature field is excluded to avoid circularity —
// the signature is over the content, not over itself.
type entryPayload struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Ecosystem string   `json:"ecosystem"`
	Package   string   `json:"package"`
	Versions  []string `json:"versions"`
	Severity  string   `json:"severity"`
	SourceURL string   `json:"source_url,omitempty"`
	CatalogSource string `json:"catalog_source,omitempty"`
}

// VerifySignatureWithKey performs real Ed25519 cryptographic verification of
// the catalog entry's catalog_signature against the provided trusted public key.
//
// Returns true ONLY when:
//   - CatalogSignature is non-empty
//   - CatalogSignature decodes as valid base64
//   - ed25519.Verify(pubKey, canonicalEntryJSON, sig) == true
//
// Returns false when:
//   - CatalogSignature is empty (unsigned — warn-only, not an error)
//   - CatalogSignature is not valid base64 (malformed signature)
//   - Signature is present but does not verify (tampered entry)
//   - pubKey is nil (no trusted key configured — fall back to presence check)
//
// CTLG-07: an unsigned entry (CatalogSignature == "") must remain Signed=false
// and yield warn-only via corroboration — do NOT return an error or hard-fail
// unsigned sources (that would break the default Bumblebee flow which currently
// has unsigned entries).
func VerifySignatureWithKey(e Entry, pubKey ed25519.PublicKey) bool {
	if len(pubKey) == 0 {
		// No trusted key configured — fall back to presence-only.
		return VerifySignature(e)
	}

	if e.CatalogSignature == "" {
		// Unsigned entry — not an error; warn-only via corroboration.
		return false
	}

	sigBytes, err := base64.StdEncoding.DecodeString(e.CatalogSignature)
	if err != nil {
		// Malformed base64 — treat as unverified (not a crash).
		return false
	}

	// Canonical payload: entry content without the signature field.
	payload := entryPayload{
		ID:        e.ID,
		Name:      e.Name,
		Ecosystem: e.Ecosystem,
		Package:   e.Package,
		Versions:  e.Versions,
		Severity:  e.Severity,
		SourceURL: e.SourceURL,
		CatalogSource: e.CatalogSource,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		// Should not happen with a valid Entry struct.
		return false
	}

	return ed25519.Verify(pubKey, payloadJSON, sigBytes)
}
