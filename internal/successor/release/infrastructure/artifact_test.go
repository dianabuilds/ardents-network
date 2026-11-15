package infrastructure

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestZeroAndDetachedFactsCannotCreateAuthority(t *testing.T) {
	if _, err := AuthenticateInitial(context.Background(), nil, enrollment.InfrastructureBundle{}, release.Inputs{}); err == nil {
		t.Fatal("zero initial accepted")
	}
	if _, err := AuthenticateCandidate(context.Background(), nil, enrollment.InfrastructureCandidate{}, release.Inputs{}); err == nil {
		t.Fatal("zero candidate accepted")
	}
	a := Authorization{descriptor: []byte("public"), executable: []byte("program"), template: []byte("template")}
	a.Descriptor()[0] ^= 1
	a.Program()[0] ^= 1
	a.Template()[0] ^= 1
	if string(a.Descriptor()) != "public" || string(a.Program()) != "program" || string(a.Template()) != "template" {
		t.Fatal("copies aliased")
	}
	p, g := a.Targets()
	for _, proof := range []release.Authorization{p, g} {
		if _, ok := proof.AcceptedDecision(); ok {
			t.Fatal("facts forged proof")
		}
	}
	if _, ok := a.InitialFacts(); ok {
		t.Fatal("facts forged initial provenance")
	}
}

func TestCoherenceIncludesCommonSafetyAndReference(t *testing.T) {
	d := Declaration{Platform: "linux-amd64", ReleaseIdentity: "one", ReleaseVersion: 1}
	p := release.Decision{Path: ProgramTarget, Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: 1, Floors: release.FloorSet{TargetsVersion: 1, TargetsDigest: make([]byte, 32)}}
	g := p
	g.Path = GenerationTarget
	if !CoherentTargets(p, g, d, GenerationTarget) {
		t.Fatal("healthy counterpart")
	}
	for _, change := range []func(*release.Decision){func(g *release.Decision) { g.Network = "other" }, func(g *release.Decision) { g.AttestationPolicy = "other" }, func(g *release.Decision) { g.BuildState = "revoked" }, func(g *release.Decision) { g.SourceRevision = "other" }, func(g *release.Decision) { g.ReleaseVersion++ }, func(g *release.Decision) { g.Floors.TargetsVersion++ }, func(g *release.Decision) { g.Path = "ardents/linux-amd64/protected-endpoint" }} {
		other := g
		change(&other)
		if CoherentTargets(p, other, d, GenerationTarget) {
			t.Fatal("incoherent pair accepted")
		}
	}
}
