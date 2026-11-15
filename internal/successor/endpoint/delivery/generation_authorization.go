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

func retainedTarget(target targetObservation) generationauthorization.RetainedTarget {
	return generationauthorization.RetainedTarget{
		Path: target.Path, Digest: target.Digest, Length: target.Length,
		ReleaseIdentity: target.ReleaseIdentity, ReleaseVersion: target.ReleaseVersion,
		Platform: target.Platform, Architecture: target.Architecture, Environment: target.Environment, Network: target.Network,
		ReferenceTime: target.ReferenceTime, TargetsVersion: target.TargetsVersion, TargetsDigest: target.TargetsDigest,
	}
}
