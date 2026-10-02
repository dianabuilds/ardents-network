package main

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/issuance"
)

type issuanceResultPlan struct {
	Admission          admissionPlan
	Keys               issuancePlan
	Root, ResponseFile string
}

func decodeIssuanceResultPlan(raw []byte, operation string) (issuanceResultPlan, error) {
	var p issuanceResultPlan
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if uniqueJSON(d) != nil {
		return p, issuance.ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return p, issuance.ErrInvalid
	}
	wanted := []string{"admission_root", "admission_binding", "key_root", "key_binding", "result_root"}
	if operation == "issue" {
		wanted = append(wanted, "batch_file", "facts", "kind", "response_file")
	} else if operation != "initialize-results" {
		return p, issuance.ErrInvalid
	}
	if !requiredAdmissionObject(raw, wanted...) {
		return p, issuance.ErrInvalid
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	if json.Unmarshal(fields["result_root"], &p.Root) != nil || !filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root {
		return p, issuance.ErrInvalid
	}
	admissionInput := map[string]json.RawMessage{"root": fields["admission_root"], "binding": fields["admission_binding"]}
	op := "initialize"
	if operation == "issue" {
		op = "debit"
		for _, name := range []string{"batch_file", "facts", "kind"} {
			admissionInput[name] = fields[name]
		}
	}
	encoded, e := json.Marshal(admissionInput)
	if e != nil {
		return p, issuance.ErrInvalid
	}
	p.Admission, e = decodeAdmissionPlan(encoded, op)
	if e != nil {
		return p, issuance.ErrInvalid
	}
	keysInput := map[string]json.RawMessage{"root": fields["key_root"], "binding": fields["key_binding"], "inventory_file": fields["result_root"]}
	encoded, e = json.Marshal(keysInput)
	if e != nil {
		return p, issuance.ErrInvalid
	}
	p.Keys, e = decodeIssuancePlan(encoded)
	if e != nil {
		return p, issuance.ErrInvalid
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
		if json.Unmarshal(fields["response_file"], &p.ResponseFile) != nil || !filepath.IsAbs(p.ResponseFile) || filepath.Clean(p.ResponseFile) != p.ResponseFile {
			return p, issuance.ErrInvalid
		}
	}
	return p, nil
}
