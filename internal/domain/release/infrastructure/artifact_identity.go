package infrastructure

const (
	ProgramTarget    = "ardents/linux-amd64/node"
	GenerationTarget = "ardents/linux-amd64/infrastructure/source"
	ProgramName      = "ardents-node-linux-amd64"
	DescriptorName   = "infrastructure-source.json"
	TemplateName     = "ardents-source.service"

	ForwardingGenerationTarget = "ardents/linux-amd64/infrastructure/forwarding"
	ForwardingDescriptorName   = "infrastructure-forwarding.json"
	ForwardingTemplateName     = "ardents-forwarding.service"

	JoinGenerationTarget = "ardents/linux-amd64/infrastructure/join"
	JoinDescriptorName   = "infrastructure-join.json"
	JoinTemplateName     = "ardents-join.service"

	DescriptorGenerationTarget = "ardents/linux-amd64/infrastructure/descriptor"
	DescriptorArtifactName     = "infrastructure-descriptor.json"
	DescriptorTemplateName     = "ardents-descriptor.service"
)

func runtimeNames(role string) (descriptor, template, target string, ok bool) {
	switch role {
	case "source":
		return DescriptorName, TemplateName, GenerationTarget, true
	case "forwarding":
		return ForwardingDescriptorName, ForwardingTemplateName, ForwardingGenerationTarget, true
	case "join":
		return JoinDescriptorName, JoinTemplateName, JoinGenerationTarget, true
	case "descriptor":
		return DescriptorArtifactName, DescriptorTemplateName, DescriptorGenerationTarget, true
	case "issuer", "introduction":
		return "infrastructure-" + role + ".json", "ardents-" + role + ".service", "ardents/linux-amd64/infrastructure/" + role, true
	default:
		return "", "", "", false
	}
}
