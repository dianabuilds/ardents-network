//go:build linux

package tokens

import "errors"

// TransferFailure records the authority stage that rejected a token transfer
// so the transport layer can classify a consumption failure without
// interpreting permission internals.
type TransferFailure struct {
	stage string
	cause error
}

func (failure *TransferFailure) Error() string { return failure.cause.Error() }

func (failure *TransferFailure) Unwrap() error { return failure.cause }

func TransferFailureAt(stage string, cause error) error {
	return &TransferFailure{stage: stage, cause: cause}
}

func TransferFailureStage(cause error) string {
	var failure *TransferFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}
