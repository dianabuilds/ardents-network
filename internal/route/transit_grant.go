package route

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"
)

const (
	transitGrantVersion = uint16(1)
	transitGrantPrefix  = "ardents-transit-grant-v1\x00"
	transitGrantDomain  = "ardents-transit-grant-signature-v1\x00"
)

// TransitGrant is the offline, State-authorized, one-use admission capability
// carried opaque in an EndpointTransitBinding. It contains only the exact
// transit tuple and never authorizes an Entry, Service, or peer selection.
type TransitGrant struct {
	IssuerID, GrantID, NetworkID, Digest, AttachmentID, TransitNodeID, ClientKeyDigest [32]byte
	Epoch                                                                              uint64
	TransitRole                                                                        byte
	NotAfter                                                                           time.Time
}

// IssueTransitGrant returns the sole canonical Transit Grant v1 encoding. The
// caller owns State-authority custody; Route only checks that its signing key
// matches the declared current authority identifier.
func IssueTransitGrant(input TransitGrant, signer ed25519.PrivateKey) ([]byte, error) {
	if err := validTransitGrant(input); err != nil || len(signer) != ed25519.PrivateKeySize ||
		input.IssuerID != sha256.Sum256(signer.Public().(ed25519.PublicKey)) {
		return nil, errors.New("transit grant issue input is invalid")
	}
	body := encodeTransitGrantBody(input)
	signature := ed25519.Sign(signer, append([]byte(transitGrantDomain), body...))
	return append(body, signature...), nil
}

// VerifyTransitGrant decodes one closed Transit Grant v1 and proves its
// signature under one State authority public key. Exact binding and one-use
// spending remain the receiving Node's responsibility.
func VerifyTransitGrant(raw []byte, authority ed25519.PublicKey) (TransitGrant, error) {
	if len(authority) != ed25519.PublicKeySize || len(raw) != transitGrantBodyLength()+ed25519.SignatureSize {
		return TransitGrant{}, errors.New("transit grant verification input is invalid")
	}
	input, err := decodeTransitGrantBody(raw[:transitGrantBodyLength()])
	if err != nil || input.IssuerID != sha256.Sum256(authority) ||
		!ed25519.Verify(authority, append([]byte(transitGrantDomain), raw[:transitGrantBodyLength()]...), raw[transitGrantBodyLength():]) {
		return TransitGrant{}, errors.New("transit grant signature or body is invalid")
	}
	return input, nil
}

// DecodeTransitGrant checks the closed wire grammar without granting trust to
// its signer. A Node uses IssuerID only to select one current State authority
// key, then must call VerifyTransitGrant before admission.
func DecodeTransitGrant(raw []byte) (TransitGrant, error) {
	if len(raw) != transitGrantBodyLength()+ed25519.SignatureSize {
		return TransitGrant{}, errors.New("transit grant decoding input is invalid")
	}
	return decodeTransitGrantBody(raw[:transitGrantBodyLength()])
}

func encodeTransitGrantBody(input TransitGrant) []byte {
	body := make([]byte, 0, transitGrantBodyLength())
	body = append(body, transitGrantPrefix...)
	body = appendUint16(body, transitGrantVersion)
	for _, value := range [][32]byte{input.IssuerID, input.GrantID, input.NetworkID, input.Digest, input.AttachmentID,
		input.TransitNodeID, input.ClientKeyDigest} {
		body = append(body, value[:]...)
	}
	body = appendUint64(body, input.Epoch)
	body = append(body, input.TransitRole)
	return appendUint64(body, uint64(input.NotAfter.Unix()))
}

func decodeTransitGrantBody(raw []byte) (TransitGrant, error) {
	if len(raw) != transitGrantBodyLength() || string(raw[:len(transitGrantPrefix)]) != transitGrantPrefix {
		return TransitGrant{}, errors.New("transit grant encoding is malformed")
	}
	reader := wireReader{raw: raw[len(transitGrantPrefix):]}
	if reader.uint16() != transitGrantVersion {
		return TransitGrant{}, errors.New("transit grant version is unsupported")
	}
	result := TransitGrant{}
	for _, destination := range []*[32]byte{&result.IssuerID, &result.GrantID, &result.NetworkID, &result.Digest,
		&result.AttachmentID, &result.TransitNodeID, &result.ClientKeyDigest} {
		copy(destination[:], reader.take(32))
	}
	result.Epoch = reader.uint64()
	result.TransitRole = reader.uint8()
	notAfter := reader.uint64()
	if notAfter > uint64(^uint64(0)>>1) || reader.off != len(reader.raw) {
		return TransitGrant{}, errors.New("transit grant expiry is invalid")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if err := validTransitGrant(result); err != nil {
		return TransitGrant{}, err
	}
	return result, nil
}

func transitGrantBodyLength() int {
	return len(transitGrantPrefix) + 2 + 7*32 + 8 + 1 + 8
}

func validTransitGrant(input TransitGrant) error {
	if input.IssuerID == [32]byte{} || input.GrantID == [32]byte{} || input.NetworkID == [32]byte{} || input.Digest == [32]byte{} ||
		input.AttachmentID == [32]byte{} || input.TransitNodeID == [32]byte{} || input.ClientKeyDigest == [32]byte{} || input.Epoch == 0 ||
		(input.TransitRole != IntroductionRole && input.TransitRole != ResponderRole) || input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 ||
		!input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return errors.New("transit grant is invalid")
	}
	return nil
}
