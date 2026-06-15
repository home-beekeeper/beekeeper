package pkgparse

import (
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// TestParse is the table-driven test for Parse. It covers every behaviour
// specified in the 08-01-PLAN task 1 <behavior> block.
func TestParse(t *testing.T) {
	type want struct {
		Manager   string
		Ecosystem string
		Verb      string
		Package   string
		Version   string
		IsInstall bool
		IsExec    bool
		Sudo      bool
		Unpinned  bool
	}

	tests := []struct {
		name   string
		input  string
		want   want
		wantOK bool
	}{
		// ── npm ──────────────────────────────────────────────────────────────
		{
			name:   "npm install bare",
			input:  "npm install foo",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "foo", IsInstall: true, Unpinned: true},
		},
		{
			name:   "npm install with version",
			input:  "npm install lodash@4.17.20",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "lodash", Version: "4.17.20", IsInstall: true},
		},
		{
			name:   "npm i shorthand",
			input:  "npm i react",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "i", Package: "react", IsInstall: true, Unpinned: true},
		},
		// npm add — PRD §6.4 closes the §10-7/§10-9 silent hole
		{
			name:   "npm add left-pad",
			input:  "npm add left-pad",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "add", Package: "left-pad", IsInstall: true, Unpinned: true},
		},
		// No-arg install (§10-8)
		{
			name:   "npm install no args",
			input:  "npm install",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", IsInstall: true},
		},
		// Scoped package — splitVersion uses LAST "@"
		{
			name:   "npm install scoped with version",
			input:  "npm install @scope/pkg@1.0.0",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "@scope/pkg", Version: "1.0.0", IsInstall: true},
		},
		{
			name:   "npm install scoped no version",
			input:  "npm install @scope/pkg",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "@scope/pkg", IsInstall: true, Unpinned: true},
		},
		// sudo strip (§6.4 criterion 10)
		{
			name:   "sudo npm install foo",
			input:  "sudo npm install foo",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "foo", IsInstall: true, Sudo: true, Unpinned: true},
		},
		// @latest (NUDGE-05)
		{
			name:   "npm install @latest suffix",
			input:  "npm install some-pkg@latest",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "some-pkg", Version: "latest", IsInstall: true, Unpinned: true},
		},
		// ^ range (NUDGE-05)
		{
			name:   "npm install caret range",
			input:  "npm install some-pkg@^1.0.0",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "some-pkg", Version: "^1.0.0", IsInstall: true, Unpinned: true},
		},
		// ~ range (NUDGE-05)
		{
			name:   "npm install tilde range",
			input:  "npm install some-pkg@~1.0.0",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "some-pkg", Version: "~1.0.0", IsInstall: true, Unpinned: true},
		},
		// Exact pinned (NUDGE-05: Unpinned=false)
		{
			name:   "npm install exact pinned version",
			input:  "npm install chalk@5.4.0",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "chalk", Version: "5.4.0", IsInstall: true, Unpinned: false},
		},

		// ── npx (exec + install, §10-9) ──────────────────────────────────
		{
			name:   "npx create-app",
			input:  "npx create-app",
			wantOK: true,
			want:   want{Manager: "npx", Ecosystem: "npm", Package: "create-app", IsInstall: true, IsExec: true, Unpinned: true},
		},

		// ── pnpm (Ecosystem "npm" — F3/SC1) ──────────────────────────────
		{
			name:   "pnpm add evil-pkg",
			input:  "pnpm add evil-pkg",
			wantOK: true,
			want:   want{Manager: "pnpm", Ecosystem: "npm", Verb: "add", Package: "evil-pkg", IsInstall: true, Unpinned: true},
		},
		{
			name:   "pnpm install",
			input:  "pnpm install",
			wantOK: true,
			want:   want{Manager: "pnpm", Ecosystem: "npm", Verb: "install", IsInstall: true},
		},
		{
			name:   "pnpm dlx (exec)",
			input:  "pnpm dlx foo",
			wantOK: true,
			want:   want{Manager: "pnpm", Ecosystem: "npm", Verb: "dlx", Package: "foo", IsInstall: true, IsExec: true, Unpinned: true},
		},

		// ── bun (Ecosystem "npm") ─────────────────────────────────────────
		{
			name:   "bun add chalk with version",
			input:  "bun add chalk@5.4.0",
			wantOK: true,
			want:   want{Manager: "bun", Ecosystem: "npm", Verb: "add", Package: "chalk", Version: "5.4.0", IsInstall: true, Unpinned: false},
		},
		{
			name:   "bun x (exec)",
			input:  "bun x foo",
			wantOK: true,
			want:   want{Manager: "bun", Ecosystem: "npm", Verb: "x", Package: "foo", IsInstall: true, IsExec: true, Unpinned: true},
		},
		{
			name:   "bun install no args",
			input:  "bun install",
			wantOK: true,
			want:   want{Manager: "bun", Ecosystem: "npm", Verb: "install", IsInstall: true},
		},

		// ── yarn (Ecosystem "npm") ────────────────────────────────────────
		{
			name:   "yarn add lodash",
			input:  "yarn add lodash",
			wantOK: true,
			want:   want{Manager: "yarn", Ecosystem: "npm", Verb: "add", Package: "lodash", IsInstall: true, Unpinned: true},
		},
		{
			name:   "yarn install",
			input:  "yarn install",
			wantOK: true,
			want:   want{Manager: "yarn", Ecosystem: "npm", Verb: "install", IsInstall: true},
		},

		// ── pip / pip3 ───────────────────────────────────────────────────
		{
			name:   "pip install requests",
			input:  "pip install requests",
			wantOK: true,
			want:   want{Manager: "pip", Ecosystem: "pypi", Verb: "install", Package: "requests", IsInstall: true, Unpinned: true},
		},
		{
			name:   "pip3 install requests",
			input:  "pip3 install requests",
			wantOK: true,
			want:   want{Manager: "pip3", Ecosystem: "pypi", Verb: "install", Package: "requests", IsInstall: true, Unpinned: true},
		},

		// ── go ──────────────────────────────────────────────────────────
		{
			name:   "go get",
			input:  "go get golang.org/x/text",
			wantOK: true,
			want:   want{Manager: "go", Ecosystem: "go", Verb: "get", Package: "golang.org/x/text", IsInstall: true, Unpinned: true},
		},

		// ── gem ─────────────────────────────────────────────────────────
		{
			name:   "gem install rails",
			input:  "gem install rails",
			wantOK: true,
			want:   want{Manager: "gem", Ecosystem: "rubygems", Verb: "install", Package: "rails", IsInstall: true, Unpinned: true},
		},

		// ── cargo ───────────────────────────────────────────────────────
		{
			name:   "cargo add serde",
			input:  "cargo add serde",
			wantOK: true,
			want:   want{Manager: "cargo", Ecosystem: "cargo", Verb: "add", Package: "serde", IsInstall: true, Unpinned: true},
		},
		{
			name:   "cargo install ripgrep",
			input:  "cargo install ripgrep",
			wantOK: true,
			want:   want{Manager: "cargo", Ecosystem: "cargo", Verb: "install", Package: "ripgrep", IsInstall: true, Unpinned: true},
		},

		// ── composer ────────────────────────────────────────────────────
		{
			name:   "composer require laravel",
			input:  "composer require laravel/framework",
			wantOK: true,
			want:   want{Manager: "composer", Ecosystem: "packagist", Verb: "require", Package: "laravel/framework", IsInstall: true, Unpinned: true},
		},

		// ── Non-install verbs (§10-7) → ok=false ─────────────────────────
		{
			name:   "npm ls rejected",
			input:  "npm ls",
			wantOK: false,
		},
		{
			name:   "npm run start rejected",
			input:  "npm run start",
			wantOK: false,
		},
		{
			name:   "npm publish rejected",
			input:  "npm publish",
			wantOK: false,
		},
		{
			name:   "npm view rejected",
			input:  "npm view x",
			wantOK: false,
		},
		{
			name:   "npm whoami rejected",
			input:  "npm whoami",
			wantOK: false,
		},
		{
			name:   "unrecognized command rejected",
			input:  "echo hello",
			wantOK: false,
		},
		{
			name:   "empty string rejected",
			input:  "",
			wantOK: false,
		},
		// ── Case insensitivity ───────────────────────────────────────────
		{
			name:   "NPM INSTALL uppercased",
			input:  "NPM INSTALL foo",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "foo", IsInstall: true, Unpinned: true},
		},
		// ── Flags are skipped ────────────────────────────────────────────
		{
			name:   "npm install with flag before package",
			input:  "npm install --save-dev webpack",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "webpack", IsInstall: true, Unpinned: true},
		},
		// ── Raw is preserved ─────────────────────────────────────────────
		{
			name:   "raw preserved verbatim",
			input:  "  npm install foo  ",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "foo", IsInstall: true, Unpinned: true},
		},
		// ── Compound + env-prefix coverage (bypass fix) ──────────────────
		// These previously returned ok=false (prefix parser saw "cd"/"NODE_ENV")
		// and silently escaped BOTH the nudge block and the catalog block.
		{
			name:   "compound: cd && npm install caught",
			input:  "cd /project && npm install evil-pkg",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "evil-pkg", IsInstall: true, Unpinned: true},
		},
		{
			name:   "env-prefix: NODE_ENV=prod npm install caught",
			input:  "NODE_ENV=production npm install lodash@4.17.20",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "lodash", Version: "4.17.20", IsInstall: true},
		},
		{
			name:   "compound + env + pnpm caught (ecosystem npm)",
			input:  "cd app && FORCE_COLOR=1 pnpm add chalk",
			wantOK: true,
			want:   want{Manager: "pnpm", Ecosystem: "npm", Verb: "add", Package: "chalk", IsInstall: true, Unpinned: true},
		},
		{
			name:   "semicolon: npm install foo; echo done — package is clean",
			input:  "npm install foo; echo done",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "foo", IsInstall: true, Unpinned: true},
		},
		{
			name:   "env + sudo: FOO=bar sudo npm install baz",
			input:  "FOO=bar sudo npm install baz",
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "baz", IsInstall: true, Sudo: true, Unpinned: true},
		},
		{
			name:   "compound non-install stays false (cd && npm ls)",
			input:  "cd /tmp && npm ls",
			wantOK: false,
		},
		{
			name:   "compound with no install verb stays false",
			input:  "echo hi && ls -la",
			wantOK: false,
		},
		// ── Quoting honoured (no false positives on quoted text) ─────────
		// A separator inside quotes is literal — the install never executes, so it
		// must NOT be detected (otherwise block mode would deny e.g. commit messages).
		{
			name:   "quoted '&& npm install' in commit message NOT matched",
			input:  `git commit -m "fix: handle 'cd x && npm install foo'"`,
			wantOK: false,
		},
		{
			name:   "single-quoted '; pnpm add evil' NOT matched",
			input:  `echo 'oops ; pnpm add evil'`,
			wantOK: false,
		},
		{
			name:   "quoted path before real unquoted install IS matched",
			input:  `cd "/my project dir" && npm install foo`,
			wantOK: true,
			want:   want{Manager: "npm", Ecosystem: "npm", Verb: "install", Package: "foo", IsInstall: true, Unpinned: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Parse(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("Parse(%q) ok=%v, want %v", tc.input, ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if got.Raw != tc.input {
				t.Errorf("Raw = %q, want %q (must be verbatim)", got.Raw, tc.input)
			}
			if got.Manager != tc.want.Manager {
				t.Errorf("Manager = %q, want %q", got.Manager, tc.want.Manager)
			}
			if got.Ecosystem != tc.want.Ecosystem {
				t.Errorf("Ecosystem = %q, want %q", got.Ecosystem, tc.want.Ecosystem)
			}
			if got.Verb != tc.want.Verb {
				t.Errorf("Verb = %q, want %q", got.Verb, tc.want.Verb)
			}
			if got.Package != tc.want.Package {
				t.Errorf("Package = %q, want %q", got.Package, tc.want.Package)
			}
			if got.Version != tc.want.Version {
				t.Errorf("Version = %q, want %q", got.Version, tc.want.Version)
			}
			if got.IsInstall != tc.want.IsInstall {
				t.Errorf("IsInstall = %v, want %v", got.IsInstall, tc.want.IsInstall)
			}
			if got.IsExec != tc.want.IsExec {
				t.Errorf("IsExec = %v, want %v", got.IsExec, tc.want.IsExec)
			}
			if got.Sudo != tc.want.Sudo {
				t.Errorf("Sudo = %v, want %v", got.Sudo, tc.want.Sudo)
			}
			if got.Unpinned != tc.want.Unpinned {
				t.Errorf("Unpinned = %v, want %v (pkg=%q ver=%q)", got.Unpinned, tc.want.Unpinned, got.Package, got.Version)
			}
		})
	}
}

