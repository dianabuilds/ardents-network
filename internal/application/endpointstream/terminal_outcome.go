package endpointstream

import (
	"errors"
	"unicode/utf8"
)

const (
	maximumOutcomeClassBytes  = 128
	maximumOutcomeReasonBytes = 512
)

const (
	CleanClose           OutcomeClass = "clean service connection close"
	ServiceUnavailable   OutcomeClass = "service unavailable"
	CapacityUnavailable  OutcomeClass = "insufficient local or network capacity"
	LocalFailure         OutcomeClass = "local attachment failure"
	LocalCancellation    OutcomeClass = "local cancellation"
	LocalTimeout         OutcomeClass = "local timeout or cancellation"
	IndeterminateFailure OutcomeClass = "indeterminate failure"
)

// OutcomeClass is one bounded terminal classification. The shared constants
// cover transport-owned results; an Endpoint implementation may add a
// domain-owned class, but the local transport admits only 1..128 bytes.
type OutcomeClass string

// Outcome is the single terminal result returned to the local Application.
// Reason is diagnostic, limited to 512 bytes by the local transport, and
// never carries a Target,
// Route, peer, credential, or Application bytes.
type Outcome struct {
	Class  OutcomeClass
	Reason string
}

// SetupRefusalError carries one bounded AAI4 setup refusal to the local
// caller. It keeps the class distinct from unclassified Endpoint failures.
// Its Outcome is the exact bounded value decoded from the local transport.
type SetupRefusalError struct{ outcome Outcome }

func (failure SetupRefusalError) Error() string {
	if failure.outcome.Reason == "" {
		return string(failure.outcome.Class)
	}
	return string(failure.outcome.Class) + ": " + failure.outcome.Reason
}

// Outcome returns the bounded class and diagnostic carried by this refusal.
func (failure SetupRefusalError) Outcome() Outcome { return failure.outcome }

func validOutcome(outcome Outcome) error {
	if len(outcome.Class) == 0 || len(outcome.Class) > maximumOutcomeClassBytes ||
		len(outcome.Reason) > maximumOutcomeReasonBytes ||
		!utf8.ValidString(string(outcome.Class)) || !utf8.ValidString(outcome.Reason) {
		return errors.New("application Connection terminal outcome is invalid")
	}
	return nil
}

// Refuse returns a typed refusal for a server-side Interface implementation.
func Refuse(outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	return SetupRefusalError{outcome: outcome}
}
