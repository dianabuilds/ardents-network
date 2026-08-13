package servicenegative

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func serveInstanceProof(connection io.ReadWriter, credential serviceconn.Credential,
	private ed25519.PrivateKey) error {
	challenge := make([]byte, 77)
	if _, err := io.ReadFull(connection, challenge); err != nil {
		return err
	}
	if string(challenge[:4]) != "ASCH" || challenge[4] != 1 ||
		binary.BigEndian.Uint64(challenge[37:45]) != credential.Generation {
		return errors.New("instance challenge is invalid")
	}
	proof := make([]byte, 69)
	copy(proof[:4], "ASPR")
	proof[4] = 1
	message := append([]byte("ardents-h3-instance-proof-v1\x00"), challenge...)
	copy(proof[5:], ed25519.Sign(private, message))
	return writeAttackAll(connection, proof)
}

func encodeAttackProof(continuity, exporter [32]byte, credential serviceconn.Credential,
	binding serviceconn.Recovery, encodedGeneration, attachmentGeneration uint64) []byte {
	proof := make([]byte, attackProofSize)
	copy(proof[:4], "ASAT")
	proof[4], proof[5] = 1, 2
	binary.BigEndian.PutUint64(proof[6:14], encodedGeneration)
	nonce := attackNonce(continuity, exporter, attachmentGeneration)
	copy(proof[38:70], nonce[:])
	connection := attackConnectionBinding(credential, binding)
	copy(proof[70:102], connection[:])
	copy(proof[102:134], exporter[:])
	mac := hmac.New(sha256.New, continuity[:])
	_, _ = mac.Write([]byte("ardents-h3-attachment-proof-v1\x00"))
	_, _ = mac.Write(proof[:134])
	copy(proof[134:], mac.Sum(nil))
	return proof
}

func attackNonce(continuity, exporter [32]byte, generation uint64) [32]byte {
	mac := hmac.New(sha256.New, continuity[:])
	_, _ = mac.Write([]byte("ardents-h3-attachment-nonce-v1\x00"))
	_, _ = mac.Write([]byte{2})
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], generation)
	_, _ = mac.Write(encoded[:])
	_, _ = mac.Write(exporter[:])
	var nonce [32]byte
	copy(nonce[:], mac.Sum(nil))
	return nonce
}

func attackConnectionBinding(credential serviceconn.Credential, recovery serviceconn.Recovery) [32]byte {
	value := append([]byte("ardents-h3-connection-binding-v1\x00"), credential.Target[:]...)
	value = append(value, credential.InstancePublic[:]...)
	value = append(value, credential.NetworkID[:]...)
	value = append(value, attackCredentialBody(credential)...)
	value = append(value, recovery.CandidateView[:]...)
	value = append(value, recovery.IsolationContext[:]...)
	value = append(value, recovery.DestinationBinding[:]...)
	value = append(value, byte(len(recovery.RouteProfile)))
	value = append(value, recovery.RouteProfile...)
	for _, bound := range []int64{recovery.WorkSafetyNotAfter, recovery.WorkSafetyMaximum, recovery.NoNewRecoveryAfter} {
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], uint64(bound))
		value = append(value, encoded[:]...)
	}
	return sha256.Sum256(value)
}

func attackCredentialBody(value serviceconn.Credential) []byte {
	encoded := make([]byte, 161)
	copy(encoded[:4], "ASCR")
	encoded[4] = 1
	offset := 5
	for _, field := range [][32]byte{value.AuthorityPublic, value.Target, value.InstancePublic} {
		copy(encoded[offset:offset+32], field[:])
		offset += 32
	}
	for _, number := range []uint64{value.Generation, uint64(value.NotBefore), uint64(value.NotAfter)} {
		binary.BigEndian.PutUint64(encoded[offset:offset+8], number)
		offset += 8
	}
	copy(encoded[offset:offset+32], value.NetworkID[:])
	offset += 32
	binary.BigEndian.PutUint32(encoded[offset:offset+4], value.Capabilities)
	return encoded
}

func writeAttackAll(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		written, err := writer.Write(value)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrNoProgress
		}
		value = value[written:]
	}
	return nil
}
