// Package corpus holds the typed Go schema and push-envelope wire format for the
// Beekeeper corpus store. This package is PURE: no I/O, no goroutines, no side
// effects. All behavior is in types, constants, pure constructors, and invariant
// guards. The impure store and adjudication engine land in Phase 23.
package corpus

// ActionHint is the closed typed set of fleet-pushable action hints in a
// PushEnvelope. It is a named string type so the Go compiler rejects untyped
// string literals at assignment sites — assigning any string other than
// ActionHintWatchAndBlock to a PushEnvelope.ActionHint field is a compile error
// without an explicit type conversion.
//
// This is the SCHEMA-04 compile-time blast-radius guard: only one pushable
// action is defined and destructive fleet actions are intentionally unrepresentable
// in a well-typed PushEnvelope without an explicit unsafe conversion that a strict
// reviewer will immediately identify as a red flag.
//
// The Phase 23 BuildPushEnvelope builder validates the constant set at the
// API boundary, and the Phase 23 ENV-03 fuzz gate proves no code path can
// emit a non-allowlisted action_hint at runtime. Phase 22 provides the
// type-level guarantee; Phase 23 provides the builder + fuzz belt-and-suspenders.
type ActionHint string

// ActionHintWatchAndBlock is the sole fleet-pushable action hint in the
// allowed set. It instructs a receiving machine to:
//
//   - Raise a Sentry watch on the process tree associated with the flagged package.
//   - Block new installs of that version.
//   - Arm the local quarantine card.
//
// Purge and other destructive fleet actions are deliberately NOT defined as
// constants and are therefore unrepresentable in a well-typed PushEnvelope.
// Any future contributor who needs to introduce a new action hint must add a
// new ActionHint constant here and update the Phase 23 builder allowlist —
// the type system forces the conversation.
const ActionHintWatchAndBlock ActionHint = "watch_and_block"
