package catalog

import (
	"crypto/ed25519"
	"encoding/hex"
)

// selfCatalogKeyHex is the compile-time embedded Ed25519 public key used to
// verify beekeeper-self feed signatures.
//
// IMPORTANT: This key is SEPARATE from the release-signing identity (cosign /
// Sigstore). Compromising the release pipeline key alone is NOT sufficient to
// forge a self-catalog signature — an attacker must additionally compromise
// this second, independently-managed key (T-09-12).
//
// Key generation procedure (for reference — do not run at startup):
//
//	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
//	hex.EncodeToString(pub)
//
// The test private key is kept in selfcatalog_test.go (test-only, never shipped).
//
// GOVERNANCE NOTE: In v1.0.0, this key is managed by the single maintainer. A
// future governance milestone targets a separate maintainer identity for the
// self-catalog key, requiring two independent compromises to forge a quarantine
// entry or suppress a real one.
const selfCatalogKeyHex = "e09f12f0cb1e09cfcf238ccffaeafb301fabd187756ee140ef56f6d62dbae23e"

// SelfCatalogPublicKey is the embedded Ed25519 public key for verifying
// beekeeper-self feed signatures. It is populated at package init time from
// the compile-time constant selfCatalogKeyHex. Any binary that imports this
// package carries the key at link time — no runtime fetch is performed.
var SelfCatalogPublicKey ed25519.PublicKey

func init() {
	b, err := hex.DecodeString(selfCatalogKeyHex)
	if err != nil {
		// This is a programmer error (malformed compile-time constant) — panic
		// at startup is the correct behavior rather than silently using a nil key.
		panic("selfkey: invalid selfCatalogKeyHex constant: " + err.Error())
	}
	if len(b) != ed25519.PublicKeySize {
		panic("selfkey: selfCatalogKeyHex has wrong length — expected 32 bytes for Ed25519 public key")
	}
	SelfCatalogPublicKey = ed25519.PublicKey(b)
}
