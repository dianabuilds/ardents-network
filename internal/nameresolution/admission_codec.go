package nameresolution

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/nameadmission"
	"github.com/dianabuilds/ardents-network/internal/naming"
)

func encodeAdmissionProof(proof nameadmission.Proof) ([]byte, error) {
	challenge := proof.Challenge
	if challenge.Node == [32]byte{} || challenge.Network == [32]byte{} || challenge.Epoch == 0 ||
		challenge.Surface != "resolution" || len(challenge.Surface) > 255 || challenge.OperationDigest == [32]byte{} ||
		challenge.IsolationBinding == [32]byte{} || challenge.IssuedAt < 0 || challenge.ExpiresAt <= challenge.IssuedAt ||
		challenge.Nonce == [16]byte{} || challenge.WorkBits != 16 || challenge.AuthenticationTag == [32]byte{} {
		return nil, errors.New("resolution admission proof is invalid")
	}
	out := append([]byte(nil), challenge.Node[:]...)
	out = append(out, challenge.Network[:]...)
	out = binary.BigEndian.AppendUint64(out, challenge.Epoch)
	out = append(out, byte(len(challenge.Surface)))
	out = append(out, challenge.Surface...)
	out = append(out, challenge.OperationDigest[:]...)
	out = append(out, challenge.IsolationBinding[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.IssuedAt))
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.ExpiresAt))
	out = append(out, challenge.Nonce[:]...)
	out = append(out, challenge.WorkBits)
	out = append(out, challenge.AuthenticationTag[:]...)
	return binary.BigEndian.AppendUint64(out, proof.WorkNonce), nil
}

func resolutionAdmissionDigest(network [32]byte, name string, deadline int64) ([32]byte, error) {
	parsed, err := naming.Parse(name)
	if err != nil {
		return [32]byte{}, err
	}
	wire, err := naming.EncodeWire(parsed)
	if err != nil {
		return [32]byte{}, err
	}
	out := []byte("ardents-name-resolution-operation-v1\x00")
	out = append(out, network[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(deadline))
	out = binary.BigEndian.AppendUint16(out, uint16(len(wire)))
	out = append(out, wire...)
	return sha256.Sum256(out), nil
}

func decodeAdmissionProof(raw []byte) (nameadmission.Proof, error) {
	const fixed = 32 + 32 + 8 + 1 + 32 + 32 + 8 + 8 + 16 + 1 + 32 + 8
	if len(raw) < fixed {
		return nameadmission.Proof{}, errors.New("resolution admission proof is truncated")
	}
	var proof nameadmission.Proof
	offset := 0
	copy(proof.Challenge.Node[:], raw[offset:offset+32])
	offset += 32
	copy(proof.Challenge.Network[:], raw[offset:offset+32])
	offset += 32
	proof.Challenge.Epoch = binary.BigEndian.Uint64(raw[offset:])
	offset += 8
	surfaceSize := int(raw[offset])
	offset++
	if surfaceSize == 0 || len(raw) != fixed+surfaceSize {
		return nameadmission.Proof{}, errors.New("resolution admission surface is malformed")
	}
	proof.Challenge.Surface = string(raw[offset : offset+surfaceSize])
	offset += surfaceSize
	copy(proof.Challenge.OperationDigest[:], raw[offset:offset+32])
	offset += 32
	copy(proof.Challenge.IsolationBinding[:], raw[offset:offset+32])
	offset += 32
	proof.Challenge.IssuedAt = int64(binary.BigEndian.Uint64(raw[offset:]))
	offset += 8
	proof.Challenge.ExpiresAt = int64(binary.BigEndian.Uint64(raw[offset:]))
	offset += 8
	copy(proof.Challenge.Nonce[:], raw[offset:offset+16])
	offset += 16
	proof.Challenge.WorkBits = raw[offset]
	offset++
	copy(proof.Challenge.AuthenticationTag[:], raw[offset:offset+32])
	offset += 32
	proof.WorkNonce = binary.BigEndian.Uint64(raw[offset:])
	if canonical, err := encodeAdmissionProof(proof); err != nil || string(canonical) != string(raw) {
		return nameadmission.Proof{}, errors.New("resolution admission proof is not canonical")
	}
	return proof, nil
}
