package main

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/issuerprofile/signing"
	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/pathgrammar"
)

func issuerConfigObject(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if uniqueJSON(d) != nil {
		return nil, issuance.ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, issuance.ErrInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || !requiredAdmissionFields(fields, names...) {
		return nil, issuance.ErrInvalid
	}
	return fields, nil
}
func issuerConfigPath(raw json.RawMessage) (string, error) {
	var path string
	if json.Unmarshal(raw, &path) != nil || !pathgrammar.Strict(path) {
		return "", issuance.ErrInvalid
	}
	return path, nil
}
func decodeIdentityBinding(raw []byte) (signing.Binding, error) {
	var b signing.Binding
	fields, e := issuerConfigObject(raw, "network", "node", "signer")
	if e != nil {
		return b, e
	}
	for _, item := range []struct {
		name   string
		target *[32]byte
	}{{"network", &b.Network}, {"node", &b.Node}, {"signer", &b.Signer}} {
		var s string
		if json.Unmarshal(fields[item.name], &s) != nil {
			return b, issuance.ErrInvalid
		}
		decoded, ok := canonicaldigest.Decode(s)
		if !ok {
			return b, issuance.ErrInvalid
		}
		*item.target = decoded
		if *item.target == ([32]byte{}) {
			return b, issuance.ErrInvalid
		}
	}
	return b, nil
}
func decodeProfilePlan(raw []byte) (issuer.ProfilePlan, string, error) {
	var p issuer.ProfilePlan
	fields, e := issuerConfigObject(raw, "identity_root", "identity_binding", "key_root", "key_binding", "profile_root", "profile_file")
	if e != nil {
		return p, "", e
	}
	for _, item := range []struct {
		name   string
		target *string
	}{{"identity_root", &p.IdentityRoot}, {"key_root", &p.KeyRoot}, {"profile_root", &p.ProfileRoot}} {
		*item.target, e = issuerConfigPath(fields[item.name])
		if e != nil {
			return p, "", e
		}
	}
	p.IdentityBinding, e = decodeIdentityBinding(fields["identity_binding"])
	if e != nil {
		return p, "", e
	}
	p.KeyBinding, e = issuance.DecodeKeyBinding(fields["key_binding"])
	if e != nil {
		return p, "", e
	}
	output, e := issuerConfigPath(fields["profile_file"])
	if e == nil && !issuance.OutputOutsideRoots(output, p.IdentityRoot, p.KeyRoot, p.ProfileRoot) {
		return p, "", issuance.ErrInvalid
	}
	return p, output, e
}
