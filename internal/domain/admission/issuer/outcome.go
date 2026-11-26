package issuer

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuerprofile/signing"
)

// ErrorOutcome returns the original offline Issuance refusal category.
func ErrorOutcome(err error) string {
	switch {
	case err == nil:
		return "completed"
	case errors.Is(err, issuance.ErrUncertain), errors.Is(err, signing.ErrUncertain):
		return "storage-uncertain"
	case errors.Is(err, issuance.ErrUnsupported), errors.Is(err, signing.ErrUnsupported):
		return "unsupported-platform"
	case errors.Is(err, issuance.ErrBusy), errors.Is(err, signing.ErrBusy):
		return "busy"
	case errors.Is(err, issuance.ErrConflict):
		return "request-conflict"
	case errors.Is(err, issuance.ErrValidity):
		return "outside-validity"
	case errors.Is(err, issuance.ErrCapacity):
		return "result-capacity"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, issuance.ErrInvalid), errors.Is(err, signing.ErrInvalid):
		return "invalid-input"
	default:
		return "storage-unavailable"
	}
}
