package endpoint

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

// Signed-byte and owned-history behavior is exercised by the independent
// authorization integration tests; these zero-value controls grant no authority.
func TestZeroInventoryAndAuthorizationGrantNothing(t *testing.T) {
	if _, err := AuthenticateInitial(context.Background(), nil, enrollment.Bundle{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatalf("zero initial inventory: %v", err)
	}
	if _, err := AuthenticateCandidate(context.Background(), nil, enrollment.Candidate{}, release.Inputs{}); !errors.Is(err, ErrInput) {
		t.Fatalf("zero candidate: %v", err)
	}
	p, g := (Authorization{}).Targets()
	if _, ok := p.AcceptedDecision(); ok {
		t.Fatal("zero program authorized")
	}
	if _, ok := g.AcceptedDecision(); ok {
		t.Fatal("zero generation authorized")
	}
	if _, ok := (Authorization{}).InitialFacts(); ok {
		t.Fatal("zero pair acquired initial pin provenance")
	}
}

func TestDetachedInventoryCannotMutateOrMintPrivateProofs(t *testing.T) {
	original := []byte("original bytes")
	observed := Authorization{descriptor: bytes.Clone(original), resources: map[string][]byte{"program": bytes.Clone(original)}}
	copy := observed.Resources()
	copy["program"][0] ^= 1
	delete(copy, "program")
	copy["foreign"] = original
	descriptor := observed.Descriptor()
	descriptor[0] ^= 1
	body, ok := observed.Resources()["program"]
	if !ok || !bytes.Equal(body, original) || !bytes.Equal(observed.Descriptor(), original) || len(observed.Resources()) != 1 {
		t.Fatal("detached bytes changed the retained inventory")
	}
	p, g := observed.Targets()
	if _, ok := p.AcceptedDecision(); ok {
		t.Fatal("observed program bytes became private proof")
	}
	if _, ok := g.AcceptedDecision(); ok {
		t.Fatal("observed descriptor bytes became private proof")
	}
	if _, ok := observed.InitialFacts(); ok {
		t.Fatal("observed bytes became initial pin provenance")
	}
}

func TestSharedPolicyRequiresEveryCommonFact(t *testing.T) {
	at := time.Unix(1234, 0).UTC()
	d := GenerationDeclaration{Platform: "linux-amd64", ReleaseIdentity: "release", ReleaseVersion: 1}
	original := release.Decision{Path: ProgramTarget, Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, ProtocolTransitionDeadline: at}
	same := original
	same.Path = GenerationTarget
	same.ProtocolTransitionDeadline = at.In(time.FixedZone("offset", 3600))
	same.Outcome = release.OutcomeNoUpdate
	if !coherentReleasePair(original, same, d) {
		t.Fatal("equal instants and distinct overall outcomes must agree")
	}
	for field, change := range map[string]func(*release.Decision){
		"SourceRevision":             func(d *release.Decision) { d.SourceRevision = "different" },
		"BuildInputCommitment":       func(d *release.Decision) { d.BuildInputCommitment = "different" },
		"AttestationPolicy":          func(d *release.Decision) { d.AttestationPolicy = "different" },
		"Qualification":              func(d *release.Decision) { d.Qualification = "different" },
		"BuildState":                 func(d *release.Decision) { d.BuildState = "different" },
		"ProtocolPhase":              func(d *release.Decision) { d.ProtocolPhase = "different" },
		"BuildSafety":                func(d *release.Decision) { d.BuildSafety = "different" },
		"Protocol":                   func(d *release.Decision) { d.Protocol = "different" },
		"BuildSafetyNoNewWorkAfter":  func(d *release.Decision) { d.BuildSafetyNoNewWorkAfter = at.Add(time.Hour) },
		"BuildSafetyTerminateAfter":  func(d *release.Decision) { d.BuildSafetyTerminateAfter = at.Add(time.Hour) },
		"ProtocolTransitionDeadline": func(d *release.Decision) { d.ProtocolTransitionDeadline = at.Add(time.Hour) },
	} {
		changed := same
		change(&changed)
		if coherentReleasePair(original, changed, d) {
			t.Errorf("accepted different %s", field)
		}
	}
}
