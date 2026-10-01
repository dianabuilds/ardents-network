package credential

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/cloudflare/circl/blindsign/blindrsa"
)

// ClosedTokenKeyID extracts the unverified key ID from a structurally framed
// token. The caller must still verify the token against its selected context
// and key before accepting it.
func ClosedTokenKeyID(token []byte) ([32]byte, bool) {
	if len(token) != closedTokenSize || binary.BigEndian.Uint16(token[:2]) != closedTokenType {
		return [32]byte{}, false
	}
	var keyID [32]byte
	copy(keyID[:], token[closedTokenKeyIDOffset:closedTokenInputSize])
	return keyID, true
}

// VerifyClosedToken verifies one finalized RFC 9578 token against the exact
// receiver context and State-admitted issuer SPKI. It neither persists a
// spend nor decides capacity; those receiver-local effects belong to Route.
func VerifyClosedToken(context ClosedTokenContext, spki, token []byte) error {
	if len(token) != closedTokenSize {
		return errors.New("closed token length is invalid")
	}
	var nonce [32]byte
	copy(nonce[:], token[2:34])
	_, expected, _, err := ClosedTokenChallenge(context, spki, nonce)
	if err != nil || !bytes.Equal(token[:len(expected)], expected) {
		return errors.New("closed token input does not match receiver context")
	}
	public, err := parseClosedTokenPublicKey(spki)
	if err != nil {
		return err
	}
	verifier, err := blindrsa.NewVerifier(blindrsa.SHA384PSSDeterministic, public)
	if err != nil || verifier.Verify(expected, token[len(expected):]) != nil {
		return errors.New("closed token signature is invalid")
	}
	return nil
}
