package introduction

import (
	"bytes"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
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
	facts, hashes, matches, err := releaseartifact.CheckIntroductionInventory(files, releaseartifact.InventoryCommitments{
		Program: b.ProgramSHA256, Template: b.TemplateSHA256, Artifact: b.Generation, Inventory: b.InventorySHA256,
		Targets: b.TargetsSHA256,
	})
	if err != nil {
		return plan, errors.Join(ErrBinding, err)
	}
	if !matches || digest(files["introduction-request.json"]) != b.RequestSHA256 || digest(files["introduction-plan.json"]) != b.PlanSHA256 || digest(files["rendered-introduction.service"]) != b.UnitSHA256 {
		return plan, ErrBinding
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
		return Plan{}, errors.Join(ErrBinding, releaseartifact.ErrBinding)
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
