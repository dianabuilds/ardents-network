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

// EntryBinding is the User-to-Initiator v1 admission context. Invite is an
// opaque R-077 capability: only Entry validates it. ClientKeyDigest binds that
// capability to the fresh mTLS client key for this one attachment and is never
// a User identity or Route authority.
type EntryBinding struct {
	NetworkID, Digest, AttachmentID, InitiatorNodeID, ClientKeyDigest [32]byte
	Epoch                                                             uint64
	NotAfter                                                          time.Time
	Invite                                                            []byte
}

// EntryAdmission is the non-secret Initiator fact returned by the Entry port.
// It deliberately contains no User identity, credential, Route topology, or
// raw Invite bytes.
type EntryAdmission struct {
	InviteID, NetworkID, Digest, InitiatorNodeID [32]byte
	Epoch                                        uint64
	NotAfter                                     time.Time
}

// EntryBindingAdmitter verifies an opaque Invite against current Entry/State
// facts and records its replay tuple in one owner-controlled operation. The
// composition adapter maps the Entry-owned result to EntryAdmission; Route
// never reads an Entry root or verifies an Invite signature itself.
type EntryBindingAdmitter func([]byte, [32]byte, [32]byte, time.Time) (EntryAdmission, error)

// EncodeEntryBinding returns the only canonical v1 User-to-Initiator binding.
func EncodeEntryBinding(input EntryBinding) ([]byte, error) {
	if err := validEntryBinding(input); err != nil {
		return nil, err
	}
	body := entryBindingPrefix(input)
	body = appendUint16(body, uint16(len(input.Invite)))
	body = append(body, input.Invite...)
	return routeEnvelope(body)
}

// DecodeEntryBinding rejects non-canonical, incomplete, and surplus v1 bytes
// before the Initiator asks Entry to validate the opaque Invite.
func DecodeEntryBinding(raw []byte) (EntryBinding, error) {
	reader, err := routeBody(raw, entryBindingKind)
	if err != nil {
		return EntryBinding{}, err
	}
	result := EntryBinding{}
	if result.NetworkID, err = wireIdentifier(reader, "network identifier"); err != nil {
		return EntryBinding{}, err
	}
	result.Epoch = reader.uint64()
	if result.Digest, err = wireIdentifier(reader, "epoch digest"); err != nil {
		return EntryBinding{}, err
	}
	if result.AttachmentID, err = wireIdentifier(reader, "attachment identifier"); err != nil {
		return EntryBinding{}, err
	}
	if result.InitiatorNodeID, err = wireIdentifier(reader, "Initiator node identifier"); err != nil {
		return EntryBinding{}, err
	}
	notAfter := reader.uint64()
	if notAfter > uint64(^uint64(0)>>1) {
		return EntryBinding{}, errors.New("entry binding expiry is invalid")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if result.ClientKeyDigest, err = wireIdentifier(reader, "client TLS key digest"); err != nil {
		return EntryBinding{}, err
	}
	inviteLength := int(reader.uint16())
	result.Invite = append([]byte(nil), reader.take(inviteLength)...)
	if reader.off != len(reader.raw) {
		return EntryBinding{}, errors.New("entry binding has surplus bytes")
	}
	if err := validEntryBinding(result); err != nil {
		return EntryBinding{}, err
	}
	return result, nil
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

// AdmitEntryBinding proves the presenting TLS peer and exact Route attempt
// belong together, then delegates Invite validation and tuple consumption as
// one Entry-owned operation. Callers allocate Route work only after it
// succeeds.
func AdmitEntryBinding(input EntryBinding, peer *x509.Certificate, now time.Time, admit EntryBindingAdmitter) error {
	if err := validEntryBinding(input); err != nil {
		return err
	}
	if now.IsZero() || !now.UTC().Before(input.NotAfter) {
		return errors.New("entry binding is expired")
	}
	if admit == nil {
		return errors.New("entry binding admission port is incomplete")
	}
	digest, err := ClientTLSKeyDigest(peer)
	if err != nil {
		return err
	}
	if digest != input.ClientKeyDigest {
		return errors.New("entry binding does not match the TLS client key")
	}
	admission, err := admit(append([]byte(nil), input.Invite...), input.AttachmentID, input.ClientKeyDigest, input.NotAfter)
	if err != nil {
		return err
	}
	if admission.InviteID == [32]byte{} || admission.NetworkID != input.NetworkID || admission.Digest != input.Digest ||
		admission.Epoch != input.Epoch || admission.InitiatorNodeID != input.InitiatorNodeID || admission.NotAfter.IsZero() ||
		input.NotAfter.After(admission.NotAfter) {
		return errors.New("entry binding does not match current Invite authorization")
	}
	return nil
}

func entryBindingPrefix(input EntryBinding) []byte {
	body := make([]byte, 0, 2+1+1+len(routeProfile)+32+8+32+32+32+8+32+2)
	body = appendUint16(body, 1)
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
