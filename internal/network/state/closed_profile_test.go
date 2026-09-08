package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/binary"
	"testing"
	"time"
)

func TestParseClosedProfileRejectsChangedStateAndNoncanonicalEntries(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	authority := ed25519.NewKeyFromSeed(seed)
	network := sha256.Sum256([]byte("network"))
	generation := sha256.Sum256([]byte("generation"))
	epochDigest := sha256.Sum256([]byte("epoch"))
	first := sha256.Sum256([]byte("first"))
	second := sha256.Sum256([]byte("second"))
	nodes := []closedProfileNode{
		{nodeID: first, recordDigest: sha256.Sum256([]byte("record-first")), domain: 1, subrole: 1, generation: 1},
		{nodeID: second, recordDigest: sha256.Sum256([]byte("record-second")), domain: 2, subrole: 6, generation: 2},
	}
	if bytes.Compare(nodes[0].nodeID[:], nodes[1].nodeID[:]) > 0 {
		nodes[0], nodes[1] = nodes[1], nodes[0]
	}
	raw := testClosedProfile(t, authority, network, generation, epochDigest, now, nodes)
	profile, err := parseClosedProfile(raw, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now)
	if err != nil || profile.digest != sha256.Sum256(raw) || len(profile.nodes) != 2 || len(profile.keys) != 1 {
		t.Fatalf("parse closed profile = %+v, %v", profile, err)
	}
	if _, err := parseClosedProfile(raw, sha256.Sum256([]byte("other")), network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err == nil {
		t.Fatal("accepted profile for different State generation")
	}
	changed := append([]byte(nil), raw...)
	changed[len(changed)-1] ^= 1
	if _, err := parseClosedProfile(changed, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err == nil {
		t.Fatal("accepted changed profile signature")
	}
	unorderedNodes := append([]closedProfileNode(nil), nodes...)
	unorderedNodes[0], unorderedNodes[1] = unorderedNodes[1], unorderedNodes[0]
	unordered := testClosedProfile(t, authority, network, generation, epochDigest, now, unorderedNodes)
	if _, err := parseClosedProfile(unordered, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err == nil {
		t.Fatal("accepted unordered nodes")
	}
}

func TestPrepareSignAndInspectClosedProfileUsesOnePurposeBoundSigner(t *testing.T) {
	now := time.Unix(1_800_003_600, 0).UTC()
	signer := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	network := sha256.Sum256([]byte("closed profile network"))
	generation := sha256.Sum256([]byte("closed profile generation"))
	epochDigest := sha256.Sum256([]byte("closed profile epoch"))
	issuer := sha256.Sum256([]byte("closed profile issuer"))
	other := sha256.Sum256([]byte("closed profile other"))
	nodes := []ClosedProfileNodeInput{
		{NodeID: other, RecordDigest: sha256.Sum256([]byte("other record")), RoleDomain: 1, Subrole: 1, DutyGeneration: 2},
		{NodeID: issuer, RecordDigest: sha256.Sum256([]byte("issuer record")), RoleDomain: 2, Subrole: 6, DutyGeneration: 3},
	}
	if bytes.Compare(nodes[0].NodeID[:], nodes[1].NodeID[:]) > 0 {
		nodes[0], nodes[1] = nodes[1], nodes[0]
	}
	input := ClosedProfileInput{NetworkID: network, StateGeneration: generation, EpochDigest: epochDigest, Epoch: 7,
		IssuerNodeID: issuer, IssuanceAuthorityKey: sha256.Sum256([]byte("admission authority")), NotBefore: now, NotAfter: now.Add(time.Hour),
		Nodes: nodes, TokenKeys: []ClosedProfileTokenKeyInput{{WindowStart: now, Class: 1, SPKI: testClosedProfileSPKI(t)}}}
	body, err := PrepareClosedProfile(input)
	if err != nil || !bytes.HasPrefix(body, []byte(closedProfileMagic)) {
		t.Fatalf("prepare closed profile = %x / %v", body, err)
	}
	raw, err := SignClosedProfile(input, signer)
	if err != nil {
		t.Fatal(err)
	}
	view, err := InspectClosedProfile(raw, generation, network, epochDigest, 7, signer.Public().(ed25519.PublicKey), now)
	if err != nil || view.Epoch != 7 || view.Digest != sha256.Sum256(raw) || view.IssuerNodeID != issuer || view.TokenKeyCount != 1 ||
		view.TokenKeys[0].WindowStart != now || view.TokenKeys[0].Class != 1 || !bytes.Equal(view.TokenKeys[0].SPKI[:], input.TokenKeys[0].SPKI) {
		t.Fatalf("inspect closed profile = %+v / %v", view, err)
	}
	if _, err := SignClosedProfile(input, nil); err == nil {
		t.Fatal("signed closed profile without an authority")
	}
}

func testClosedProfile(t *testing.T, authority ed25519.PrivateKey, network, generation, epochDigest [32]byte, now time.Time, nodes []closedProfileNode) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString(closedProfileMagic)
	var u16 [2]byte
	binary.BigEndian.PutUint16(u16[:], closedProfileVersion)
	body.Write(u16[:])
	body.Write(network[:])
	body.Write(generation[:])
	var u64 [8]byte
	binary.BigEndian.PutUint64(u64[:], 9)
	body.Write(u64[:])
	body.Write(epochDigest[:])
	binary.BigEndian.PutUint64(u64[:], uint64(now.Truncate(time.Hour).Unix()))
	body.Write(u64[:])
	binary.BigEndian.PutUint64(u64[:], uint64(now.Truncate(time.Hour).Add(2*time.Hour).Unix()))
	body.Write(u64[:])
	issuer := [32]byte{}
	for _, node := range nodes {
		if node.subrole == 6 {
			issuer = node.nodeID
		}
	}
	body.Write(issuer[:])
	issuanceAuthority := sha256.Sum256([]byte("separate issuance authority"))
	body.Write(issuanceAuthority[:])
	binary.BigEndian.PutUint16(u16[:], uint16(len(nodes)))
	body.Write(u16[:])
	for _, node := range nodes {
		body.Write(node.nodeID[:])
		body.Write(node.recordDigest[:])
		body.WriteByte(node.domain)
		body.WriteByte(node.subrole)
		binary.BigEndian.PutUint64(u64[:], node.generation)
		body.Write(u64[:])
	}
	binary.BigEndian.PutUint16(u16[:], 1)
	body.Write(u16[:])
	window := uint64(now.Truncate(time.Hour).Unix())
	binary.BigEndian.PutUint64(u64[:], window)
	body.Write(u64[:])
	body.WriteByte(1)
	binary.BigEndian.PutUint16(u16[:], 346)
	body.Write(u16[:])
	body.Write(testClosedProfileSPKI(t))
	unsigned := body.Bytes()
	return append(unsigned, ed25519.Sign(authority, append([]byte("ardents-closed-profile-v3\x00"), unsigned...))...)
}

func testClosedProfileSPKI(t *testing.T) []byte {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	hash := closedProfileAlgorithmIdentifier{Algorithm: closedProfileSHA384, Parameters: asn1.RawValue{FullBytes: closedProfileNull}}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := asn1.Marshal(closedProfilePSSParameters{HashAlgorithm: hash,
		MaskGenAlgorithm: closedProfileAlgorithmIdentifier{Algorithm: closedProfileMGF1, Parameters: asn1.RawValue{FullBytes: hashDER}}, SaltLength: 48})
	if err != nil {
		t.Fatal(err)
	}
	rsaDER, err := asn1.Marshal(privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := asn1.Marshal(closedProfileSubjectPublicKeyInfo{Algorithm: closedProfileAlgorithmIdentifier{Algorithm: closedProfileRSAPSS,
		Parameters: asn1.RawValue{FullBytes: parameters}}, SubjectPublicKey: asn1.BitString{Bytes: rsaDER, BitLength: len(rsaDER) * 8}})
	if err != nil || len(spki) != 346 {
		t.Fatalf("encode test RSA-PSS SPKI = %d bytes, %v", len(spki), err)
	}
	return spki
}
