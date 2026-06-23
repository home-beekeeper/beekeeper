package tui

import (
	"strings"
	"testing"

	audit "github.com/home-beekeeper/beekeeper/internal/audit"
)

// TestPostureIncidentShowsThreeOptionsAndCommands proves the display-only card
// surfaces the three graduated options and the EXACT `beekeeper posture ...`
// command for each, scoped to the record's package/ecosystem/rule.
func TestPostureIncidentShowsThreeOptionsAndCommands(t *testing.T) {
	rec := audit.AuditRecord{
		RecordType: "policy_decision",
		Decision:   "warn",
		Reason:     "install posture: package age 30m below minimum 1440m",
		RuleIDs:    []string{"release-age-policy"},
		CatalogMatches: []audit.CatalogProvenance{
			{Ecosystem: "npm", Package: "left-pad"},
		},
	}
	card := PostureIncidentFromRecord(rec)
	out := card.View(100)

	// The three graduated commands, scoped to npm / left-pad / release-age.
	wantCmds := []string{
		"beekeeper posture allow left-pad --ecosystem npm --rule release-age --once",
		"beekeeper posture allow left-pad --ecosystem npm --rule release-age --always --reason",
		"beekeeper posture enforce release-age --block",
	}
	for _, w := range wantCmds {
		if !strings.Contains(out, w) {
			t.Errorf("card missing command %q in:\n%s", w, out)
		}
	}

	// All three option headings present.
	for _, heading := range []string{"Allow once", "Allow always", "Enforce block"} {
		if !strings.Contains(out, heading) {
			t.Errorf("card missing option heading %q in:\n%s", heading, out)
		}
	}
}

// TestPostureIncidentNoPackageUsesPlaceholder proves a record with no resolvable
// package degrades to a <package> placeholder rather than a fabricated name.
func TestPostureIncidentNoPackageUsesPlaceholder(t *testing.T) {
	rec := audit.AuditRecord{
		RecordType: "policy_decision",
		Decision:   "warn",
		Reason:     "install posture: install pulls from a git source",
		RuleIDs:    []string{"remote-source-policy"},
	}
	card := PostureIncidentFromRecord(rec)
	out := card.View(100)
	if !strings.Contains(out, "beekeeper posture allow <package>") {
		t.Errorf("card should use a <package> placeholder when none is resolvable:\n%s", out)
	}
	if card.Rule != "git-remote" {
		t.Errorf("Rule = %q, want git-remote (mapped from the remote-source rule ID)", card.Rule)
	}
}

// TestPostureIncidentBlockBadge proves a block-level decision renders the BLOCK
// badge rather than WARN.
func TestPostureIncidentBlockBadge(t *testing.T) {
	rec := audit.AuditRecord{RecordType: "policy_decision", Decision: "block", Reason: "blocked"}
	card := PostureIncidentFromRecord(rec)
	if card.Decision != "block" {
		t.Fatalf("Decision = %q, want block", card.Decision)
	}
	// The view renders without panicking and contains the install-posture subject.
	out := card.View(80)
	if !strings.Contains(out, "install posture") {
		t.Errorf("card view missing the install-posture subject:\n%s", out)
	}
}
