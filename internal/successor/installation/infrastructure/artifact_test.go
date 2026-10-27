//go:build linux

package infrastructure

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func TestCanonicalDescriptorIndependentOracle(t *testing.T) {
	p, u := []byte("program"), []byte("closed public template")
	ps, us := sha256.Sum256(p), sha256.Sum256(u)
	want := []byte(fmt.Sprintf("{\"schema\":\"ardents-infrastructure-artifact-v1\",\"platform\":\"linux-amd64\",\"release_identity\":\"release-one\",\"release_version\":7,\"invocation\":\"source\",\"files\":{\"ardents-node-linux-amd64\":\"%s\",\"ardents-source.service\":\"%s\"}}\n", hex.EncodeToString(ps[:]), hex.EncodeToString(us[:])))
	got, err := PrepareDescriptor("release-one", 7, p, u)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("canonical bytes %s %v", got, err)
	}
	if _, err := DecodeDescriptor(want); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{string(want) + "\n", strings.TrimSpace(string(want)), strings.Replace(string(want), "\"source\"", "\"forwarding\"", 1), strings.Replace(string(want), "\"release_version\":7", "\"release_version\":0", 1), strings.Replace(string(want), "\"schema\":", "\"extra\":0,\"schema\":", 1), strings.Replace(string(want), "\"platform\":", "\"platform\":\"linux-amd64\",\"platform\":", 1), strings.Replace(string(want), "ardents-source.service", "foreign.service", 1)} {
		if _, err := DecodeDescriptor([]byte(raw)); err == nil {
			t.Fatalf("accepted noncanonical %s", raw)
		}
	}
	for _, raw := range [][]byte{nil, bytes.Repeat([]byte(" "), (16<<10)+1)} {
		if _, err := DecodeDescriptor(raw); err == nil {
			t.Fatal("accepted absent/oversized")
		}
	}
}

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
	if !coherentTarget(p, g, d, GenerationTarget) {
		t.Fatal("healthy counterpart")
	}
	for _, change := range []func(*release.Decision){func(g *release.Decision) { g.Network = "other" }, func(g *release.Decision) { g.AttestationPolicy = "other" }, func(g *release.Decision) { g.BuildState = "revoked" }, func(g *release.Decision) { g.SourceRevision = "other" }, func(g *release.Decision) { g.ReleaseVersion++ }, func(g *release.Decision) { g.Floors.TargetsVersion++ }, func(g *release.Decision) { g.Path = "ardents/linux-amd64/protected-endpoint" }} {
		other := g
		change(&other)
		if coherentTarget(p, other, d, GenerationTarget) {
			t.Fatal("incoherent pair accepted")
		}
	}
}