// TestParseExecPackageFlag covers Fix 5: for exec verbs (npx, pnpm dlx, bun x)
// the package can ride on a `-p`/`--package[=]` flag value rather than be the
// first positional token. Those forms must bind the real package so it is NOT
// silently dropped (which would ALLOW a malicious package). It must also remain
// conservative: unrelated flags never produce a spurious package, and the
// positional fallback is unchanged when no package flag is present.
func TestParseExecPackageFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPkg  string
		wantExec bool
		wantOK   bool
	}{
		{
			name:     "npx --package= equals form",
			input:    "npx --package=evil-pkg run-bin",
			wantPkg:  "evil-pkg",
			wantExec: true,
			wantOK:   true,
		},
		{
			name:     "npx -p space form",
			input:    "npx -p evil-pkg run-bin",
			wantPkg:  "evil-pkg",
			wantExec: true,
			wantOK:   true,
		},
		{
			name:     "npx --package space form",
			input:    "npx --package evil-pkg run-bin",
			wantPkg:  "evil-pkg",
			wantExec: true,
			wantOK:   true,
		},
		{
			name:     "npx -p= equals form",
			input:    "npx -p=evil-pkg run-bin",
			wantPkg:  "evil-pkg",
			wantExec: true,
			wantOK:   true,
		},
		{
			name:     "pnpm dlx with --package",
			input:    "pnpm dlx --package=evil-pkg bin",
			wantPkg:  "evil-pkg",
			wantExec: true,
			wantOK:   true,
		},
		{
			name:     "package flag value carries a version",
			input:    "npx --package=evil-pkg@1.2.3 run-bin",
			wantPkg:  "evil-pkg",
			wantExec: true,
			wantOK:   true,
		},
		{
			// Conservative: an unrelated flag value must NOT be treated as a
			// package; the positional token wins.
			name:     "unrelated flag does not bind package",
			input:    "npx --yes create-app",
			wantPkg:  "create-app",
			wantExec: true,
			wantOK:   true,
		},
		{
			// No package flag → positional fallback unchanged (regression guard).
			name:     "positional package still works",
			input:    "npx create-app",
			wantPkg:  "create-app",
			wantExec: true,
			wantOK:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Parse(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("Parse(%q) ok=%v, want %v", tc.input, ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if got.Package != tc.wantPkg {
				t.Errorf("Package = %q, want %q", got.Package, tc.wantPkg)
			}
			if got.IsExec != tc.wantExec {
				t.Errorf("IsExec = %v, want %v", got.IsExec, tc.wantExec)
			}
			if !got.IsInstall {
				t.Errorf("IsInstall = false, want true (exec verbs are install-class)")
			}
		})
	}
}

