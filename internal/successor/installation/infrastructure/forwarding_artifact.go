//go:build linux

package infrastructure

const (
	ForwardingGenerationTarget = "ardents/linux-amd64/infrastructure/forwarding"
	ForwardingDescriptorName   = "infrastructure-forwarding.json"
	ForwardingTemplateName     = "ardents-forwarding.service"
)

// PrepareForwardingDescriptor binds only the public Forwarding release resources.
// It supplies neither a Release signature nor native installation permission.
func PrepareForwardingDescriptor(identity string, version int64, program, template []byte) ([]byte, error) {
	return prepareDescriptor(identity, version, program, template, "forwarding", ForwardingTemplateName)
}

// DecodeForwardingDescriptor preserves the separate Source grammar. A descriptor
// for another duty cannot select a runtime or extend Source installation rights.
func DecodeForwardingDescriptor(raw []byte) (Declaration, error) {
	return decodeDescriptor(raw, "forwarding", ForwardingTemplateName)
}

// ValidateForwardingResources checks exact public bytes before Release history is
// opened. It grants no initial provenance, native custody or accepting authority.
func ValidateForwardingResources(raw, program, template []byte) (Declaration, error) {
	d, err := DecodeForwardingDescriptor(raw)
	if err != nil {
		return Declaration{}, err
	}
	if len(program) == 0 || len(program) > 64<<20 || len(template) == 0 || len(template) > 64<<20 || d.Files[ProgramName] != digest(program) || d.Files[ForwardingTemplateName] != digest(template) {
		return Declaration{}, ErrBinding
	}
	return d, nil
}
