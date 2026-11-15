package infrastructure

import (
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
