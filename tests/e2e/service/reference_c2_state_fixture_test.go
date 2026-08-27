package service_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

// referenceC2StateFixture is a black-box, signed native-route State decision
// for the separate-process C2 tracer. It builds only test input; State remains
// the verifier and owner of each process's authenticated duty view.
type referenceC2StateFixture struct {
	network, digest [32]byte
	epoch           uint64
	now, deadline   time.Time
	authority       ed25519.PrivateKey
	inputs          [][]byte
	raw             []byte
	materials       map[string][]byte
	roles           map[string]referenceC2StateRecord
}

type referenceC2StateRecord struct {
	role                 string
	nodeID               [32]byte
	material             referenceC2CertificateMaterial
	endpoint             string
	family               string
	carrier              string
	raw                  []byte
	capacity             uint16
	materializationIndex uint32
}

func TestReferenceC2StateFixtureRetainsExplicitTCPCarrier(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	roles := []string{"destination-resolution", "initiator", "introduction", "rendezvous", "responder"}
	peers := make(map[string]referenceC2StateRecord, len(roles))
	for index, role := range roles {
		marker := byte(index + 1)
		record := referenceC2StateRecord{role: role, nodeID: referenceC2ID(marker),
			material: referenceC2Certificate(t, int64(marker), "explicit-carrier-"+role), endpoint: referenceC2Address(t),
			family: "h42-" + role}
		if role == "rendezvous" {
			record.carrier = string(route.CarrierTCP)
		}
		peers[role] = record
	}
	fixture := newReferenceC2StateFixture(t, now, now.Add(time.Minute), authority, peers)
	for _, role := range []string{"initiator", "rendezvous", "responder"} {
		referenceC2AcceptState(t, fixture, filepath.Join(t.TempDir(), role), role)
	}
}

func newReferenceC2StateFixture(t *testing.T, now, deadline time.Time, authority ed25519.PrivateKey,
	peers map[string]referenceC2StateRecord) referenceC2StateFixture {
	return newReferenceC2StateFixtureAtEpoch(t, now, deadline, authority, 1, [32]byte{}, peers)
}

func newReferenceC2StateFixtureAtEpoch(t *testing.T, now, deadline time.Time, authority ed25519.PrivateKey, epoch uint64,
	previous [32]byte, peers map[string]referenceC2StateRecord) referenceC2StateFixture {
	t.Helper()
	if epoch == 0 || len(peers) != 5 {
		t.Fatal("reference C2 State fixture requires five role records")
	}
	network := referenceC2ID(1)
	domains := []string{"destination-resolution", "initiator", "introduction", "rendezvous", "responder"}
	var seed [32]byte
	for marker := uint64(1); ; marker++ {
		seed = sha256.Sum256([]byte("reference-c2-native-state-seed-" + string(binary.BigEndian.AppendUint64(nil, marker))))
		matches := true
		for role, record := range peers {
			selected, err := assignment.Select(network, 1, seed, record.family, domains)
			matches = matches && err == nil && selected == role
		}
		if matches {
			break
		}
	}
	records := make([]referenceC2StateRecord, 0, len(peers))
	for _, record := range peers {
		private, err := referenceC2PrivateKey(record.material)
		if err != nil {
			t.Fatal(err)
		}
		var raw []byte
		if record.carrier == "" {
			raw, err = referenceC2BuildStateRecord(network, record.nodeID, record.family, record.endpoint, now, deadline, private)
		} else {
			raw, err = referenceC2BuildStateRecordWithCarrier(network, record.nodeID, record.family, record.endpoint,
				record.carrier, now, deadline, private)
		}
		if err != nil {
			t.Fatal(err)
		}
		record.raw, record.capacity = raw, 4
		records = append(records, record)
	}
	sort.Slice(records, func(left, right int) bool {
		return bytes.Compare(records[left].nodeID[:], records[right].nodeID[:]) < 0
	})
	inputs := make([][]byte, len(records))
	for index := range records {
		records[index].materializationIndex = uint32(index)
		inputs[index] = records[index].raw
	}
	raw, digest, materials, err := referenceC2BuildStateEpoch(network, epoch, previous, now, deadline, inputs, records, seed, domains, authority)
	if err != nil {
		t.Fatal(err)
	}
	roles := make(map[string]referenceC2StateRecord, len(records))
	byRoleMaterials := make(map[string][]byte, len(records))
	for index, record := range records {
		roles[record.role] = record
		byRoleMaterials[record.role] = materials[index]
	}
	return referenceC2StateFixture{network: network, digest: digest, epoch: epoch, now: now, deadline: deadline, authority: authority,
		inputs: inputs, raw: raw, materials: byRoleMaterials, roles: roles}
}

