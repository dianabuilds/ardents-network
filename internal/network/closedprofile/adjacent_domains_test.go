package closedprofile

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestClosedProfileAdjacentDomainsPreserveIntroductionAndRejectRendezvous(t *testing.T) {
	public, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(signer)
	now := time.Unix(1800000000, 0).UTC()
	network, generation, digest := [32]byte{1}, [32]byte{2}, [32]byte{3}
	for _, domain := range []byte{1, 2, 3, 4} {
		for _, subrole := range []byte{1, 2} {
			nodes := []Node{
				{NodeID: [32]byte{4}, RecordDigest: [32]byte{5}, RoleDomain: domain, Subrole: subrole, DutyGeneration: 1},
				{NodeID: [32]byte{6}, RecordDigest: [32]byte{7}, RoleDomain: 2, Subrole: 6, DutyGeneration: 2},
			}
			raw := testClosedProfile(t, signer, network, generation, digest, now, nodes)
			_, err := Verify(raw, Context{StateGeneration: generation, NetworkID: network, EpochDigest: digest, Epoch: 9, Authority: public, Now: now})
			if (err == nil) != (domain != 2) {
				t.Errorf("signed profile domain %d/%d: %v", domain, subrole, err)
			}
		}
	}
}
