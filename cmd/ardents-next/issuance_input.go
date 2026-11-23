package main

import (
	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
)

type issuanceBindingInput struct {
	Network string `json:"network"`
	Issuer  string `json:"issuer"`
	Signer  string `json:"signer"`
	Start   string `json:"start"`
	End     string `json:"end"`
}
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
