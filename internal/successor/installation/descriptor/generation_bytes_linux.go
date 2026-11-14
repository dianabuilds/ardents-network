package descriptor

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

// This finite byte grammar is consumed by independent original root readers. It carries no descriptors, private key or live right.
func descriptorGenerationNames() []string {
	return []string{programName, infrastructure.DescriptorTemplateName, infrastructure.DescriptorArtifactName, enrollment.InfrastructureInventoryName,
		"root.json", "timestamp.json", "snapshot.json", "targets.json", "descriptor-request.json", "descriptor-plan.json", "rendered-descriptor.service", "binding.json"}
}

func validateInitialGeneration(b binding, selected []byte, files map[string][]byte) (Plan, error) {
	var plan Plan
	selection, selectionErr := decodeBinding(selected)
	if selectionErr != nil || selection != b {
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
	public := make(map[string][]byte, 7)
	for _, name := range []string{programName, infrastructure.DescriptorTemplateName, infrastructure.DescriptorArtifactName, "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		public[name] = files[name]
	}
	facts, hashes, err := enrollment.ValidateInfrastructureInventory(files[enrollment.InfrastructureInventoryName], public)
	if err != nil {
		return plan, errors.Join(ErrBinding, err)
	}
	// These digests belong only to this immutable byte image. Reuse each fresh
	// computation for its inventory, descriptor and binding commitments; retain
	// no digest or accepting observation across calls or physical observations.
	imageDigest := func(name string) string {
		if sum, ok := hashes[name]; ok {
			return sum
		}
		sum := digest(files[name])
		hashes[name] = sum
		return sum
	}
	for name, expected := range map[string]string{
		programName: b.ProgramSHA256, infrastructure.DescriptorTemplateName: b.TemplateSHA256,
		infrastructure.DescriptorArtifactName: b.Generation, enrollment.InfrastructureInventoryName: b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "descriptor-request.json": b.RequestSHA256,
		"descriptor-plan.json": b.PlanSHA256, "rendered-descriptor.service": b.UnitSHA256,
	} {
		if imageDigest(name) != expected {
			return plan, ErrBinding
		}
	}
	request, err := DecodeRequest(files["descriptor-request.json"])
	if err != nil || request != b.Request {
		return plan, errors.Join(ErrBinding, err)
	}
	plan, err = DecodePlan(files["descriptor-plan.json"])
	if err != nil || b.Request.MatchPlan(plan) != nil {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	d, err := infrastructure.DecodeDescriptorArtifact(files[infrastructure.DescriptorArtifactName])
	if err != nil {
		return Plan{}, err
	}
	// Inventory validation already checked the same actual program/template
	// bytes and their original 64 MiB bounds. Decoding here retains the separate
	// Descriptor grammar; its declarations must match those fresh byte digests.
	if d.Files[programName] != hashes[programName] || d.Files[infrastructure.DescriptorTemplateName] != hashes[infrastructure.DescriptorTemplateName] {
		return Plan{}, errors.Join(ErrBinding, infrastructure.ErrBinding)
	}
	if facts != (enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}) {
		return Plan{}, ErrBinding
	}
	rendered, err := renderUnit(files[infrastructure.DescriptorTemplateName], b.Request, b.Generation)
	if err != nil || !bytes.Equal(rendered, files["rendered-descriptor.service"]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	return plan, nil
}
