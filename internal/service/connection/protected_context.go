//go:build linux

package connection

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// ProtectedContextInput is the immutable shared tuple selected for the
// protected Service Connection. It contains no local scope identifier, salt,
// Route peer, join secret or per-Attachment handshake material. Its caller must
// independently verify publication and local authority before accepting it.
type ProtectedContextInput struct {
	Network, Target, InstancePublic, PublicationDigest        [32]byte
	InstanceGeneration                                        uint64
	ProfileDigest, ConnectionNonce, InitiatorBinding          [32]byte
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter int64
}

// ProtectedContext hashes the fixed generation-3 logical tuple. Validation of
// authority/currentness belongs to Endpoint; no hash creates that authority.
func ProtectedContext(input ProtectedContextInput) ([32]byte, error) {
	if input.Network == [32]byte{} || input.Target == [32]byte{} || input.InstancePublic == [32]byte{} ||
		input.PublicationDigest == [32]byte{} || input.InstanceGeneration == 0 || input.ProfileDigest == [32]byte{} ||
		input.ConnectionNonce == [32]byte{} || input.InitiatorBinding == [32]byte{} ||
		input.WorkSafetyNotAfter <= 0 || input.WorkSafetyMaximum < input.WorkSafetyNotAfter ||
		input.NoNewRecoveryAfter <= 0 || input.NoNewRecoveryAfter > input.WorkSafetyNotAfter {
		return [32]byte{}, errors.New("protected Service context is incomplete")
	}
	encoded := []byte("ardents-service-context-v3\x00")
	for _, field := range [][32]byte{input.Network, input.Target, input.InstancePublic} {
		encoded = append(encoded, field[:]...)
	}
	encoded = binary.BigEndian.AppendUint64(encoded, input.InstanceGeneration)
	for _, field := range [][32]byte{input.PublicationDigest, input.ProfileDigest, input.ConnectionNonce, input.InitiatorBinding} {
		encoded = append(encoded, field[:]...)
	}
	for _, bound := range []int64{input.WorkSafetyNotAfter, input.WorkSafetyMaximum, input.NoNewRecoveryAfter} {
		encoded = binary.BigEndian.AppendUint64(encoded, uint64(bound))
	}
	return sha256.Sum256(encoded), nil
}

// ProtectedAttachmentContext binds an Attachment's exporter to its exact
// authenticated capsule plaintext and generation. It never replaces the
// immutable logical context or the first Attachment's continuity secret.
func ProtectedAttachmentContext(logical, capsuleDigest [32]byte, generation uint64) ([32]byte, error) {
	if logical == [32]byte{} || capsuleDigest == [32]byte{} || generation == 0 {
		return [32]byte{}, errors.New("protected Attachment context is incomplete")
	}
	encoded := []byte("ardents-attachment-context-v3\x00")
	encoded = append(encoded, logical[:]...)
	encoded = append(encoded, capsuleDigest[:]...)
	encoded = binary.BigEndian.AppendUint64(encoded, generation)
	return sha256.Sum256(encoded), nil
}
