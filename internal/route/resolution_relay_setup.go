package route

import (
	"errors"
	"time"
)

const (
	resolutionRelaySetupKind = 10
	resolutionRelayReadyKind = 11

	// ResolutionEnvelopeCapacity is the sole v1 opaque OHTTP capacity profile.
	// It is a hard carrier ceiling, not an instruction to pad a valid OHTTP
	// encapsulation after encryption.
	ResolutionEnvelopeCapacity = 8 << 10
)

// ResolutionRelaySetup authorizes exactly one opaque OHTTP exchange after an
// admitted Entry attachment. Its Gateway fields are identities only: the
// Initiator obtains its literal endpoint from its authenticated State facts.
type ResolutionRelaySetup struct {
	NetworkID, Digest, AttachmentID     [32]byte
	InitiatorNodeID                     [32]byte
	GatewayNodeID, GatewayNodePublicKey [32]byte
	Epoch                               uint64
	NotAfter                            time.Time
	EnvelopeCapacity                    uint16
}

// ResolutionRelayReady is the exact confirmation of one accepted finite
// ResolutionRelaySetup. It never substitutes a Gateway or a deadline.
type ResolutionRelayReady struct{ Setup ResolutionRelaySetup }

// EncodeResolutionRelaySetup returns the sole canonical v1 private lookup
// operation setup. It carries no OHTTP, Target, Service, or endpoint literal.
func EncodeResolutionRelaySetup(input ResolutionRelaySetup) ([]byte, error) {
	if err := validResolutionRelaySetup(input); err != nil {
		return nil, err
	}
	return resolutionRelayRecord(resolutionRelaySetupKind, input)
}

// DecodeResolutionRelaySetup rejects incomplete or substituted lookup setup.
func DecodeResolutionRelaySetup(raw []byte) (ResolutionRelaySetup, error) {
	return decodeResolutionRelayRecord(raw, resolutionRelaySetupKind)
}

// EncodeResolutionRelayReady confirms the exact validated setup.
func EncodeResolutionRelayReady(input ResolutionRelayReady) ([]byte, error) {
	if err := validResolutionRelaySetup(input.Setup); err != nil {
		return nil, err
	}
	return resolutionRelayRecord(resolutionRelayReadyKind, input.Setup)
}

// DecodeResolutionRelayReady rejects malformed or substituted confirmation.
func DecodeResolutionRelayReady(raw []byte) (ResolutionRelayReady, error) {
	setup, err := decodeResolutionRelayRecord(raw, resolutionRelayReadyKind)
	return ResolutionRelayReady{Setup: setup}, err
}

// VerifyResolutionRelayReady requires byte-for-byte setup identity before the
// Endpoint supplies its one opaque OHTTP envelope.
func (input ResolutionRelaySetup) VerifyResolutionRelayReady(ready ResolutionRelayReady) error {
	if err := validResolutionRelaySetup(input); err != nil {
		return err
	}
	if ready.Setup != input {
		return errors.New("resolution relay ready does not match Endpoint-selected setup")
	}
	return nil
}

func resolutionRelayRecord(kind byte, input ResolutionRelaySetup) ([]byte, error) {
	body := make([]byte, 0, 2+1+1+len(Profile)+32+8+32+32+32+32+32+2+8)
	body = appendUint16(body, routeWireVersion)
	body = append(body, kind)
	body = appendProfile(body)
	body = append(body, input.NetworkID[:]...)
	body = appendUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, input.AttachmentID[:]...)
	body = append(body, input.InitiatorNodeID[:]...)
	body = append(body, input.GatewayNodeID[:]...)
	body = append(body, input.GatewayNodePublicKey[:]...)
	body = appendUint16(body, input.EnvelopeCapacity)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	return routeEnvelope(body)
}

func decodeResolutionRelayRecord(raw []byte, kind byte) (ResolutionRelaySetup, error) {
	reader, err := routeBody(raw, kind)
	if err != nil {
		return ResolutionRelaySetup{}, err
	}
	result := ResolutionRelaySetup{}
	if result.NetworkID, err = wireIdentifier(reader, "network identifier"); err != nil {
		return ResolutionRelaySetup{}, err
	}
	result.Epoch = reader.uint64()
	if result.Digest, err = wireIdentifier(reader, "epoch digest"); err != nil {
		return ResolutionRelaySetup{}, err
	}
	if result.AttachmentID, err = wireIdentifier(reader, "attachment identifier"); err != nil {
		return ResolutionRelaySetup{}, err
	}
	if result.InitiatorNodeID, err = wireIdentifier(reader, "initiator node identifier"); err != nil {
		return ResolutionRelaySetup{}, err
	}
	if result.GatewayNodeID, err = wireIdentifier(reader, "Gateway node identifier"); err != nil {
		return ResolutionRelaySetup{}, err
	}
	if result.GatewayNodePublicKey, err = wireIdentifier(reader, "Gateway node public key"); err != nil {
		return ResolutionRelaySetup{}, err
	}
	result.EnvelopeCapacity = reader.uint16()
	notAfter := reader.uint64()
	if reader.off != len(reader.raw) || notAfter > uint64(^uint64(0)>>1) {
		return ResolutionRelaySetup{}, errors.New("resolution relay setup has surplus or invalid expiry")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if err := validResolutionRelaySetup(result); err != nil {
		return ResolutionRelaySetup{}, err
	}
	return result, nil
}

func validResolutionRelaySetup(input ResolutionRelaySetup) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.InitiatorNodeID == [32]byte{} || input.GatewayNodeID == [32]byte{} || input.GatewayNodePublicKey == [32]byte{} ||
		input.InitiatorNodeID == input.GatewayNodeID || input.Epoch == 0 || input.EnvelopeCapacity != ResolutionEnvelopeCapacity ||
		input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 || !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return errors.New("resolution relay setup is invalid")
	}
	return nil
}
