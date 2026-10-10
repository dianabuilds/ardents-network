package installation

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Genuine signed-byte/owned-history acceptance and refusal are exercised through
// the production command in cmd/ardents-next; these controls grant no authority.
func TestZeroInventoryAndAuthorizationGrantNothing(t *testing.T) {
	if _, err := AuthenticateInitial(context.Background(), nil, enrollment.Bundle{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatalf("zero initial inventory: %v", err)
	}
	if _, err := AuthenticateCandidate(context.Background(), nil, enrollment.Candidate{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatalf("zero candidate: %v", err)
	}
	p, g := (Authorization{}).Targets()
	if _, ok := p.AcceptedDecision(); ok {
		t.Fatal("zero program authorized")
	}
	if _, ok := g.AcceptedDecision(); ok {
		t.Fatal("zero generation authorized")
	}
	if _, ok := (Authorization{}).InitialFacts(); ok {
		t.Fatal("zero pair acquired initial pin provenance")
	}
}

func TestDetachedInventoryCannotMutateOrMintPrivateProofs(t *testing.T) {
	original := []byte("original bytes")
	observed := Authorization{descriptor: bytes.Clone(original), resources: map[string][]byte{"program": bytes.Clone(original)}}
	copy := observed.Resources()
	copy["program"][0] ^= 1
	delete(copy, "program")
	copy["foreign"] = original
	descriptor := observed.Descriptor()
	descriptor[0] ^= 1
	body, ok := observed.Resource("program")
	if !ok || !bytes.Equal(body, original) || !bytes.Equal(observed.Descriptor(), original) || len(observed.Resources()) != 1 {
		t.Fatal("detached bytes changed the retained inventory")
	}
	p, g := observed.Targets()
	if _, ok := p.AcceptedDecision(); ok {
		t.Fatal("observed program bytes became private proof")
	}
	if _, ok := g.AcceptedDecision(); ok {
		t.Fatal("observed descriptor bytes became private proof")
	}
	if _, ok := observed.InitialFacts(); ok {
		t.Fatal("observed bytes became initial pin provenance")
	}
}
