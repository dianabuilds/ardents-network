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
