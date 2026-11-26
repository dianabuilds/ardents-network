package operator

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/telemetry"
	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
)

type admissionResult struct {
	Operation string            `json:"operation"`
	Phase     string            `json:"phase"`
	Outcome   admission.Outcome `json:"outcome"`
}

// RunAdmission handles offline quota initialization and its command report.
func RunAdmission(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	r := admissionResult{"admission", "input", admission.InvalidInput}
	write := func() int {
		return writeAdmissionReport(out, r, string(r.Outcome), string(quota.Initialized))
	}
	if len(args) != 3 && len(args) != 4 {
		return write()
	}
	if args[1] != "--config" || args[0] != "initialize" {
		return write()
	}
	r.Operation = "admission." + args[0]
	if outcome := quota.OperatorAvailability(); outcome != "" {
		r.Outcome = outcome
		return write()
	}
	raw, endpoint, outcome := readAdmissionOperatorConfig(ctx, args[2:])
	if outcome != "" {
		r.Outcome = admission.Outcome(outcome)
		return write()
	}
	defer clear(raw)
	p, err := issuance.DecodeLedgerDeclaration(raw)
	if err != nil {
		return write()
	}
	o, telemetryErr := telemetry.Open(endpoint)
	observedOperation := telemetry.LookupOperation(r.Operation)
	finish := o.Begin(observedOperation)
	r.Phase = "execute"
	r.Outcome = quota.InitializeResult(ctx, p.Root, p.Binding)
	finish(observedOperation.Classify(r.Phase, string(r.Outcome)))
	delivery := o.CloseWithStatus(telemetryErr)
	_ = json.NewEncoder(diagnostic).Encode(struct{ Operation, Phase, Outcome, Telemetry string }{r.Operation, r.Phase, string(r.Outcome), string(delivery)})
	return write()
}
