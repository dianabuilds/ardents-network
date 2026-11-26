package state

import (
	"context"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

var sourceStatusErrors = [...]error{
	nil,
	errors.New("source returned NOT_FOUND"),
	errors.New("source returned BUSY"),
	errors.New("source returned BAD_REQUEST"),
	errors.New("source returned INTERNAL"),
}

// errUnknownSourceStatus keeps an unrecognized non-ok status fail-closed. The
// production framing guard already rejects unknown status bytes, but the
// injected fetch seam must not inherit a silent success default.
var errUnknownSourceStatus = errors.New("source returned an unknown status")

func sourceStatusError(status string) error {
	switch status {
	case "not-found":
		return sourceStatusErrors[1]
	case "busy":
		return sourceStatusErrors[2]
	case "bad-request":
		return sourceStatusErrors[3]
	case "internal":
		return sourceStatusErrors[4]
	default:
		return errUnknownSourceStatus
	}
}

func classifySourceOutcome(err error) byte {
	if err == nil {
		return distribution.OutcomeValid
	}
	if errors.Is(err, context.Canceled) {
		return distribution.OutcomeCanceled
	}
	for status := byte(1); status < byte(len(sourceStatusErrors)); status++ {
		if errors.Is(err, sourceStatusErrors[status]) {
			return [...]byte{distribution.OutcomeValid, distribution.OutcomeNotFound, distribution.OutcomeBusy, distribution.OutcomeBadRequest, distribution.OutcomeInternal}[status]
		}
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
		return distribution.OutcomeTimeout
	}
	switch {
	case errors.Is(err, source.ErrAuthentication):
		return distribution.OutcomeAuthentication
	case errors.Is(err, source.ErrUnavailable):
		return distribution.OutcomeUnavailable
	case errors.Is(err, source.ErrFraming), errors.Is(err, errSourceObjectMismatch):
		return distribution.OutcomeFraming
	default:
		return distribution.OutcomeInvalidState
	}
}

var sourceOutcomeNames = map[byte]string{
	0: "not-attempted", distribution.OutcomeValid: "valid", distribution.OutcomeUnavailable: "unavailable",
	distribution.OutcomeAuthentication: "authentication-failed", distribution.OutcomeTimeout: "timeout",
	distribution.OutcomeFraming: "framing-failed", distribution.OutcomeResource: "resource-failed",
	distribution.OutcomeCanceled: "canceled", distribution.OutcomeInvalidState: "invalid-state",
	distribution.OutcomeInterrupted: "interrupted",
	distribution.OutcomeNotFound:    "not-found", distribution.OutcomeBusy: "busy",
	distribution.OutcomeBadRequest: "bad-request", distribution.OutcomeInternal: "source-internal",
}

func sourceOutcomeName(outcome byte) string {
	return sourceOutcomeNames[outcome]
}
