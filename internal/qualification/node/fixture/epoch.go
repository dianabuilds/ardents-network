package fixture

import (
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/epochfixture"
)

type nodeEpoch struct {
	number    uint64
	seed      [32]byte
	raw       []byte
	digest    [32]byte
	materials [2][]byte
}

func (fixture nodeFixture) makeEpoch(number uint64, previous, seed [32]byte) (nodeEpoch, error) {
	inputs := make([][]byte, len(fixture.records))
	accepted := make([]epochfixture.Record, len(fixture.records))
	for index := range fixture.records {
		inputs[index] = fixture.records[index].raw
		accepted[index] = epochfixture.Record{Raw: fixture.records[index].raw, NodeID: fixture.records[index].nodeID,
			Family: fixture.records[index].family, Capacity: fixture.records[index].capacity}
	}
	built, err := epochfixture.BuildEpoch(epochfixture.EpochSpec{NetworkID: fixture.network, Number: number,
		Previous: previous, ValidFrom: fixture.now.Add(-30 * time.Second), ValidUntil: fixture.now.Add(48 * time.Hour),
		Inputs: inputs, Accepted: accepted, AssignmentSeed: seed, Domains: []string{"alpha", "beta"},
		Authorities: []ed25519.PrivateKey{fixture.authorityPrivate}})
	if err != nil {
		return nodeEpoch{}, err
	}
	epoch := nodeEpoch{number: number, seed: seed, raw: built.Raw, digest: built.Digest}
	copy(epoch.materials[:], built.Materials)
	return epoch, nil
}
