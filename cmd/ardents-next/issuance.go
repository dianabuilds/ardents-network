package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/issuerprofile/signing"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry"
)

type issuanceResult struct {
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Outcome   string `json:"outcome"`
}

func issuanceOutcome(err error) string {
	switch {
	case err == nil:
		return "completed"
	case errors.Is(err, issuance.ErrUncertain), errors.Is(err, signing.ErrUncertain):
		return "storage-uncertain"
	case errors.Is(err, issuance.ErrUnsupported), errors.Is(err, signing.ErrUnsupported):
		return "unsupported-platform"
	case errors.Is(err, issuance.ErrBusy), errors.Is(err, signing.ErrBusy):
		return "busy"
	case errors.Is(err, issuance.ErrConflict):
		return "request-conflict"
	case errors.Is(err, issuance.ErrValidity):
		return "outside-validity"
	case errors.Is(err, issuance.ErrCapacity):
		return "result-capacity"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, issuance.ErrInvalid), errors.Is(err, signing.ErrInvalid):
		return "invalid-input"
	default:
		return "storage-unavailable"
	}
}
func runIssuance(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "inspect-public-profile" {
		return runIssuerPublicInspection(ctx, args[1:], out)
	}
	if len(args) > 0 && (args[0] == "initialize-profile" || args[0] == "inspect-profile") {
		return runIssuerProfile(ctx, "issuance."+args[0], args[1:], out, diagnostic)
	}
	if len(args) > 0 && (args[0] == "issue" || args[0] == "initialize-results") {
		return runIssuanceResults(ctx, args, out, diagnostic)
	}
	r := issuanceResult{"issuance", "input", "invalid-input"}
	write := func() int {
		return writeAdmissionReport(out, r, r.Outcome, "completed")
	}
	if len(args) != 3 && len(args) != 4 {
		return write()
	}
	if args[1] != "--config" || args[0] != "initialize" && args[0] != "inspect" {
		return write()
	}
	r.Operation = "issuance." + args[0]
	if !admissionOperatorSupported(issuance.Supported()) {
		r.Outcome = "unsupported-platform"
		return write()
	}
	raw, endpoint, outcome := readAdmissionOperatorConfig(ctx, args[2:])
	if outcome != "" {
		r.Outcome = outcome
		return write()
	}
	p, err := decodeIssuancePlan(raw)
	clear(raw)
	if err != nil {
		return write()
	}
	if err = issuance.CheckExportDestination(ctx, p.InventoryFile); err != nil {
		r.Outcome = issuanceOutcome(err)
		return write()
	}
	o, _ := telemetry.Open(endpoint)
	finish := o.Begin("ardents.issuance", r.Operation)
	r.Phase, err = issuance.DeliverInventory(ctx, p.Root, p.Binding, p.InventoryFile, args[0] == "initialize")
	r.Outcome = issuanceOutcome(err)
	finish(r.Phase, r.Outcome)
	_ = o.Close()
	_ = json.NewEncoder(diagnostic).Encode(r)
	return write()
}

