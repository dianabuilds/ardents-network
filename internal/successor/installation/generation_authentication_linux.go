// Native successor fresh authorization under independently retained installed facts.
package installation

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func authenticateSuccessor(ctx context.Context, verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs, previous generationBinding) (Authorization, error) {
	if ctx == nil || verifier == nil {
		return Authorization{}, ErrInput
	}
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil {
		return Authorization{}, err
	}
	// Compare the original history before either fresh evaluation can advance it.
	if err := successorContinuity(previous, floors, input.Local); err != nil {
		return Authorization{}, err
	}
	authorized, err := AuthenticateCandidate(ctx, verifier, candidate, input)
	if err != nil {
		return Authorization{}, err
	}
	_, generation := authorized.Targets()
	decision, valid := generation.AcceptedDecision()
	if !valid || decision.ReleaseVersion <= previous.Generation.ReleaseVersion {
		return Authorization{}, ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, errors.Join(ErrAuthorization, err)
	}
	return authorized, nil
}
