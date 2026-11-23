package main

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
)

type admissionBindingInput struct {
	Network   string              `json:"network"`
	Issuer    string              `json:"issuer"`
	Authority string              `json:"authority"`
	Profile   string              `json:"profile"`
	Duty      string              `json:"duty"`
	Start     string              `json:"start"`
	End       string              `json:"end"`
	Keys      []admissionKeyInput `json:"keys"`
}
type admissionKeyInput struct {
	Window string `json:"window"`
	Class  string `json:"class"`
	SPKI   string `json:"spki"`
}
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
func admissionHex(value string, n int) ([]byte, error) {
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != n || hex.EncodeToString(raw) != value {
		return nil, errors.New("invalid-input")
	}
	return raw, nil
}
func admissionTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil || t.Format(time.RFC3339) != value || t.Location() != time.UTC || t.Nanosecond() != 0 {
		return time.Time{}, errors.New("invalid-input")
	}
	return t, nil
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
	p.Facts, err = decodeFacts(fields["facts"])
	if err != nil {
		return bad
	}
	// decodeFacts already enforces exact field names and duplicates; strengthen
	// canonical hex/time for this persisted accounting operation.
	var values map[string]string
	_ = json.Unmarshal(fields["facts"], &values)
	for _, name := range []string{"network", "issuer", "authority", "holder"} {
		if _, e := admissionHex(values[name], 32); e != nil {
			return bad
		}
	}
	for _, name := range []string{"duty_not_before", "duty_not_after", "now"} {
		if _, e := admissionTime(values[name]); e != nil {
			return bad
		}
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
