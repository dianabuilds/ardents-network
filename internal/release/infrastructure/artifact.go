package infrastructure

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

const (
	ProgramTarget    = "ardents/linux-amd64/node"
	GenerationTarget = "ardents/linux-amd64/infrastructure/source"
	ProgramName      = "ardents-node-linux-amd64"
	DescriptorName   = "infrastructure-source.json"
	TemplateName     = "ardents-source.service"
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
		value := d.Files[name]
		if len(value) != 64 || strings.ToLower(value) != value {
			return Declaration{}, ErrInput
		}
		if _, err := hex.DecodeString(value); err != nil {
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

func runtimeNames(role string) (descriptor, template, target string, ok bool) {
	switch role {
	case "source", "forwarding", "issuer", "descriptor", "introduction", "join":
		return "infrastructure-" + role + ".json", "ardents-" + role + ".service", "ardents/linux-amd64/infrastructure/" + role, true
	default:
		return "", "", "", false
	}
}

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
