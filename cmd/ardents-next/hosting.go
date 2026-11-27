package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
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
	if !hosting.Supported() {
		r.Outcome = hostingOutcome(hosting.ErrUnsupportedPlatform)
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
	p, err := decodeHostingConfig(raw, args[0])
	if err != nil {
		return write()
	}
	o, telemetryErr := telemetry.Open(endpoint)
	observedOperation := runtimeevent.LookupOperation(r.Operation)
	finish := o.Begin(observedOperation)
	r.Phase = "execute"
	result, err := executeHosting(ctx, args[0], p)
	if result.Sampled {
		observation, at, until := result.Snapshot.Observation(), result.Snapshot.ObservedAt(), result.Snapshot.ValidUntil()
		r.Observation, r.ObservedAt, r.ValidUntil = &observation, &at, &until
	}
	if result.CloseFailed {
		r.Phase = "close"
	}
	r.Outcome = hostingOutcome(err)
	finish(observedOperation.Classify(r.Phase, r.Outcome))
	delivery := o.CloseWithStatus(telemetryErr)
	_ = json.NewEncoder(diagnostic).Encode(struct{ Operation, Phase, Outcome, Telemetry string }{r.Operation, r.Phase, string(r.Outcome), string(delivery)})
	return write()
}

// executeHosting initializes a budget or obtains a joined observation.
func executeHosting(ctx context.Context, operation string, plan hostingConfig) (hosting.SampleResult, error) {
	switch operation {
	case "initialize":
		return hosting.SampleResult{}, hosting.InitializeContext(ctx, plan.Root, plan.Policy)
	case "observe":
		return hosting.SampleRoot(ctx, plan.Root, time.Second)
	default:
		return hosting.SampleResult{}, hosting.ErrInvalid
	}
}

// hostingOutcome classifies a local Hosting operation without exposing private causes.
func hostingOutcome(err error) string {
	switch {
	case errors.Is(err, hosting.ErrUncertain):
		return "storage-uncertain"
	case errors.Is(err, hosting.ErrUnsupportedPlatform):
		return "unsupported-platform"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, hosting.ErrInvalid):
		return "invalid-input"
	case errors.Is(err, hosting.ErrCapacity):
		return "budget-exhausted"
	case err != nil:
		return "budget-unavailable"
	default:
		return "completed"
	}
}
