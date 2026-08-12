package fixture

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch/assignment"
	"github.com/dianabuilds/ardents-network/internal/qualification/epochfixture"
)

type nodeFixture struct {
	now              time.Time
	network          [32]byte
	authorityPublic  ed25519.PublicKey
	authorityPrivate ed25519.PrivateKey
	records          []nodeRecord
	epochs           [2]nodeEpoch
	sourceCA         nodeCredential
	sourceClients    [3]nodeCredential
	sourceServers    [2]nodeCredential
	roleCA           nodeCredential
	roleServers      [2]nodeCredential
	harness          nodeCredential
}

type nodeRecord struct {
	raw      []byte
	nodeID   [32]byte
	private  ed25519.PrivateKey
	family   string
	endpoint string
	capacity uint16
}

func newNodeFixture(now time.Time) (nodeFixture, error) {
	if now.Year() < 2026 {
		return nodeFixture{}, errors.New("node fixture time is outside the supported campaign era")
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nodeFixture{}, err
	}
	fixture := nodeFixture{now: now, network: sha256.Sum256([]byte("ardents-h3-node-controlled-network-v1")),
		authorityPublic: public, authorityPrivate: private}
	type identity struct {
		nodeID  [32]byte
		private ed25519.PrivateKey
	}
	identities := make([]identity, 2)
	for index := range 2 {
		nodePublic, nodePrivate, keyErr := ed25519.GenerateKey(rand.Reader)
		if keyErr != nil {
			return nodeFixture{}, keyErr
		}
		identities[index] = identity{nodeID: sha256.Sum256(append([]byte("ardents-h3-node-v1\x00"), nodePublic...)), private: nodePrivate}
	}
	sort.Slice(identities, func(i, j int) bool {
		return bytes.Compare(identities[i].nodeID[:], identities[j].nodeID[:]) < 0
	})
	for index, value := range identities {
		record, recordErr := newNodeRecord(fixture.network, value.nodeID, value.private, index, now)
		if recordErr != nil {
			return nodeFixture{}, recordErr
		}
		fixture.records = append(fixture.records, record)
	}
	firstSeed := sha256.Sum256([]byte("ardents-h3-node-assignment-1"))
	fixture.epochs[0], err = fixture.makeEpoch(1, [32]byte{}, firstSeed)
	if err != nil {
		return nodeFixture{}, err
	}
	for marker := byte(1); marker != 0; marker++ {
		seed := sha256.Sum256([]byte{marker, 0x52})
		candidate, candidateErr := fixture.makeEpoch(2, fixture.epochs[0].digest, seed)
		if candidateErr != nil {
			return nodeFixture{}, candidateErr
		}
		if fixture.assignmentsChanged(firstSeed, seed) {
			fixture.epochs[1] = candidate
			break
		}
	}
	if fixture.epochs[1].number == 0 {
		return nodeFixture{}, errors.New("node fixture could not derive a reassignment seed")
	}
	return fixture.withCredentials()
}

func newNodeRecord(network, nodeID [32]byte, private ed25519.PrivateKey, index int, now time.Time) (nodeRecord, error) {
	family := []string{"synthetic-a", "synthetic-b"}[index]
	endpoint := []string{"0.0.0.0:4401", "0.0.0.0:4402"}[index]
	record, err := epochfixture.BuildRecord(epochfixture.RecordSpec{NetworkID: network, NodeID: nodeID, Generation: 1,
		ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(48 * time.Hour), Family: family, Endpoint: endpoint,
		Capability: 1, Capacity: 4, PrivateKey: private})
	if err != nil {
		return nodeRecord{}, err
	}
	return nodeRecord{raw: record.Raw, nodeID: nodeID, private: private, family: family, endpoint: endpoint, capacity: 4}, nil
}

func (fixture nodeFixture) withCredentials() (nodeFixture, error) {
	var err error
	fixture.sourceCA, err = newNodeAuthority("node-source-client-root", fixture.now)
	if err != nil {
		return fixture, err
	}
	fixture.roleCA, err = newNodeAuthority("node-role-client-root", fixture.now)
	if err != nil {
		return fixture, err
	}
	for index := range fixture.sourceClients {
		fixture.sourceClients[index], err = newNodeLeaf(fixture.sourceCA, "node-source-client", x509.ExtKeyUsageClientAuth, fixture.now)
		if err != nil {
			return fixture, err
		}
	}
	for index := range 2 {
		authority, authorityErr := newNodeAuthority("node-source-root", fixture.now)
		if authorityErr != nil {
			return fixture, authorityErr
		}
		fixture.sourceServers[index], err = newNodeLeaf(authority, "source.node", x509.ExtKeyUsageServerAuth, fixture.now)
		if err != nil {
			return fixture, err
		}
		fixture.roleServers[index], err = newNodeLeaf(fixture.roleCA, "node.node", x509.ExtKeyUsageServerAuth, fixture.now)
		if err != nil {
			return fixture, err
		}
	}
	fixture.harness, err = newNodeLeaf(fixture.roleCA, "harness.node", x509.ExtKeyUsageClientAuth, fixture.now)
	return fixture, err
}

func (fixture nodeFixture) assignmentsChanged(first, second [32]byte) bool {
	for _, record := range fixture.records {
		if fixture.selectedDomain(1, first, record.family) != fixture.selectedDomain(2, second, record.family) {
			return true
		}
	}
	return false
}

func (fixture nodeFixture) selectedDomain(epoch uint64, seed [32]byte, family string) string {
	selected, err := assignment.Select(fixture.network, epoch, seed, family, []string{"alpha", "beta"})
	if err != nil {
		return ""
	}
	return selected
}
