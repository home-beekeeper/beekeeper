package policy

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestLifecycleScriptPresentNotAllowlisted: postinstall script + not allowlisted → block.
func TestLifecycleScriptPresentNotAllowlisted(t *testing.T) {
	input := LifecycleInput{
		Ecosystem:      "npm",
		Package:        "some-pkg",
		ScriptsPresent: []string{"postinstall"},
	}
	d := EvaluateLifecycle(input, nil)

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (lifecycle script not allowlisted must block)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (lifecycle script not allowlisted must block)")
	}
	if !strings.Contains(d.Reason, "postinstall") {
		t.Errorf("Reason %q should mention the script name 'postinstall'", d.Reason)
	}
	if len(d.RuleIDs) == 0 {
		t.Errorf("RuleIDs is empty, want lifecycle rule ID")
	}
}

// TestLifecycleScriptPresentAllowlisted: postinstall + package IS allowlisted → allow.
func TestLifecycleScriptPresentAllowlisted(t *testing.T) {
	input := LifecycleInput{
		Ecosystem:      "npm",
		Package:        "trusted-pkg",
		ScriptsPresent: []string{"postinstall"},
	}
	d := EvaluateLifecycle(input, []string{"trusted-pkg"})

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (allowlisted package must be exempt)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (allowlisted package must be exempt)")
	}
	if !strings.Contains(d.Reason, "allowlist") {
		t.Errorf("Reason %q should mention 'allowlist'", d.Reason)
	}
}

// TestLifecycleNoScriptsAllowed: no lifecycle scripts → allow.
func TestLifecycleNoScriptsAllowed(t *testing.T) {
	input := LifecycleInput{
		Ecosystem:      "npm",
		Package:        "safe-pkg",
		ScriptsPresent: []string{},
	}
	d := EvaluateLifecycle(input, nil)

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (no scripts must allow)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (no scripts must allow)")
	}
}

// TestLifecycleRegistryCheckFailedBlocks: RegistryCheckFailed true → fail-closed block.
func TestLifecycleRegistryCheckFailedBlocks(t *testing.T) {
	input := LifecycleInput{
		Ecosystem:           "npm",
		Package:             "some-pkg",
		ScriptsPresent:      nil,
		RegistryCheckFailed: true,
	}
	d := EvaluateLifecycle(input, nil)

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q (registry check failure must fail closed)", d.Level, "block")
	}
	if d.Allow {
		t.Errorf("Allow = true, want false (registry check failure must fail closed)")
	}
	if !strings.Contains(d.Reason, "unavailable") {
		t.Errorf("Reason %q should contain 'unavailable'", d.Reason)
	}
}

// TestLifecycleNilScriptsAllowed: nil ScriptsPresent (not populated) → allow.
func TestLifecycleNilScriptsAllowed(t *testing.T) {
	input := LifecycleInput{
		Ecosystem:      "npm",
		Package:        "another-pkg",
		ScriptsPresent: nil,
	}
	d := EvaluateLifecycle(input, nil)

	if d.Level != "allow" {
		t.Errorf("Level = %q, want %q (nil scripts must allow)", d.Level, "allow")
	}
	if !d.Allow {
		t.Errorf("Allow = false, want true (nil scripts must allow)")
	}
}

// TestLifecycleMultipleScriptsReason: multiple scripts named in block reason.
func TestLifecycleMultipleScriptsReason(t *testing.T) {
	input := LifecycleInput{
		Ecosystem:      "npm",
		Package:        "multi-script-pkg",
		ScriptsPresent: []string{"preinstall", "postinstall"},
	}
	d := EvaluateLifecycle(input, nil)

	if d.Level != "block" {
		t.Errorf("Level = %q, want %q", d.Level, "block")
	}
	if !strings.Contains(d.Reason, "preinstall") {
		t.Errorf("Reason %q should contain 'preinstall'", d.Reason)
	}
	if !strings.Contains(d.Reason, "postinstall") {
		t.Errorf("Reason %q should contain 'postinstall'", d.Reason)
	}
}

// TestLifecycleEachScriptTypeFires asserts every lifecycle script type the
// evaluator can be handed fires the block (and is named in the reason), not just
// postinstall. EvaluateLifecycle treats ANY non-empty ScriptsPresent entry as a
// fired rule and joins the names into the reason; this table locks that each of
// the npm install-time hooks (preinstall, install, postinstall) AND the
// uninstall-time hooks (preuninstall, postuninstall) -- any of which the I/O
// adapter can surface in ScriptsPresent -- produces a block whose reason names
// the script.
func TestLifecycleEachScriptTypeFires(t *testing.T) {
	for _, script := range []string{
		"preinstall",
		"install",
		"postinstall",
		"prepare",
		"preuninstall",
		"postuninstall",
	} {
		t.Run(script, func(t *testing.T) {
			input := LifecycleInput{
				Ecosystem:      "npm",
				Package:        "scripted-pkg",
				ScriptsPresent: []string{script},
			}
			d := EvaluateLifecycle(input, nil)

			if d.Level != "block" {
				t.Errorf("Level = %q, want %q (a %s lifecycle script must block)", d.Level, "block", script)
			}
			if d.Allow {
				t.Errorf("Allow = true, want false (a %s lifecycle script must block)", script)
			}
			if !strings.Contains(d.Reason, script) {
				t.Errorf("Reason %q should name the firing script %q", d.Reason, script)
			}
			if len(d.RuleIDs) == 0 {
				t.Errorf("RuleIDs is empty, want the lifecycle rule ID for a %s block", script)
			}
		})
	}
}

// TestLifecycleImportsArePure enforces the pure-library contract on lifecycle.go.
func TestLifecycleImportsArePure(t *testing.T) {
	const srcPath = "lifecycle.go"
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
			t.Errorf("lifecycle.go imports forbidden package %q — violates pure-library contract", path)
		}
	}
}
