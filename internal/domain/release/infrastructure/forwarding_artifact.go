package infrastructure

const (
	ForwardingGenerationTarget = "ardents/linux-amd64/infrastructure/forwarding"
	ForwardingDescriptorName   = "infrastructure-forwarding.json"
	ForwardingTemplateName     = "ardents-forwarding.service"
)

// ValidateForwardingResources checks exact public bytes before Release history is
// opened. It grants no initial provenance, native custody or accepting authority.
func ValidateForwardingResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "forwarding", ForwardingTemplateName)
}
