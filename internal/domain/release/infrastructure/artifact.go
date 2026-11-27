package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
)

var (
	ErrInput         = errors.New("infrastructure: invalid input")
	ErrBinding       = errors.New("infrastructure: artifact binding differs")
	ErrAuthorization = errors.New("infrastructure: fresh Release pair refused")
)

// Declaration is a public byte declaration, never initial or Release authority.
type Declaration struct {
	Schema          string            `json:"schema"`
	Platform        string            `json:"platform"`
	ReleaseIdentity string            `json:"release_identity"`
	ReleaseVersion  int64             `json:"release_version"`
	Invocation      string            `json:"invocation"`
	Files           map[string]string `json:"files"`
}

func prepareDescriptor(identity string, version int64, program, template []byte, invocation, templateName string) ([]byte, error) {
	if len(identity) > 16<<10 || strings.TrimSpace(identity) == "" || version <= 0 || len(program) == 0 || len(program) > 64<<20 || len(template) == 0 || len(template) > 64<<20 {
		return nil, ErrInput
	}
	d := Declaration{"ardents-infrastructure-artifact-v1", "linux-amd64", identity, version, invocation, map[string]string{ProgramName: digest(program), templateName: digest(template)}}
	raw, err := json.Marshal(d)
	if err != nil || len(raw)+1 > 16<<10 {
		return nil, errors.Join(ErrInput, err)
	}
	return append(raw, '\n'), nil
}

// DecodeDescriptor accepts only the selected Source grammar. Other invocations
// remain refused even though ADR-0123 reserves their exact vocabulary.
func DecodeDescriptor(raw []byte) (Declaration, error) {
	return decodeDescriptor(raw, "source", TemplateName)
}

func decodeDescriptor(raw []byte, invocation, templateName string) (Declaration, error) {
	var d Declaration
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &d) != nil {
		return Declaration{}, ErrInput
	}
	if d.Schema != "ardents-infrastructure-artifact-v1" || d.Platform != "linux-amd64" || strings.TrimSpace(d.ReleaseIdentity) == "" || d.ReleaseVersion <= 0 || d.Invocation != invocation || len(d.Files) != 2 {
		return Declaration{}, ErrInput
	}
	for _, name := range []string{ProgramName, templateName} {
		if !canonicaldigest.Valid(d.Files[name]) {
			return Declaration{}, ErrInput
		}
	}
	canonical, err := json.Marshal(d)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return Declaration{}, ErrInput
	}
	return d, nil
}

// ValidateResources checks public declaration/byte agreement before a caller
// opens Release history. It produces no initial or signature authority.
func ValidateResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "source", TemplateName)
}

func validateResources(raw, program, template []byte, invocation, templateName string) (Declaration, error) {
	d, err := decodeDescriptor(raw, invocation, templateName)
	if err != nil {
		return Declaration{}, err
	}
	if len(program) == 0 || len(program) > 64<<20 || len(template) == 0 || len(template) > 64<<20 || d.Files[ProgramName] != digest(program) || d.Files[templateName] != digest(template) {
		return Declaration{}, ErrBinding
	}
	return d, nil
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

// PrepareRuntimeDescriptor is public unsigned preparation for the existing
// fixed role vocabulary. The platform template is an authenticated resource;
// operator-specific configuration is separately bound at launch.
func PrepareRuntimeDescriptor(role, identity string, version int64, program, template []byte) ([]byte, error) {
	_, name, _, ok := runtimeNames(role)
	if !ok {
		return nil, ErrInput
	}
	return prepareDescriptor(identity, version, program, template, role, name)
}

// ValidateRuntimeResources checks the selected role's canonical descriptor
// against its program and unit template. The returned declaration supplies
// public facts for Enrollment without granting Release or initial trust.
func ValidateRuntimeResources(role string, descriptor, program, template []byte) (Declaration, error) {
	_, name, _, ok := runtimeNames(role)
	if !ok {
		return Declaration{}, ErrInput
	}
	return validateResources(descriptor, program, template, role, name)
}

// ValidateForwardingResources checks exact public bytes before Release history is
// opened. It grants no initial provenance, native custody or accepting authority.
func ValidateForwardingResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "forwarding", ForwardingTemplateName)
}

// ValidateJoinResources compares exact public bytes before Release history is
// opened. Its detached declaration supplies no pin, authority or runtime right.
func ValidateJoinResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "join", JoinTemplateName)
}

// ValidateDescriptorResources compares exact public bytes before Release history is
// opened. Its detached declaration supplies no pin, authority or runtime right.
func ValidateDescriptorResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "descriptor", DescriptorTemplateName)
}
