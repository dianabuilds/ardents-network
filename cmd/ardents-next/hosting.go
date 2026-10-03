package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

type hostingResult struct {
	Operation   string               `json:"operation"`
	Phase       string               `json:"phase"`
	Outcome     string               `json:"outcome"`
	ObservedAt  *time.Time           `json:"observed_at,omitempty"`
	ValidUntil  *time.Time           `json:"valid_until,omitempty"`
	Observation *hosting.Observation `json:"observation,omitempty"`
}

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
	if args[1] != "--config" || (args[0] != "initialize" && args[0] != "observe" && args[0] != "hold") {
		return write()
	}
	r.Operation = "hosting." + args[0]
	if !hosting.Supported() {
		r.Outcome = "unsupported-platform"
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
	if collectorEndpoint(endpoint) != nil {
		return write()
	}
	raw, err := readBounded(args[2], 16<<10)
	if err != nil {
		return write()
	}
	defer clear(raw)
	p, err := decodeHostingPlan(raw, args[0])
	if err != nil {
		return write()
	}
	o, telemetryErr := newObservation(endpoint)
	finish := o.beginHosting(r.Operation)
	r.Phase = "execute"
	err = executeHosting(ctx, args[0], p, &r, diagnostic)
	r.Outcome = hostingOutcome(err)
	finish(r.Phase, r.Outcome)
	ok := o.close()
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

func executeHosting(ctx context.Context, operation string, p hostingPlan, r *hostingResult, diagnostic io.Writer) (err error) {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if operation == "initialize" {
		return hosting.Initialize(p.Root, p.Policy)
	}
	b, err := hosting.Open(p.Root)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			r.Phase = "close"
			err = errors.Join(hosting.ErrUncertain, err, closeErr)
		}
	}()
	if operation == "observe" {
		view, e := b.Sample(ctx, time.Second)
		if e == nil {
			observation, at, until := view.Observation(), view.ObservedAt(), view.ValidUntil()
			r.Observation, r.ObservedAt, r.ValidUntil = &observation, &at, &until
		}
		return e
	}
	interval := time.Duration(p.HoldMS) * time.Millisecond
	holdUntil := time.Now().Add(interval)
	reservation, err := b.Reserve(ctx, hosting.ReservationRequest{Work: p.Work, Termination: p.Termination, WorkUntil: holdUntil, HoldUntil: holdUntil.Add(5 * time.Second)})
	if err != nil {
		return err
	}
	r.Phase = "hold"
	_ = json.NewEncoder(diagnostic).Encode(struct{ Operation, Phase, Outcome string }{r.Operation, "hold", "reserved"})
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if e := reservation.Release(cleanup); e != nil {
			r.Phase = "release"
			err = errors.Join(hosting.ErrUncertain, err, e)
		}
	}()
	// Waiting for the durable reservation cannot extend its original window.
	timer := time.NewTimer(time.Until(holdUntil))
	defer timer.Stop()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			r.Phase = "release"
			return nil
		case <-tick.C:
			view, e := b.Observe(ctx)
			if e != nil {
				return e
			}
			if view.Drain {
				return hosting.ErrCapacity
			}
		}
	}
}
