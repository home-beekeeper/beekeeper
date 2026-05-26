package catalog

import "testing"

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
