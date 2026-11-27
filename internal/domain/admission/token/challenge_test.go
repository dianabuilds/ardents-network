package token

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestClosedTokenChallengeBindsOnlyPublicReceiverFacts(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := encodeSelectedRSAPSSSPKI(&private.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	context := ClosedTokenContext{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, IssuerNodeID: [32]byte{4},
		ReceiverDutyGeneration: 5, Class: 2, WindowStart: time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)}
	nonce := [32]byte{6}
	challenge, input, keyID, err := ClosedTokenChallenge(context, spki, nonce)
	challengeDigest := sha256.Sum256(challenge)
	if err != nil || len(input) != 98 || keyID != sha256.Sum256(spki) || binary.BigEndian.Uint16(input[:2]) != closedTokenType ||
		!bytes.Equal(input[2:34], nonce[:]) || !bytes.Equal(input[34:66], challengeDigest[:]) {
		t.Fatalf("closed token challenge = %x / %x / %x / %v", challenge, input, keyID, err)
	}
	// These hashes come from the documented redemption transcript using
	// the same inputs except for ProfileDigest, with a fixed UTC window.
	for _, vector := range []struct {
		profile byte
		want    string
	}{
		{2, "1fb1c2ba89f4ca534c122f26667c08fbfa85c0f14c374b2e3ca18f9670069852"},
		{9, "803318c6a03981711b52eca22daed80a6f135dc2ea269cac88c72eb76d0cc6e5"},
	} {
		changed := context
		changed.ProfileDigest = [32]byte{vector.profile}
		encoded, _, _, err := ClosedTokenChallenge(changed, spki, nonce)
		if err != nil || len(encoded) < 4 {
			t.Fatalf("profile %d challenge unavailable: %v", vector.profile, err)
		}
		offset := 4 + int(binary.BigEndian.Uint16(encoded[2:4]))
		if len(encoded) < offset+33 || encoded[offset] != 32 {
			t.Fatalf("profile %d challenge lacks its 32-byte redemption context", vector.profile)
		}
		if got := hex.EncodeToString(encoded[offset+1 : offset+33]); got != vector.want {
			t.Errorf("profile %d redemption context = %s, want %s", vector.profile, got, vector.want)
		}
	}
	if _, _, _, err := ClosedTokenChallenge(context, append([]byte(nil), spki[:len(spki)-1]...), nonce); err == nil {
		t.Fatal("accepted malformed exact State SPKI")
	}
	token := append(append([]byte(nil), input...), make([]byte, closedTokenSize-len(input))...)
	if got, ok := ClosedTokenKeyID(token); !ok || got != keyID {
		t.Fatalf("unverified token key ID = %x, %v; want %x", got, ok, keyID)
	}
	for name, malformed := range map[string][]byte{
		"short":      token[:len(token)-1],
		"long":       append(append([]byte(nil), token...), 0),
		"wrong type": append([]byte{0, 3}, token[2:]...),
	} {
		if got, ok := ClosedTokenKeyID(malformed); ok || got != [32]byte{} {
			t.Errorf("%s token yielded key ID %x, %v", name, got, ok)
		}
	}
	issuerDigest := sha256.Sum256(context.IssuerNodeID[:])
	receiverDigest := sha256.Sum256(context.ReceiverNodeID[:])
	encodeName := func(prefix string, digest [32]byte) string {
		return prefix + "-" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:])) + ".invalid"
	}
	if !bytes.Contains(challenge, []byte(encodeName("i", issuerDigest))) || !bytes.Contains(challenge, []byte(encodeName("n", receiverDigest))) {
		t.Fatalf("challenge did not use hashed Node IDs: %x", challenge)
	}
}
