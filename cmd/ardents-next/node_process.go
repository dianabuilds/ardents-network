package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/processresource"
)

// inspectNodeProcess drives the same genuine process owner as the original
// Forwarding composition. It grants no duty or readiness and emits no native
// path/error text. Queue and timer counts are explicitly unmeasured here.
func inspectNodeProcess(ctx context.Context, out io.Writer) int {
	guard, err := processresource.Open(ctx, processresource.Profile)
	if err != nil {
		return writeNodeProcessResult(out, "unavailable", nil)
	}
	observation, err := guard.Check()
	closeErr := guard.Close()
	if err != nil || closeErr != nil {
		return writeNodeProcessResult(out, "unavailable", nil)
	}
	return writeNodeProcessResult(out, "observed", &observation)
}

func writeNodeProcessResult(out io.Writer, outcome string, observation *processresource.Observation) int {
	result := struct {
		Operation   string                       `json:"operation"`
		Outcome     string                       `json:"outcome"`
		Queues      string                       `json:"queues"`
		Timers      string                       `json:"timers"`
		Observation *processresource.Observation `json:"observation,omitempty"`
	}{"node.inspect-process", outcome, "unmeasured", "unmeasured", observation}
	if json.NewEncoder(out).Encode(result) != nil {
		return 2
	}
	if outcome != "observed" {
		return 1
	}
	return 0
}
