package inspection

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestPrepareInitialEvidenceRejectsCrossDomainSubstitution(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	networkID := [32]byte{7}
	epochBytes, digest := signedEmptyEpoch(networkID, time.Unix(2_000_400_000, 0).UTC(), private)
	release := ReleaseEvidence{ArtifactDigest: [32]byte{1}, TargetPath: "ardents/linux-amd64/endpoint", ReleaseIdentity: "release-one", BuildIdentity: "build-one", ProtocolPhase: "announced", BuildState: "current"}
	network := NetworkEvidence{NetworkID: networkID, EpochDigest: digest, Profile: "h3-role-probe-v1", Threshold: 1, Authorities: []ed25519.PublicKey{public}, Epoch: epochBytes}
	compatibility := CompatibilityEvidence{ReleaseDigest: release.ArtifactDigest, ReleaseBuildIdentity: release.BuildIdentity, ProtocolPhase: release.ProtocolPhase, NetworkDigest: digest, NetworkEpoch: 1, NetworkProfile: network.Profile}
	prepared, err := PrepareInitialEvidence(release, network, compatibility)
	if err != nil {
		t.Fatal(err)
	}
	if decoded, err := decodeReleaseEvidence(prepared[0]); err != nil || decoded != release {
		t.Fatalf("release payload lost identity: %v", err)
	}
	if decoded, err := decodeNetworkEvidence(prepared[1]); err != nil || decoded.EpochDigest != digest {
		t.Fatalf("network payload lost identity: %v", err)
	}
	if decoded, err := decodeCompatibilityEvidence(prepared[2]); err != nil || decoded != compatibility {
		t.Fatalf("compatibility payload lost identity: %v", err)
	}
	for _, test := range []struct {
		name   string
		change func(*ReleaseEvidence, *NetworkEvidence, *CompatibilityEvidence)
	}{
		{"artifact", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) { c.ReleaseDigest[0]++ }},
		{"build", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) {
			c.ReleaseBuildIdentity = "other"
		}},
		{"protocol", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) { c.ProtocolPhase = "required" }},
		{"epoch-digest", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) { n.EpochDigest[0]++ }},
		{"network-id", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) { n.NetworkID[0]++ }},
		{"epoch-number", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) { c.NetworkEpoch++ }},
		{"profile", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) { c.NetworkProfile = "other" }},
		{"truncated-epoch", func(r *ReleaseEvidence, n *NetworkEvidence, c *CompatibilityEvidence) { n.Epoch = n.Epoch[:10] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, n, c := release, network, compatibility
			test.change(&r, &n, &c)
			result, err := PrepareInitialEvidence(r, n, c)
			if err == nil {
				t.Fatal("substituted evidence accepted")
			}
			for _, raw := range result {
				if len(raw) != 0 {
					t.Fatal("refusal exposed partial prepared payloads")
				}
			}
		})
	}
}
