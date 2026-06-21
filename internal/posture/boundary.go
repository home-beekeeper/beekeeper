package posture

// boundary.go — the SINGLE canonical enforcement-boundary statement (IPBND-01).
//
// This is the one source of truth for how Beekeeper describes WHERE install
// posture is prevented versus merely observed. Every later surface — the posture
// adapter comment (internal/check), the `beekeeper posture` view, `beekeeper
// check`/posture help text, and the docs/web copy — references these constants
// rather than re-typing the prose. Re-typed copies drift; keep ONE source so the
// honesty standard is enforced from a single place.
//
// Style: no em dashes, sentence case. This is the PRD boundary statement, lightly
// adapted. The boundary-statement test asserts it stays non-empty, em-dash-free,
// and names the four surfaces (hook, Sentry, gateway, shim-roadmap) so the
// content cannot silently drift.

// BoundaryStatement is the full canonical enforcement-boundary statement.
const BoundaryStatement = "Install posture is enforced pre-execution at the agent hook for hooked " +
	"harnesses that support it, inheriting each harness tier's caveats. For " +
	"harnesses with no pre-exec hook and for installs a person runs directly in a " +
	"terminal, it is observed and audited by the Sentry layer rather than " +
	"prevented. The MCP gateway is not a general install surface. The " +
	"package-manager shim that would extend pre-exec enforcement to every install, " +
	"including ones a user types directly, exists today as an experimental, " +
	"roadmap-tier surface and is not a headline v1.0 guarantee."

// BoundaryShort is the one-line short form for help text and compact UI.
const BoundaryShort = "Posture is prevented at the hook for hooked harnesses only; everything else Sentry observes and audits, it does not prevent."
