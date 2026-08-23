package connection

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

const contextDomain = "ardents-service-connection-context-v1\x00"

// Context derives the one immutable ConnectionContext digest from the exact
// facts selected in ADR-0028.
func Context(input ContextInput) ([32]byte, error) {
	if input.Network == [32]byte{} || input.Target == [32]byte{} || input.InstancePublic == [32]byte{} ||
		input.PublicationDigest == [32]byte{} || input.InstanceGeneration == 0 {
		return [32]byte{}, errors.New("native connection context identity is incomplete")
	}
	deadlines := [3]int64{input.WorkSafetyNotAfter, input.WorkSafetyMaximum, input.NoNewRecoveryAfter}
	if deadlines != [3]int64{} && (input.WorkSafetyNotAfter == 0 || input.WorkSafetyMaximum < input.WorkSafetyNotAfter ||
		input.NoNewRecoveryAfter == 0 || input.NoNewRecoveryAfter > input.WorkSafetyNotAfter) {
		return [32]byte{}, errors.New("native connection context deadlines are invalid")
	}
	encoded := make([]byte, 0, len(contextDomain)+32*7+8+1+len(Profile)+24)
	encoded = append(encoded, contextDomain...)
	for _, value := range [][32]byte{input.Network, input.Target, input.InstancePublic} {
		encoded = append(encoded, value[:]...)
	}
	var generation [8]byte
	binary.BigEndian.PutUint64(generation[:], input.InstanceGeneration)
	encoded = append(encoded, generation[:]...)
	for _, value := range [][32]byte{input.PublicationDigest, input.CandidateView, input.IsolationContext, input.DestinationBinding} {
		encoded = append(encoded, value[:]...)
	}
	encoded = append(encoded, byte(len(Profile)))
	encoded = append(encoded, Profile...)
	for _, deadline := range deadlines {
		var value [8]byte
		binary.BigEndian.PutUint64(value[:], uint64(deadline))
		encoded = append(encoded, value[:]...)
	}
	return sha256.Sum256(encoded), nil
}

func continuityNonce(key [32]byte, role Role, generation uint64, exporter [32]byte) [32]byte {
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte("ardents-service-connection-nonce-v1\x00"))
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
	_, _ = mac.Write([]byte("ardents-service-connection-continuity-v1\x00"))
	_, _ = mac.Write(continuityTranscript(value))
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

// NewContinuity creates one deterministic local continuity record. The
// exporter commitment has already been derived from this fresh TLS attachment.
func NewContinuity(key [32]byte, role Role, generation, sendBase, sendEnd, receiveNext uint64,
	context, exporter [32]byte) (Continuity, error) {
	if !validRole(role) || generation == 0 || sendBase > sendEnd || context == [32]byte{} || exporter == [32]byte{} {
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
	expectedNonce := continuityNonce(key, expectedRole, generation, exporter)
	expectedMAC := continuityMAC(key, value)
	if value.Role != expectedRole || value.AttachmentGeneration != generation || value.SendBase > value.SendEnd ||
		value.Context != context || value.ExporterCommitment != exporter ||
		!hmac.Equal(value.Nonce[:], expectedNonce[:]) || !hmac.Equal(value.MAC[:], expectedMAC[:]) {
		return errors.New("native continuity record is invalid")
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

func validRole(role Role) bool { return role == RoleClient || role == RolePublisher }
