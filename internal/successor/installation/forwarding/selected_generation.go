package forwarding

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// This is a finite byte grammar, shared by independently acquired root and
// runtime observations. It carries no descriptors, private key or live right.
func forwardingGenerationNames() []string {
	return []string{programName, releaseartifact.ForwardingTemplateName, releaseartifact.ForwardingDescriptorName, enrollment.InfrastructureInventoryName,
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
	facts, hashes, matches, err := releaseartifact.CheckForwardingInventory(files, releaseartifact.InventoryCommitments{
		Program: b.ProgramSHA256, Template: b.TemplateSHA256, Artifact: b.Generation, Inventory: b.InventorySHA256,
		Targets: b.TargetsSHA256,
	})
	if err != nil {
		return plan, errors.Join(ErrBinding, err)
	}
	if !matches || digest(files["forwarding-request.json"]) != b.RequestSHA256 || digest(files["forwarding-plan.json"]) != b.PlanSHA256 || digest(files["rendered-forwarding.service"]) != b.UnitSHA256 {
		return plan, ErrBinding
	}

	request, err := DecodeRequest(files["forwarding-request.json"])
	if err != nil || request != b.Request {
		return plan, errors.Join(ErrBinding, err)
	}
	plan, err = DecodePlan(files["forwarding-plan.json"])
	if err != nil || b.Request.MatchPlan(plan) != nil {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	d, err := releaseartifact.DecodeForwardingDescriptor(files[releaseartifact.ForwardingDescriptorName])
	if err != nil {
		return Plan{}, err
	}
	// Inventory validation already checked the same actual program/template
	// bytes and their original 64 MiB bounds. Decoding here retains the separate
	// Forwarding grammar; its declarations must match those fresh byte digests.
	if d.Files[programName] != hashes[programName] || d.Files[releaseartifact.ForwardingTemplateName] != hashes[releaseartifact.ForwardingTemplateName] {
		return Plan{}, errors.Join(ErrBinding, releaseartifact.ErrBinding)
	}
	if facts != (enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}) {
		return Plan{}, ErrBinding
	}
	rendered, err := RenderUnit(files[releaseartifact.ForwardingTemplateName], b.Request, b.Generation)
	if err != nil || !bytes.Equal(rendered, files["rendered-forwarding.service"]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	return plan, nil
}
