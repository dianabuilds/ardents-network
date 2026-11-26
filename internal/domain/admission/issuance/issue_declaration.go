package issuance

import (
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
)

// IssueDeclaration binds a result store to its offline issuance input and output.
type IssueDeclaration struct {
	Setup        ResultSetup
	BatchFile    string
	Facts        admission.Facts
	Kind         quota.Kind
	ResponseFile string
}

// DecodeIssueDeclaration validates all roots, pins and issuance inputs before effects.
func DecodeIssueDeclaration(raw []byte, issue bool) (IssueDeclaration, error) {
	var plan IssueDeclaration
	fieldsWanted := []string{"admission_root", "admission_binding", "key_root", "key_binding", "result_root"}
	if issue {
		fieldsWanted = append(fieldsWanted, "batch_file", "facts", "kind", "response_file")
	}
	fields, err := resultFields(raw, fieldsWanted...)
	if err != nil {
		return plan, err
	}
	plan.Setup, err = DecodeResultSetup(fields)
	if err != nil || !issue {
		return plan, err
	}
	if json.Unmarshal(fields["batch_file"], &plan.BatchFile) != nil || !pathcheck.IsCleanAbsolute(plan.BatchFile) {
		return plan, ErrInvalid
	}
	plan.Facts, err = DecodeBatchFacts(fields["facts"])
	if err != nil {
		return plan, ErrInvalid
	}
	var kind string
	if json.Unmarshal(fields["kind"], &kind) != nil {
		return plan, ErrInvalid
	}
	switch kind {
	case "bootstrap":
		plan.Kind = quota.Bootstrap
	case "admitted":
		plan.Kind = quota.Admitted
	default:
		return plan, ErrInvalid
	}
	if json.Unmarshal(fields["response_file"], &plan.ResponseFile) != nil || !pathcheck.IsCleanAbsolute(plan.ResponseFile) ||
		!OutputOutsideRoots(plan.ResponseFile, plan.Setup.ResultRoot, plan.Setup.AdmissionRoot, plan.Setup.KeyRoot) {
		return plan, ErrInvalid
	}
	return plan, nil
}
