package service_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

type publisherProcessPeer struct {
	nodeID   [32]byte
	private  ed25519.PrivateKey
	endpoint string
}

type publisherProcessNetwork struct {
	root            string
	snapshot        state.Snapshot
	authorityPublic ed25519.PublicKey
	nodePrivate     ed25519.PrivateKey
}

type publisherProcessRecord struct {
	peer     publisherProcessPeer
	family   string
	domain   string
	capacity uint16
	raw      []byte
}

func preparePublisherCommandNetwork(t *testing.T, directory string, now, notAfter time.Time,
	networkID [32]byte, authority ed25519.PrivateKey, seed [32]byte, peers map[string]publisherProcessPeer,
	destinationProfile, issuerProfile []byte,
) publisherProcessNetwork {
	t.Helper()
	domains := []string{"destination-resolution", "initiator", "introduction", "rendezvous", "responder", "transit-issuance"}
	records := make([]publisherProcessRecord, 0, len(domains))
	for _, domain := range domains {
		peer, ok := peers[domain]
		if !ok {
			t.Fatalf("publisher process fixture lacks %q peer", domain)
		}
		family := publisherProcessFamily(t, networkID, seed, domain, domains)
		records = append(records, publisherProcessRecord{peer: peer, family: family, domain: domain, capacity: 1,
			raw: publisherProcessRecordBytes(networkID, peer, family, now.Add(-time.Minute), notAfter)})
	}
	sort.Slice(records, func(i, j int) bool { return bytes.Compare(records[i].peer.nodeID[:], records[j].peer.nodeID[:]) < 0 })
	inputs := make([][]byte, len(records))
	for index := range records {
		inputs[index] = records[index].raw
	}
	epoch, digest, materials := publisherProcessEpoch(t, networkID, authority, seed, domains, records, inputs,
		now, notAfter, peers["destination-resolution"].nodeID, destinationProfile,
		peers["transit-issuance"].nodeID, issuerProfile)
	root := filepath.Join(directory, "network-state")
	authorityPublic := authority.Public().(ed25519.PublicKey)
	authorityID := sha256.Sum256(authorityPublic)
	owner, err := state.Open(state.Config{Root: root, NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{authorityID: authorityPublic}, Threshold: 1,
		Now: now, AcceptedProfile: "ardents-interactive-route-v1"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, acceptErr := owner.Accept(t.Context(), epoch, inputs, materials)
	closeErr := owner.Close()
	if acceptErr != nil {
		t.Fatal(acceptErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if snapshot.Digest != digest || snapshot.CandidateCount != uint8(len(records)) || snapshot.Candidates[0].Domain != "initiator" {
		t.Fatalf("publisher process State fixture = digest %x candidates %d first %q", snapshot.Digest, snapshot.CandidateCount, snapshot.Candidates[0].Domain)
	}
	return publisherProcessNetwork{root: root, snapshot: snapshot, authorityPublic: authorityPublic,
		nodePrivate: peers["initiator"].private}
}

func publisherProcessFamily(t *testing.T, network, seed [32]byte, wanted string, domains []string) string {
	t.Helper()
	label := sha256.Sum256([]byte(wanted))
	for marker := 1; marker < 10_000; marker++ {
		family := fmt.Sprintf("pf-%x-%d", label[:2], marker)
		selected, err := publisherProcessDomain(network, seed, family, domains)
		if err != nil {
			t.Fatal(err)
		}
		if selected == wanted {
			return family
		}
	}
	t.Fatalf("could not select publisher process family for %q", wanted)
	return ""
}

func publisherProcessRecordBytes(network [32]byte, peer publisherProcessPeer, family string, from, until time.Time) []byte {
	var raw bytes.Buffer
	raw.WriteString("ARNR")
	raw.WriteByte(1)
	raw.Write(network[:])
	raw.Write(peer.nodeID[:])
	writePublisherProcessU64(&raw, 1)
	writePublisherProcessI64(&raw, from.Unix())
	writePublisherProcessI64(&raw, until.Unix())
	writePublisherProcessText(&raw, family)
	raw.WriteByte(2)
	writePublisherProcessText(&raw, peer.endpoint)
	writePublisherProcessU16(&raw, 1)
	raw.Write(peer.private.Public().(ed25519.PublicKey))
	raw.Write(ed25519.Sign(peer.private, raw.Bytes()))
	return raw.Bytes()
}

func publisherProcessEpoch(t *testing.T, network [32]byte, authority ed25519.PrivateKey, seed [32]byte,
	domains []string, records []publisherProcessRecord, inputs [][]byte, now, notAfter time.Time,
	destination [32]byte, destinationProfile []byte, issuer [32]byte, issuerProfile []byte,
) ([]byte, [32]byte, [][]byte) {
	t.Helper()
	var raw bytes.Buffer
	raw.WriteString("AREP")
	raw.WriteByte(3)
	raw.Write(network[:])
	writePublisherProcessU64(&raw, 1)
	raw.Write(make([]byte, 32))
	writePublisherProcessI64(&raw, now.Add(-time.Minute).Unix())
	writePublisherProcessI64(&raw, notAfter.Unix())
	writePublisherProcessU32(&raw, uint32(len(inputs)))
	writePublisherProcessText(&raw, "ardents-interactive-route-v1")
	inputRoot, viewRoot := publisherProcessMerkleRoot(inputs, 0x10), publisherProcessMerkleRoot(inputs, 0x11)
	raw.Write(inputRoot[:])
	raw.Write(viewRoot[:])
	writePublisherProcessU32(&raw, uint32(len(inputs)))
	rejectedRoot := sha256.Sum256([]byte{0x12})
	raw.Write(rejectedRoot[:])
	writePublisherProcessU32(&raw, 0)
	raw.Write(seed[:])
	writePublisherProcessText(&raw, "ardents-h3-role-domain-v1")
	writePublisherProcessU32(&raw, uint32(len(records)))
	writePublisherProcessU32(&raw, uint32(len(records)))
	writePublisherProcessU16(&raw, uint16(len(records)))
	writePublisherProcessU16(&raw, 1)
	writePublisherProcessU32(&raw, 1)
	raw.WriteByte(byte(len(domains)))
	for _, domain := range domains {
		writePublisherProcessText(&raw, domain)
		writePublisherProcessU16(&raw, 1)
		writePublisherProcessU32(&raw, 1)
	}
	raw.Write(destination[:])
	writePublisherProcessU16(&raw, uint16(len(destinationProfile)))
	raw.Write(destinationProfile)
	raw.Write(issuer[:])
	writePublisherProcessU16(&raw, uint16(len(issuerProfile)))
	raw.Write(issuerProfile)
	digest := sha256.Sum256(raw.Bytes())
	authorityPublic := authority.Public().(ed25519.PublicKey)
	raw.WriteByte(1)
	authorityID := sha256.Sum256(authorityPublic)
	raw.Write(authorityID[:])
	raw.Write(ed25519.Sign(authority, digest[:]))
	materials := make([][]byte, len(inputs))
	for index, record := range inputs {
		var material bytes.Buffer
		material.Write(digest[:])
		writePublisherProcessU32(&material, uint32(index))
		writePublisherProcessU32(&material, uint32(len(record)))
		material.Write(record)
		siblings := publisherProcessMerkleProof(inputs, index, 0x11)
		writePublisherProcessU16(&material, uint16(len(siblings)))
		for _, sibling := range siblings {
			material.Write(sibling[:])
		}
		materials[index] = material.Bytes()
	}
	return raw.Bytes(), digest, materials
}

func publisherProcessDomain(network, seed [32]byte, family string, domains []string) (string, error) {
	var selected string
	var selectedDigest [32]byte
	for index, domain := range domains {
		encoded := append([]byte("ardents-h3-role-domain-v1\x00"), network[:]...)
		encoded = binary.BigEndian.AppendUint64(encoded, 1)
		encoded = append(encoded, seed[:]...)
		encoded = append(encoded, family...)
		encoded = append(encoded, domain...)
		digest := sha256.Sum256(encoded)
		if index > 0 && digest == selectedDigest {
			return "", fmt.Errorf("publisher process role assignment digest tie")
		}
		if selected == "" || bytes.Compare(digest[:], selectedDigest[:]) < 0 {
			selected, selectedDigest = domain, digest
		}
	}
	if selected == "" {
		return "", fmt.Errorf("publisher process role assignment is empty")
	}
	return selected, nil
}

func publisherProcessMerkleRoot(values [][]byte, emptyTag byte) [32]byte {
	leaves := make([][32]byte, len(values))
	for index, value := range values {
		encoded := make([]byte, 5+len(value))
		binary.BigEndian.PutUint32(encoded[1:5], uint32(len(value)))
		copy(encoded[5:], value)
		leaves[index] = sha256.Sum256(encoded)
	}
	return publisherProcessHashedRoot(leaves, emptyTag)
}

func publisherProcessHashedRoot(leaves [][32]byte, emptyTag byte) [32]byte {
	if len(leaves) == 0 {
		return sha256.Sum256([]byte{emptyTag})
	}
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := publisherProcessMerkleSplit(len(leaves))
	left, right := publisherProcessHashedRoot(leaves[:split], emptyTag), publisherProcessHashedRoot(leaves[split:], emptyTag)
	encoded := make([]byte, 65)
	encoded[0] = 1
	copy(encoded[1:33], left[:])
	copy(encoded[33:], right[:])
	return sha256.Sum256(encoded)
}

func publisherProcessMerkleProof(values [][]byte, index int, emptyTag byte) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := publisherProcessMerkleSplit(len(values))
	if index < split {
		return append(publisherProcessMerkleProof(values[:split], index, emptyTag), publisherProcessMerkleRoot(values[split:], emptyTag))
	}
	return append(publisherProcessMerkleProof(values[split:], index-split, emptyTag), publisherProcessMerkleRoot(values[:split], emptyTag))
}

func publisherProcessMerkleSplit(length int) int {
	split := 1
	for split<<1 < length {
		split <<= 1
	}
	return split
}

func writePublisherProcessText(raw *bytes.Buffer, value string) {
	raw.WriteByte(byte(len(value)))
	raw.WriteString(value)
}

func writePublisherProcessU16(raw *bytes.Buffer, value uint16) {
	_ = binary.Write(raw, binary.BigEndian, value)
}

func writePublisherProcessU32(raw *bytes.Buffer, value uint32) {
	_ = binary.Write(raw, binary.BigEndian, value)
}

func writePublisherProcessU64(raw *bytes.Buffer, value uint64) {
	_ = binary.Write(raw, binary.BigEndian, value)
}

func writePublisherProcessI64(raw *bytes.Buffer, value int64) {
	_ = binary.Write(raw, binary.BigEndian, value)
}
