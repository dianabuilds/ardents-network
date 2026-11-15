package descriptor

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// This finite byte grammar is consumed by independent original root readers. It carries no descriptors, private key or live right.
func descriptorGenerationNames() []string {
	return []string{programName, releaseartifact.DescriptorTemplateName, releaseartifact.DescriptorArtifactName, enrollment.InfrastructureInventoryName,
		"root.json", "timestamp.json", "snapshot.json", "targets.json", "descriptor-request.json", "descriptor-plan.json", "rendered-descriptor.service", "binding.json"}
}

func validateInitialGeneration(b binding, selected []byte, files map[string][]byte) (Plan, error) {
	var plan Plan
	selection, selectionErr := decodeSelection(selected)
	if selectionErr != nil || selection.binding != b {
		return plan, ErrBinding
	}
	if len(files) != len(descriptorGenerationNames()) || !bytes.Equal(selected, files["binding.json"]) {
		return plan, ErrBinding
	}
	for _, name := range descriptorGenerationNames() {
		if len(files[name]) == 0 {
			return plan, ErrBinding
		}
	}
	facts, hashes, matches, err := releaseartifact.CheckDescriptorInventory(files, releaseartifact.InventoryCommitments{
		Program: b.ProgramSHA256, Template: b.TemplateSHA256, Artifact: b.Generation, Inventory: b.InventorySHA256,
		Targets: b.TargetsSHA256,
	})
	if err != nil {
		return plan, errors.Join(ErrBinding, err)
	}
	if !matches || digest(files["descriptor-request.json"]) != b.RequestSHA256 || digest(files["descriptor-plan.json"]) != b.PlanSHA256 || digest(files["rendered-descriptor.service"]) != b.UnitSHA256 {
		return plan, ErrBinding
	}

	request, err := DecodeRequest(files["descriptor-request.json"])
	if err != nil || request != b.Request {
		return plan, errors.Join(ErrBinding, err)
	}
	plan, err = DecodePlan(files["descriptor-plan.json"])
	if err != nil || b.Request.MatchPlan(plan) != nil {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	d, err := releaseartifact.DecodeDescriptorArtifact(files[releaseartifact.DescriptorArtifactName])
	if err != nil {
		return Plan{}, err
	}
	// Inventory validation already checked the same actual program/template
	// bytes and their original 64 MiB bounds. Decoding here retains the separate
	// Descriptor grammar; its declarations must match those fresh byte digests.
	if d.Files[programName] != hashes[programName] || d.Files[releaseartifact.DescriptorTemplateName] != hashes[releaseartifact.DescriptorTemplateName] {
		return Plan{}, errors.Join(ErrBinding, releaseartifact.ErrBinding)
	}
	if facts != (enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}) {
		return Plan{}, ErrBinding
	}
	rendered, err := renderUnit(files[releaseartifact.DescriptorTemplateName], b.Request, b.Generation)
	if err != nil || !bytes.Equal(rendered, files["rendered-descriptor.service"]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	return plan, nil
}

// The independent history reader rederives forwardness; signed or complete
// replacement bytes cannot substitute this policy or current Release admission.
func validateForwardImage(oldDescriptor, nextDescriptor, oldPlan, nextPlan []byte, programChanged bool) error {
	old, err := releaseartifact.DecodeDescriptorArtifact(oldDescriptor)
	if err != nil {
		return err
	}
	next, err := releaseartifact.DecodeDescriptorArtifact(nextDescriptor)
	if err != nil || next.ReleaseVersion <= old.ReleaseVersion {
		return errors.Join(ErrBinding, err)
	}
	return checkReplacementPlan(oldPlan, nextPlan, programChanged)
}
