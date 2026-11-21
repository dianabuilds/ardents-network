package closedprofile

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
	nodes := []Node{
		{NodeID: first, RecordDigest: sha256.Sum256([]byte("record-first")), RoleDomain: 1, Subrole: 1, DutyGeneration: 1},
		{NodeID: second, RecordDigest: sha256.Sum256([]byte("record-second")), RoleDomain: 2, Subrole: 6, DutyGeneration: 2},
	}
	if bytes.Compare(nodes[0].NodeID[:], nodes[1].NodeID[:]) > 0 {
		nodes[0], nodes[1] = nodes[1], nodes[0]
	}
	raw := testClosedProfile(t, authority, network, generation, epochDigest, now, nodes)
	profile, err := verifyFixture(raw, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now)
	if err != nil || profile.Digest != sha256.Sum256(raw) || len(profile.Nodes) != 2 || len(profile.Keys) != 1 {
		t.Fatalf("parse closed profile = %+v, %v", profile, err)
	}
	if _, err := verifyFixture(raw, sha256.Sum256([]byte("other")), network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err == nil {
		t.Fatal("accepted profile for different State generation")
	}
	changed := append([]byte(nil), raw...)
	changed[len(changed)-1] ^= 1
	if _, err := verifyFixture(changed, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err == nil {
		t.Fatal("accepted changed profile signature")
	}
	unorderedNodes := append([]Node(nil), nodes...)
	unorderedNodes[0], unorderedNodes[1] = unorderedNodes[1], unorderedNodes[0]
	unordered := testClosedProfile(t, authority, network, generation, epochDigest, now, unorderedNodes)
	if _, err := verifyFixture(unordered, generation, network, epochDigest, 9, authority.Public().(ed25519.PublicKey), now); err == nil {
		t.Fatal("accepted unordered nodes")
	}
}

func TestUnsignedPreparationAndExternalSignatureMatchVerifiedGrammar(t *testing.T) {
	now := time.Unix(1_800_003_600, 0).UTC()
	signer := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	network := sha256.Sum256([]byte("closed profile network"))
	generation := sha256.Sum256([]byte("closed profile generation"))
	epochDigest := sha256.Sum256([]byte("closed profile epoch"))
	issuer := sha256.Sum256([]byte("closed profile issuer"))
	other := sha256.Sum256([]byte("closed profile other"))
	nodes := []NodeInput{
		{NodeID: other, RecordDigest: sha256.Sum256([]byte("other record")), RoleDomain: 1, Subrole: 1, DutyGeneration: 2},
		{NodeID: issuer, RecordDigest: sha256.Sum256([]byte("issuer record")), RoleDomain: 2, Subrole: 6, DutyGeneration: 3},
	}
	if bytes.Compare(nodes[0].NodeID[:], nodes[1].NodeID[:]) > 0 {
		nodes[0], nodes[1] = nodes[1], nodes[0]
	}
	input := Input{NetworkID: network, StateGeneration: generation, EpochDigest: epochDigest, Epoch: 7,
		IssuerNodeID: issuer, IssuanceAuthorityKey: sha256.Sum256([]byte("admission authority")), NotBefore: now, NotAfter: now.Add(time.Hour),
		Nodes: nodes, TokenKeys: []TokenKeyInput{{WindowStart: now, Class: 1, SPKI: testClosedProfileSPKI(t)}}}
	body, err := Prepare(input)
	if err != nil || !bytes.HasPrefix(body, []byte(profileMagic)) {
		t.Fatalf("prepare closed profile = %x / %v", body, err)
	}
	raw := append(bytes.Clone(body), ed25519.Sign(signer, append([]byte("ardents-closed-profile-v3\x00"), body...))...)
	if len(raw) != len(body)+ed25519.SignatureSize || !bytes.Equal(raw[:len(body)], body) {
		t.Fatal("signed profile changed the prepared canonical body")
	}
	view, err := Verify(raw, Context{StateGeneration: generation, NetworkID: network, EpochDigest: epochDigest, Epoch: 7, Authority: signer.Public().(ed25519.PublicKey), Now: now})
	if err != nil || view.NetworkID != network || view.StateGeneration != generation || view.EpochDigest != epochDigest || view.Epoch != 7 || view.Digest != sha256.Sum256(raw) || view.IssuerNodeID != issuer || len(view.Keys) != 1 ||
		view.Keys[0].WindowStart != uint64(now.Unix()) || view.Keys[0].Class != 1 || !bytes.Equal(view.Keys[0].SPKI, input.TokenKeys[0].SPKI) {
		t.Fatalf("inspect closed profile = %+v / %v", view, err)
	}
	issuerFound := false
	for _, node := range view.Nodes {
		if node.NodeID == issuer {
			issuerFound = node.DutyGeneration == 3
		}
	}
	if !issuerFound {
		t.Fatal("verified profile lost the issuer duty generation")
	}
	originalSPKI := append([]byte(nil), view.Keys[0].SPKI...)
	raw[len(raw)-ed25519.SignatureSize-1] ^= 1
	if !bytes.Equal(view.Keys[0].SPKI, originalSPKI) {
		t.Fatal("verified profile retained caller-owned SPKI bytes")
	}
	unordered := input
	unordered.Nodes = append([]NodeInput(nil), input.Nodes...)
	unordered.Nodes[0], unordered.Nodes[1] = unordered.Nodes[1], unordered.Nodes[0]
	if _, err := Prepare(unordered); err == nil {
		t.Fatal("prepared a profile with noncanonical Node order")
	}
}

func verifyFixture(raw []byte, generation, network, digest [32]byte, number uint64, authority ed25519.PublicKey, now time.Time) (Profile, error) {
	return Verify(raw, Context{StateGeneration: generation, NetworkID: network, EpochDigest: digest,
		Epoch: number, Authority: authority, Now: now})
}

func testClosedProfile(t *testing.T, authority ed25519.PrivateKey, network, generation, epochDigest [32]byte, now time.Time, nodes []Node) []byte {
	return testClosedProfileAt(t, authority, network, generation, epochDigest, 9, now, nodes)
}

func testClosedProfileAt(t *testing.T, authority ed25519.PrivateKey, network, generation, epochDigest [32]byte, epoch uint64, now time.Time, nodes []Node) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString(profileMagic)
	var u16 [2]byte
	binary.BigEndian.PutUint16(u16[:], profileVersion)
	body.Write(u16[:])
	body.Write(network[:])
	body.Write(generation[:])
	var u64 [8]byte
	binary.BigEndian.PutUint64(u64[:], epoch)
	body.Write(u64[:])
	body.Write(epochDigest[:])
	binary.BigEndian.PutUint64(u64[:], uint64(now.Truncate(time.Hour).Unix()))
	body.Write(u64[:])
	binary.BigEndian.PutUint64(u64[:], uint64(now.Truncate(time.Hour).Add(2*time.Hour).Unix()))
	body.Write(u64[:])
	issuer := [32]byte{}
	for _, node := range nodes {
		if node.Subrole == 6 {
			issuer = node.NodeID
		}
	}
	body.Write(issuer[:])
	issuanceAuthority := sha256.Sum256([]byte("separate issuance authority"))
	body.Write(issuanceAuthority[:])
	binary.BigEndian.PutUint16(u16[:], uint16(len(nodes)))
	body.Write(u16[:])
	for _, node := range nodes {
		body.Write(node.NodeID[:])
		body.Write(node.RecordDigest[:])
		body.WriteByte(node.RoleDomain)
		body.WriteByte(node.Subrole)
		binary.BigEndian.PutUint64(u64[:], node.DutyGeneration)
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

// Independent canonical fixture grammar: production key validation belongs
// to Admission and must not supply its own acceptance vectors.
var (
	pssMGF1      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	pssAlgorithm = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	pssSHA384    = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	asn1Null     = []byte{0x05, 0x00}
)

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type pssParameters struct {
	HashAlgorithm    algorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm algorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                 `asn1:"explicit,tag:2"`
}

type subjectPublicKeyInfo struct {
	Algorithm        algorithmIdentifier
	SubjectPublicKey asn1.BitString
}

func testClosedProfileSPKI(t *testing.T) []byte {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	hash := algorithmIdentifier{Algorithm: pssSHA384, Parameters: asn1.RawValue{FullBytes: asn1Null}}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := asn1.Marshal(pssParameters{HashAlgorithm: hash,
		MaskGenAlgorithm: algorithmIdentifier{Algorithm: pssMGF1, Parameters: asn1.RawValue{FullBytes: hashDER}}, SaltLength: 48})
	if err != nil {
		t.Fatal(err)
	}
	rsaDER, err := asn1.Marshal(privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := asn1.Marshal(subjectPublicKeyInfo{Algorithm: algorithmIdentifier{Algorithm: pssAlgorithm,
		Parameters: asn1.RawValue{FullBytes: parameters}}, SubjectPublicKey: asn1.BitString{Bytes: rsaDER, BitLength: len(rsaDER) * 8}})
	if err != nil || len(spki) != 346 {
		t.Fatalf("encode test RSA-PSS SPKI = %d bytes, %v", len(spki), err)
	}
	return spki
}
