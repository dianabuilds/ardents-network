package closedprofile

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"math/big"
	"testing"
	"time"
)

type vectorAlgorithm struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type vectorPSS struct {
	HashAlgorithm    vectorAlgorithm `asn1:"explicit,tag:0"`
	MaskGenAlgorithm vectorAlgorithm `asn1:"explicit,tag:1"`
	SaltLength       int             `asn1:"explicit,tag:2"`
}

type vectorSPKI struct {
	Algorithm        vectorAlgorithm
	SubjectPublicKey asn1.BitString
}

func fixedTokenSPKI(t *testing.T) []byte {
	t.Helper()
	modulus := make([]byte, 256)
	modulus[0], modulus[len(modulus)-1] = 0x80, 1
	rsaDER := x509.MarshalPKCS1PublicKey(&rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: 65537})
	sha384 := asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	hash := vectorAlgorithm{Algorithm: sha384, Parameters: asn1.RawValue{FullBytes: []byte{0x05, 0x00}}}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		t.Fatal(err)
	}
	params, err := asn1.Marshal(vectorPSS{HashAlgorithm: hash,
		MaskGenAlgorithm: vectorAlgorithm{Algorithm: asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8},
			Parameters: asn1.RawValue{FullBytes: hashDER}}, SaltLength: 48})
	if err != nil {
		t.Fatal(err)
	}
	spki, err := asn1.Marshal(vectorSPKI{Algorithm: vectorAlgorithm{
		Algorithm:  asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10},
		Parameters: asn1.RawValue{FullBytes: params}},
		SubjectPublicKey: asn1.BitString{Bytes: rsaDER, BitLength: len(rsaDER) * 8}})
	if err != nil || len(spki) != 346 {
		t.Fatalf("fixed SPKI = %d bytes, %v", len(spki), err)
	}
	return spki
}

func TestSignedProfileCanonicalVector(t *testing.T) {
	now := time.Unix(1_800_003_600, 0).UTC()
	input := Input{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2},
		EpochDigest: [32]byte{3}, IssuerNodeID: [32]byte{4}, IssuanceAuthorityKey: [32]byte{5},
		Epoch: 7, NotBefore: now, NotAfter: now.Add(time.Hour),
		Nodes: []NodeInput{{NodeID: [32]byte{4}, RecordDigest: [32]byte{6},
			RoleDomain: 2, Subrole: 6, DutyGeneration: 1}},
		TokenKeys: []TokenKeyInput{{WindowStart: now, Class: 1, SPKI: fixedTokenSPKI(t)}}}
	signer := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	raw, err := Sign(input, signer)
	if err != nil {
		t.Fatal(err)
	}
	// The pre-extraction State signer at c2126de2 produced this exact vector.
	const wantDigest = "01ae69cd360e2dad5273c1e1ab746e2ef12f87162c8a80efb1ef2fcb3ef88203"
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); len(raw) != 693 || got != wantDigest {
		t.Fatalf("canonical signed profile changed: bytes=%d digest=%s", len(raw), got)
	}
	if _, err := Verify(raw, Context{StateGeneration: input.StateGeneration,
		NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, Epoch: input.Epoch,
		Authority: signer.Public().(ed25519.PublicKey), Now: now}); err != nil {
		t.Fatalf("verify fixed signed profile: %v", err)
	}
}
