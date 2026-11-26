package issuer

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuerprofile/signing"
	"github.com/dianabuilds/ardents-network/internal/pathcheck"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

func declarationFields(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if artifacts.UniqueJSON(d) != nil {
		return nil, issuance.ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, issuance.ErrInvalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(names) {
		return nil, issuance.ErrInvalid
	}
	for _, name := range names {
		value, ok := fields[name]
		if !ok || bytes.Equal(value, []byte("null")) {
			return nil, issuance.ErrInvalid
		}
	}
	return fields, nil
}

func declarationPath(raw json.RawMessage) (string, error) {
	var path string
	if json.Unmarshal(raw, &path) != nil || !pathcheck.IsCleanAbsolute(path) {
		return "", issuance.ErrInvalid
	}
	return path, nil
}

// DecodeIdentityImport validates the local identity import declaration.
func DecodeIdentityImport(raw []byte) (root, source string, binding signing.Binding, err error) {
	fields, err := declarationFields(raw, "root", "binding", "key_file")
	if err != nil {
		return "", "", binding, err
	}
	root, err = declarationPath(fields["root"])
	if err != nil {
		return "", "", binding, err
	}
	source, err = declarationPath(fields["key_file"])
	if err != nil {
		return "", "", binding, err
	}
	binding, err = decodeIdentityBinding(fields["binding"])
	return root, source, binding, err
}

func decodeIdentityBinding(raw []byte) (signing.Binding, error) {
	var b signing.Binding
	fields, e := declarationFields(raw, "network", "node", "signer")
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

// DecodeProfilePlan validates the offline issuer profile declaration.
func DecodeProfilePlan(raw []byte) (ProfilePlan, string, error) {
	var p ProfilePlan
	fields, e := declarationFields(raw, "identity_root", "identity_binding", "key_root", "key_binding", "profile_root", "profile_file")
	if e != nil {
		return p, "", e
	}
	for _, item := range []struct {
		name   string
		target *string
	}{{"identity_root", &p.IdentityRoot}, {"key_root", &p.KeyRoot}, {"profile_root", &p.ProfileRoot}} {
		*item.target, e = declarationPath(fields[item.name])
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
	output, e := declarationPath(fields["profile_file"])
	if e == nil && !issuance.OutputOutsideRoots(output, p.IdentityRoot, p.KeyRoot, p.ProfileRoot) {
		return p, "", issuance.ErrInvalid
	}
	return p, output, e
}