func referenceC2SuccessorStateFixture(t *testing.T, current referenceC2StateFixture) referenceC2StateFixture {
	t.Helper()
	peers := make(map[string]referenceC2StateRecord, len(current.roles))
	for role, record := range current.roles {
		peers[role] = record
	}
	return newReferenceC2StateFixtureAtEpoch(t, current.now, current.deadline.Add(20*time.Second), current.authority, current.epoch+1, current.digest, peers)
}

func referenceC2PrivateKey(material referenceC2CertificateMaterial) (ed25519.PrivateKey, error) {
	certificate, err := tlsCertificate(material)
	if err != nil {
		return nil, err
	}
	private, ok := certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("reference C2 State fixture identity is not Ed25519")
	}
	return private, nil
}

func referenceC2AcceptState(t *testing.T, fixture referenceC2StateFixture, root, role string) entry.Candidate {
	t.Helper()
	record, ok := fixture.roles[role]
	if !ok {
		t.Fatalf("reference C2 State role %q is unavailable", role)
	}
	public := fixture.authority.Public().(ed25519.PublicKey)
	store, err := state.Open(state.Config{Root: root, NetworkID: fixture.network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		Now: fixture.now, AcceptedProfile: route.Profile, Source: source.Config{MaterialIndex: record.materializationIndex}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Accept(context.Background(), fixture.raw, fixture.inputs, [][]byte{fixture.materials[role]}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Current()
	if err != nil {
		t.Fatal(err)
	}
	for index := uint8(0); index < snapshot.CandidateCount; index++ {
		candidate := snapshot.Candidates[index]
		if candidate.NodeID == record.nodeID {
			return entry.Candidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, KeyID: candidate.KeyID, FamilyID: candidate.FamilyID,
				RecordDigest: candidate.RecordDigest, DomainProofDigest: candidate.DomainProofDigest, Endpoint: candidate.Endpoint, Capacity: candidate.Capacity,
				Domain: candidate.Domain, ValidFrom: candidate.ValidFrom, ValidUntil: candidate.ValidUntil, AssignmentNotAfter: candidate.AssignmentNotAfter}
		}
	}
	t.Fatalf("reference C2 State did not expose role %q", role)
	return entry.Candidate{}
}

func tlsCertificate(material referenceC2CertificateMaterial) (tls.Certificate, error) {
	return tls.X509KeyPair([]byte(material.certificate), []byte(material.privateKey))
}

func referenceC2BuildStateRecord(network, nodeID [32]byte, family, endpoint string, now, deadline time.Time, private ed25519.PrivateKey) ([]byte, error) {
	return referenceC2BuildStateRecordVersion(network, nodeID, family, endpoint, "", now, deadline, private, 1)
}

func referenceC2BuildStateRecordWithCarrier(network, nodeID [32]byte, family, endpoint, carrier string, now, deadline time.Time,
	private ed25519.PrivateKey) ([]byte, error) {
	return referenceC2BuildStateRecordVersion(network, nodeID, family, endpoint, carrier, now, deadline, private, 2)
}

func referenceC2BuildStateRecordVersion(network, nodeID [32]byte, family, endpoint, carrier string, now, deadline time.Time,
	private ed25519.PrivateKey, version byte) ([]byte, error) {
	if family == "" || endpoint == "" || len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("reference C2 State record input is invalid")
	}
	buffer := new(bytes.Buffer)
	buffer.WriteString("ARNR")
	buffer.WriteByte(version)
	buffer.Write(network[:])
	buffer.Write(nodeID[:])
	referenceC2U64(buffer, 1)
	referenceC2I64(buffer, now.Add(-time.Minute).Unix())
	referenceC2I64(buffer, deadline.Unix())
	referenceC2Text(buffer, family)
	buffer.WriteByte(2)
	referenceC2Text(buffer, endpoint)
	if version >= 2 {
		referenceC2Text(buffer, carrier)
	}
	referenceC2U16(buffer, 4)
	buffer.Write(private.Public().(ed25519.PublicKey))
	buffer.Write(ed25519.Sign(private, buffer.Bytes()))
	return buffer.Bytes(), nil
}
