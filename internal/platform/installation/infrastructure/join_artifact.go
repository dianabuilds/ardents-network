//go:build linux

package infrastructure

const (
	JoinGenerationTarget = "ardents/linux-amd64/infrastructure/join"
	JoinDescriptorName   = "infrastructure-join.json"
	JoinTemplateName     = "ardents-join.service"
)

// PrepareJoinDescriptor binds only the public JOIN release resources. It does
// not validate native unit policy, sign a Release or authorize installation.
func PrepareJoinDescriptor(identity string, version int64, program, template []byte) ([]byte, error) {
	return prepareDescriptor(identity, version, program, template, "join", JoinTemplateName)
}

// DecodeJoinDescriptor admits only the closed JOIN byte inventory. Source and
// Forwarding declarations cannot be substituted for this separate invocation.
func DecodeJoinDescriptor(raw []byte) (Declaration, error) {
	return decodeDescriptor(raw, "join", JoinTemplateName)
}

// ValidateJoinResources compares exact public bytes before Release history is
// opened. Its detached declaration supplies no pin, authority or runtime right.
func ValidateJoinResources(raw, program, template []byte) (Declaration, error) {
	d, err := DecodeJoinDescriptor(raw)
	if err != nil {
		return Declaration{}, err
	}
	if len(program) == 0 || len(program) > 64<<20 || len(template) == 0 || len(template) > 64<<20 || d.Files[ProgramName] != digest(program) || d.Files[JoinTemplateName] != digest(template) {
		return Declaration{}, ErrBinding
	}
	return d, nil
}
