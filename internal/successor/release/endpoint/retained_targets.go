package endpoint

import (
	"encoding/hex"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// RetainedTarget is a detached target observation from original installed
// custody. Its fields do not create a Release authorization.
type RetainedTarget struct {
	Path, Digest                                 string
	Length                                       int64
	ReleaseIdentity                              string
	ReleaseVersion                               int64
	Platform, Architecture, Environment, Network string
	ReferenceTime                                string
	TargetsVersion                               int64
	TargetsDigest                                string
}

// MatchRetainedTargets requires fresh private proofs for the exact two retained
// targets. The caller separately owns original file and recovery admission.
func (a Authorization) MatchRetainedTargets(program, generation RetainedTarget, reference time.Time) error {
	p, programOK := a.program.AcceptedDecision()
	g, generationOK := a.generation.AcceptedDecision()
	if !programOK || !generationOK || !p.ReferenceTime.Equal(reference) || !g.ReferenceTime.Equal(reference) {
		return ErrAuthorization
	}
	if !matchesRetainedTarget(program, p, reference) || !matchesRetainedTarget(generation, g, reference) {
		return ErrBinding
	}
	return nil
}

func matchesRetainedTarget(old RetainedTarget, fresh release.Decision, reference time.Time) bool {
	before, err := time.Parse(time.RFC3339Nano, old.ReferenceTime)
	return err == nil && !reference.Before(before) && old.Path == fresh.Path && old.Digest == hex.EncodeToString(fresh.Digest) && old.Length == fresh.Length &&
		old.ReleaseIdentity == fresh.ReleaseIdentity && old.ReleaseVersion == fresh.ReleaseVersion && old.Platform == fresh.Platform && old.Architecture == fresh.Architecture &&
		old.Environment == fresh.Environment && old.Network == fresh.Network && fresh.Floors.TargetsVersion >= old.TargetsVersion &&
		(fresh.Floors.TargetsVersion != old.TargetsVersion || hex.EncodeToString(fresh.Floors.TargetsDigest) == old.TargetsDigest)
}
