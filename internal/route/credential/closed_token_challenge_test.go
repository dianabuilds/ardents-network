package credential

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
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
	if _, _, _, err := ClosedTokenChallenge(context, append([]byte(nil), spki[:len(spki)-1]...), nonce); err == nil {
		t.Fatal("accepted malformed exact State SPKI")
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
