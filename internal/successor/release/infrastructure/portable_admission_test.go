package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestPortableInfrastructureAdmissionCannotMintZeroProofs(t *testing.T) {
	ctx := context.Background()
	initial := enrollment.InfrastructureBundle{}
	candidate := enrollment.InfrastructureCandidate{}
	in := release.Inputs{}
	checks := []error{}
	_, err := AuthenticateInitial(ctx, nil, initial, in)
	checks = append(checks, err)
	_, err = AuthenticateCandidate(ctx, nil, candidate, in)
	checks = append(checks, err)
	_, err = AuthenticateJoinInitial(ctx, nil, initial, in)
	checks = append(checks, err)
	_, err = AuthenticateJoinCandidate(ctx, nil, candidate, in)
	checks = append(checks, err)
	_, err = AuthenticateForwardingInitial(ctx, nil, initial, in)
	checks = append(checks, err)
	_, err = AuthenticateForwardingCandidate(ctx, nil, candidate, in)
	checks = append(checks, err)
	_, err = AuthenticateDescriptorInitial(ctx, nil, initial, in)
	checks = append(checks, err)
	_, err = AuthenticateDescriptorCandidate(ctx, nil, candidate, in)
	checks = append(checks, err)
	_, err = AuthenticateIntroductionInitial(ctx, nil, initial, in)
	checks = append(checks, err)
	_, err = AuthenticateIntroductionCandidate(ctx, nil, candidate, in)
	checks = append(checks, err)
	for _, err := range checks {
		if !errors.Is(err, ErrInput) {
			t.Fatalf("zero input returned %v", err)
		}
	}
}

type detachedTestInventory map[string][]byte

func (i detachedTestInventory) Check() error { return nil }
func (i detachedTestInventory) File(name string) ([]byte, bool) {
	body, ok := i[name]
	return bytes.Clone(body), ok
}

func TestProgramInputsProjectOnlyFixedPublicBytes(t *testing.T) {
	files := detachedTestInventory{
		ProgramName:      []byte("program"),
		"root.json":      []byte(`{"signed":{"consistent_snapshot":false}}`),
		"timestamp.json": []byte(`{"signed":{}}`),
		"snapshot.json":  []byte(`{"signed":{}}`),
		"targets.json":   []byte(`{"signed":{}}`),
	}
	input, err := ProgramInputs(files)
	if err != nil || input.TargetPath != ProgramTarget || string(input.Artifact) != "program" || !bytes.Equal(input.RootBytes, files["root.json"]) || !bytes.Equal(input.Files["https://release.invalid/metadata/targets.json"], files["targets.json"]) {
		t.Fatalf("incorrect public projection: %v", err)
	}
	input.Artifact[0] ^= 1
	if string(files[ProgramName]) != "program" {
		t.Fatal("projection modified original inventory")
	}
	delete(files, "timestamp.json")
	if _, err := ProgramInputs(files); err == nil {
		t.Fatal("missing metadata accepted")
	}
}

func TestSourceAuthorizationExposesDetachedInventoryWithoutGrantingProof(t *testing.T) {
	if (Authorization{}).HasInventory() {
		t.Fatal("zero value has original inventory")
	}
	if _, ok := (Authorization{}).File(ProgramName); ok {
		t.Fatal("zero value exposes a file")
	}
	original := detachedTestInventory{ProgramName: []byte("original")}
	a := Authorization{origin: original}
	copy, ok := a.File(ProgramName)
	if !a.HasInventory() || !ok {
		t.Fatal("original inventory unavailable")
	}
	copy[0] ^= 1
	if string(original[ProgramName]) != "original" {
		t.Fatal("detached file changed original inventory")
	}
	program, generation := a.Targets()
	if _, ok := program.AcceptedDecision(); ok {
		t.Fatal("inventory forged program authorization")
	}
	if _, ok := generation.AcceptedDecision(); ok {
		t.Fatal("inventory forged generation authorization")
	}
	if err := a.Check(context.Background(), nil); !errors.Is(err, ErrInput) {
		t.Fatalf("inventory bypassed verifier: %v", err)
	}
}

func TestRoleArtifactsKeepInventoriesSeparateAndDetached(t *testing.T) {
	common := []string{ProgramName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"}
	roles := []struct {
		name, template, descriptor string
		project                    func(Authorization) map[string][]byte
	}{
		{"join", JoinTemplateName, JoinDescriptorName, func(a Authorization) map[string][]byte { return (JoinAuthorization{pair: a}).Artifacts() }},
		{"forwarding", ForwardingTemplateName, ForwardingDescriptorName, func(a Authorization) map[string][]byte { return (ForwardingAuthorization{pair: a}).Artifacts() }},
		{"descriptor", DescriptorTemplateName, DescriptorArtifactName, func(a Authorization) map[string][]byte { return (DescriptorAuthorization{pair: a}).Artifacts() }},
		{"introduction", IntroductionTemplateName, IntroductionArtifactName, func(a Authorization) map[string][]byte { return (IntroductionAuthorization{pair: a}).Artifacts() }},
	}
	for _, role := range roles {
		t.Run(role.name, func(t *testing.T) {
			original := detachedTestInventory{}
			for _, name := range append(append([]string(nil), common...), role.template, role.descriptor) {
				original[name] = []byte(name)
			}
			a := Authorization{origin: original}
			files := role.project(a)
			if len(files) != 8 || string(files[role.template]) != role.template || string(files[role.descriptor]) != role.descriptor {
				t.Fatal("exact role inventory not projected")
			}
			files[role.template][0] ^= 1
			if string(original[role.template]) != role.template {
				t.Fatal("projection changed original inventory")
			}
			delete(original, role.descriptor)
			if role.project(a) != nil {
				t.Fatal("incomplete role inventory was projected")
			}
		})
	}
}
