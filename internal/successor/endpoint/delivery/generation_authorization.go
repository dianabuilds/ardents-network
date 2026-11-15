package delivery

import (
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/release/endpoint"
)

type generationDeclaration = generationauthorization.GenerationDeclaration

const programTarget = generationauthorization.ProgramTarget
const generationTarget = generationauthorization.GenerationTarget

// The caller supplies these facts from its still-leased native inspection.
func retainedGeneration(previous generationBinding) generationauthorization.RetainedGeneration {
	g := previous.Generation
	return generationauthorization.RetainedGeneration{
		Platform: g.Platform, Architecture: g.Architecture, Environment: g.Environment, Network: g.Network,
		ReferenceTime: g.ReferenceTime, TargetsVersion: g.TargetsVersion, TargetsDigest: g.TargetsDigest, ReleaseVersion: g.ReleaseVersion,
	}
}