func runIssuanceResults(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	r := issuanceResult{Operation: "issuance", Phase: "input", Outcome: "invalid-input"}
	write := func() int {
		return writeAdmissionReport(out, r, r.Outcome, "initialized-results", "issued-offline", "already-issued")
	}
	if len(args) != 3 && len(args) != 4 {
		return write()
	}
	if args[1] != "--config" {
		return write()
	}
	r.Operation = "issuance." + args[0]
	if !admissionOperatorSupported(issuance.Supported()) {
		r.Outcome = "unsupported-platform"
		return write()
	}
	raw, endpoint, outcome := readAdmissionOperatorConfig(ctx, args[2:])
	if outcome != "" {
		r.Outcome = outcome
		return write()
	}
	p, err := decodeIssuanceResultPlan(raw, args[0])
	clear(raw)
	if err != nil {
		return write()
	}
	var batch []byte
	if args[0] == "issue" {
		if err = issuance.CheckExportDestination(ctx, p.ResponseFile); err != nil {
			r.Outcome = issuanceOutcome(err)
			return write()
		}
		batch, err = readBounded(p.Admission.BatchFile, 16<<10)
		if err != nil {
			return write()
		}
		defer clear(batch)
	}
	o, _ := telemetry.Open(endpoint)
	finish := o.Begin("ardents.issuance", r.Operation)
	plan := issuer.Plan{AdmissionRoot: p.Admission.Root, AdmissionBinding: p.Admission.Binding, KeyRoot: p.Keys.Root, KeyBinding: p.Keys.Binding, ResultRoot: p.Root}
	var result issuer.Result
	if args[0] == "initialize-results" {
		result = issuer.Initialize(ctx, plan)
	} else {
		result = issuer.Issue(ctx, plan, batch, p.Admission.Facts, p.Admission.Kind)
	}
	r.Phase, r.Outcome = result.Status()
	if result.Response != nil {
		r.Phase = "export"
		if exportErr := issuance.Export(ctx, p.ResponseFile, result.Response); exportErr != nil {
			r.Outcome = issuanceOutcome(exportErr)
		}
	}
	finish(r.Phase, r.Outcome)
	_ = o.Close()
	_ = json.NewEncoder(diagnostic).Encode(r)
	return write()
}

func runIssuerProfile(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	r := issuanceResult{Operation: operation, Phase: "input", Outcome: "invalid-input"}
	write := func() int {
		return writeAdmissionReport(out, r, r.Outcome, "identity-imported", "profile-initialized", "profile-verified", "binding-prepared")
	}
	if !admissionOperatorSupported(issuance.Supported()) {
		r.Outcome = "unsupported-platform"
		return write()
	}
	if len(args) < 2 || len(args) > 3 || args[0] != "--config" {
		return write()
	}
	raw, collector, outcome := readAdmissionOperatorConfig(ctx, args[1:])
	if outcome != "" {
		r.Outcome = outcome
		return write()
	}
	var err error
	defer clear(raw)
	observation, _ := telemetry.Open(collector)
	finish := observation.Begin("ardents.issuance", operation)
	var output string
	var bytes []byte
	switch operation {
	case "node-identity.import":
		fields, e := issuerConfigObject(raw, "root", "binding", "key_file")
		err = e
		var root, source string
		var binding signing.Binding
		if err == nil {
			root, err = issuerConfigPath(fields["root"])
		}
		if err == nil {
			source, err = issuerConfigPath(fields["key_file"])
		}
		if err == nil {
			binding, err = decodeIdentityBinding(fields["binding"])
		}
		if err == nil {
			r.Phase = "import"
			err = signing.Import(ctx, root, source, binding)
			r.Outcome = "identity-imported"
		}
	case "issuance.initialize-profile", "issuance.inspect-profile":
		var p issuer.ProfilePlan
		p, output, err = decodeProfilePlan(raw)
		if err == nil {
			err = issuance.CheckExportDestination(ctx, output)
		}
		if err == nil {
			var result issuer.Result
			if operation == "issuance.initialize-profile" {
				result = issuer.InitializeProfile(ctx, p)
			} else {
				result = issuer.InspectProfile(ctx, p)
			}
			r.Phase, r.Outcome = result.Status()
			bytes = result.Response
		}
	case "admission.prepare-binding":
		output, bytes, err = issuance.PrepareProfileBinding(raw)
		if err == nil {
			r.Phase = "prepare-binding"
			r.Outcome = "binding-prepared"
		}
	default:
		err = issuance.ErrInvalid
	}
	if err != nil {
		r.Outcome = issuanceOutcome(err)
	}
	if err == nil && bytes != nil {
		r.Phase = "export"
		err = issuance.Export(ctx, output, bytes)
		if err != nil {
			r.Outcome = issuanceOutcome(err)
		}
	}
	finish(r.Phase, r.Outcome)
	_ = observation.Close()
	_ = json.NewEncoder(diagnostic).Encode(r)
	return write()
}
