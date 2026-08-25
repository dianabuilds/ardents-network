package inspection

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/release"
)

func TestVerifyCompatibilityBindsReleaseAndNetworkIdentities(t *testing.T) {
	releaseDecision := release.Decision{Digest: make([]byte, 32), BuildIdentity: "build-7", ProtocolPhase: "h4-a"}
	releaseDecision.Digest[0] = 1
	var releaseDigest [32]byte
	copy(releaseDigest[:], releaseDecision.Digest)
	network := state.Snapshot{Epoch: 7, Digest: [32]byte{2}, Profile: "h3-role-probe-v1"}
	body, err := EncodeCompatibilityEvidence(CompatibilityEvidence{ReleaseDigest: releaseDigest, ReleaseBuildIdentity: "build-7", ProtocolPhase: "h4-a",
		NetworkDigest: network.Digest, NetworkEpoch: network.Epoch, NetworkProfile: network.Profile})
	if err != nil {
		t.Fatal(err)
	}
	if outcome := verifyCompatibility(body, releaseDecision, true, network, true); outcome != alphacontrol.OutcomeAccepted {
		t.Fatalf("compatibility outcome = %q", outcome)
	}
	network.Epoch++
	if outcome := verifyCompatibility(body, releaseDecision, true, network, true); outcome != alphacontrol.OutcomeInvalid {
		t.Fatalf("changed network identity outcome = %q", outcome)
	}
}
