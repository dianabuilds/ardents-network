package state_test

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestCurrentNodeDutyExposesOnlyCurrentAuthenticatedDutyFacts(t *testing.T) {
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
	view, err := opened.CurrentNodeDuty()
	if err != nil {
		t.Fatal(err)
	}
	if view.DutyNetworkID() != value.networkID || view.DutyEpoch() != 1 || view.DutyDigest() != value.epochDigest ||
		!view.DutyRecordPresent() || view.DutyAssignment() == "" || view.DutyProbeCapacity() == 0 {
		t.Fatalf("Node duty view lacks authenticated duty facts")
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := opened.CurrentNodeDuty(); err == nil {
		t.Fatal("closed Network State exposed a Node duty view")
	}
}
