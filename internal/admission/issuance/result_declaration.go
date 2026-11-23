package issuance

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// ResultSetup is detached filesystem and pin configuration. Runtime owners
// must independently open and verify the corresponding histories.
type ResultSetup struct {
	AdmissionRoot    string
	AdmissionBinding quota.LedgerBinding
	KeyRoot          string
	KeyBinding       Binding
	ResultRoot       string
}

// DecodeInstalledResultSetup reads the Linux plan as POSIX data on any host.
func DecodeInstalledResultSetup(raw []byte) (ResultSetup, error) {
	fields, err := resultFields(raw, "admission_root", "admission_binding", "key_root", "key_binding", "result_root")
	if err != nil {
		return ResultSetup{}, err
	}
	setup, err := decodeResultSetup(fields, func(name string) bool {
		return path.IsAbs(name) && path.Clean(name) == name
	}, "/")
	if err != nil || setup.KeyRoot == "/" {
		return ResultSetup{}, ErrInvalid
	}
	return setup, nil
}

// DecodeResultSetup reads bindings and roots from a validated local declaration.
func DecodeResultSetup(fields map[string]json.RawMessage) (ResultSetup, error) {
	setup, err := decodeResultSetup(fields, pathgrammar.Strict, string(filepath.Separator))
	if err != nil || !OutputOutsideRoots(setup.ResultRoot, setup.KeyRoot) {
		return ResultSetup{}, ErrInvalid
	}
	return setup, nil
}

func decodeResultSetup(fields map[string]json.RawMessage, absolute func(string) bool, separator string) (ResultSetup, error) {
	var setup ResultSetup
	for _, item := range []struct {
		name   string
		target *string
	}{
		{"admission_root", &setup.AdmissionRoot}, {"key_root", &setup.KeyRoot}, {"result_root", &setup.ResultRoot},
	} {
		if json.Unmarshal(fields[item.name], item.target) != nil || !absolute(*item.target) {
			return ResultSetup{}, ErrInvalid
		}
	}
	roots := []string{setup.AdmissionRoot, setup.KeyRoot, setup.ResultRoot}
	for i, root := range roots {
		for _, other := range roots[:i] {
			if root == other || strings.HasPrefix(root, other+separator) || strings.HasPrefix(other, root+separator) {
				return ResultSetup{}, ErrInvalid
			}
		}
	}
	var err error
	setup.AdmissionBinding, err = DecodeSetupLedgerBinding(fields["admission_binding"])
	if err != nil {
		return ResultSetup{}, ErrInvalid
	}
	setup.KeyBinding, err = DecodeKeyBinding(fields["key_binding"])
	if err != nil {
		return ResultSetup{}, err
	}
	return setup, nil
}

// DecodeSetupLedgerBinding checks the complete configured issuer ledger binding.
func DecodeSetupLedgerBinding(raw []byte) (quota.LedgerBinding, error) {
	fields, err := resultFields(raw, "network", "issuer", "authority", "profile", "duty", "start", "end", "keys")
	if err != nil {
		return quota.LedgerBinding{}, err
	}
	return DecodeLedgerBinding(raw, fields)
}

func resultFields(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if artifacts.UniqueJSON(d) != nil {
		return nil, ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || !hasJSONFields(fields, names...) {
		return nil, ErrInvalid
	}
	return fields, nil
}

func exactJSONFields(raw []byte, keys ...string) bool {
	var fields map[string]json.RawMessage
	return json.Unmarshal(raw, &fields) == nil && hasJSONFields(fields, keys...)
}

func hasJSONFields(fields map[string]json.RawMessage, keys ...string) bool {
	if len(fields) != len(keys) {
		return false
	}
	for _, key := range keys {
		field, ok := fields[key]
		if !ok || bytes.Equal(field, []byte("null")) {
			return false
		}
	}
	return true
}

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
type issuanceBindingInput struct {
	Network string `json:"network"`
	Issuer  string `json:"issuer"`
	Signer  string `json:"signer"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// DecodeLedgerBinding parses a declaration whose caller has checked its exact outer fields.
func DecodeLedgerBinding(raw []byte, fields map[string]json.RawMessage) (quota.LedgerBinding, error) {
	var binding quota.LedgerBinding
	bad := errors.New("invalid-input")
	var input admissionBindingInput
	if json.Unmarshal(raw, &input) != nil {
		return binding, bad
	}
	for _, item := range []struct {
		value  string
		target *[32]byte
	}{{input.Network, &binding.Network}, {input.Issuer, &binding.Issuer}, {input.Authority, &binding.Authority}, {input.Profile, &binding.Profile}} {
		b, err := admissionHex(item.value, 32)
		if err != nil {
			return binding, bad
		}
		copy(item.target[:], b)
	}
	var err error
	binding.Duty, err = admissionNumber(input.Duty, 64)
	if err != nil {
		return binding, bad
	}
	binding.Start, err = admissionTime(input.Start)
	if err != nil {
		return binding, bad
	}
	binding.End, err = admissionTime(input.End)
	if err != nil {
		return binding, bad
	}
	if _, hasKeys := fields["keys"]; !hasKeys {
		return binding, nil
	}
	var keyFields []json.RawMessage
	if json.Unmarshal(fields["keys"], &keyFields) != nil || len(keyFields) < 3 || len(keyFields) > 18 {
		return binding, bad
	}
	for _, key := range keyFields {
		if !exactJSONFields(key, "window", "class", "spki") {
			return binding, bad
		}
	}
	for _, key := range input.Keys {
		t, e := admissionTime(key.Window)
		if e != nil || t.Unix() < 0 {
			return binding, bad
		}
		class, e := admissionNumber(key.Class, 8)
		if e != nil {
			return binding, bad
		}
		spki, e := admissionHex(key.SPKI, 346)
		if e != nil {
			return binding, bad
		}
		binding.Keys = append(binding.Keys, issuerprofile.Key{Window: uint64(t.Unix()), Class: uint8(class), SPKI: spki})
	}
	return binding, nil
}

func DecodeKeyBinding(raw []byte) (Binding, error) {
	var binding Binding
	if !exactJSONFields(raw, "network", "issuer", "signer", "start", "end") {
		return binding, ErrInvalid
	}
	var b issuanceBindingInput
	if json.Unmarshal(raw, &b) != nil {
		return binding, ErrInvalid
	}
	for _, item := range []struct {
		value  string
		target *[32]byte
	}{{b.Network, &binding.Network}, {b.Issuer, &binding.Issuer}, {b.Signer, &binding.Signer}} {
		value, err := admissionHex(item.value, 32)
		if err != nil {
			return binding, ErrInvalid
		}
		copy(item.target[:], value)
	}
	var err error
	binding.Start, err = admissionTime(b.Start)
	if err != nil {
		return binding, ErrInvalid
	}
	binding.End, err = admissionTime(b.End)
	if err != nil {
		return binding, ErrInvalid
	}
	return binding, nil
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

// OutputOutsideRoots excludes even not-yet-created state roots.
func OutputOutsideRoots(output string, roots ...string) bool {
	for _, root := range roots {
		relative, err := filepath.Rel(root, output)
		if err != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return false
		}
	}
	return true
}
