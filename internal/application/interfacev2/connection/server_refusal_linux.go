//go:build linux

package connection

import (
	"errors"
)

func (failure refusalError) Error() string {
	if failure.outcome.Reason == "" {
		return string(failure.outcome.Class)
	}
	return string(failure.outcome.Class) + ": " + failure.outcome.Reason
}

// Refuse returns a typed refusal for a server-side Interface implementation.
func Refuse(outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	return refusalError{outcome: outcome}
}

func refusal(cause error) Outcome {
	var classified refusalError
	if errors.As(cause, &classified) {
		return classified.outcome
	}
	return Outcome{Class: ServiceUnavailable, Reason: "Endpoint could not open the selected Target Link"}
}

type refusalError struct{ outcome Outcome }
