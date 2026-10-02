package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

func profileConfigObject(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if uniqueJSON(d) != nil {
		return nil, issuance.ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF || !requiredAdmissionObject(raw, names...) {
		return nil, issuance.ErrInvalid
	}
	var fields map[string]json.RawMessage
	e := json.Unmarshal(raw, &fields)
	return fields, e
}
func profilePath(raw json.RawMessage) (string, error) {
	var path string
	if json.Unmarshal(raw, &path) != nil || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", issuance.ErrInvalid
	}
	return path, nil
}
func decodeIdentityBinding(raw []byte) (nodeidentity.Binding, error) {
	var b nodeidentity.Binding
	fields, e := profileConfigObject(raw, "network", "node", "signer")
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
		decoded, e := admissionHex(s, 32)
		if e != nil {
			return b, issuance.ErrInvalid
		}
		copy(item.target[:], decoded)
		if *item.target == ([32]byte{}) {
			return b, issuance.ErrInvalid
		}
	}
	return b, nil
}
func decodeProfilePlan(raw []byte) (issuer.ProfilePlan, string, error) {
	var p issuer.ProfilePlan
	fields, e := profileConfigObject(raw, "identity_root", "identity_binding", "key_root", "key_binding", "profile_root", "profile_file")
	if e != nil {
		return p, "", e
	}
	for _, item := range []struct {
		name   string
		target *string
	}{{"identity_root", &p.IdentityRoot}, {"key_root", &p.KeyRoot}, {"profile_root", &p.ProfileRoot}} {
		*item.target, e = profilePath(fields[item.name])
		if e != nil {
			return p, "", e
		}
	}
	p.IdentityBinding, e = decodeIdentityBinding(fields["identity_binding"])
	if e != nil {
		return p, "", e
	}
	p.KeyBinding, e = decodeIssuanceBinding(fields["key_binding"])
	if e != nil {
		return p, "", e
	}
	output, e := profilePath(fields["profile_file"])
	if e == nil {
		for _, root := range []string{p.IdentityRoot, p.KeyRoot, p.ProfileRoot} {
			relative, err := filepath.Rel(root, output)
			if err != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return p, "", issuance.ErrInvalid
			}
		}
	}
	return p, output, e
}
func prepareProfileBinding(ctxRaw []byte) (string, []byte, error) {
	fields, e := profileConfigObject(ctxRaw, "profile_file", "expected_profile", "binding", "binding_file")
	if e != nil {
		return "", nil, e
	}
	source, e := profilePath(fields["profile_file"])
	if e != nil {
		return "", nil, e
	}
	output, e := profilePath(fields["binding_file"])
	if e != nil {
		return "", nil, e
	}
	expected, e := decodeIssuanceBinding(fields["expected_profile"])
	if e != nil {
		return "", nil, e
	}
	raw, e := readBounded(source, issuerprofile.MaximumSize)
	if e != nil {
		return "", nil, issuance.ErrUnavailable
	}
	verified, e := issuerprofile.Verify(raw, issuerprofile.Binding{Network: expected.Network, Issuer: expected.Issuer, Signer: expected.Signer, Start: expected.Start, End: expected.End})
	if e != nil {
		return "", nil, issuance.ErrInvalid
	}
	baseFields, e := profileConfigObject(fields["binding"], "network", "issuer", "authority", "profile", "duty", "start", "end")
	if e != nil {
		return "", nil, e
	}
	_, keys, _ := verified.Snapshot()
	entries := make([]admissionKeyInput, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, admissionKeyInput{Window: time.Unix(int64(k.Window), 0).UTC().Format(time.RFC3339), Class: strconv.FormatUint(uint64(k.Class), 10), SPKI: hex.EncodeToString(k.SPKI)})
	}
	baseFields["keys"], e = json.Marshal(entries)
	if e != nil {
		return "", nil, e
	}
	encoded, e := json.Marshal(baseFields)
	if e != nil {
		return "", nil, e
	}
	wrapper, e := json.Marshal(map[string]any{"root": output, "binding": json.RawMessage(encoded)})
	if e != nil {
		return "", nil, e
	}
	parsed, e := decodeAdmissionPlan(wrapper, "initialize")
	if e != nil {
		return "", nil, e
	}
	parsed.Binding.Keys = nil
	bound, e := admission.PrepareLedgerBinding(parsed.Binding, verified)
	if e != nil {
		return "", nil, issuance.ErrInvalid
	}
	// The previously parsed binding establishes the canonical input spelling;
	// the domain owner establishes matching inventory and independently pinned facts.
	if len(bound.Keys) != len(entries) {
		return "", nil, issuance.ErrInvalid
	}
	return output, encoded, nil
}
