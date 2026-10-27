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
	for name, expected := range map[string]string{
		programName: b.ProgramSHA256, infrastructure.ForwardingTemplateName: b.TemplateSHA256,
		infrastructure.ForwardingDescriptorName: b.Generation, enrollment.InfrastructureInventoryName: b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "forwarding-request.json": b.RequestSHA256,
		"forwarding-plan.json": b.PlanSHA256, "rendered-forwarding.service": b.UnitSHA256,
	} {
		if digest(files[name]) != expected {
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
	d, err := infrastructure.ValidateForwardingResources(files[infrastructure.ForwardingDescriptorName], files[programName], files[infrastructure.ForwardingTemplateName])
	if err != nil {
		return Plan{}, err
	}
	public := make(map[string][]byte, 7)
	for _, name := range []string{programName, infrastructure.ForwardingTemplateName, infrastructure.ForwardingDescriptorName, "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		public[name] = files[name]
	}
	inventory, err := enrollment.PrepareInfrastructureInventory(enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}, public)
	if err != nil || !bytes.Equal(inventory, files[enrollment.InfrastructureInventoryName]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	rendered, err := RenderUnit(files[infrastructure.ForwardingTemplateName], b.Request, b.Generation)
	if err != nil || !bytes.Equal(rendered, files["rendered-forwarding.service"]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	return plan, nil
}
