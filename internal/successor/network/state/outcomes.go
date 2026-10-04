package state

import (
	"context"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

const (
	sourceOutcomeValid byte = iota + 1
	sourceOutcomeUnavailable
	sourceOutcomeAuthentication
	sourceOutcomeTimeout
	sourceOutcomeFraming
	sourceOutcomeResource
	sourceOutcomeCanceled
	sourceOutcomeInvalidState
	sourceOutcomeInterrupted
	sourceOutcomeNotFound
	sourceOutcomeBusy
	sourceOutcomeBadRequest
	sourceOutcomeInternal
)

var sourceStatusErrors = [...]error{
	nil,
	errors.New("source returned NOT_FOUND"),
	errors.New("source returned BUSY"),
	errors.New("source returned BAD_REQUEST"),
	errors.New("source returned INTERNAL"),
}

func sourceStatusError(status string) error {
	return map[string]error{
		"not-found":   sourceStatusErrors[1],
		"busy":        sourceStatusErrors[2],
		"bad-request": sourceStatusErrors[3],
		"internal":    sourceStatusErrors[4],
	}[status]
}

func classifySourceOutcome(err error) byte {
	if err == nil {
		return sourceOutcomeValid
	}
	if errors.Is(err, context.Canceled) {
		return sourceOutcomeCanceled
	}
	for status := byte(1); status < byte(len(sourceStatusErrors)); status++ {
		if errors.Is(err, sourceStatusErrors[status]) {
			return [...]byte{sourceOutcomeValid, sourceOutcomeNotFound, sourceOutcomeBusy, sourceOutcomeBadRequest, sourceOutcomeInternal}[status]
		}
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
		return sourceOutcomeTimeout
	}
	switch {
	case errors.Is(err, source.ErrAuthentication):
		return sourceOutcomeAuthentication
	case errors.Is(err, source.ErrUnavailable):
		return sourceOutcomeUnavailable
	case errors.Is(err, source.ErrFraming), errors.Is(err, errSourceObjectMismatch):
		return sourceOutcomeFraming
	default:
		return sourceOutcomeInvalidState
	}
}

func sourceOutcomeName(outcome byte) string {
	return map[byte]string{
		0: "not-attempted", sourceOutcomeValid: "valid", sourceOutcomeUnavailable: "unavailable",
		sourceOutcomeAuthentication: "authentication-failed", sourceOutcomeTimeout: "timeout",
		sourceOutcomeFraming: "framing-failed", sourceOutcomeResource: "resource-failed",
		sourceOutcomeCanceled: "canceled", sourceOutcomeInvalidState: "invalid-state",
		sourceOutcomeInterrupted: "interrupted",
		sourceOutcomeNotFound:    "not-found", sourceOutcomeBusy: "busy",
		sourceOutcomeBadRequest: "bad-request", sourceOutcomeInternal: "source-internal",
	}[outcome]
}
