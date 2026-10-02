package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/successor/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/tokenissuance"
)

func runIssuanceResults(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	r := issuanceResult{Operation: "issuance", Phase: "input", Outcome: "invalid-input"}
	write := func() int {
		code := 1
		switch r.Outcome {
		case "initialized-results", "issued-offline", "already-issued":
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
	if args[1] != "--config" {
		return write()
	}
	r.Operation = "issuance." + args[0]
	if !issuance.Supported() {
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
	p, err := decodeIssuanceResultPlan(raw, args[0])
	clear(raw)
	if err != nil {
		return write()
	}
	var batch []byte
	if args[0] == "issue" {
		batch, err = readBounded(p.Admission.BatchFile, 16<<10)
		if err != nil {
			return write()
		}
		defer clear(batch)
	}
	o, _ := newObservation(endpoint)
	finish := o.beginIssuance(r.Operation)
	plan := tokenissuance.Plan{AdmissionRoot: p.Admission.Root, AdmissionBinding: p.Admission.Binding, KeyRoot: p.Keys.Root, KeyBinding: p.Keys.Binding, ResultRoot: p.Root}
	var result tokenissuance.Result
	if args[0] == "initialize-results" {
		result = tokenissuance.Initialize(ctx, plan)
	} else {
		result = tokenissuance.Issue(ctx, plan, batch, p.Admission.Facts, p.Admission.Kind)
	}
	r.Phase = result.Phase
	r.Outcome = result.Outcome
	if result.Response != nil {
		r.Phase = "export"
		if exportErr := exportIssuanceInventory(ctx, p.ResponseFile, result.Response); exportErr != nil {
			r.Outcome = issuanceOutcome(exportErr)
		}
	}
	finish(r.Phase, r.Outcome)
	_ = o.close()
	_ = json.NewEncoder(diagnostic).Encode(r)
	return write()
}
