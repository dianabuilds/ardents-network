package infrastructure

const (
	DescriptorGenerationTarget = "ardents/linux-amd64/infrastructure/descriptor"
	DescriptorArtifactName     = "infrastructure-descriptor.json"
	DescriptorTemplateName     = "ardents-descriptor.service"
)

// ValidateDescriptorResources compares exact public bytes before Release history is
// opened. Its detached declaration supplies no pin, authority or runtime right.
func ValidateDescriptorResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "descriptor", DescriptorTemplateName)
}
