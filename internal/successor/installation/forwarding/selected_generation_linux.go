package forwarding

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

// This is a finite byte grammar, shared by independently acquired root and
// runtime observations. It carries no descriptors, private key or live right.
func forwardingGenerationNames() []string {
	return []string{programName, infrastructure.ForwardingTemplateName, infrastructure.ForwardingDescriptorName, enrollment.InfrastructureInventoryName,
		"root.json", "timestamp.json", "snapshot.json", "targets.json", "forwarding-request.json", "forwarding-plan.json", "rendered-forwarding.service", "binding.json"}
}

func validateSelectedGeneration(b binding, selected []byte, files map[string][]byte) (Plan, error) {
	var plan Plan
	if len(files) != len(forwardingGenerationNames()) || !bytes.Equal(selected, files["binding.json"]) {
		return plan, ErrBinding
	}
	for _, name := range forwardingGenerationNames() {
		if len(files[name]) == 0 {
			return plan, ErrBinding
		}
	}
	public := make(map[string][]byte, 7)
	for _, name := range []string{programName, infrastructure.ForwardingTemplateName, infrastructure.ForwardingDescriptorName, "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
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
		programName: b.ProgramSHA256, infrastructure.ForwardingTemplateName: b.TemplateSHA256,
		infrastructure.ForwardingDescriptorName: b.Generation, enrollment.InfrastructureInventoryName: b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "forwarding-request.json": b.RequestSHA256,
		"forwarding-plan.json": b.PlanSHA256, "rendered-forwarding.service": b.UnitSHA256,
	} {
		if imageDigest(name) != expected {
			return plan, ErrBinding
		}
	}
	request, err := DecodeRequest(files["forwarding-request.json"])
	if err != nil || request != b.Request {
		return plan, errors.Join(ErrBinding, err)
	}
	plan, err = DecodePlan(files["forwarding-plan.json"])
	if err != nil || b.Request.MatchPlan(plan) != nil {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	d, err := infrastructure.DecodeForwardingDescriptor(files[infrastructure.ForwardingDescriptorName])
	if err != nil {
		return Plan{}, err
	}
	// Inventory validation already checked the same actual program/template
	// bytes and their original 64 MiB bounds. Decoding here retains the separate
	// Forwarding grammar; its declarations must match those fresh byte digests.
	if d.Files[programName] != hashes[programName] || d.Files[infrastructure.ForwardingTemplateName] != hashes[infrastructure.ForwardingTemplateName] {
		return Plan{}, errors.Join(ErrBinding, infrastructure.ErrBinding)
	}
	if facts != (enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}) {
		return Plan{}, ErrBinding
	}
	rendered, err := RenderUnit(files[infrastructure.ForwardingTemplateName], b.Request, b.Generation)
	if err != nil || !bytes.Equal(rendered, files["rendered-forwarding.service"]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	return plan, nil
}
