package endpoint

import (
	"encoding/hex"
	"time"

	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/installation"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Authorization is the separately owned opaque pair consumed by installation.
// Endpoint cannot populate its private proofs, pin or immutable byte fields.
type Authorization = generationauthorization.Authorization

type generationDeclaration = generationauthorization.GenerationDeclaration

const programTarget = generationauthorization.ProgramTarget
const generationTarget = generationauthorization.GenerationTarget

var ErrInput = generationauthorization.ErrInput
var ErrBinding = generationauthorization.ErrBinding
var ErrAuthorization = generationauthorization.ErrAuthorization

// Local stored facts constrain continuity, never create fresh authorization.
// The caller must supply these facts from its still-leased native inspection.
func successorContinuity(previous generationBinding, floors release.FloorSet, local release.LocalEnvironment) error {
	g := previous.Generation
	if !floors.Complete() || g.TargetsVersion < 1 || !canonicalDigest(g.TargetsDigest) ||
		floors.TargetsVersion < g.TargetsVersion ||
		(floors.TargetsVersion == g.TargetsVersion && hex.EncodeToString(floors.TargetsDigest) != g.TargetsDigest) {
		return release.ErrTrustUnavailable
	}
	if local.Platform != g.Platform || local.Architecture != g.Architecture ||
		local.Environment != g.Environment || local.Network != g.Network {
		return ErrBinding
	}
	before, err := time.Parse(time.RFC3339Nano, g.ReferenceTime)
	if err != nil || local.RefTime.IsZero() || local.RefTime.Before(before) {
		return ErrBinding
	}
	return nil
}
