package hosting

import (
	"context"
	"errors"
)

// Outcome classifies a local Hosting operation without exposing private causes.
func Outcome(err error) string {
	switch {
	case errors.Is(err, ErrUncertain):
		return "storage-uncertain"
	case errors.Is(err, ErrUnsupportedPlatform):
		return "unsupported-platform"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, ErrInvalid):
		return "invalid-input"
	case errors.Is(err, ErrCapacity):
		return "budget-exhausted"
	case err != nil:
		return "budget-unavailable"
	default:
		return "completed"
	}
}

// LocalAvailability reports whether the local Hosting operation can open its store.
func LocalAvailability() string {
	if !Supported() {
		return Outcome(ErrUnsupportedPlatform)
	}
	return ""
}
