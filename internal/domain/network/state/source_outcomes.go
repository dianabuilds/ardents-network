package state

import (
	"context"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/journal"
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
		return journal.OutcomeValid
	}
	if errors.Is(err, context.Canceled) {
		return journal.OutcomeCanceled
	}
	for status := byte(1); status < byte(len(sourceStatusErrors)); status++ {
		if errors.Is(err, sourceStatusErrors[status]) {
			return [...]byte{journal.OutcomeValid, journal.OutcomeNotFound, journal.OutcomeBusy, journal.OutcomeBadRequest, journal.OutcomeInternal}[status]
		}
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError) && networkError.Timeout() {
		return journal.OutcomeTimeout
	}
	switch {
	case errors.Is(err, source.ErrAuthentication):
		return journal.OutcomeAuthentication
	case errors.Is(err, source.ErrUnavailable):
		return journal.OutcomeUnavailable
	case errors.Is(err, source.ErrFraming), errors.Is(err, errSourceObjectMismatch):
		return journal.OutcomeFraming
	default:
		return journal.OutcomeInvalidState
	}
}

var sourceOutcomeNames = map[byte]string{
	0: "not-attempted", journal.OutcomeValid: "valid", journal.OutcomeUnavailable: "unavailable",
	journal.OutcomeAuthentication: "authentication-failed", journal.OutcomeTimeout: "timeout",
	journal.OutcomeFraming: "framing-failed", journal.OutcomeResource: "resource-failed",
	journal.OutcomeCanceled: "canceled", journal.OutcomeInvalidState: "invalid-state",
	journal.OutcomeInterrupted: "interrupted",
	journal.OutcomeNotFound:    "not-found", journal.OutcomeBusy: "busy",
	journal.OutcomeBadRequest: "bad-request", journal.OutcomeInternal: "source-internal",
}

func sourceOutcomeName(outcome byte) string {
	return sourceOutcomeNames[outcome]
}
