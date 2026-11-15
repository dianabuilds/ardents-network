// Fresh successor authorization under independently retained installed facts.
package delivery

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

func authenticateSuccessor(ctx context.Context, verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs, previous generationBinding) (generationauthorization.Authorization, error) {
	if ctx == nil || verifier == nil {
		return generationauthorization.Authorization{}, ErrInput
	}
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil {
		return generationauthorization.Authorization{}, err
	}
	// Compare the original history before either fresh evaluation can advance it.
	if err := successorContinuity(previous, floors, input.Local); err != nil {
		return generationauthorization.Authorization{}, err
	}
	authorized, err := generationauthorization.AuthenticateCandidate(ctx, verifier, candidate, input)
	if err != nil {
		return generationauthorization.Authorization{}, err
	}
	_, generation := authorized.Targets()
	decision, valid := generation.AcceptedDecision()
	if !valid || decision.ReleaseVersion <= previous.Generation.ReleaseVersion {
		return generationauthorization.Authorization{}, ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return generationauthorization.Authorization{}, errors.Join(ErrAuthorization, err)
	}
	return authorized, nil
}
