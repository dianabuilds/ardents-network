package join

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

// This finite byte grammar is consumed by independent original root readers. It carries no descriptors, private key or live right.
func joinGenerationNames() []string {
	return []string{programName, infrastructure.JoinTemplateName, infrastructure.JoinDescriptorName, enrollment.InfrastructureInventoryName,
		"root.json", "timestamp.json", "snapshot.json", "targets.json", "join-request.json", "join-plan.json", "rendered-join.service", "binding.json"}
}

func validateInitialGeneration(b binding, selected []byte, files map[string][]byte) (Plan, error) {
	var plan Plan
	if len(files) != len(joinGenerationNames()) || !bytes.Equal(selected, files["binding.json"]) {
		return plan, ErrBinding
	}
	for _, name := range joinGenerationNames() {
		if len(files[name]) == 0 {
			return plan, ErrBinding
		}
	}
	public := make(map[string][]byte, 7)
	for _, name := range []string{programName, infrastructure.JoinTemplateName, infrastructure.JoinDescriptorName, "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
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
		programName: b.ProgramSHA256, infrastructure.JoinTemplateName: b.TemplateSHA256,
		infrastructure.JoinDescriptorName: b.Generation, enrollment.InfrastructureInventoryName: b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "join-request.json": b.RequestSHA256,
		"join-plan.json": b.PlanSHA256, "rendered-join.service": b.UnitSHA256,
	} {
		if imageDigest(name) != expected {
			return plan, ErrBinding
		}
	}
	request, err := DecodeRequest(files["join-request.json"])
	if err != nil || request != b.Request {
		return plan, errors.Join(ErrBinding, err)
	}
	plan, err = DecodePlan(files["join-plan.json"])
	if err != nil || b.Request.MatchPlan(plan) != nil {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	d, err := infrastructure.DecodeJoinDescriptor(files[infrastructure.JoinDescriptorName])
	if err != nil {
		return Plan{}, err
	}
	// Inventory validation already checked the same actual program/template
	// bytes and their original 64 MiB bounds. Decoding here retains the separate
	// Join grammar; its declarations must match those fresh byte digests.
	if d.Files[programName] != hashes[programName] || d.Files[infrastructure.JoinTemplateName] != hashes[infrastructure.JoinTemplateName] {
		return Plan{}, errors.Join(ErrBinding, infrastructure.ErrBinding)
	}
	if facts != (enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}) {
		return Plan{}, ErrBinding
	}
	rendered, err := renderUnit(files[infrastructure.JoinTemplateName], b.Request, b.Generation)
	if err != nil || !bytes.Equal(rendered, files["rendered-join.service"]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	return plan, nil
}
