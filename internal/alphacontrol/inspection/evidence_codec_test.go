package inspection

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestEvidenceCodecsAreCanonicalAndBounded(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	release := ReleaseEvidence{ArtifactDigest: [32]byte{1}, TargetPath: "ardents/linux-amd64/endpoint", ReleaseIdentity: "alpha-1", BuildIdentity: "build-1", ProtocolPhase: "h4-a", BuildState: "accepted"}
	releaseRaw, err := encodeReleaseEvidence(release)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := decodeReleaseEvidence(releaseRaw); err != nil || decoded != release {
		t.Fatalf("release decode = %+v, %v", decoded, err)
	}
	network := NetworkEvidence{NetworkID: [32]byte{2}, EpochDigest: [32]byte{3}, Profile: "h3-role-probe-v1", Threshold: 1, Authorities: []ed25519.PublicKey{public}, Epoch: []byte("epoch"), Inputs: [][]byte{[]byte("input")}, Materials: [][]byte{[]byte("material")}}
	networkRaw, err := encodeNetworkEvidence(network)
	if err != nil {
		t.Fatal(err)
	}
	decodedNetwork, err := decodeNetworkEvidence(networkRaw)
	if err != nil || decodedNetwork.NetworkID != network.NetworkID || decodedNetwork.EpochDigest != network.EpochDigest || decodedNetwork.Profile != network.Profile || decodedNetwork.Threshold != network.Threshold ||
		string(decodedNetwork.Epoch) != "epoch" || len(decodedNetwork.Authorities) != 1 || string(decodedNetwork.Authorities[0]) != string(public) {
		t.Fatalf("network decode = %+v, %v", decodedNetwork, err)
	}
	compatibility := CompatibilityEvidence{ReleaseDigest: [32]byte{3}, ReleaseBuildIdentity: "build-1", ProtocolPhase: "h4-a", NetworkDigest: [32]byte{4}, NetworkEpoch: 1, NetworkProfile: "h3-role-probe-v1"}
	compatibilityRaw, err := encodeCompatibilityEvidence(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := decodeCompatibilityEvidence(compatibilityRaw); err != nil || decoded != compatibility {
		t.Fatalf("compatibility decode = %+v, %v", decoded, err)
	}
	networkRaw = append(networkRaw, 0)
	if _, err := decodeNetworkEvidence(networkRaw); err == nil {
		t.Fatal("network evidence with trailing bytes was accepted")
	}
}
