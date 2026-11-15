package introduction

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// This finite byte grammar is consumed by independent original root readers. It carries no file descriptor, private key or live right.
func introductionGenerationNames() []string {
	return []string{programName, releaseartifact.IntroductionTemplateName, releaseartifact.IntroductionArtifactName, enrollment.InfrastructureInventoryName,
		"root.json", "timestamp.json", "snapshot.json", "targets.json", "introduction-request.json", "introduction-plan.json", "rendered-introduction.service", "binding.json"}
}

func validateInitialGeneration(b binding, selected []byte, files map[string][]byte) (Plan, error) {
	var plan Plan
	selection, selectionErr := decodeBinding(selected)
	if selectionErr != nil || selection != b {
		return plan, ErrBinding
	}
	if len(files) != len(introductionGenerationNames()) || !bytes.Equal(selected, files["binding.json"]) {
		return plan, ErrBinding
	}
	for _, name := range introductionGenerationNames() {
		if len(files[name]) == 0 {
			return plan, ErrBinding
		}
	}
	public := make(map[string][]byte, 7)
	for _, name := range []string{programName, releaseartifact.IntroductionTemplateName, releaseartifact.IntroductionArtifactName, "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		public[name] = files[name]
	}
	facts, hashes, err := enrollment.ValidateInfrastructureInventory(files[enrollment.InfrastructureInventoryName], public)
	if err != nil {
		return plan, errors.Join(ErrBinding, err)
	}
	// These digests belong only to this immutable byte image. Reuse each fresh
	// computation for its inventory, introduction and binding commitments; retain
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
		programName: b.ProgramSHA256, releaseartifact.IntroductionTemplateName: b.TemplateSHA256,
		releaseartifact.IntroductionArtifactName: b.Generation, enrollment.InfrastructureInventoryName: b.InventorySHA256,
		"targets.json": b.TargetsSHA256, "introduction-request.json": b.RequestSHA256,
		"introduction-plan.json": b.PlanSHA256, "rendered-introduction.service": b.UnitSHA256,
	} {
		if imageDigest(name) != expected {
			return plan, ErrBinding
		}
	}
	request, err := DecodeRequest(files["introduction-request.json"])
	if err != nil || request != b.Request {
		return plan, errors.Join(ErrBinding, err)
	}
	plan, err = DecodePlan(files["introduction-plan.json"])
	if err != nil || b.Request.MatchPlan(plan) != nil {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	d, err := releaseartifact.DecodeIntroductionArtifact(files[releaseartifact.IntroductionArtifactName])
	if err != nil {
		return Plan{}, err
	}
	// Inventory validation already checked the same actual program/template
	// bytes and their original 64 MiB bounds. Decoding here retains the separate
	// Introduction grammar; its declarations must match those fresh byte digests.
	if d.Files[programName] != hashes[programName] || d.Files[releaseartifact.IntroductionTemplateName] != hashes[releaseartifact.IntroductionTemplateName] {
		return Plan{}, errors.Join(ErrBinding, infrastructure.ErrBinding)
	}
	if facts != (enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}) {
		return Plan{}, ErrBinding
	}
	rendered, err := renderUnit(files[releaseartifact.IntroductionTemplateName], b.Request, b.Generation)
	if err != nil || !bytes.Equal(rendered, files["rendered-introduction.service"]) {
		return Plan{}, errors.Join(ErrBinding, err)
	}
	return plan, nil
}
