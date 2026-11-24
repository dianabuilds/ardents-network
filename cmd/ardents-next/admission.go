package main

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"runtime"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry"
)

type admissionResult struct {
	Operation string            `json:"operation"`
	Phase     string            `json:"phase"`
	Outcome   admission.Outcome `json:"outcome"`
}

func admissionInitializationOutcome(err error) admission.Outcome {
	switch {
	case err == nil:
		return quota.Initialized
	case errors.Is(err, quota.ErrUncertain):
		return quota.Uncertain
	case errors.Is(err, quota.ErrUnsupported):
		return quota.Unsupported
	case errors.Is(err, quota.ErrBusy):
		return quota.Busy
	case errors.Is(err, quota.ErrInvalid):
		return admission.InvalidInput
	default:
		return quota.Unavailable
	}
}
func runAdmission(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "holder", "receiver", "allocate", "issue-current":
			return runAdmissionLocal(ctx, args[0], args[1:], out, diagnostic)
		}
	}
	if len(args) > 0 && args[0] == "prepare-binding" {
		return runIssuerProfile(ctx, "admission.prepare-binding", args[1:], out, diagnostic)
	}
	r := admissionResult{"admission", "input", admission.InvalidInput}
	write := func() int {
		code := 1
		switch r.Outcome {
		case quota.Initialized:
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
	if args[1] != "--config" || args[0] != "initialize" {
		return write()
	}
	r.Operation = "admission." + args[0]
	if runtime.GOOS != "linux" || !quota.Supported() {
		r.Outcome = quota.Unsupported
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
	if telemetry.ValidateCollector(endpoint) != nil {
		return write()
	}
	raw, err := readBounded(args[2], 16<<10)
	if err != nil {
		return write()
	}
	defer clear(raw)
	p, err := decodeAdmissionPlan(raw)
	if err != nil {
		return write()
	}
	o, telemetryErr := telemetry.Open(endpoint)
	finish := o.Begin("ardents.admission", r.Operation)
	r.Phase = "execute"
	if ctx.Err() != nil {
		r.Outcome = admission.Canceled
	} else {
		r.Outcome = admissionInitializationOutcome(quota.Initialize(p.Root, p.Binding))
	}
	finish(r.Phase, string(r.Outcome))
	ok := o.Close()
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
