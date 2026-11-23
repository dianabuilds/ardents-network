//go:build linux

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"github.com/dianabuilds/ardents-network/internal/processresource"
)

func sourceStartupFailure(out io.Writer, owner string, cause, cleanup error) int {
	if processresource.Failure(cause) != nil {
		owner = "process"
	}
	if sourceFailureReport(out, "refused", &sourceTerminal{FirstOwner: owner, First: cause}, cleanup) != nil {
		return 2
	}
	return 1
}

func sourceFailureReport(out io.Writer, phase string, operation, processClose error) error {
	first, cleanup := "", ""
	var terminal *sourceTerminal
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
	return artifacts.WriteReport(context.Background(), out, append(raw, '\n'))
}
