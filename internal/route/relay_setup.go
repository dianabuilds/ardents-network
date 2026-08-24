package route

import (
	"errors"
	"time"
)

const (
	relaySetupKind = 4
	relayReadyKind = 5
)

// RelaySetup is the Endpoint-to-transit-node authorization for one next
// adjacent native leg. It travels only after admitted Entry TLS. It contains
// no literal endpoint: the transit node obtains that only by rechecking its
// current authenticated State facts.
type RelaySetup struct {
	NetworkID, Digest, AttachmentID              [32]byte
	TransitNodeID, NextNodeID, NextNodePublicKey [32]byte
	Epoch                                        uint64
	TransitRole, NextRole                        byte
	NotAfter                                     time.Time
}

// RelayReady is the exact confirmation of one accepted RelaySetup. It is sent
// only after the transit node has validated current State and established the
// reciprocal next-node LegBinding.
type RelayReady struct{ Setup RelaySetup }

// EncodeRelaySetup returns the sole canonical Endpoint-to-transit setup form.
func EncodeRelaySetup(input RelaySetup) ([]byte, error) {
	if err := validRelaySetup(input); err != nil {
		return nil, err
	}
	return relayEnvelope(relaySetupKind, input)
}

// DecodeRelaySetup rejects every malformed, surplus, unsupported, or
// non-canonical setup record before a transit node can allocate a next-leg
// dial or TLS worker.
func DecodeRelaySetup(raw []byte) (RelaySetup, error) { return decodeRelayRecord(raw, relaySetupKind) }

// EncodeRelayReady returns the canonical confirmation for one exact setup.
func EncodeRelayReady(input RelayReady) ([]byte, error) {
	if err := validRelaySetup(input.Setup); err != nil {
		return nil, err
	}
	return relayEnvelope(relayReadyKind, input.Setup)
}

// DecodeRelayReady rejects a malformed or incomplete transit confirmation.
func DecodeRelayReady(raw []byte) (RelayReady, error) {
	setup, err := decodeRelayRecord(raw, relayReadyKind)
	return RelayReady{Setup: setup}, err
}

// VerifyRelayReady requires the transit confirmation to reproduce the exact
// Endpoint-selected setup. A ready record never authorizes a substitute peer.
func (input RelaySetup) VerifyRelayReady(ready RelayReady) error {
	if err := validRelaySetup(input); err != nil {
		return err
	}
	if ready.Setup != input {
		return errors.New("relay ready does not match Endpoint-selected setup")
	}
	return nil
}

func relayEnvelope(kind byte, input RelaySetup) ([]byte, error) {
	body := make([]byte, 0, 2+1+1+len(Profile)+32+8+32+32+1+1+32+32+32+8)
	body = appendUint16(body, 1)
	body = append(body, kind)
	body = appendProfile(body)
	body = append(body, input.NetworkID[:]...)
	body = appendUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, input.AttachmentID[:]...)
	body = append(body, input.TransitRole, input.NextRole)
	body = append(body, input.TransitNodeID[:]...)
	body = append(body, input.NextNodeID[:]...)
	body = append(body, input.NextNodePublicKey[:]...)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	return routeEnvelope(body)
}

func decodeRelayRecord(raw []byte, kind byte) (RelaySetup, error) {
	reader, err := routeBody(raw, kind)
	if err != nil {
		return RelaySetup{}, err
	}
	result := RelaySetup{}
	if result.NetworkID, err = wireIdentifier(reader, "network identifier"); err != nil {
		return RelaySetup{}, err
	}
	result.Epoch = reader.uint64()
	if result.Digest, err = wireIdentifier(reader, "epoch digest"); err != nil {
		return RelaySetup{}, err
	}
	if result.AttachmentID, err = wireIdentifier(reader, "attachment identifier"); err != nil {
		return RelaySetup{}, err
	}
	result.TransitRole, result.NextRole = reader.uint8(), reader.uint8()
	if result.TransitNodeID, err = wireIdentifier(reader, "transit node identifier"); err != nil {
		return RelaySetup{}, err
	}
	if result.NextNodeID, err = wireIdentifier(reader, "next node identifier"); err != nil {
		return RelaySetup{}, err
	}
	if result.NextNodePublicKey, err = wireIdentifier(reader, "next node public key"); err != nil {
		return RelaySetup{}, err
	}
	notAfter := reader.uint64()
	if reader.off != len(reader.raw) || notAfter > uint64(^uint64(0)>>1) {
		return RelaySetup{}, errors.New("relay setup has surplus or invalid expiry")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if err := validRelaySetup(result); err != nil {
		return RelaySetup{}, err
	}
	return result, nil
}

func validRelaySetup(input RelaySetup) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.TransitNodeID == [32]byte{} || input.NextNodeID == [32]byte{} || input.NextNodePublicKey == [32]byte{} ||
		input.TransitNodeID == input.NextNodeID || input.Epoch == 0 ||
		(input.TransitRole != InitiatorRole && input.TransitRole != ResponderRole) || input.NextRole != RendezvousRole ||
		input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 || !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) {
		return errors.New("relay setup is invalid")
	}
	return nil
}
