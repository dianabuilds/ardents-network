//go:build linux

package infrastructure

const (
	IntroductionGenerationTarget = "ardents/linux-amd64/infrastructure/introduction"
	IntroductionArtifactName     = "infrastructure-introduction.json"
	IntroductionTemplateName     = "ardents-introduction.service"
)

// PrepareIntroductionArtifact binds only the public Introduction release resources. It does
// not validate native unit policy, sign a Release or authorize installation.
func PrepareIntroductionArtifact(identity string, version int64, program, template []byte) ([]byte, error) {
	return prepareDescriptor(identity, version, program, template, "introduction", IntroductionTemplateName)
}

// DecodeIntroductionArtifact admits only the closed Introduction byte inventory. Source and
// Forwarding declarations cannot be substituted for this separate invocation.
func DecodeIntroductionArtifact(raw []byte) (Declaration, error) {
	return decodeDescriptor(raw, "introduction", IntroductionTemplateName)
}

// ValidateIntroductionResources compares exact public bytes before Release history is
// opened. Its detached declaration supplies no pin, authority or runtime right.
func ValidateIntroductionResources(raw, program, template []byte) (Declaration, error) {
	return validateResources(raw, program, template, "introduction", IntroductionTemplateName)
}
