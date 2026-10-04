package closedprofile

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"time"
)

// NodeInput is one exact State-verified Node Record reference in
// an ARDCPR03 profile. It contains no address or Node private key.
type NodeInput struct {
	NodeID, RecordDigest [32]byte
	RoleDomain, Subrole  byte
	DutyGeneration       uint64
}

// TokenKeyInput is one exact public issuer token key selected by
// the closed profile. SPKI must use the closed-profile RSA-PSS grammar.
type TokenKeyInput struct {
	WindowStart time.Time
	Class       byte
	SPKI        []byte
}

// Input is the complete finite fact set signed in one ARDCPR03
// record. Private signing belongs to the provisioning owner.
type Input struct {
	NetworkID, StateGeneration, EpochDigest, IssuerNodeID, IssuanceAuthorityKey [32]byte
	Epoch                                                                       uint64
	NotBefore, NotAfter                                                         time.Time
	Nodes                                                                       []NodeInput
	TokenKeys                                                                   []TokenKeyInput
}

// Prepare returns the canonical unsigned ARDCPR03 body. It
// validates every framing, ordering and finite-resource invariant before a
// State signer is consulted.
func Prepare(input Input) ([]byte, error) {
	if input.NetworkID == [32]byte{} || input.StateGeneration == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.IssuerNodeID == [32]byte{} || input.IssuanceAuthorityKey == [32]byte{} || input.Epoch == 0 ||
		input.NotBefore.IsZero() || input.NotBefore != input.NotBefore.UTC() || input.NotAfter != input.NotAfter.UTC() ||
		!input.NotAfter.After(input.NotBefore) || input.NotAfter.Sub(input.NotBefore) > 6*time.Hour ||
		len(input.Nodes) == 0 || len(input.Nodes) > MaxNodes || len(input.TokenKeys) == 0 || len(input.TokenKeys) > MaxKeys {
		return nil, errors.New("closed profile preparation is invalid")
	}
	body := make([]byte, 0, 256+len(input.Nodes)*74+len(input.TokenKeys)*357)
	body = append(body, profileMagic...)
	body = binary.BigEndian.AppendUint16(body, profileVersion)
	for _, value := range [][32]byte{input.NetworkID, input.StateGeneration} {
		body = append(body, value[:]...)
	}
	body = binary.BigEndian.AppendUint64(body, input.Epoch)
	body = append(body, input.EpochDigest[:]...)
	body = binary.BigEndian.AppendUint64(body, uint64(input.NotBefore.Unix()))
	body = binary.BigEndian.AppendUint64(body, uint64(input.NotAfter.Unix()))
	for _, value := range [][32]byte{input.IssuerNodeID, input.IssuanceAuthorityKey} {
		body = append(body, value[:]...)
	}
	body = binary.BigEndian.AppendUint16(body, uint16(len(input.Nodes)))
	for _, node := range input.Nodes {
		body = append(body, node.NodeID[:]...)
		body = append(body, node.RecordDigest[:]...)
		body = append(body, node.RoleDomain, node.Subrole)
		body = binary.BigEndian.AppendUint64(body, node.DutyGeneration)
	}
	body = binary.BigEndian.AppendUint16(body, uint16(len(input.TokenKeys)))
	for _, key := range input.TokenKeys {
		if key.WindowStart.IsZero() || key.WindowStart != key.WindowStart.UTC() || len(key.SPKI) > 0xffff {
			return nil, errors.New("closed profile token key is invalid")
		}
		body = binary.BigEndian.AppendUint64(body, uint64(key.WindowStart.Unix()))
		body = append(body, key.Class)
		body = binary.BigEndian.AppendUint16(body, uint16(len(key.SPKI)))
		body = append(body, key.SPKI...)
	}
	if len(body)+ed25519.SignatureSize > MaxSize {
		return nil, errors.New("closed profile exceeds its public bound")
	}
	if _, err := parseBody(body, input.StateGeneration, input.NetworkID, input.EpochDigest, input.Epoch, input.NotBefore); err != nil {
		return nil, errors.New("closed profile preparation is invalid")
	}
	return body, nil
}
