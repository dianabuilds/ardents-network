package quota

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
)

// InitializeResult initializes one offline ledger and projects its bounded outcome.
func InitializeResult(ctx context.Context, root string, binding LedgerBinding) admission.Outcome {
	if ctx == nil {
		return admission.InvalidInput
	}
	if ctx.Err() != nil {
		return admission.Canceled
	}
	err := Initialize(root, binding)
	switch {
	case err == nil:
		return Initialized
	case errors.Is(err, ErrUncertain):
		return Uncertain
	case errors.Is(err, ErrUnsupported):
		return Unsupported
	case errors.Is(err, ErrBusy):
		return Busy
	case errors.Is(err, ErrInvalid):
		return admission.InvalidInput
	default:
		return Unavailable
	}
}

// OperatorAvailability reports the selected local operation refusal before input access.
func OperatorAvailability() admission.Outcome {
	if !admission.SupportsOfflineOperatorCommands() || !Supported() {
		return Unsupported
	}
	return ""
}
