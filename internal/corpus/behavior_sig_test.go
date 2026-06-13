package corpus

import (
	"strings"
	"testing"
)

// TestBehaviorSigHash verifies the SCHEMA-05 frozen normalization rules for
// behavior_signature_hash:
//   - Deterministic: same inputs → same hash across calls
//   - Normalization-stable: port-stripped network_destination and lowercase
//     action_type produce the same hash as their normalized equivalents
//   - NUL-separated: ("a","bc","") != ("ab","c","") — no prefix collision
//   - Returns a 64-char hex string (full SHA-256)
//
// Covers: SCHEMA-05
func TestBehaviorSigHash(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		h1 := BehaviorSigHash("Bash", "/home/u/.ssh/id_rsa", "evil.example.com:443")
		h2 := BehaviorSigHash("Bash", "/home/u/.ssh/id_rsa", "evil.example.com:443")
		if h1 != h2 {
			t.Errorf("BehaviorSigHash not deterministic: %q != %q", h1, h2)
		}
	})

	t.Run("normalization_stable", func(t *testing.T) {
		// port stripped from network_destination; action lowercased →
		// both calls must produce the same hash
		h1 := BehaviorSigHash("Bash", "/home/u/.ssh/id_rsa", "evil.example.com:443")
		h2 := BehaviorSigHash("bash", "/home/u/.ssh/id_rsa", "evil.example.com")
		if h1 != h2 {
			t.Errorf("BehaviorSigHash normalization not stable: port-stripped+lowercased should equal original; got %q vs %q", h1, h2)
		}
	})

	t.Run("home_prefix_normalization", func(t *testing.T) {
		// /home/u/ and /Users/u/ both normalize to ~/
		h1 := BehaviorSigHash("bash", "/home/u/.ssh/id_rsa", "")
		h2 := BehaviorSigHash("bash", "/Users/u/.ssh/id_rsa", "")
		if h1 != h2 {
			t.Errorf("BehaviorSigHash home-prefix normalization: /home/u/ and /Users/u/ should produce same hash; got %q vs %q", h1, h2)
		}
	})

	t.Run("windows_home_prefix_normalization", func(t *testing.T) {
		// C:/Users/u/ also normalizes to ~/
		h1 := BehaviorSigHash("bash", "/home/u/.ssh/id_rsa", "")
		h2 := BehaviorSigHash("bash", "C:/Users/u/.ssh/id_rsa", "")
		if h1 != h2 {
			t.Errorf("BehaviorSigHash C:/Users/ home-prefix: should normalize same as /home/u/; got %q vs %q", h1, h2)
		}
	})

	t.Run("action_base_name_only", func(t *testing.T) {
		// path-like action_type normalizes to base name
		h1 := BehaviorSigHash("bash", "", "")
		h2 := BehaviorSigHash("/usr/bin/bash", "", "")
		if h1 != h2 {
			t.Errorf("BehaviorSigHash action base-name normalization: bash vs /usr/bin/bash should produce same hash; got %q vs %q", h1, h2)
		}
	})

	t.Run("nul_separation_no_prefix_collision", func(t *testing.T) {
		// ("a","bc","") must not equal ("ab","c","") — NUL separator prevents prefix collision
		h1 := BehaviorSigHash("a", "bc", "")
		h2 := BehaviorSigHash("ab", "c", "")
		if h1 == h2 {
			t.Errorf("BehaviorSigHash prefix collision: (\"a\",\"bc\",\"\") == (\"ab\",\"c\",\"\") — NUL separation broken")
		}
	})

	t.Run("returns_64_char_hex", func(t *testing.T) {
		h := BehaviorSigHash("bash", "~/.ssh/id_rsa", "evil.example.com")
		if len(h) != 64 {
			t.Errorf("BehaviorSigHash: expected 64-char hex string; got %d chars: %q", len(h), h)
		}
		// Must be lowercase hex.
		if strings.ToLower(h) != h {
			t.Errorf("BehaviorSigHash: expected lowercase hex; got %q", h)
		}
	})

	t.Run("ipv4_port_stripped", func(t *testing.T) {
		// IPv4:port → hostname only
		h1 := BehaviorSigHash("", "", "192.168.1.1:443")
		h2 := BehaviorSigHash("", "", "192.168.1.1")
		if h1 != h2 {
			t.Errorf("BehaviorSigHash IPv4 port stripping: got %q vs %q", h1, h2)
		}
	})

	t.Run("ipv6_not_port_stripped", func(t *testing.T) {
		// IPv6 address with bracket form: "[::1]:443" — port MAY be stripped
		// but bare IPv6 "::1" must not be altered
		h := BehaviorSigHash("", "", "::1")
		if len(h) != 64 {
			t.Errorf("BehaviorSigHash IPv6 bare: got %d chars hash", len(h))
		}
	})
}

// TestScanClusterID verifies the SCHEMA-02 stable key:
//   - Same (pkg, version, fp) → same 16-char hex ID across calls
//   - Different version → different ID (version-sensitive)
//   - Returns exactly 16 hex chars
//
// Covers: SCHEMA-02 (OQ-2 locked stable key)
func TestScanClusterID(t *testing.T) {
	t.Run("stable_across_calls", func(t *testing.T) {
		id1 := ScanClusterID("nx-console", "1.2.3", "fp")
		id2 := ScanClusterID("nx-console", "1.2.3", "fp")
		if id1 != id2 {
			t.Errorf("ScanClusterID not stable: %q != %q", id1, id2)
		}
	})

	t.Run("version_sensitive", func(t *testing.T) {
		id1 := ScanClusterID("nx-console", "1.2.3", "fp")
		id2 := ScanClusterID("nx-console", "1.2.4", "fp")
		if id1 == id2 {
			t.Errorf("ScanClusterID not version-sensitive: 1.2.3 and 1.2.4 produced same ID %q", id1)
		}
	})

	t.Run("pkg_sensitive", func(t *testing.T) {
		id1 := ScanClusterID("nx-console", "1.2.3", "fp")
		id2 := ScanClusterID("nx-console-evil", "1.2.3", "fp")
		if id1 == id2 {
			t.Errorf("ScanClusterID not package-sensitive: different packages produced same ID %q", id1)
		}
	})

	t.Run("returns_16_char_hex", func(t *testing.T) {
		id := ScanClusterID("nx-console", "1.2.3", "fingerprint")
		if len(id) != 16 {
			t.Errorf("ScanClusterID: expected 16-char hex; got %d chars: %q", len(id), id)
		}
		if strings.ToLower(id) != id {
			t.Errorf("ScanClusterID: expected lowercase hex; got %q", id)
		}
	})

	t.Run("nul_separation_no_prefix_collision", func(t *testing.T) {
		// ("pkg","1.0foo","") must not equal ("pkg1.0","foo","") — NUL separator
		id1 := ScanClusterID("pkg", "1.0foo", "")
		id2 := ScanClusterID("pkg1.0", "foo", "")
		if id1 == id2 {
			t.Errorf("ScanClusterID prefix collision: NUL separation broken")
		}
	})
}
