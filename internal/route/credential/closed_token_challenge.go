package credential

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

const (
	closedTokenType = uint16(2)
	closedTokenSize = 354
)

// ClosedTokenContext is the exact public receiver fact set bound into one
// RFC 9577 challenge. It intentionally contains no Target, holder, Persona,
// or Endpoint-local context identity.
type ClosedTokenContext struct {
	NetworkID, ProfileDigest, ReceiverNodeID, IssuerNodeID [32]byte
	ReceiverDutyGeneration                                 uint64
	Class                                                  uint8
	WindowStart                                            time.Time
}

// ClosedTokenChallenge creates the RFC 9577 challenge and the matching token
// input for the selected RFC 9578 Blind RSA type. The key ID is derived only
// from the exact State-accepted RSA-PSS SPKI bytes.
func ClosedTokenChallenge(context ClosedTokenContext, spki []byte, nonce [32]byte) ([]byte, []byte, [32]byte, error) {
	if context.NetworkID == [32]byte{} || context.ProfileDigest == [32]byte{} || context.ReceiverNodeID == [32]byte{} ||
		context.IssuerNodeID == [32]byte{} || context.ReceiverDutyGeneration == 0 || context.Class < 1 || context.Class > 3 ||
		context.WindowStart.IsZero() || context.WindowStart != context.WindowStart.UTC() || context.WindowStart.Truncate(time.Hour) != context.WindowStart ||
		nonce == [32]byte{} || !state.ValidateClosedTokenSPKI(spki) {
		return nil, nil, [32]byte{}, errors.New("closed token context is invalid")
	}
	issuerName := closedTokenServerName('i', context.IssuerNodeID)
	originName := closedTokenServerName('n', context.ReceiverNodeID)
	redemption := closedTokenRedemptionContext(context)
	if len(issuerName) == 0 || len(originName) == 0 || len(issuerName) > 0xffff || len(originName) > 0xffff {
		return nil, nil, [32]byte{}, errors.New("closed token names are invalid")
	}
	challenge := make([]byte, 0, 2+2+len(issuerName)+1+32+2+len(originName))
	challenge = binary.BigEndian.AppendUint16(challenge, closedTokenType)
	challenge = binary.BigEndian.AppendUint16(challenge, uint16(len(issuerName)))
	challenge = append(challenge, issuerName...)
	challenge = append(challenge, byte(len(redemption)))
	challenge = append(challenge, redemption[:]...)
	challenge = binary.BigEndian.AppendUint16(challenge, uint16(len(originName)))
	challenge = append(challenge, originName...)
	keyID := sha256.Sum256(spki)
	digest := sha256.Sum256(challenge)
	input := make([]byte, 0, 98)
	input = binary.BigEndian.AppendUint16(input, closedTokenType)
	input = append(input, nonce[:]...)
	input = append(input, digest[:]...)
	input = append(input, keyID[:]...)
	return challenge, input, keyID, nil
}

func closedTokenRedemptionContext(context ClosedTokenContext) [32]byte {
	input := make([]byte, 0, len("ardents-admission-context-v1\x00")+32*3+8+1+8)
	input = append(input, "ardents-admission-context-v1\x00"...)
	for _, value := range [][32]byte{context.NetworkID, context.ProfileDigest, context.ReceiverNodeID} {
		input = append(input, value[:]...)
	}
	input = binary.BigEndian.AppendUint64(input, context.ReceiverDutyGeneration)
	input = append(input, context.Class)
	input = binary.BigEndian.AppendUint64(input, uint64(context.WindowStart.Unix()))
	return sha256.Sum256(input)
}

func closedTokenServerName(prefix byte, node [32]byte) string {
	if node == [32]byte{} || prefix != 'i' && prefix != 'n' {
		return ""
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(node[:])
	return string(prefix) + "-" + strings.ToLower(encoded) + ".invalid"
}
