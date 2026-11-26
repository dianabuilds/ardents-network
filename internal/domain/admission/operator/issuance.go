package operator

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
)

type issuanceResult struct {
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Outcome   string `json:"outcome"`
}

// RunIssuance handles local inventory, results and Issuer profile commands.
func RunIssuance(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && (args[0] == "initialize-profile" || args[0] == "inspect-profile") {
		return RunIssuerProfile(ctx, "issuance."+args[0], args[1:], out, diagnostic)
	}
	if len(args) > 0 && (args[0] == "issue" || args[0] == "initialize-results") {
		return runIssuanceResults(ctx, args, out, diagnostic)
	}
	r := issuanceResult{"issuance", "input", "invalid-input"}
	write := func() int { return writeAdmissionReport(out, r, r.Outcome, "completed") }
	if len(args) != 3 && len(args) != 4 {
		return write()
	}
	if args[1] != "--config" || args[0] != "initialize" && args[0] != "inspect" {
		return write()
	}
	r.Operation = "issuance." + args[0]
	if outcome := issuance.OperatorAvailability(); outcome != "" {
		r.Outcome = outcome
		return write()
	}
	raw, endpoint, outcome := readAdmissionOperatorConfig(ctx, args[2:])
	if outcome != "" {
		r.Outcome = outcome
		return write()
	}
	p, err := issuance.DecodeInventoryPlan(raw)
	clear(raw)
	if err != nil {
		return write()
	}
	if err = issuance.CheckExportDestination(ctx, p.InventoryFile); err != nil {
		r.Outcome = issuer.ErrorOutcome(err)
		return write()
	}
	o, exportErr := telemetry.Open(endpoint)
	observedOperation := telemetry.LookupOperation(r.Operation)
	finish := o.Begin(observedOperation)
	r.Phase, err = issuance.DeliverInventory(ctx, p.Root, p.Binding, p.InventoryFile, args[0] == "initialize")
	r.Outcome = issuer.ErrorOutcome(err)
	finish(observedOperation.Classify(r.Phase, r.Outcome))
	delivery := o.CloseWithStatus(exportErr)
	writeIssuanceDiagnostic(diagnostic, r, delivery)
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
	if outcome := issuance.OperatorAvailability(); outcome != "" {
		r.Outcome = outcome
		return write()
	}
	raw, endpoint, outcome := readAdmissionOperatorConfig(ctx, args[2:])
	if outcome != "" {
		r.Outcome = outcome
		return write()
	}
	prepared, err := issuer.PrepareResult(ctx, raw, args[0] == "issue")
	clear(raw)
	if err != nil {
		r.Outcome = issuer.ErrorOutcome(err)
		return write()
	}
	o, exportErr := telemetry.Open(endpoint)
	observedOperation := telemetry.LookupOperation(r.Operation)
	finish := o.Begin(observedOperation)
	result := prepared.Execute(ctx)
	r.Phase, r.Outcome = result.Phase, result.Outcome
	finish(observedOperation.Classify(r.Phase, r.Outcome))
	delivery := o.CloseWithStatus(exportErr)
	writeIssuanceDiagnostic(diagnostic, r, delivery)
	return write()
}

// RunIssuerProfile handles one CLI-selected Issuer profile operation.
func RunIssuerProfile(ctx context.Context, operation string, args []string, out, diagnostic io.Writer) int {
	r := issuanceResult{Operation: operation, Phase: "input", Outcome: "invalid-input"}
	write := func() int {
		return writeAdmissionReport(out, r, r.Outcome, "identity-imported", "profile-initialized", "profile-verified", "binding-prepared")
	}
	if outcome := issuance.OperatorAvailability(); outcome != "" {
		r.Outcome = outcome
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
	defer clear(raw)
	observation, exportErr := telemetry.Open(collector)
	observedOperation := telemetry.LookupOperation(operation)
	finish := observation.Begin(observedOperation)
	result := issuer.ProfileOperation(ctx, operation, raw)
	r.Phase, r.Outcome = result.Phase, result.Outcome
	finish(observedOperation.Classify(r.Phase, r.Outcome))
	delivery := observation.CloseWithStatus(exportErr)
	writeIssuanceDiagnostic(diagnostic, r, delivery)
	return write()
}

func writeIssuanceDiagnostic(out io.Writer, result issuanceResult, delivery telemetry.DeliveryStatus) {
	_ = json.NewEncoder(out).Encode(struct {
		issuanceResult
		Telemetry telemetry.DeliveryStatus `json:"telemetry"`
	}{result, delivery})
}
