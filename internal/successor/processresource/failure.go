package processresource

import (
	"errors"
	"time"
)

// FailureFacts is a detached, bounded local diagnostic, never admission evidence.
// A missing observation remains unavailable rather than measured zero.
type FailureFacts struct {
	Phase               string       `json:"phase"`
	Cause               string       `json:"cause"`
	ElapsedMS           *int64       `json:"elapsed_ms,omitempty"`
	DeadlineRemainingMS *int64       `json:"deadline_remaining_ms,omitempty"`
	Observation         *Observation `json:"observation,omitempty"`
}

type observedFailure struct {
	cause error
	facts FailureFacts
}

type admissionFailure struct{ reason string }

func (failure *admissionFailure) Error() string {
	return "original process admission refused: " + failure.reason
}

func (failure *observedFailure) Error() string { return "original process admission failed" }
func (failure *observedFailure) Unwrap() error { return failure.cause }

func retainFailure(cause error, phase string, observation Observation, original *originalProcess) error {
	now := time.Now()
	elapsed := now.Sub(original.started).Milliseconds()
	facts := FailureFacts{Phase: phase, ElapsedMS: &elapsed}
	facts.Cause = "native-evidence"
	var refusal *admissionFailure
	if errors.As(cause, &refusal) {
		facts.Cause = refusal.reason
	}
	if deadline, ok := original.caller.Deadline(); ok {
		remaining := deadline.Sub(now).Milliseconds()
		facts.DeadlineRemainingMS = &remaining
	}
	if !observation.Completed.IsZero() {
		facts.Observation = &observation
	}
	return &observedFailure{cause: cause, facts: facts}
}
