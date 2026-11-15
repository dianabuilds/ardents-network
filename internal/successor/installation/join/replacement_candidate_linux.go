package join

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"

	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

type replacementCandidate struct {
	selection generationSelection
	files     map[string][]byte
	unit      []byte
}

// The lifecycle caller must retain actual independent Started/retirement,
// input/verifier and semantic owners. Preparation alone writes nothing and
// grants no native generation birth, selection, reload, Start or recovery right.
func prepareReplacement(retired *started, a releaseartifact.JoinAuthorization, input Inputs) (*replacementCandidate, error) {
	if retired == nil || retired.completed.original == nil || retired.retirement == nil || retired.snapshot == nil || retired.snapshot.number+1 >= attemptLimit || input.original == nil {
		return nil, ErrInput
	}
	if _, initial := a.InitialFacts(); initial {
		return nil, releaseartifact.ErrAuthorization
	}
	c := retired.completed.original
	old, found := c.selections[digest(c.files[filepath.Join(c.binding.Request.InstallationRoot, "binding.json")].body)]
	if !found {
		return nil, ErrBinding
	}
	r := retired.retirement
	if r.bindingSHA != digest(old.raw) || r.binding.Generation != old.binding.Generation || r.number+1 >= attemptLimit || r.result.Code != 1 || r.result.Status != 0 {
		return nil, ErrBinding
	}
	previous, err := releaseartifact.DecodeJoinDescriptor(c.files[filepath.Join(c.sealed, releaseartifact.JoinDescriptorName)].body)
	if err != nil {
		return nil, err
	}
	next, err := releaseartifact.DecodeJoinDescriptor(a.Descriptor())
	if err != nil || next.ReleaseVersion <= previous.ReleaseVersion {
		return nil, errors.Join(ErrBinding, err)
	}
	p := input.original
	delivery, err := p.readDelivery()
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, f := range delivery {
			clear(f.raw)
		}
	}()
	if !sameRetainedRequest(p.request, old.binding.Request) {
		return nil, ErrBinding
	}
	oldPlan := c.files[filepath.Join(c.sealed, "join-plan.json")].body
	if err := checkReplacementPlan(oldPlan, delivery[1].raw, digest(a.Program()) != old.binding.ProgramSHA256); err != nil {
		return nil, err
	}
	for n, name := range []string{"certificate.pem", "private-key.pem"} {
		original := c.files[filepath.Join(old.binding.Request.CredentialDirectory, name)]
		if original == nil || !bytes.Equal(original.body, delivery[n+2].raw) {
			return nil, ErrBinding
		}
	}
	files := a.Artifacts()
	if len(files) != 8 {
		return nil, ErrBinding
	}
	generation := digest(a.Descriptor())
	unit, err := renderUnit(a.Template(), p.request, generation)
	if err != nil {
		return nil, err
	}
	files["join-request.json"], files["join-plan.json"], files["rendered-join.service"] = bytes.Clone(delivery[0].raw), bytes.Clone(delivery[1].raw), unit
	b := binding{Schema: "ardents-join-replacement-configuration-v1", Invocation: "join", Generation: generation,
		ProgramTarget: releaseartifact.ProgramTarget, GenerationTarget: releaseartifact.JoinGenerationTarget,
		ProgramSHA256: digest(a.Program()), TemplateSHA256: digest(a.Template()), InventorySHA256: digest(files["infrastructure-enrollment.json"]), TargetsSHA256: digest(files["targets.json"]),
		RequestSHA256: digest(files["join-request.json"]), PlanSHA256: digest(files["join-plan.json"]), UnitSHA256: digest(unit), CertificateSHA256: digest(delivery[2].raw),
		Account: old.binding.Account, UID: old.binding.UID, GID: old.binding.GID, History: old.binding.History, Request: p.request}
	outer := replacementBinding{Schema: "ardents-join-replacement-binding-v1", Configuration: b, InitialGeneration: old.initialGeneration(),
		PredecessorGeneration: old.binding.Generation, PredecessorBindingSHA256: digest(old.raw), RetirementSHA256: r.sha256, PredecessorAttempt: uint32(r.number)}
	raw, err := json.Marshal(outer)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	selected, err := decodeSelection(raw)
	if err != nil || selected.matchPredecessor(old) != nil {
		return nil, errors.Join(ErrBinding, err)
	}
	files["binding.json"] = bytes.Clone(raw)
	if _, err := validateInitialGeneration(b, raw, files); err != nil {
		return nil, err
	}
	return &replacementCandidate{selection: selected, files: files, unit: unit}, nil
}
