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
	if view.DutyCandidateCount() != 2 || view.DutyCandidateNodeID(0) != value.accepted[0].nodeID ||
		view.DutyCandidatePublicKey(0) == [32]byte{} || view.DutyCandidateEndpoint(0) == "" ||
		view.DutyCandidateAssignment(0) == "" || view.DutyCandidateValidFrom(0).IsZero() ||
		view.DutyCandidateValidUntil(0).IsZero() {
		t.Fatalf("Node duty view lacks authenticated candidate facts")
	}
	if view.DutyCandidateKeyID(0) == [32]byte{} || view.DutyCandidateFamilyID(0) == [32]byte{} ||
		view.DutyCandidateRecordDigest(0) == [32]byte{} || view.DutyCandidateDomainProofDigest(0) == [32]byte{} ||
		view.DutyCandidateCapacity(0) == 0 || view.DutyCandidateAssignmentNotAfter(0).IsZero() {
		t.Fatal("Node duty view lacks the bounded Entry-verification candidate facts")
	}
	if view.DutyAuthorityCount() != 1 || view.DutyAuthorityID(0) != value.authorityID ||
		view.DutyAuthorityPublicKey(0) != [32]byte(value.authorityPublic) {
		t.Fatal("Node duty view lacks the current State authority verification fact")
	}
	if view.DutyAuthorityID(1) != [32]byte{} || view.DutyAuthorityPublicKey(1) != [32]byte{} {
		t.Fatal("Node duty view exposed an out-of-range State authority")
	}
	if view.DutyCandidateNodeID(2) != [32]byte{} || view.DutyCandidateEndpoint(2) != "" ||
		!view.DutyCandidateValidUntil(2).IsZero() {
		t.Fatal("Node duty view exposed an out-of-range candidate")
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := opened.CurrentNodeDuty(); err == nil {
		t.Fatal("closed Network State exposed a Node duty view")
	}
}
