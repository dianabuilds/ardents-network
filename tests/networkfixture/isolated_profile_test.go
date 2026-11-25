//go:build linux || darwin

package networkfixture

import (
	"testing"

	epochfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

func TestIndependentReceivingStateSurvivesPublisherProfileConflict(t *testing.T) {
	publisher := OpenSignedAdmission(t)
	original, err := publisher.State.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	receiver := OpenIndependentState(t, publisher.Spec, original.Profile().EpochDigest)
	before, err := receiver.CurrentRuntime()
	if err != nil || before.Profile().ProfileBinding != original.Profile().ProfileBinding {
		t.Fatal("independent receiver did not start with the same signed profile", err)
	}
	conflicting := publisher.Spec
	conflicting.IssuanceAuthority[0] ^= 1
	bundle, err := epochfixture.BuildClosed(conflicting)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Epoch.Digest != original.Profile().EpochDigest {
		t.Fatal("profile conflict changed the signed Epoch")
	}
	if _, err := publisher.State.AcceptClosedProfile(bundle.Profile); err == nil || err.Error() != "closed profile has a durable conflict" {
		t.Fatalf("publisher did not retain the signed conflict: %v", err)
	}
	if _, err := publisher.State.CurrentRuntime(); err == nil || err.Error() != "closed profile is unavailable" {
		t.Fatalf("publisher still yielded current authority: %v", err)
	}
	after, err := receiver.CurrentRuntime()
	if err != nil || after.Profile().ProfileBinding != before.Profile().ProfileBinding {
		t.Fatal("publisher conflict changed independent receiver authority", err)
	}
	for _, node := range publisher.Spec.Nodes {
		member, err := after.Member(node.NodeID, after.ObservedAt())
		if err != nil || member.DutyGeneration != node.Generation {
			t.Fatalf("independent member %x lost its original assignment: %v", node.NodeID, err)
		}
	}
}
