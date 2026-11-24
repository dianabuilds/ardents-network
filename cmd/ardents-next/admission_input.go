package main

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
)

type admissionPlan struct {
	Root      string
	Binding   quota.LedgerBinding
	BatchFile string
	Facts     admission.Facts
	Kind      quota.Kind
}

func requiredAdmissionFields(fields map[string]json.RawMessage, keys ...string) bool {
	if len(fields) != len(keys) {
		return false
	}
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || bytes.Equal(value, []byte("null")) {
			return false
		}
	}
	return true
}
func decodeAdmissionPlan(raw []byte) (admissionPlan, error) {
	fields, err := issuerConfigObject(raw, "root", "binding")
	if err != nil {
		return admissionPlan{}, err
	}
	return decodeAdmissionScope(fields["root"], fields["binding"])
}

func decodeAdmissionScope(root, binding json.RawMessage) (admissionPlan, error) {
	var p admissionPlan
	var err error
	p.Root, err = issuerConfigPath(root)
	if err != nil {
		return p, err
	}
	p.Binding, err = issuance.DecodeSetupLedgerBinding(binding)
	return p, err
}

func (p *admissionPlan) decodeBatch(fields map[string]json.RawMessage) error {
	bad := errors.New("invalid-input")
	var err error
	p.BatchFile, err = issuerConfigPath(fields["batch_file"])
	if err != nil {
		return err
	}
	p.Facts, err = issuance.DecodeBatchFacts(fields["facts"])
	if err != nil {
		return bad
	}
	var kind string
	if json.Unmarshal(fields["kind"], &kind) != nil {
		return bad
	}
	switch kind {
	case "bootstrap":
		p.Kind = quota.Bootstrap
	case "admitted":
		p.Kind = quota.Admitted
	default:
		return bad
	}
	return nil
}
