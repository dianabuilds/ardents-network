package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
)

type hostingResult struct {
	Operation   string               `json:"operation"`
	Phase       string               `json:"phase"`
	Outcome     string               `json:"outcome"`
	ObservedAt  *time.Time           `json:"observed_at,omitempty"`
	ValidUntil  *time.Time           `json:"valid_until,omitempty"`
	Observation *hosting.Observation `json:"observation,omitempty"`
}

func runHosting(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	r := hostingResult{Operation: "hosting", Phase: "input", Outcome: "invalid-input"}
	write := func() int {
		code := 1
		switch r.Outcome {
		case "completed":
			code = 0
		case "invalid-input":
			code = 2
		case "canceled":
			code = 130
		}
		if json.NewEncoder(out).Encode(r) != nil {
			return 2
		}
		return code
	}
	if len(args) != 3 && len(args) != 4 {
		return write()
	}
	if args[1] != "--config" || (args[0] != "initialize" && args[0] != "observe") {
		return write()
	}
	r.Operation = "hosting." + args[0]
	if outcome := hosting.LocalAvailability(); outcome != "" {
		r.Outcome = outcome
		return write()
	}
	if ctx == nil {
		return write()
	}
	if ctx.Err() != nil {
		r.Outcome = "canceled"
		return write()
	}
	endpoint := ""
	if len(args) == 4 {
		endpoint = args[3]
	}
	if telemetry.ValidateCollector(endpoint) != nil {
		return write()
	}
	raw, err := readBounded(args[2], 16<<10)
	if err != nil {
		return write()
	}
	defer clear(raw)
	p, err := hosting.DecodeCommandPlan(raw, args[0])
	if err != nil {
		return write()
	}
	o, telemetryErr := telemetry.Open(endpoint)
	finish := o.Begin("ardents.hosting", r.Operation)
	r.Phase = "execute"
	result, err := hosting.ExecuteLocal(ctx, args[0], p)
	if result.Sampled {
		observation, at, until := result.Snapshot.Observation(), result.Snapshot.ObservedAt(), result.Snapshot.ValidUntil()
		r.Observation, r.ObservedAt, r.ValidUntil = &observation, &at, &until
	}
	if result.CloseFailed {
		r.Phase = "close"
	}
	r.Outcome = hosting.Outcome(err)
	finish(r.Phase, r.Outcome)
	ok := o.Close()
	telemetry := "completed"
	if telemetryErr != nil || !ok {
		telemetry = "unavailable"
	}
	if endpoint == "" {
		telemetry = "local-only"
	}
	_ = json.NewEncoder(diagnostic).Encode(struct{ Operation, Phase, Outcome, Telemetry string }{r.Operation, r.Phase, r.Outcome, telemetry})
	return write()
}
