package state_test

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestCurrentRouteViewOwnsEveryAuthenticatedCandidate(t *testing.T) {
	value := newFixture(t)
	opened, err := state.Open(state.Config{
		Root: t.TempDir(), NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := opened.Accept(context.Background(), value.epoch, value.inputs, value.materializations); err != nil {
		t.Fatal(err)
	}
	view, err := opened.Current()
	if err != nil {
		t.Fatal(err)
	}
	if view.Epoch != 1 || view.Digest != value.epochDigest || view.CandidateCount != 2 {
		t.Fatalf("unexpected Route view: epoch=%d candidates=%d", view.Epoch, view.CandidateCount)
	}
	for index, candidate := range view.Candidates[:view.CandidateCount] {
		if candidate.NodeID != value.accepted[index].nodeID || candidate.Family != value.accepted[index].family ||
			candidate.Endpoint == "" || candidate.Domain == "" || candidate.PublicKey == [32]byte{} {
			t.Fatalf("candidate %d is not the authenticated accepted record: %+v", index, candidate)
		}
	}
	view.Candidates[0].Family = "mutated"
	again, err := opened.Current()
	if err != nil {
		t.Fatal(err)
	}
	if again.Candidates[0].Family == "mutated" {
		t.Fatal("caller mutation changed the owned authenticated Route view")
	}
}

func TestRouteViewUnavailableAfterStateClose(t *testing.T) {
	value := newFixture(t)
	opened, err := state.Open(state.Config{
		Root: t.TempDir(), NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := opened.Current(); err == nil {
		t.Fatal("closed Network State exposed a Route view")
	}
}

func TestRouteProfileCannotConsumeRoleProbeEpoch(t *testing.T) {
	value := newFixture(t)
	opened, err := state.Open(state.Config{
		Root: t.TempDir(), NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0), AcceptedProfile: "h3-route-tracer-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := opened.Accept(context.Background(), value.epoch, value.inputs, value.materializations); err == nil {
		t.Fatal("Route-configured Network State accepted the Stage 1 role-probe profile")
	}
}
