package route

import (
	"errors"
	"time"
)

const (
	credentialRelaySetupKind = 14
	credentialRelayReadyKind = 15

	// CredentialEnvelopeCapacity bounds the sole opaque request and response
	// of the membership-level Transit Grant operation. It is intentionally a
	// separate, smaller carrier from private Target resolution.
	CredentialEnvelopeCapacity = 2 << 10
)

// CredentialRelaySetup authorizes one opaque Transit Grant issuance exchange
// after a distinct admitted Entry attachment. Issuer fields are identities
// only: the Initiator obtains the literal HTTPS endpoint from its State duty.
// It carries no Name, Target, Descriptor, Publisher, or C-2 selection.
type CredentialRelaySetup struct {
	NetworkID, Digest, AttachmentID   [32]byte
	InitiatorNodeID                   [32]byte
	IssuerNodeID, IssuerNodePublicKey [32]byte
	IssuerProfileDigest               [32]byte
	Epoch                             uint64
	NotAfter                          time.Time
	EnvelopeCapacity                  uint16
}

// CredentialRelayReady confirms the exact selected issuer and exchange limit
// before the Endpoint writes an opaque OHTTP request.
type CredentialRelayReady struct{ Setup CredentialRelaySetup }

// EncodeCredentialRelaySetup returns the sole canonical membership-credential
// relay setup.
func EncodeCredentialRelaySetup(input CredentialRelaySetup) ([]byte, error) {
	if err := validCredentialRelaySetup(input); err != nil {
		return nil, err
	}
	return credentialRelayRecord(credentialRelaySetupKind, input)
}

// DecodeCredentialRelaySetup rejects malformed or substituted issuance setup.
func DecodeCredentialRelaySetup(raw []byte) (CredentialRelaySetup, error) {
	return decodeCredentialRelayRecord(raw, credentialRelaySetupKind)
}

// EncodeCredentialRelayReady returns the exact accepted setup confirmation.
func EncodeCredentialRelayReady(input CredentialRelayReady) ([]byte, error) {
	if err := validCredentialRelaySetup(input.Setup); err != nil {
		return nil, err
	}
	return credentialRelayRecord(credentialRelayReadyKind, input.Setup)
}

// DecodeCredentialRelayReady rejects malformed or substituted confirmation.
func DecodeCredentialRelayReady(raw []byte) (CredentialRelayReady, error) {
	setup, err := decodeCredentialRelayRecord(raw, credentialRelayReadyKind)
	return CredentialRelayReady{Setup: setup}, err
}

// VerifyCredentialRelayReady requires exact setup identity before an opaque
// issuance envelope may leave the Endpoint.
func (input CredentialRelaySetup) VerifyCredentialRelayReady(ready CredentialRelayReady) error {
	if err := validCredentialRelaySetup(input); err != nil {
		return err
	}
	if ready.Setup != input {
		return errors.New("credential relay ready does not match Endpoint-selected setup")
	}
	return nil
}

func credentialRelayRecord(kind byte, input CredentialRelaySetup) ([]byte, error) {
	body := make([]byte, 0, 2+1+1+len(Profile)+32+8+32+32+32+32+32+32+2+8)
	body = appendUint16(body, routeWireVersion)
	body = append(body, kind)
	body = appendProfile(body)
	body = append(body, input.NetworkID[:]...)
	body = appendUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, input.AttachmentID[:]...)
	body = append(body, input.InitiatorNodeID[:]...)
	body = append(body, input.IssuerNodeID[:]...)
	body = append(body, input.IssuerNodePublicKey[:]...)
	body = append(body, input.IssuerProfileDigest[:]...)
	body = appendUint16(body, input.EnvelopeCapacity)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	return credentialRouteEnvelope(body)
}

func decodeCredentialRelayRecord(raw []byte, kind byte) (CredentialRelaySetup, error) {
	reader, err := credentialRouteBody(raw, kind)
	if err != nil {
		return CredentialRelaySetup{}, err
	}
	result := CredentialRelaySetup{}
	if result.NetworkID, err = wireIdentifier(reader, "network identifier"); err != nil {
		return CredentialRelaySetup{}, err
	}
	result.Epoch = reader.uint64()
	if result.Digest, err = wireIdentifier(reader, "epoch digest"); err != nil {
		return CredentialRelaySetup{}, err
	}
	if result.AttachmentID, err = wireIdentifier(reader, "attachment identifier"); err != nil {
		return CredentialRelaySetup{}, err
	}
	if result.InitiatorNodeID, err = wireIdentifier(reader, "initiator node identifier"); err != nil {
		return CredentialRelaySetup{}, err
	}
	if result.IssuerNodeID, err = wireIdentifier(reader, "issuer node identifier"); err != nil {
		return CredentialRelaySetup{}, err
	}
	if result.IssuerNodePublicKey, err = wireIdentifier(reader, "issuer node public key"); err != nil {
		return CredentialRelaySetup{}, err
	}
	if result.IssuerProfileDigest, err = wireIdentifier(reader, "issuer profile digest"); err != nil {
		return CredentialRelaySetup{}, err
	}
	result.EnvelopeCapacity = reader.uint16()
	notAfter := reader.uint64()
	if reader.off != len(reader.raw) || notAfter > uint64(^uint64(0)>>1) {
		return CredentialRelaySetup{}, errors.New("credential relay setup has surplus or invalid expiry")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if err := validCredentialRelaySetup(result); err != nil {
		return CredentialRelaySetup{}, err
	}
	return result, nil
}

func validCredentialRelaySetup(input CredentialRelaySetup) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.InitiatorNodeID == [32]byte{} || input.IssuerNodeID == [32]byte{} || input.IssuerNodePublicKey == [32]byte{} ||
		input.IssuerProfileDigest == [32]byte{} || input.InitiatorNodeID == input.IssuerNodeID || input.Epoch == 0 || input.EnvelopeCapacity != CredentialEnvelopeCapacity ||
		input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 || !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return errors.New("credential relay setup is invalid")
	}
	return nil
}
