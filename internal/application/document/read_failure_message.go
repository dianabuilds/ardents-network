package document

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// Failure projects local document errors without exposing private details.
func Failure(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "read cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "read timed out"
	case errors.Is(err, endpointstream.ErrInvalidLocalInput):
		return "document destination or local input is invalid"
	}
	if refusal, ok := errors.AsType[endpointstream.SetupRefusalError](err); ok {
		switch refusal.Outcome().Class {
		case endpointstream.LocalCancellation:
			return "read cancelled"
		case endpointstream.LocalTimeout:
			return "read timed out"
		case endpointstream.LocalFailure:
			return "local connection failed"
		case endpointstream.IndeterminateFailure:
			return "connection outcome unknown"
		case endpointstream.CapacityUnavailable:
			return "document capacity unavailable"
		}
	}
	return "document operation unavailable"
}
