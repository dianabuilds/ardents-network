package serving

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/processresource"
)

// Outcome is the original attempt's process-independent terminal class.
type Outcome uint8

const (
	Refused Outcome = iota + 1
	Invalid
	Canceled
)

// Event is the bounded observation presented by the command. The causes
// retain physical failures for State and diagnostic classification.
type Event struct {
	Phase, Outcome        string
	Failure, ProcessClose error
}

// Terminal keeps the original failure order through physical join.
type Terminal struct {
	FirstOwner                                   string
	First, Process, Wait, StateClose, InputClose error
}

func (*Terminal) Error() string { return "original Source failed" }

func (result *Terminal) Unwrap() []error {
	var causes []error
	for _, cause := range []error{result.First, result.Process, result.Wait, result.StateClose, result.InputClose} {
		if cause != nil {
			causes = append(causes, cause)
		}
	}
	return causes
}

// FailureDescription contains only fixed owner categories and bounded resource
// facts. It omits the original errors, paths, and private Source configuration.
type FailureDescription struct {
	FirstOwner    string
	CleanupOwners string
	Resource      *processresource.FailureFacts
}

// DescribeFailure derives the Source's failure order from the original joined
// operation and the separate process-guard close result.
func DescribeFailure(operation, processClose error) FailureDescription {
	first, cleanup := "", ""
	var terminal *Terminal
	if errors.As(operation, &terminal) {
		first = terminal.FirstOwner
		if processresource.Failure(terminal.First) != nil {
			first = "process"
		}
		if terminal.Wait != nil || terminal.StateClose != nil {
			cleanup = "state"
		}
		if terminal.InputClose != nil {
			if cleanup != "" {
				cleanup += ","
			}
			cleanup += "input"
		}
	}
	if processClose != nil {
		if cleanup != "" {
			cleanup += ","
		}
		cleanup += "process"
	}
	return FailureDescription{FirstOwner: first, CleanupOwners: cleanup, Resource: processresource.Failure(operation)}
}
