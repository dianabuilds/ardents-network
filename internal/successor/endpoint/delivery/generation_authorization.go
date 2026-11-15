package delivery

import (
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

type generationDeclaration = generationauthorization.GenerationDeclaration

const programTarget = generationauthorization.ProgramTarget
const generationTarget = generationauthorization.GenerationTarget

// Local stored facts constrain continuity, never create fresh authorization.
// The caller must supply these facts from its still-leased native inspection.
func successorContinuity(previous generationBinding, floors release.FloorSet, local release.LocalEnvironment) error {
	return generationauthorization.CheckContinuity(retainedGeneration(previous), floors, local)
}

func retainedGeneration(previous generationBinding) generationauthorization.RetainedGeneration {
	g := previous.Generation
	return generationauthorization.RetainedGeneration{
		Platform: g.Platform, Architecture: g.Architecture, Environment: g.Environment, Network: g.Network,
		ReferenceTime: g.ReferenceTime, TargetsVersion: g.TargetsVersion, TargetsDigest: g.TargetsDigest, ReleaseVersion: g.ReleaseVersion,
	}
}
