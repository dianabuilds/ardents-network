package main

import (
	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
)

type issuancePlan struct {
	Root          string
	Binding       issuance.Binding
	InventoryFile string
}

func decodeIssuancePlan(raw []byte) (issuancePlan, error) {
	var p issuancePlan
	fields, err := issuerConfigObject(raw, "root", "binding", "inventory_file")
	if err != nil {
		return p, err
	}
	p.Root, err = issuerConfigPath(fields["root"])
	if err != nil {
		return p, err
	}
	p.InventoryFile, err = issuerConfigPath(fields["inventory_file"])
	if err != nil {
		return p, err
	}
	if !issuance.OutputOutsideRoots(p.InventoryFile, p.Root) {
		return p, issuance.ErrInvalid
	}
	binding, err := issuance.DecodeKeyBinding(fields["binding"])
	if err != nil {
		return p, err
	}
	p.Binding = binding
	return p, nil
}

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
	setup, err := issuance.DecodeResultSetup(fields)
	if err != nil {
		return p, err
	}
	p.Root = setup.ResultRoot
	p.Admission.Root, p.Admission.Binding = setup.AdmissionRoot, setup.AdmissionBinding
	p.Keys.Root, p.Keys.Binding = setup.KeyRoot, setup.KeyBinding
	if operation == "issue" {
		if err := p.Admission.decodeBatch(fields); err != nil {
			return p, issuance.ErrInvalid
		}
		p.ResponseFile, err = issuerConfigPath(fields["response_file"])
		if err != nil {
			return p, issuance.ErrInvalid
		}
		if !issuance.OutputOutsideRoots(p.ResponseFile, p.Root, p.Admission.Root, p.Keys.Root) {
			return p, issuance.ErrInvalid
		}
	}
	return p, nil
}
