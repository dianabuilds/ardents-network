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

const (
	closedProfileMagic   = "ARDCPR03"
	closedProfileVersion = uint16(3)
)

var (
	closedProfileMGF1   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	closedProfileRSAPSS = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	closedProfileSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	closedProfileNull   = []byte{0x05, 0x00}
)

type closedProfileNode struct {
	nodeID, recordDigest [32]byte
	domain, subrole      byte
	generation           uint64
}
type closedProfileAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}
type closedProfilePSSParameters struct {
	HashAlgorithm    closedProfileAlgorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm closedProfileAlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                              `asn1:"explicit,tag:2"`
}
type closedProfileSubjectPublicKeyInfo struct {
	Algorithm        closedProfileAlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

func testClosedProfile(t *testing.T, authority ed25519.PrivateKey, network, generation, epochDigest [32]byte, now time.Time, nodes []closedProfileNode) []byte {
	return testClosedProfileAt(t, authority, network, generation, epochDigest, 9, now, nodes)
}

func testClosedProfileAt(t *testing.T, authority ed25519.PrivateKey, network, generation, epochDigest [32]byte, epoch uint64, now time.Time, nodes []closedProfileNode) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString(closedProfileMagic)
	var u16 [2]byte
	binary.BigEndian.PutUint16(u16[:], closedProfileVersion)
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
