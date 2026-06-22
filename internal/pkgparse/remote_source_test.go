package pkgparse

import "testing"

// TestParseRemoteSourceClassification drives Parse over install commands that pull
// from non-registry sources and asserts the RemoteSource kind, plus that a normal
// registry install (including a scoped name) is NOT misclassified.
func TestParseRemoteSourceClassification(t *testing.T) {
	cases := []struct {
		name       string
		cmd        string
		wantOK     bool
		wantRemote string
		wantPkg    string // expected Package for registry installs ("" for remote specs)
	}{
		{"git+https", "npm install git+https://github.com/a/b.git", true, "git", ""},
		{"git protocol", "npm i git://github.com/a/b", true, "git", ""},
		{"git scp", "npm install git@github.com:a/b.git", true, "git", ""},
		{"dot-git suffix", "pnpm add https://example.com/a/b.git", true, "git", ""},
		{"github shorthand", "npm i github:a/b", true, "github", ""},
		{"gitlab shorthand", "pnpm add gitlab:a/b", true, "github", ""},
		{"tarball tgz", "npm i https://example.com/p.tgz", true, "tarball", ""},
		{"tarball tar.gz", "npm i https://example.com/p-1.2.3.tar.gz", true, "tarball", ""},
		{"plain url", "npm i https://example.com/pkg", true, "url", ""},
		{"url with fragment", "npm i https://example.com/p.tgz#readme", true, "tarball", ""},
		{"file spec", "npm i file:../local-pkg", true, "file", ""},
		{"relative dot", "npm i ./local-pkg", true, "file", ""},
		{"relative dotdot", "npm i ../local-pkg", true, "file", ""},
		// Regression: a normal registry install must NOT be flagged, and a scoped
		// package name with a leading @ must still parse correctly.
		{"normal pinned", "npm i left-pad@1.0.0", true, "", "left-pad"},
		{"scoped pinned", "npm i @scope/pkg@1.0.0", true, "", "@scope/pkg"},
		{"bare name", "npm i react", true, "", "react"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pc, ok := Parse(tc.cmd)
			if ok != tc.wantOK {
				t.Fatalf("Parse(%q) ok = %v, want %v", tc.cmd, ok, tc.wantOK)
			}
			if pc.RemoteSource != tc.wantRemote {
				t.Errorf("RemoteSource = %q, want %q", pc.RemoteSource, tc.wantRemote)
			}
			if tc.wantRemote == "" && pc.Package != tc.wantPkg {
				t.Errorf("Package = %q, want %q (registry install must still parse the name)", pc.Package, tc.wantPkg)
			}
			if tc.wantRemote != "" && pc.Package != "" {
				t.Errorf("Package = %q, want empty for a remote-source spec", pc.Package)
			}
		})
	}
}