// TestPackageFlagValueOnly directly unit-tests the helper so its boundary
// behavior is pinned independent of Parse.
func TestPackageFlagValueOnly(t *testing.T) {
	cases := map[string]string{
		"--package=evil-pkg run-bin": "evil-pkg",
		"-p=evil-pkg run-bin":        "evil-pkg",
		"--package evil-pkg run-bin": "evil-pkg",
		"-p evil-pkg run-bin":        "evil-pkg",
		"run-bin --package=late":     "late",
		"--yes create-app":           "", // no package flag
		"create-app":                 "", // positional only
		"-p":                         "", // flag with no value
		"-p --foo":                   "", // value is itself a flag → none
		"":                           "",
	}
	for in, want := range cases {
		if got := packageFlagValue(in); got != want {
			t.Errorf("packageFlagValue(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPkgparseImportsArePure enforces the pure-library contract on pkgparse.go.
// It AST-parses the source file and fails if any forbidden import is found.
// pkgparse MUST import only "strings" so that internal/policy can import it
// without breaking its own purity test.
func TestPkgparseImportsArePure(t *testing.T) {
	const srcPath = "pkgparse.go"
	src, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("reading %s: %v", srcPath, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, srcPath, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", srcPath, err)
	}

	forbidden := map[string]bool{
		"os":       true,
		"net":      true,
		"net/http": true,
		"io":       true,
		"sync":     true,
		"time":     true,
		"context":  true,
	}

	for _, imp := range f.Imports {
		path := imp.Path.Value
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		if forbidden[path] {
			t.Errorf("pkgparse.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
