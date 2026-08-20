package releasedecision

// Outcome is one bounded runtime result of an authenticated release
// evaluation. The string values are the lifecycle-spec names; comparison is
// value-based so callers can render, log, or branch on the result.
type Outcome string

// The accepted runtime outcomes are defined as unexported constants. They
// match the Stage 7 lifecycle specification exactly and are stable across
// versions; the package, the offline-import caller, and the evidence cells
// compare against these values. Callers compare a returned Outcome against
// the string literal of the same name when they cannot import the
// unexported name.
const (
	outcomeReleaseAccepted     Outcome = "release-accepted"
	outcomeNoUpdate            Outcome = "no-update"
	outcomeUpdateRequired      Outcome = "update-required"
	outcomeReleaseExpired      Outcome = "release-expired"
	outcomeReleaseConflict     Outcome = "release-conflict"
	outcomeReleaseRevoked      Outcome = "release-revoked"
	outcomeReleaseIncompatible Outcome = "release-incompatible"
	outcomeReleaseUnavailable  Outcome = "release-unavailable"
	outcomeReleaseInvalid      Outcome = "release-invalid"
)
