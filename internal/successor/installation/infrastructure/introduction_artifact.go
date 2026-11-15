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
	d, err := DecodeIntroductionArtifact(raw)
	if err != nil {
		return Declaration{}, err
	}
	if len(program) == 0 || len(program) > 64<<20 || len(template) == 0 || len(template) > 64<<20 || d.Files[ProgramName] != digest(program) || d.Files[IntroductionTemplateName] != digest(template) {
		return Declaration{}, ErrBinding
	}
	return d, nil
}
