package reachability

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

// Draft owns one immutable canonical Descriptor transcript. It supplies public
// codec facts, never a signer, registered slot or accepting readiness.
type Draft struct {
	raw        []byte
	transcript []byte
	target     [32]byte
	network    [32]byte
	profile    [32]byte
}

// PrepareDescriptor validates public inputs before constructing their exact v3
// transcript. The Instance caller independently owns live registration/key use.
func PrepareDescriptor(record []byte, target, network, profile [32]byte, recipient Introduction, at time.Time) (Draft, error) {
	proof, err := publication.VerifyPublish(record, target, network, at)
	if err != nil {
		return Draft{}, err
	}
	value := proof.Delegation()
	before, after := recipient.NotBefore, recipient.NotAfter
	if profile == [32]byte{} || recipient.Revision == 0 || recipient.Node == [32]byte{} || recipient.Slot == [32]byte{} || recipient.RecipientKey == [32]byte{} || before.Unix() <= 0 || !before.Equal(before.UTC().Truncate(time.Second)) || !after.Equal(after.UTC().Truncate(time.Second)) || !before.Before(after) || after.Sub(before) > 600*time.Second || at.Before(before) || !at.Before(after) || before.Before(value.NotBefore) || after.After(value.NotAfter) {
		return Draft{}, errors.New("reachability recipient bounds invalid")
	}
	raw := []byte{0, 3}
	for _, field := range [][32]byte{network, target, value.Authority, proof.Digest(), profile} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, recipient.Revision)
	for _, field := range [][32]byte{recipient.Node, recipient.Slot, recipient.RecipientKey} {
		raw = append(raw, field[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, uint64(before.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(after.Unix()))
	transcript := []byte("ardents-private-reachability-v3\x00")
	for _, field := range [][32]byte{network, profile, proof.Digest()} {
		transcript = append(transcript, field[:]...)
	}
	transcript = append(transcript, raw[162:282]...)
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(record)))
	raw = append(raw, record...)
	return Draft{raw: raw, transcript: transcript, target: target, network: network, profile: profile}, nil
}

func (draft Draft) Transcript() []byte { return append([]byte(nil), draft.transcript...) }

// Complete verifies the exact supplied signature and all public facts again.
// A detached signature or Draft is never live Instance/Store authority.
func (draft Draft) Complete(signature []byte, at time.Time) (Descriptor, error) {
	if len(draft.raw) <= descriptorHeaderSize || len(signature) != ed25519.SignatureSize {
		return Descriptor{}, errors.New("reachability Descriptor signature absent")
	}
	raw := append(append([]byte(nil), draft.raw...), signature...)
	return verifyDescriptor(raw, draft.target, draft.network, draft.profile, at, true)
}
