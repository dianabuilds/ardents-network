package main

import "github.com/dianabuilds/ardents-network/internal/successor/admission/quota"

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
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

func requiredAdmissionObject(raw []byte, keys ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(keys) {
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
func admissionNumber(value string, bits int) (uint64, error) {
	n, err := strconv.ParseUint(value, 10, bits)
	if err != nil || strconv.FormatUint(n, 10) != value {
		return 0, errors.New("invalid-input")
	}
	return n, nil
}
func decodeAdmissionPlan(raw []byte, operation string) (admissionPlan, error) {
	var p admissionPlan
	bad := errors.New("invalid-input")
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if uniqueJSON(d) != nil {
		return p, bad
	}
	if _, err := d.Token(); err != io.EOF {
		return p, bad
	}
	keys := []string{"root", "binding"}
	if operation == "debit" {
		keys = append(keys, "batch_file", "facts", "kind")
	} else if operation != "initialize" {
		return p, bad
	}
	if !requiredAdmissionObject(raw, keys...) {
		return p, bad
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	if json.Unmarshal(fields["root"], &p.Root) != nil || !filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root {
		return p, bad
	}
	bindingRaw := fields["binding"]
	if !requiredAdmissionObject(bindingRaw, "network", "issuer", "authority", "profile", "duty", "start", "end", "keys") {
		return p, bad
	}
	var input admissionBindingInput
	if json.Unmarshal(bindingRaw, &input) != nil {
		return p, bad
	}
	var keyFields []json.RawMessage
	var bindingFields map[string]json.RawMessage
	_ = json.Unmarshal(bindingRaw, &bindingFields)
	_ = json.Unmarshal(bindingFields["keys"], &keyFields)
	if len(keyFields) < 3 || len(keyFields) > 18 {
		return p, bad
	}
	for _, key := range keyFields {
		if !requiredAdmissionObject(key, "window", "class", "spki") {
			return p, bad
		}
	}
	for _, item := range []struct {
		value  string
		target *[32]byte
	}{{input.Network, &p.Binding.Network}, {input.Issuer, &p.Binding.Issuer}, {input.Authority, &p.Binding.Authority}, {input.Profile, &p.Binding.Profile}} {
		b, err := admissionHex(item.value, 32)
		if err != nil {
			return p, bad
		}
		copy(item.target[:], b)
	}
	var err error
	p.Binding.Duty, err = admissionNumber(input.Duty, 64)
	if err != nil {
		return p, bad
	}
	p.Binding.Start, err = admissionTime(input.Start)
	if err != nil {
		return p, bad
	}
	p.Binding.End, err = admissionTime(input.End)
	if err != nil {
		return p, bad
	}
	for _, key := range input.Keys {
		t, e := admissionTime(key.Window)
		if e != nil || t.Unix() < 0 {
			return p, bad
		}
		class, e := admissionNumber(key.Class, 8)
		if e != nil {
			return p, bad
		}
		spki, e := admissionHex(key.SPKI, 346)
		if e != nil {
			return p, bad
		}
		p.Binding.Keys = append(p.Binding.Keys, issuerprofile.Key{Window: uint64(t.Unix()), Class: uint8(class), SPKI: spki})
	}
	if operation == "debit" {
		if json.Unmarshal(fields["batch_file"], &p.BatchFile) != nil || !filepath.IsAbs(p.BatchFile) || filepath.Clean(p.BatchFile) != p.BatchFile {
			return p, bad
		}
		p.Facts, err = decodeFacts(fields["facts"])
		if err != nil {
			return p, bad
		}
		// decodeFacts already enforces exact field names and duplicates; strengthen
		// canonical hex/time for this persisted accounting operation.
		var values map[string]string
		_ = json.Unmarshal(fields["facts"], &values)
		for _, name := range []string{"network", "issuer", "authority", "holder"} {
			if _, e := admissionHex(values[name], 32); e != nil {
				return p, bad
			}
		}
		for _, name := range []string{"duty_not_before", "duty_not_after", "now"} {
			if _, e := admissionTime(values[name]); e != nil {
				return p, bad
			}
		}
		var kind string
		if json.Unmarshal(fields["kind"], &kind) != nil {
			return p, bad
		}
		switch kind {
		case "bootstrap":
			p.Kind = quota.Bootstrap
		case "admitted":
			p.Kind = quota.Admitted
		default:
			return p, bad
		}
	}
	return p, nil
}
