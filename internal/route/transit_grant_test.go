package route

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

func TestTransitGrantV1IssuesAndVerifiesCanonicalVector(t *testing.T) {
	signer := ed25519.NewKeyFromSeed([]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
	input := transitGrantFixture(signer)
	raw, err := IssueTransitGrant(input, signer)
	if err != nil {
		t.Fatal(err)
	}
	const want = "617264656e74732d7472616e7369742d6772616e742d763100000134750f98bd59fcfc946da45aaabe933be154a4b5094e1c4abf42866505f3c97e51000000000000000000000000000000000000000000000000000000000000005200000000000000000000000000000000000000000000000000000000000000530000000000000000000000000000000000000000000000000000000000000054000000000000000000000000000000000000000000000000000000000000005500000000000000000000000000000000000000000000000000000000000000560000000000000000000000000000000000000000000000000000000000000000000000000000570200000000684ee1809632d79e753015fb66ca7604db3f562d22d1fe9da54e48fa2e37df43177ec188b125ad2c4a87158697fc3a42ad25ce8eca96bf879a657abd22d04ce0d3b1d60d"
	if hex.EncodeToString(raw) != want {
		t.Fatalf("canonical Transit Grant vector = %x, want %s", raw, want)
	}
	verified, err := VerifyTransitGrant(raw, signer.Public().(ed25519.PublicKey))
	if err != nil || verified != input {
		t.Fatalf("verified Transit Grant = %+v, %v", verified, err)
	}
}

func TestTransitGrantV1RefusesChangedSignerAndMalformedBytes(t *testing.T) {
	signer := ed25519.NewKeyFromSeed([]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
		2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2})
	raw, err := IssueTransitGrant(transitGrantFixture(signer), signer)
	if err != nil {
		t.Fatal(err)
	}
	other := ed25519.NewKeyFromSeed([]byte{3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
		3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3})
	mutated := append([]byte(nil), raw...)
	mutated[20] ^= 1
	for index, value := range [][]byte{nil, raw[:len(raw)-1], append(raw, 0), mutated} {
		if _, err := VerifyTransitGrant(value, signer.Public().(ed25519.PublicKey)); err == nil {
			t.Fatalf("mutation %d was accepted", index)
		}
	}
	if _, err := VerifyTransitGrant(raw, other.Public().(ed25519.PublicKey)); err == nil {
		t.Fatal("substituted State authority was accepted")
	}
}

func transitGrantFixture(signer ed25519.PrivateKey) TransitGrant {
	issuer := sha256.Sum256(signer.Public().(ed25519.PublicKey))
	return TransitGrant{IssuerID: issuer, GrantID: identifier(81), NetworkID: identifier(82), Digest: identifier(83),
		AttachmentID: identifier(84), TransitNodeID: identifier(85), ClientKeyDigest: identifier(86), Epoch: 87,
		TransitRole: IntroductionRole, NotAfter: time.Unix(1_750_000_000, 0).UTC()}
}
