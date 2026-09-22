//go:build linux

package connection

import (
	"context"
	"errors"
)

// Refuse returns a typed refusal for a server-side Interface implementation.
func Refuse(outcome Outcome) error {
	if err := validOutcome(outcome); err != nil {
		return err
	}
	return SetupRefusalError{outcome: outcome}
}

func refusal(cause error) Outcome {
	if errors.Is(cause, context.DeadlineExceeded) {
		return Outcome{Class: LocalTimeout, Reason: "local Application setup timed out"}
	}
	if errors.Is(cause, context.Canceled) {
		return Outcome{Class: LocalCancellation, Reason: "local Application setup was cancelled"}
	}
	var classified SetupRefusalError
	if errors.As(cause, &classified) {
		return classified.outcome
	}
	return Outcome{Class: ServiceUnavailable, Reason: "Endpoint could not open the selected Target Link"}
}
