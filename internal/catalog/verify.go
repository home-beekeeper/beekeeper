package catalog

// VerifySignature reports whether a catalog entry carries a non-empty
// catalog_signature.
//
// Phase 1 is presence-only: an entry is considered "signed" purely if a
// signature string is present. The policy engine uses this to treat unsigned
// entries as warn-only regardless of corroboration count (CTLG-07). Real
// cryptographic verification (Ed25519 / cosign bundle) is deferred to Phase 2;
// the boolean contract here stays stable so downstream consumers do not change
// when verification is upgraded.
func VerifySignature(e Entry) bool {
	return e.CatalogSignature != ""
}
