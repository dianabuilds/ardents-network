package issuer

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuerprofile/signing"
)

// ProfileOperation performs one offline identity or issuer profile operation
// and finishes any requested public export before reporting its result.
func ProfileOperation(ctx context.Context, operation string, raw []byte) Completion {
	result := Completion{Phase: "input", Outcome: "invalid-input"}
	var err error
	var output string
	var response []byte
	switch operation {
	case "node-identity.import":
		var root, source string
		var binding signing.Binding
		root, source, binding, err = DecodeIdentityImport(raw)
		if err == nil {
			result.Phase = "import"
			err = signing.Import(ctx, root, source, binding)
			result.Outcome = "identity-imported"
		}
	case "issuance.initialize-profile", "issuance.inspect-profile":
		var plan ProfilePlan
		plan, output, err = DecodeProfilePlan(raw)
		if err == nil {
			err = issuance.CheckExportDestination(ctx, output)
		}
		if err == nil {
			var outcome Result
			if operation == "issuance.initialize-profile" {
				outcome = InitializeProfile(ctx, plan)
			} else {
				outcome = InspectProfile(ctx, plan)
			}
			result.Phase, result.Outcome = outcome.Status()
			response = outcome.Response
		}
	case "admission.prepare-binding":
		output, response, err = issuance.PrepareProfileBinding(raw)
		if err == nil {
			result.Phase, result.Outcome = "prepare-binding", "binding-prepared"
		}
	default:
		err = issuance.ErrInvalid
	}
	if err != nil {
		result.Outcome = ErrorOutcome(err)
	}
	if err == nil && response != nil {
		result.Phase = "export"
		if err := issuance.Export(ctx, output, response); err != nil {
			result.Outcome = ErrorOutcome(err)
		}
	}
	return result
}
