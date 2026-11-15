package infrastructure

const (
	DescriptorGenerationTarget = "ardents/linux-amd64/infrastructure/descriptor"
	DescriptorArtifactName     = "infrastructure-descriptor.json"
	DescriptorTemplateName     = "ardents-descriptor.service"
)

// PrepareDescriptorArtifact binds only the public Descriptor release resources. It does
// not validate native unit policy, sign a Release or authorize installation.
func PrepareDescriptorArtifact(identity string, version int64, program, template []byte) ([]byte, error) {
	return prepareDescriptor(identity, version, program, template, "descriptor", DescriptorTemplateName)
}

// DecodeDescriptorArtifact admits only the closed Descriptor byte inventory. Source and
// Forwarding declarations cannot be substituted for this separate invocation.
func DecodeDescriptorArtifact(raw []byte) (Declaration, error) {
	return decodeDescriptor(raw, "descriptor", DescriptorTemplateName)
}

// ValidateDescriptorResources compares exact public bytes before Release history is
// opened. Its detached declaration supplies no pin, authority or runtime right.
func ValidateDescriptorResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "descriptor", DescriptorTemplateName)
}
