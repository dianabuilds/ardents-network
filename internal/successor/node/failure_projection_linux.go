//go:build linux

package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

// FailureDiagnostic retains only original owner-local categories and numeric
// context. It grants no authority and includes no identity, path or error text.
type FailureDiagnostic struct {
	Kind                string                        `json:"kind"`
	Phase               string                        `json:"phase"`
	Cause               string                        `json:"cause"`
	ElapsedMS           int64                         `json:"elapsed_ms"`
	DeadlineRemainingMS *int64                        `json:"deadline_remaining_ms,omitempty"`
	Resource            *processresource.FailureFacts `json:"resource,omitempty"`
}

type startupFailure struct {
	cause error
	fact  *FailureDiagnostic
}

type expiredProcessObservation struct{ observation processresource.Observation }

func (failure *expiredProcessObservation) Error() string {
	return "original process measurement expired during authority observation"
}

func expiredMeasurement(observation processresource.Observation) error {
	return &expiredProcessObservation{observation: observation}
}

func resourceDiagnostic(cause error) *processresource.FailureFacts {
	if fact := processresource.Failure(cause); fact != nil {
		return fact
	}
	var expired *expiredProcessObservation
	if !errors.As(cause, &expired) {
		return nil
	}
	observation := expired.observation
	return &processresource.FailureFacts{Phase: "authority-observation", Cause: "observation-stale", Observation: &observation}
}

func (failure *startupFailure) Error() string { return "original Node startup failed" }
func (failure *startupFailure) Unwrap() error { return failure.cause }

// StartupFailure returns detached facts from a physically joined rejected start.
func StartupFailure(err error) *FailureDiagnostic {
	var failure *startupFailure
	if !errors.As(err, &failure) || failure.fact == nil {
		return nil
	}
	copy := *failure.fact
	copy.Resource = resourceDiagnostic(failure.cause)
	if copy.DeadlineRemainingMS != nil {
		remaining := *copy.DeadlineRemainingMS
		copy.DeadlineRemainingMS = &remaining
	}
	return &copy
}

type failureRecord struct {
	started time.Time
	serving bool
	first   *FailureDiagnostic
}

// Called only under the original invocation's state lock. It performs no
// foreign observation, I/O or callback; copied errors supply no accepting fact.
func (record *failureRecord) retain(cause error, caller context.Context, owner string) {
	if record.first != nil || cause == nil {
		return
	}
	now := time.Now()
	phase := "startup"
	if record.serving {
		phase = "serving"
	}
	code := "owner-refusal"
	if owner == "network" {
		code = "authority-refusal"
	}
	if cause == context.Canceled {
		code = "canceled"
	} else if cause == context.DeadlineExceeded {
		code = "deadline"
	}
	resource := resourceDiagnostic(cause)
	if resource != nil {
		code = "resource-refusal"
	}
	if owner == "route" {
		code = "physical-role"
	}
	fact := &FailureDiagnostic{Kind: "logical", Phase: phase, Cause: code, ElapsedMS: now.Sub(record.started).Milliseconds(), Resource: resource}
	if caller != nil {
		if deadline, ok := caller.Deadline(); ok {
			remaining := deadline.Sub(now).Milliseconds()
			fact.DeadlineRemainingMS = &remaining
		}
	}
	record.first = fact
}

func (record *failureRecord) retainCleanup(cause error, caller context.Context, owner string) {
	if record.first != nil || cause == nil {
		return
	}
	record.retain(cause, caller, owner)
	record.first.Kind, record.first.Phase, record.first.Cause = "physical", "cleanup", "cleanup-failure"
}

func (record *failureRecord) snapshot(cause error) *FailureDiagnostic {
	if record.first == nil {
		return nil
	}
	copy := *record.first
	copy.Resource = resourceDiagnostic(cause)
	if copy.DeadlineRemainingMS != nil {
		remaining := *copy.DeadlineRemainingMS
		copy.DeadlineRemainingMS = &remaining
	}
	return &copy
}
