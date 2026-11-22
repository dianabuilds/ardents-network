package infrastructure

const (
	JoinGenerationTarget = "ardents/linux-amd64/infrastructure/join"
	JoinDescriptorName   = "infrastructure-join.json"
	JoinTemplateName     = "ardents-join.service"
)

// DecodeJoinDescriptor admits only the closed JOIN byte inventory. Source and
// Forwarding declarations cannot be substituted for this separate invocation.
func DecodeJoinDescriptor(raw []byte) (Declaration, error) {
	return decodeDescriptor(raw, "join", JoinTemplateName)
}

// ValidateJoinResources compares exact public bytes before Release history is
// opened. Its detached declaration supplies no pin, authority or runtime right.
func ValidateJoinResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "join", JoinTemplateName)
}
