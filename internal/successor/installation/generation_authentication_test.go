package installation

import (
	"context"
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Failure-only public-boundary checks provide no successful authority fixture.
// Genuine signed-byte/owned-history success and refusal are exercised by the
// production command integration tests in cmd/ardents-next.
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
}
