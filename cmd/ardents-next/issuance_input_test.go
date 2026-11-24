package main

import (
	"encoding/json"
	nodefixture "github.com/dianabuilds/ardents-network/tests/e2e/node"
	"path/filepath"
	"strings"
	"testing"
)

type issuanceBindingInput struct {
	Network string `json:"network"`
	Issuer  string `json:"issuer"`
	Signer  string `json:"signer"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

func TestIssuanceStrictInput(t *testing.T) {
	base := map[string]any{"root": filepath.Join(t.TempDir(), "keys"), "inventory_file": filepath.Join(t.TempDir(), "public"), "binding": issuanceBindingInput{strings.Repeat("ab", 32), strings.Repeat("cd", 32), strings.Repeat("ef", 32), "2030-01-01T00:00:00Z", "2030-01-01T01:00:00Z"}}
	raw, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, e := decodeIssuancePlan(raw); e != nil {
		t.Fatal(e)
	}
	for name, value := range map[string]string{
		"duplicate":  strings.Replace(string(raw), `"network":`, `"network":"`+strings.Repeat("ab", 32)+`","network":`, 1),
		"case":       strings.Replace(string(raw), `"network":`, `"Network":`, 1),
		"missing":    strings.Replace(string(raw), `"signer":"`+strings.Repeat("ef", 32)+`",`, "", 1),
		"unknown":    strings.Replace(string(raw), `"binding":{`, `"binding":{"extra":0,`, 1),
		"uppercase":  strings.Replace(string(raw), strings.Repeat("ab", 32), strings.Repeat("AB", 32), 1),
		"null":       strings.Replace(string(raw), `"binding":{`, `"extra":null,"binding":{`, 1),
		"fractional": strings.Replace(string(raw), "00:00:00Z", "00:00:00.001Z", 1),
		"timezone":   strings.Replace(string(raw), "00:00:00Z", "00:00:00+00:00", 1),
		"trailing":   string(raw) + " {}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, e := decodeIssuancePlan([]byte(value)); e == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestIssuanceResultStrictBatch(t *testing.T) {
	_, facts, binding := nodefixture.AdmissionCommandFixture(t, 2, 1, 2, 1)
	base := t.TempDir()
	key := issuanceBindingInput{
		Network: strings.Repeat("ab", 32), Issuer: strings.Repeat("cd", 32), Signer: strings.Repeat("ef", 32),
		Start: "2030-01-01T00:00:00Z", End: "2030-01-01T01:00:00Z",
	}
	raw, err := json.Marshal(map[string]any{
		"admission_root": filepath.Join(base, "admission"), "admission_binding": admissionBindingConfig(binding),
		"key_root": filepath.Join(base, "keys"), "key_binding": key,
		"result_root": filepath.Join(base, "results"), "batch_file": filepath.Join(base, "batch"),
		"facts": admissionFactsConfig(facts), "kind": "bootstrap", "response_file": filepath.Join(base, "response"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan, err := decodeIssuanceResultPlan(raw, "issue"); err != nil ||
		plan.Admission.Facts.Holder != facts.Holder || plan.Admission.Binding.Authority != binding.Authority ||
		plan.Keys.Root != filepath.Join(base, "keys") || plan.Root != filepath.Join(base, "results") {
		t.Fatal("valid original fields did not bind", err)
	}
	for name, mutated := range map[string]string{
		"fact alias":         strings.Replace(string(raw), `"holder":`, `"Holder":`, 1),
		"invalid kind":       strings.Replace(string(raw), `"kind":"bootstrap"`, `"kind":"unknown"`, 1),
		"noncanonical count": strings.Replace(string(raw), `"count":"1"`, `"count":"01"`, 1),
		"nested key alias":   strings.Replace(string(raw), `"signer":`, `"Signer":`, 1),
		"duplicate root":     strings.Replace(string(raw), `"key_root":`, `"key_root":null,"key_root":`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeIssuanceResultPlan([]byte(mutated), "issue"); err == nil {
				t.Fatal("accepted malformed original fields")
			}
		})
	}
}
