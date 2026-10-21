//go:build linux

package main

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

func sourceStartupFailure(out io.Writer, owner string, cause, cleanup error) int {
	if processresource.Failure(cause) != nil {
		owner = "process"
	}
	if sourceFailureReport(out, "refused", &sourceTerminal{firstOwner: owner, first: cause}, cleanup) != nil {
		return 2
	}
	return 1
}

func sourceFailureReport(out io.Writer, phase string, operation, processClose error) error {
	first, cleanup := "", ""
	var terminal *sourceTerminal
	if errors.As(operation, &terminal) {
		first = terminal.firstOwner
		if processresource.Failure(terminal.first) != nil {
			first = "process"
		}
		if terminal.wait != nil || terminal.cleanup != nil {
			cleanup = "state"
		}
		if terminal.inputCleanup != nil {
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
	raw, err := json.Marshal(struct {
		Operation string                        `json:"operation"`
		Phase     string                        `json:"phase"`
		Outcome   string                        `json:"outcome"`
		First     string                        `json:"first_owner,omitempty"`
		Cleanup   string                        `json:"cleanup_owner,omitempty"`
		Resource  *processresource.FailureFacts `json:"resource,omitempty"`
	}{"source.serve", phase, "failed", first, cleanup, processresource.Failure(operation)})
	if err != nil {
		return err
	}
	return writeSourceReport(out, append(raw, '\n'))
}
