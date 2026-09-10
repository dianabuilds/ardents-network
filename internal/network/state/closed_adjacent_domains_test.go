package state

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
			nodes := []closedProfileNode{
				{nodeID: [32]byte{4}, recordDigest: [32]byte{5}, domain: domain, subrole: subrole, generation: 1},
				{nodeID: [32]byte{6}, recordDigest: [32]byte{7}, domain: 2, subrole: 6, generation: 2},
			}
			raw := testClosedProfile(t, signer, network, generation, digest, now, nodes)
			_, err := parseClosedProfile(raw, generation, network, digest, 9, public, now)
			if (err == nil) != (domain != 2) {
				t.Errorf("signed profile domain %d/%d: %v", domain, subrole, err)
			}
		}
	}
}
