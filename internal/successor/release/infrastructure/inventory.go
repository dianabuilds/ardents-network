package infrastructure

import "github.com/dianabuilds/ardents-network/internal/successor/enrollment"

// InventoryCommitments are detached expected digests. They grant neither
// Enrollment provenance nor Release authorization.
type InventoryCommitments struct {
	Program, Template, Artifact, Inventory, Targets string
}

// MatchesInventoryResources compares a decoded role declaration with digests
// already checked against the actual inventory bytes. It grants no authority.
func MatchesInventoryResources(d Declaration, hashes map[string]string) bool {
	var template string
	switch d.Invocation {
	case "forwarding":
		template = ForwardingTemplateName
	case "join":
		template = JoinTemplateName
	case "descriptor":
		template = DescriptorTemplateName
	case "introduction":
		template = IntroductionTemplateName
	case "source":
		template = TemplateName
	default:
		return false
	}
	program, unit := d.Files[ProgramName], d.Files[template]
	return program != "" && unit != "" && program == hashes[ProgramName] && unit == hashes[template]
}

// CheckJoinInventory compares the fixed JOIN generation's original bytes.
func CheckJoinInventory(files map[string][]byte, expected InventoryCommitments) (enrollment.InfrastructureFacts, map[string]string, bool, error) {
	return checkInventory(files, expected, JoinTemplateName, JoinDescriptorName)
}

// CheckDescriptorInventory compares the fixed Descriptor generation's original bytes.
func CheckDescriptorInventory(files map[string][]byte, expected InventoryCommitments) (enrollment.InfrastructureFacts, map[string]string, bool, error) {
	return checkInventory(files, expected, DescriptorTemplateName, DescriptorArtifactName)
}

// CheckIntroductionInventory compares the fixed Introduction generation's original bytes.
func CheckIntroductionInventory(files map[string][]byte, expected InventoryCommitments) (enrollment.InfrastructureFacts, map[string]string, bool, error) {
	return checkInventory(files, expected, IntroductionTemplateName, IntroductionArtifactName)
}

// CheckForwardingInventory compares the fixed Forwarding generation's original bytes.
func CheckForwardingInventory(files map[string][]byte, expected InventoryCommitments) (enrollment.InfrastructureFacts, map[string]string, bool, error) {
	return checkInventory(files, expected, ForwardingTemplateName, ForwardingDescriptorName)
}

func checkInventory(files map[string][]byte, expected InventoryCommitments, template, artifact string) (enrollment.InfrastructureFacts, map[string]string, bool, error) {
	public := make(map[string][]byte, 7)
	for _, name := range []string{ProgramName, template, artifact, "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		public[name] = files[name]
	}
	facts, hashes, err := enrollment.ValidateInfrastructureInventory(files[enrollment.InfrastructureInventoryName], public)
	if err != nil {
		return facts, nil, false, err
	}
	imageDigest := func(name string) string {
		if sum, ok := hashes[name]; ok {
			return sum
		}
		sum := digest(files[name])
		hashes[name] = sum
		return sum
	}
	for name, wanted := range map[string]string{
		ProgramName: expected.Program, template: expected.Template,
		artifact: expected.Artifact, enrollment.InfrastructureInventoryName: expected.Inventory,
		"targets.json": expected.Targets,
	} {
		if imageDigest(name) != wanted {
			return facts, hashes, false, nil
		}
	}
	return facts, hashes, true, nil
}
