package state

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"time"
)

// ClosedProfileNodeInput is one exact State-verified Node Record reference in
// an ARDCPR03 profile. It contains no address or Node private key.
type ClosedProfileNodeInput struct {
	NodeID, RecordDigest [32]byte
	RoleDomain, Subrole  byte
	DutyGeneration       uint64
}

// ClosedProfileTokenKeyInput is one exact public issuer token key selected by
// the closed profile. SPKI must use the State-owned RSA-PSS grammar.
type ClosedProfileTokenKeyInput struct {
	WindowStart time.Time
	Class       byte
	SPKI        []byte
}

// ClosedProfileInput is the complete finite fact set signed in one ARDCPR03
// record. Its signing key is supplied separately and is never retained.
type ClosedProfileInput struct {
	NetworkID, StateGeneration, EpochDigest, IssuerNodeID, IssuanceAuthorityKey [32]byte
	Epoch                                                                       uint64
	NotBefore, NotAfter                                                         time.Time
	Nodes                                                                       []ClosedProfileNodeInput
	TokenKeys                                                                   []ClosedProfileTokenKeyInput
}

// PrepareClosedProfile returns the canonical unsigned ARDCPR03 body. It
// validates every framing, ordering and finite-resource invariant before a
// State signer is consulted.
func PrepareClosedProfile(input ClosedProfileInput) ([]byte, error) {
	if input.NetworkID == [32]byte{} || input.StateGeneration == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.IssuerNodeID == [32]byte{} || input.IssuanceAuthorityKey == [32]byte{} || input.Epoch == 0 ||
		input.NotBefore.IsZero() || input.NotBefore != input.NotBefore.UTC() || input.NotAfter != input.NotAfter.UTC() ||
		!input.NotAfter.After(input.NotBefore) || input.NotAfter.Sub(input.NotBefore) > 6*time.Hour ||
		len(input.Nodes) == 0 || len(input.Nodes) > maximumClosedProfileNodes || len(input.TokenKeys) == 0 || len(input.TokenKeys) > maximumClosedProfileKeys {
		return nil, errors.New("closed profile preparation is invalid")
	}
	body := make([]byte, 0, 256+len(input.Nodes)*74+len(input.TokenKeys)*357)
	body = append(body, closedProfileMagic...)
	body = binary.BigEndian.AppendUint16(body, closedProfileVersion)
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
	if len(body)+ed25519.SignatureSize > maximumClosedProfileSize {
		return nil, errors.New("closed profile exceeds its public bound")
	}
	probe := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	raw := append(append([]byte(nil), body...), ed25519.Sign(probe, append([]byte("ardents-closed-profile-v3\x00"), body...))...)
	if _, err := parseClosedProfile(raw, input.StateGeneration, input.NetworkID, input.EpochDigest, input.Epoch, probe.Public().(ed25519.PublicKey), input.NotBefore); err != nil {
		return nil, errors.New("closed profile preparation is invalid")
	}
	return body, nil
}

// SignClosedProfile signs exactly one canonical prepared profile. It neither
// creates nor exports an authority key and cannot sign another grammar.
func SignClosedProfile(input ClosedProfileInput, signer ed25519.PrivateKey) ([]byte, error) {
	body, err := PrepareClosedProfile(input)
	if err != nil || len(signer) != ed25519.PrivateKeySize {
		return nil, errors.New("closed profile signing is unavailable")
	}
	return append(body, ed25519.Sign(signer, append([]byte("ardents-closed-profile-v3\x00"), body...))...), nil
}

// InspectClosedProfile verifies one signed profile against its exact current
// State facts without accepting it or mutating a State root.
func InspectClosedProfile(raw []byte, stateGeneration, networkID, epochDigest [32]byte, epoch uint64, authority ed25519.PublicKey, now time.Time) (ClosedProfileView, error) {
	profile, err := parseClosedProfile(raw, stateGeneration, networkID, epochDigest, epoch, authority, now.UTC())
	if err != nil {
		return ClosedProfileView{}, err
	}
	return ClosedProfileView{Digest: profile.digest, IssuanceAuthorityKey: profile.authorityKey, Epoch: profile.epoch,
		NotBefore: profile.notBefore, NotAfter: profile.notAfter}, nil
}
