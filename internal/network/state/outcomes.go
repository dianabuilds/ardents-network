package state

import (
	"context"
	"errors"
	"math"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
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

func finishWaveState(state *distributionState, now time.Time, outcomes [4]byte) error {
	state.sequence++
	state.cycleActive = false
	state.outcomes = outcomes
	for index, status := range state.attempts {
		if status == sourceAttemptInFlight {
			state.attempts[index] = sourceAttemptFailed
		}
	}
	for index, outcome := range outcomes {
		if outcome == sourceOutcomeValid {
			state.attempts[index] = sourceAttemptCompleted
		} else if outcome != 0 {
			state.attempts[index] = sourceAttemptFailed
		}
	}
	for _, outcome := range outcomes {
		if outcome != 0 && outcome != sourceOutcomeValid {
			return applyFailureBackoff(state, now, state.cycleSeed)
		}
	}
	state.consecutiveFailures, state.backoffLevel, state.nextAutomatic = 0, 0, 0
	return nil
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

func applyFailureBackoff(state *distributionState, now time.Time, seed [32]byte) error {
	if state.consecutiveFailures < math.MaxInt64 {
		state.consecutiveFailures++
	}
	level := state.consecutiveFailures - 1
	if level > 5 {
		level = 5
	}
	state.backoffLevel = byte(level)
	bases := [...]int64{60, 120, 240, 480, 960, 1800}
	base := bases[state.backoffLevel]
	state.nextAutomatic = now.Unix() + base/2 + int64(seed[1])*(base/2+1)/256
	return nil
}
