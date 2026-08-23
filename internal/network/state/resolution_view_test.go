package state_test

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestCurrentResolutionViewOwnsFreshEpochAndCandidateFacts(t *testing.T) {
	value := newFixture(t)
	opened, err := state.Open(state.Config{Root: t.TempDir(), NetworkID: value.networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{value.authorityID: value.authorityPublic},
		Threshold:   1, Now: time.Unix(value.now, 0)})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if _, err := opened.Accept(context.Background(), value.epoch, value.inputs, value.materializations); err != nil {
		t.Fatal(err)
	}
	view, err := opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Unix(value.now, 0).UTC()
	epoch, available := view.Epoch(at, at.Add(time.Second))
	if !available || epoch.NetworkID != value.networkID || epoch.Number != 1 || len(epoch.Authorities) == 0 {
		t.Fatalf("epoch=%+v available=%v", epoch, available)
	}
	epoch.Authorities[0].PublicKey = [32]byte{}
	again, available := view.Epoch(at, at.Add(time.Second))
	if !available || again.Authorities[0].PublicKey == [32]byte{} {
		t.Fatal("caller mutation changed the owned Resolution trust fact")
	}
	if _, available := view.Candidate(value.accepted[0].nodeID, at, at.Add(time.Second)); !available {
		t.Fatal("valid authenticated candidate was unavailable to Resolution")
	}
	if _, available := view.Epoch(at, at.Add(16*time.Second)); available {
		t.Fatal("Resolution view accepted a window above the accepted limit")
	}
}
