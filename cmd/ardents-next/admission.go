package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

type admissionResult struct {
	Operation string            `json:"operation"`
	Phase     string            `json:"phase"`
	Outcome   admission.Outcome `json:"outcome"`
}

func admissionError(err error) admission.Outcome {
	switch {
	case err == nil:
		return admission.Debited
	case errors.Is(err, admission.ErrUncertain):
		return admission.Uncertain
	case errors.Is(err, admission.ErrUnsupported):
		return admission.Unsupported
	case errors.Is(err, admission.ErrBusy):
		return admission.Busy
	case errors.Is(err, admission.ErrInvalid):
		return admission.InvalidInput
	default:
		return admission.Unavailable
	}
}
func runAdmission(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "prepare-binding" {
		return runIssuerProfile(ctx, "admission.prepare-binding", args[1:], out, diagnostic)
	}
	r := admissionResult{"admission", "input", admission.InvalidInput}
	write := func() int {
		code := 1
		switch r.Outcome {
		case admission.Initialized, admission.Debited, admission.AlreadyDebited:
			code = 0
		case admission.InvalidInput:
			code = 2
		case admission.Canceled:
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
	if args[1] != "--config" || args[0] != "initialize" && args[0] != "debit" {
		return write()
	}
	r.Operation = "admission." + args[0]
	if !admission.Supported() {
		r.Outcome = admission.Unsupported
		return write()
	}
	if ctx == nil {
		return write()
	}
	if ctx.Err() != nil {
		r.Outcome = admission.Canceled
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
	p, err := decodeAdmissionPlan(raw, args[0])
	if err != nil {
		return write()
	}
	var batch []byte
	if args[0] == "debit" {
		batch, err = readBounded(p.BatchFile, 16<<10)
		if err != nil {
			return write()
		}
		defer clear(batch)
	}
	o, telemetryErr := newObservation(endpoint)
	finish := o.beginAdmission(r.Operation)
	r.Phase = "execute"
	if ctx.Err() != nil {
		r.Outcome = admission.Canceled
	} else if args[0] == "initialize" {
		r.Outcome = admissionError(admission.Initialize(p.Root, p.Binding))
		if r.Outcome == admission.Debited {
			r.Outcome = admission.Initialized
		}
	} else {
		ledger, openErr := admission.Open(p.Root, p.Binding)
		if openErr != nil {
			r.Outcome = admissionError(openErr)
		} else {
			r.Outcome = ledger.Debit(ctx, batch, p.Facts, p.Kind)
			if ledger.Close() != nil {
				r.Phase = "close"
				r.Outcome = admission.Uncertain
			}
		}
	}
	finish(r.Phase, string(r.Outcome))
	ok := o.close()
	telemetry := "completed"
	if telemetryErr != nil || !ok {
		telemetry = "unavailable"
	}
	if endpoint == "" {
		telemetry = "local-only"
	}
	_ = json.NewEncoder(diagnostic).Encode(struct{ Operation, Phase, Outcome, Telemetry string }{r.Operation, r.Phase, string(r.Outcome), telemetry})
	return write()
}
