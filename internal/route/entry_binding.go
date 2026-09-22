package route

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"time"
)

const (
	entryBindingKind   = 1
	maximumEntryInvite = 1024
)

// EntryBinding is the User-to-Initiator v2 admission context. Invite is an
// opaque Entry capability: only Entry validates it. ClientKeyDigest binds that
// capability to the fresh mTLS client key for this one attachment and is never
// a User identity or Route authority.
type EntryBinding struct {
	NetworkID, Digest, AttachmentID, InitiatorNodeID, ClientKeyDigest [32]byte
	Epoch                                                             uint64
	NotAfter                                                          time.Time
	Invite                                                            []byte
}

// EncodeEntryBinding returns the only canonical v2 User-to-Initiator binding.
func EncodeEntryBinding(input EntryBinding) ([]byte, error) {
	if err := validEntryBinding(input); err != nil {
		return nil, err
	}
	body := entryBindingPrefix(input)
	body = appendUint16(body, uint16(len(input.Invite)))
	body = append(body, input.Invite...)
	return routeEnvelope(body)
}

// ClientTLSKeyDigest returns SHA-256 over the exact DER SubjectPublicKeyInfo
// from an Ed25519 client certificate. This stable certificate encoding binds a
// received TLS peer to EntryBinding without turning the fresh key into a User
// identity.
func ClientTLSKeyDigest(certificate *x509.Certificate) ([32]byte, error) {
	if certificate == nil || certificate.PublicKeyAlgorithm != x509.Ed25519 || len(certificate.RawSubjectPublicKeyInfo) == 0 {
		return [32]byte{}, errors.New("entry client certificate is not Ed25519")
	}
	if _, ok := certificate.PublicKey.(ed25519.PublicKey); !ok {
		return [32]byte{}, errors.New("entry client certificate public key is invalid")
	}
	return sha256.Sum256(certificate.RawSubjectPublicKeyInfo), nil
}

func entryBindingPrefix(input EntryBinding) []byte {
	body := make([]byte, 0, 2+1+1+len(Profile)+32+8+32+32+32+8+32+2)
	body = appendUint16(body, routeWireVersion)
	body = append(body, entryBindingKind)
	body = appendProfile(body)
	body = append(body, input.NetworkID[:]...)
	body = appendUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, input.AttachmentID[:]...)
	body = append(body, input.InitiatorNodeID[:]...)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	return append(body, input.ClientKeyDigest[:]...)
}

func validEntryBinding(input EntryBinding) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.InitiatorNodeID == [32]byte{} || input.ClientKeyDigest == [32]byte{} || input.Epoch == 0 ||
		input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 || len(input.Invite) == 0 || len(input.Invite) > maximumEntryInvite {
		return errors.New("entry binding is invalid")
	}
	return nil
}
