package wire

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

func continuityNonce(key [32]byte, role Role, generation uint64, exporter [32]byte) [32]byte {
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte("ardents-service-connection-nonce-v3\x00"))
	_, _ = mac.Write([]byte{byte(role)})
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], generation)
	_, _ = mac.Write(raw[:])
	_, _ = mac.Write(exporter[:])
	var nonce [32]byte
	copy(nonce[:], mac.Sum(nil))
	return nonce
}

func continuityMAC(key [32]byte, value Continuity) [32]byte {
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte("ardents-service-connection-continuity-v3\x00"))
	_, _ = mac.Write(continuityTranscript(value))
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

// NewContinuity creates one deterministic local continuity record. The
// exporter commitment has already been derived from this fresh TLS attachment.
func NewContinuity(key [32]byte, role Role, generation, sendBase, sendEnd, receiveNext uint64,
	context, exporter [32]byte) (Continuity, error) {
	if key == [32]byte{} || !validRole(role) || generation == 0 || sendBase > sendEnd || context == [32]byte{} || exporter == [32]byte{} {
		return Continuity{}, errors.New("native continuity input is invalid")
	}
	value := Continuity{Role: role, AttachmentGeneration: generation, SendBase: sendBase, SendEnd: sendEnd,
		ReceiveNext: receiveNext, Context: context, ExporterCommitment: exporter}
	value.Nonce = continuityNonce(key, role, generation, exporter)
	value.MAC = continuityMAC(key, value)
	return value, nil
}

// VerifyContinuity verifies every attachment-bound field and returns an
// active violation rather than a partial peer record.
func VerifyContinuity(key [32]byte, value Continuity, expectedRole Role, generation uint64,
	context, exporter [32]byte) error {
	if key == [32]byte{} || !validRole(expectedRole) || generation == 0 || context == [32]byte{} || exporter == [32]byte{} {
		return errors.New("native continuity verification input is invalid")
	}
	expectedNonce := continuityNonce(key, expectedRole, generation, exporter)
	expectedMAC := continuityMAC(key, value)
	switch {
	case value.Role != expectedRole:
		return errors.New("native continuity role is invalid")
	case value.AttachmentGeneration != generation:
		return errors.New("native continuity generation is invalid")
	case value.SendBase > value.SendEnd:
		return errors.New("native continuity offsets are invalid")
	case value.Context != context:
		return errors.New("native continuity logical context is invalid")
	case value.ExporterCommitment != exporter:
		return errors.New("native continuity Attachment exporter is invalid")
	case !hmac.Equal(value.Nonce[:], expectedNonce[:]):
		return errors.New("native continuity nonce is invalid")
	case !hmac.Equal(value.MAC[:], expectedMAC[:]):
		return errors.New("native continuity authentication is invalid")
	}
	return nil
}

func continuityTranscript(value Continuity) []byte {
	encoded := make([]byte, 0, 1+8*4+32*3)
	encoded = append(encoded, byte(value.Role))
	for _, number := range []uint64{value.AttachmentGeneration, value.SendBase, value.SendEnd, value.ReceiveNext} {
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], number)
		encoded = append(encoded, raw[:]...)
	}
	encoded = append(encoded, value.Nonce[:]...)
	encoded = append(encoded, value.Context[:]...)
	encoded = append(encoded, value.ExporterCommitment[:]...)
	return encoded
}

func validRole(role Role) bool { return role == RoleInitiator || role == RolePublisher }
