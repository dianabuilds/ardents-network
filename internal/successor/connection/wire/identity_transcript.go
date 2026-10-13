package wire

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// LogicalIdentity is the immutable shared tuple, never local permission. Its
// exact original verified provenance and lifetime remain with the caller.
type LogicalIdentity struct {
	Network, Target, Instance                                 [32]byte
	InstanceGeneration                                        uint64
	Publication, Profile, ConnectionNonce, InitiatorBinding   [32]byte
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter int64
}

// Contexts encodes the accepted tuple without clamping a bound or admitting a
// destination. Capsule plaintext and generation bind only the Attachment.
func Contexts(identity LogicalIdentity, plaintext [32]byte, generation uint64) (logical, attachment [32]byte, err error) {
	for _, field := range [][32]byte{identity.Network, identity.Target, identity.Instance, identity.Publication, identity.Profile, identity.ConnectionNonce, identity.InitiatorBinding, plaintext} {
		if field == [32]byte{} {
			return logical, attachment, errors.New("service identity field absent")
		}
	}
	if identity.InstanceGeneration == 0 || generation == 0 || identity.WorkSafetyNotAfter <= 0 || identity.WorkSafetyMaximum < identity.WorkSafetyNotAfter || identity.NoNewRecoveryAfter <= 0 || identity.NoNewRecoveryAfter > identity.WorkSafetyNotAfter {
		return logical, attachment, errors.New("service identity bounds differ")
	}
	transcript := []byte("ardents-service-context-v3\x00")
	for _, field := range [][32]byte{identity.Network, identity.Target, identity.Instance} {
		transcript = append(transcript, field[:]...)
	}
	transcript = binary.BigEndian.AppendUint64(transcript, identity.InstanceGeneration)
	for _, field := range [][32]byte{identity.Publication, identity.Profile, identity.ConnectionNonce, identity.InitiatorBinding} {
		transcript = append(transcript, field[:]...)
	}
	for _, bound := range []int64{identity.WorkSafetyNotAfter, identity.WorkSafetyMaximum, identity.NoNewRecoveryAfter} {
		transcript = binary.BigEndian.AppendUint64(transcript, uint64(bound))
	}
	logical = sha256.Sum256(transcript)
	transcript = append([]byte("ardents-attachment-context-v3\x00"), logical[:]...)
	transcript = append(transcript, plaintext[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, generation)
	return logical, sha256.Sum256(transcript), nil
}
