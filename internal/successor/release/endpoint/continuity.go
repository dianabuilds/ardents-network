package endpoint

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// RetainedGeneration is detached evidence from the independently retained
// installed binding. It is not a Release proof or an installation right.
type RetainedGeneration struct {
	Platform, Architecture, Environment, Network string
	ReferenceTime                                string
	TargetsVersion                               int64
	TargetsDigest                                string
	ReleaseVersion                               int64
}

// AuthenticateSuccessor checks the retained floor before authenticating the
// candidate, then requires a newer authorized generation. The caller must
// retain and reobserve the original binding around this operation.
func AuthenticateSuccessor(ctx context.Context, verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs, previous RetainedGeneration) (Authorization, error) {
	if ctx == nil || verifier == nil {
		return Authorization{}, ErrInput
	}
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil {
		return Authorization{}, err
	}
	if err := CheckContinuity(previous, floors, input.Local); err != nil {
		return Authorization{}, err
	}
	authorized, err := AuthenticateCandidate(ctx, verifier, candidate, input)
	if err != nil {
		return Authorization{}, err
	}
	_, generation := authorized.Targets()
	decision, valid := generation.AcceptedDecision()
	if !valid || decision.ReleaseVersion <= previous.ReleaseVersion {
		return Authorization{}, ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, errors.Join(ErrAuthorization, err)
	}
	return authorized, nil
}

// CheckContinuity compares retained facts with the verifier's current floors
// before a fresh successor is authenticated. The caller owns native provenance.
func CheckContinuity(previous RetainedGeneration, floors release.FloorSet, local release.LocalEnvironment) error {
	if !floors.Complete() || previous.TargetsVersion < 1 || len(previous.TargetsDigest) != 64 ||
		strings.Trim(previous.TargetsDigest, "0123456789abcdef") != "" ||
		floors.TargetsVersion < previous.TargetsVersion ||
		(floors.TargetsVersion == previous.TargetsVersion && hex.EncodeToString(floors.TargetsDigest) != previous.TargetsDigest) {
		return release.ErrTrustUnavailable
	}
	if local.Platform != previous.Platform || local.Architecture != previous.Architecture ||
		local.Environment != previous.Environment || local.Network != previous.Network {
		return ErrBinding
	}
	before, err := time.Parse(time.RFC3339Nano, previous.ReferenceTime)
	if err != nil || local.RefTime.IsZero() || local.RefTime.Before(before) {
		return ErrBinding
	}
	return nil
}
