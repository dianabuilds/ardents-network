//go:build linux

package source

import "errors"

// PreparationFailure marks the local preparation boundary that prevented a
// scheduled publication from obtaining its next Source opening. It retains the
// original cause for ownership and cleanup decisions.
type preparationFailure struct {
	stage string
	cause error
}

func (failure *preparationFailure) Error() string { return failure.cause.Error() }

func (failure *preparationFailure) Unwrap() error { return failure.cause }

func PreparationFailureAt(stage string, cause error) error {
	return &preparationFailure{stage: stage, cause: cause}
}

func PreparationFailureStage(cause error) string {
	var failure *preparationFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

// PrefixFailure distinguishes the local stages which can stop an expired
// Source prefix from being replaced. It deliberately retains the original
// cause without exposing that cause through the headless event.
type prefixFailure struct {
	stage string
	cause error
}

func (failure *prefixFailure) Error() string { return failure.cause.Error() }

func (failure *prefixFailure) Unwrap() error { return failure.cause }

func PrefixFailureAt(stage string, cause error) error {
	return &prefixFailure{stage: stage, cause: cause}
}

func PrefixFailureStage(cause error) string {
	var failure *prefixFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}

// TokenPresentationFailure classifies a refused token presentation inside a
// Source opening without leaking the cause through the reported stage.
type tokenPresentationFailure struct {
	stage string
	cause error
}

func (failure *tokenPresentationFailure) Error() string { return failure.cause.Error() }

func (failure *tokenPresentationFailure) Unwrap() error { return failure.cause }

func TokenPresentationFailureAt(stage string, cause error) error {
	return &tokenPresentationFailure{stage: stage, cause: cause}
}

func TokenPresentationFailureStage(cause error) string {
	var failure *tokenPresentationFailure
	if errors.As(cause, &failure) && failure.stage != "" {
		return failure.stage
	}
	return "unknown"
}
