package main

import (
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
)

type issuanceResultPlan struct {
	Admission          admissionPlan
	Keys               issuancePlan
	Root, ResponseFile string
}

func decodeIssuanceResultPlan(raw []byte, operation string) (issuanceResultPlan, error) {
	var p issuanceResultPlan
	wanted := []string{"admission_root", "admission_binding", "key_root", "key_binding", "result_root"}
	if operation == "issue" {
		wanted = append(wanted, "batch_file", "facts", "kind", "response_file")
	} else if operation != "initialize-results" {
		return p, issuance.ErrInvalid
	}
	fields, err := issuerConfigObject(raw, wanted...)
	if err != nil {
		return p, err
	}
	p.Root, err = issuerConfigPath(fields["result_root"])
	if err != nil {
		return p, err
	}
	p.Admission, err = decodeAdmissionScope(fields["admission_root"], fields["admission_binding"])
	if err != nil {
		return p, issuance.ErrInvalid
	}
	if operation == "issue" {
		if err := p.Admission.decodeBatch(fields); err != nil {
			return p, issuance.ErrInvalid
		}
	}
	p.Keys.Root, err = issuerConfigPath(fields["key_root"])
	if err != nil || !issuanceOutputOutsideRoots(p.Root, p.Keys.Root) {
		return p, issuance.ErrInvalid
	}
	p.Keys.Binding, err = decodeIssuanceBinding(fields["key_binding"])
	if err != nil {
		return p, err
	}
	roots := []string{p.Root, p.Admission.Root, p.Keys.Root}
	for i, a := range roots {
		for j, b := range roots {
			if i != j && (a == b || strings.HasPrefix(a, b+string(filepath.Separator))) {
				return p, issuance.ErrInvalid
			}
		}
	}
	if operation == "issue" {
		p.ResponseFile, err = issuerConfigPath(fields["response_file"])
		if err != nil {
			return p, issuance.ErrInvalid
		}
		if !issuanceOutputOutsideRoots(p.ResponseFile, roots...) {
			return p, issuance.ErrInvalid
		}
	}
	return p, nil
}
