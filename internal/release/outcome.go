package release

// Outcome is one bounded runtime result of an authenticated release
// evaluation. The string values are the lifecycle-spec names; comparison is
// value-based so callers can render, log, or branch on the result.
type Outcome string

// The accepted runtime outcomes match the Stage 7 lifecycle specification
// exactly and are stable across versions. They are for display and branching,
// never authorization: only Authorization carries release acceptance into
// update.
const (
	OutcomeReleaseAccepted     Outcome = "release-accepted"
	OutcomeNoUpdate            Outcome = "no-update"
	OutcomeUpdateRequired      Outcome = "update-required"
	OutcomeReleaseExpired      Outcome = "release-expired"
	OutcomeReleaseConflict     Outcome = "release-conflict"
	OutcomeReleaseRevoked      Outcome = "release-revoked"
	OutcomeReleaseIncompatible Outcome = "release-incompatible"
	OutcomeReleaseUnavailable  Outcome = "release-unavailable"
	OutcomeReleaseInvalid      Outcome = "release-invalid"
)

const (
	outcomeReleaseAccepted     = OutcomeReleaseAccepted
	outcomeNoUpdate            = OutcomeNoUpdate
	outcomeUpdateRequired      = OutcomeUpdateRequired
	outcomeReleaseExpired      = OutcomeReleaseExpired
	outcomeReleaseConflict     = OutcomeReleaseConflict
	outcomeReleaseRevoked      = OutcomeReleaseRevoked
	outcomeReleaseIncompatible = OutcomeReleaseIncompatible
	outcomeReleaseUnavailable  = OutcomeReleaseUnavailable
	outcomeReleaseInvalid      = OutcomeReleaseInvalid
)
