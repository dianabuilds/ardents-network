package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
)

type sourceReplacementCandidate struct {
	binding   sourceReplacementBinding
	raw, unit []byte
	files     map[string][]byte
}

// This finite composition prepares immutable configuration only after its
// caller retained the original complete retirement and genuine forward Release
// pair. Nothing here grants native birth, selection, manager or Start rights.
func prepareSourceReplacement(retired *sourceRetirement, a Authorization, input SourceInputs) (*sourceReplacementCandidate, error) {
	if retired == nil || retired.chain == nil || retired.chain.initial == nil || retired.chain.latest == nil || retired.chain.latestSelection == nil || input.original == nil || len(input.original.files) != 5 || a.origin == nil || retired.chain.latestSelection != retired.chain.initial.selection {
		return nil, ErrInput
	}
	if _, initial := a.InitialFacts(); initial {
		return nil, ErrAuthorization
	}
	old := retired.chain.initial.selection
	p := old.binding
	r := input.Declaration()
	if r.InstallationRoot != p.Request.InstallationRoot || r.StateRoot != p.Request.StateRoot || r.ParticipationRoot != p.Request.ParticipationRoot ||
		r.CredentialDirectory != p.Request.CredentialDirectory || r.Clock != p.Request.Clock || r.Profile != p.Request.Profile {
		return nil, ErrBinding
	}
	next, err := DecodeDescriptor(a.Descriptor())
	if err != nil {
		return nil, err
	}
	previous, err := DecodeDescriptor(old.files[filepath.Join(old.sealed, DescriptorName)].raw)
	if err != nil || next.ReleaseVersion <= previous.ReleaseVersion {
		return nil, errors.Join(ErrBinding, err)
	}
	if retired.chain.latest.number+1 >= sourceAttemptLimit {
		return nil, ErrBinding
	}
	if err := checkSourceForwardPlan(old.files[filepath.Join(old.sealed, "source-plan.json")].raw, input.original.files[1].raw); err != nil {
		return nil, err
	}
	if digest(a.Program()) == p.ProgramSHA256 && r.Address == p.Request.Address {
		return nil, ErrBinding
	}
	for i, name := range []string{"certificate.pem", "private-key.pem", "client-ca.pem"} {
		original := old.files[filepath.Join(r.CredentialDirectory, name)]
		if original == nil || !bytes.Equal(original.raw, input.original.files[i+2].raw) {
			return nil, ErrBinding
		}
	}
	if err := checkSourceRuntimePair(input.original.files[2].raw, input.original.files[3].raw); err != nil {
		return nil, err
	}
	generation := digest(a.Descriptor())
	unit, err := RenderSourceUnit(a.Template(), r, generation)
	if err != nil {
		return nil, err
	}
	c := &sourceReplacementCandidate{unit: unit, files: map[string][]byte{}}
	for _, name := range []string{ProgramName, TemplateName, DescriptorName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		raw, ok := a.origin.File(name)
		if !ok {
			return nil, ErrBinding
		}
		c.files[name] = raw
	}
	c.files["source-plan.json"], c.files["source-request.json"], c.files["rendered-source.service"] = bytes.Clone(input.original.files[1].raw), bytes.Clone(input.original.files[0].raw), unit
	c.binding = sourceReplacementBinding{Schema: "ardents-source-replacement-binding-v1", InitialGeneration: p.Generation, PredecessorBindingSHA256: digest(old.bindingBytes()), RetirementSHA256: retired.sha256, PredecessorAttempt: uint32(retired.chain.latest.number),
		Configuration: sourceBinding{Schema: "ardents-source-replacement-configuration-v1", Invocation: "source", Generation: generation, Predecessor: p.Generation, RequestSHA256: digest(c.files["source-request.json"]), ProgramTarget: ProgramTarget, GenerationTarget: GenerationTarget,
			ProgramSHA256: digest(a.Program()), TemplateSHA256: digest(a.Template()), InventorySHA256: digest(c.files["infrastructure-enrollment.json"]), TargetsSHA256: digest(c.files["targets.json"]), PlanSHA256: digest(c.files["source-plan.json"]), UnitSHA256: digest(unit), Account: p.Account, UID: p.UID, GID: p.GID, Request: r}}
	if old.replacement != nil {
		c.binding.InitialGeneration = old.replacement.InitialGeneration
	}
	c.raw, err = json.Marshal(c.binding)
	if err != nil {
		return nil, err
	}
	c.raw = append(c.raw, '\n')
	if _, err := decodeSourceReplacementBinding(c.raw); err != nil {
		return nil, err
	}
	c.files["binding.json"] = c.raw
	return c, nil
}
