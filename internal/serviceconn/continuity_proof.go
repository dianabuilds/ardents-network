package serviceconn

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

const continuityProofSize = 4 + 1 + 1 + 8 + 8 + 8 + 8 + 32 + 32 + 32 + 32

var errActiveViolation = errors.New("detected Service Connection integrity violation")

type continuityState struct {
	sendBase uint64
	sendEnd  uint64
	recvNext uint64
}

type peerContinuity struct {
	sendBase   uint64
	sendEnd    uint64
	recvNext   uint64
	peerNonce  [32]byte
	localNonce [32]byte
}

func exchangeContinuityProof(ctx context.Context, attachment *securedAttachment, continuity [32]byte,
	credential Credential, binding Recovery, client bool, state continuityState) (peerContinuity, error) {
	deadline := time.Now().Add(15 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	_ = attachment.connection.SetDeadline(deadline)
	defer attachment.connection.SetDeadline(time.Time{})
	local, err := encodeContinuityProof(continuity, credential, binding, attachment, client, state)
	if err != nil {
		return peerContinuity{}, err
	}
	peer := make([]byte, continuityProofSize)
	if client {
		if err := writeAll(attachment.connection, local); err != nil {
			return peerContinuity{}, err
		}
		if _, err := io.ReadFull(attachment.connection, peer); err != nil {
			return peerContinuity{}, err
		}
	} else {
		if _, err := io.ReadFull(attachment.connection, peer); err != nil {
			return peerContinuity{}, err
		}
		if err := writeAll(attachment.connection, local); err != nil {
			return peerContinuity{}, err
		}
	}
	result, err := decodeContinuityProof(peer, continuity, credential, binding, attachment, !client)
	copy(result.localNonce[:], local[38:70])
	return result, err
}

func encodeContinuityProof(continuity [32]byte, credential Credential, binding Recovery,
	attachment *securedAttachment, client bool, state continuityState) ([]byte, error) {
	nonce := proofNonce(continuity, attachment, client)
	return encodeContinuityProofNonce(continuity, credential, binding, attachment, client, state, nonce), nil
}

func proofNonce(continuity [32]byte, attachment *securedAttachment, client bool) [32]byte {
	mac := hmac.New(sha256.New, continuity[:])
	_, _ = mac.Write([]byte("ardents-h3-attachment-nonce-v1\x00"))
	role := byte(2)
	if client {
		role = 1
	}
	_, _ = mac.Write([]byte{role})
	var generation [8]byte
	binary.BigEndian.PutUint64(generation[:], attachment.generation)
	_, _ = mac.Write(generation[:])
	_, _ = mac.Write(attachment.exporterCommitment[:])
	var nonce [32]byte
	copy(nonce[:], mac.Sum(nil))
	return nonce
}

func encodeContinuityProofNonce(continuity [32]byte, credential Credential, binding Recovery,
	attachment *securedAttachment, client bool, state continuityState, nonce [32]byte) []byte {
	encoded := make([]byte, continuityProofSize)
	copy(encoded[:4], "ASAT")
	encoded[4] = 1
	if client {
		encoded[5] = 1
	} else {
		encoded[5] = 2
	}
	binary.BigEndian.PutUint64(encoded[6:14], attachment.generation)
	binary.BigEndian.PutUint64(encoded[14:22], state.sendBase)
	binary.BigEndian.PutUint64(encoded[22:30], state.sendEnd)
	binary.BigEndian.PutUint64(encoded[30:38], state.recvNext)
	copy(encoded[38:70], nonce[:])
	connection := connectionBinding(credential, binding)
	copy(encoded[70:102], connection[:])
	copy(encoded[102:134], attachment.exporterCommitment[:])
	mac := hmac.New(sha256.New, continuity[:])
	_, _ = mac.Write([]byte("ardents-h3-attachment-proof-v1\x00"))
	_, _ = mac.Write(encoded[:134])
	copy(encoded[134:], mac.Sum(nil))
	return encoded
}

func decodeContinuityProof(encoded []byte, continuity [32]byte, credential Credential, recovery Recovery,
	attachment *securedAttachment, expectClient bool) (peerContinuity, error) {
	if len(encoded) != continuityProofSize {
		return peerContinuity{}, errActiveViolation
	}
	expectedRole := byte(2)
	if expectClient {
		expectedRole = 1
	}
	binding := connectionBinding(credential, recovery)
	mac := hmac.New(sha256.New, continuity[:])
	_, _ = mac.Write([]byte("ardents-h3-attachment-proof-v1\x00"))
	_, _ = mac.Write(encoded[:134])
	if string(encoded[:4]) != "ASAT" || encoded[4] != 1 ||
		encoded[5] != expectedRole || binary.BigEndian.Uint64(encoded[6:14]) != attachment.generation ||
		!hmac.Equal(encoded[70:102], binding[:]) ||
		!hmac.Equal(encoded[102:134], attachment.exporterCommitment[:]) ||
		!hmac.Equal(encoded[134:], mac.Sum(nil)) {
		return peerContinuity{}, errActiveViolation
	}
	expectedNonce := proofNonce(continuity, attachment, expectClient)
	if !hmac.Equal(encoded[38:70], expectedNonce[:]) {
		return peerContinuity{}, errActiveViolation
	}
	peer := peerContinuity{sendBase: binary.BigEndian.Uint64(encoded[14:22]),
		sendEnd: binary.BigEndian.Uint64(encoded[22:30]), recvNext: binary.BigEndian.Uint64(encoded[30:38])}
	copy(peer.peerNonce[:], encoded[38:70])
	if peer.sendBase > peer.sendEnd {
		return peerContinuity{}, errActiveViolation
	}
	return peer, nil
}
