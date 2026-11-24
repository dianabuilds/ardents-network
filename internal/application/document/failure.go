package document

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/connection"
)

// Failure projects local document errors without exposing private details.
func Failure(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "read cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "read timed out"
	case errors.Is(err, connection.ErrInvalidTargetLink):
		return "document destination or local input is invalid"
	}
	if refusal, ok := errors.AsType[connection.SetupRefusalError](err); ok {
		switch refusal.Outcome().Class {
		case connection.LocalCancellation:
			return "read cancelled"
		case connection.LocalTimeout:
			return "read timed out"
		case connection.LocalFailure:
			return "local connection failed"
		case connection.IndeterminateFailure:
			return "connection outcome unknown"
		case connection.CapacityUnavailable:
			return "document capacity unavailable"
		}
	}
	return "document operation unavailable"
}
