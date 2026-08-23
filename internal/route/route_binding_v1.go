package route

import (
	"errors"
	"time"
)

const (
	legBindingKind   = 2
	initiatorRole    = 1
	introductionRole = 2
	rendezvousRole   = 3
	responderRole    = 4
)

// LegBinding is the complete non-secret context that one adjacent C-5 peer
// must reciprocally bind before it receives an attachment. It never carries a
// complete Route or Service material.
type LegBinding struct {
	NetworkID, Digest, AttachmentID [32]byte
	Epoch                           uint64
	SenderRole, PeerRole            byte
	SenderNodeID, PeerNodeID        [32]byte
	NotAfter                        time.Time
}

// EncodeLegBinding returns the one canonical v1 binding record.
func EncodeLegBinding(input LegBinding) ([]byte, error) {
	if err := validLegBinding(input); err != nil {
		return nil, err
	}
	body := make([]byte, 0, 2+1+1+len(routeProfile)+32+8+32+32+1+32+1+32+8)
	body = appendUint16(body, 1)
	body = append(body, legBindingKind)
	body = appendProfile(body)
	body = append(body, input.NetworkID[:]...)
	body = appendUint64(body, input.Epoch)
	body = append(body, input.Digest[:]...)
	body = append(body, input.AttachmentID[:]...)
	body = append(body, input.SenderRole)
	body = append(body, input.SenderNodeID[:]...)
	body = append(body, input.PeerRole)
	body = append(body, input.PeerNodeID[:]...)
	body = appendUint64(body, uint64(input.NotAfter.UTC().Unix()))
	return routeEnvelope(body)
}

// DecodeLegBinding rejects every non-canonical, unknown, incomplete, or
// surplus representation of the v1 adjacent-peer binding.
func DecodeLegBinding(raw []byte) (LegBinding, error) {
	return decodeLegBindingOrdered(raw)
}

func decodeLegBindingOrdered(raw []byte) (LegBinding, error) {
	reader, err := routeBody(raw, legBindingKind)
	if err != nil {
		return LegBinding{}, err
	}
	result := LegBinding{}
	if result.NetworkID, err = wireIdentifier(reader, "network identifier"); err != nil {
		return LegBinding{}, err
	}
	result.Epoch = reader.uint64()
	if result.Digest, err = wireIdentifier(reader, "epoch digest"); err != nil {
		return LegBinding{}, err
	}
	if result.AttachmentID, err = wireIdentifier(reader, "attachment identifier"); err != nil {
		return LegBinding{}, err
	}
	result.SenderRole = reader.uint8()
	if result.SenderNodeID, err = wireIdentifier(reader, "sender node identifier"); err != nil {
		return LegBinding{}, err
	}
	result.PeerRole = reader.uint8()
	if result.PeerNodeID, err = wireIdentifier(reader, "peer node identifier"); err != nil {
		return LegBinding{}, err
	}
	notAfter := reader.uint64()
	if reader.off != len(reader.raw) || notAfter > uint64(^uint64(0)>>1) {
		return LegBinding{}, errors.New("route leg binding has surplus or invalid expiry")
	}
	result.NotAfter = time.Unix(int64(notAfter), 0).UTC()
	if err := validLegBinding(result); err != nil {
		return LegBinding{}, err
	}
	return result, nil
}

// VerifyReciprocal proves that peer is the only binding that can share this
// attachment: common context must match and the adjacent identities reverse.
func (input LegBinding) VerifyReciprocal(peer LegBinding) error {
	if err := validLegBinding(input); err != nil {
		return err
	}
	if err := validLegBinding(peer); err != nil {
		return err
	}
	if input.NetworkID != peer.NetworkID || input.Digest != peer.Digest || input.Epoch != peer.Epoch ||
		input.AttachmentID != peer.AttachmentID || !input.NotAfter.Equal(peer.NotAfter) ||
		input.SenderRole != peer.PeerRole || input.PeerRole != peer.SenderRole ||
		input.SenderNodeID != peer.PeerNodeID || input.PeerNodeID != peer.SenderNodeID {
		return errors.New("route leg bindings are not reciprocal")
	}
	return nil
}

func validLegBinding(input LegBinding) error {
	if input.NetworkID == [32]byte{} || input.Digest == [32]byte{} || input.AttachmentID == [32]byte{} ||
		input.Epoch == 0 || input.SenderNodeID == [32]byte{} || input.PeerNodeID == [32]byte{} ||
		input.SenderNodeID == input.PeerNodeID || !validRouteRole(input.SenderRole) || !validRouteRole(input.PeerRole) ||
		input.SenderRole == input.PeerRole || input.NotAfter.IsZero() || input.NotAfter.Unix() <= 0 {
		return errors.New("route leg binding is invalid")
	}
	return nil
}

func validRouteRole(value byte) bool {
	return value >= initiatorRole && value <= responderRole
}
