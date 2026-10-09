package instance

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

const requestDomain = "ardents-service-instance-request-v3\x00"
const responseDomain = "ardents-service-instance-response-v3\x00"
const requestSize = len(requestDomain) + 112
const credentialSize = 222

var (
	ErrInvalid           = errors.New("service Instance input invalid")
	ErrUnavailable       = errors.New("service Instance unavailable")
	ErrPending           = errors.New("service Instance response pending")
	ErrSuccessorRequired = errors.New("service Instance successor required")
)

// RequestView is copied public approval input, never private Instance authority.
type RequestView struct {
	NetworkID, InstancePublic [32]byte
	NotBefore, NotAfter       int64
	Commitment                [32]byte
}

func encodeRequest(view RequestView) []byte {
	raw := append([]byte(requestDomain), view.NetworkID[:]...)
	raw = append(raw, view.InstancePublic[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(view.NotBefore))
	raw = binary.BigEndian.AppendUint64(raw, uint64(view.NotAfter))
	return append(raw, view.Commitment[:]...)
}

func requestCommitment(view RequestView) [32]byte {
	raw := encodeRequest(view)
	return sha256.Sum256(raw[:len(raw)-32])
}

// ParseRequest accepts exactly the selected v3 public approval grammar.
func ParseRequest(raw []byte) (RequestView, error) {
	if len(raw) != requestSize || string(raw[:len(requestDomain)]) != requestDomain {
		return RequestView{}, ErrInvalid
	}
	var view RequestView
	offset := len(requestDomain)
	copy(view.NetworkID[:], raw[offset:offset+32])
	offset += 32
	copy(view.InstancePublic[:], raw[offset:offset+32])
	offset += 32
	before, after := binary.BigEndian.Uint64(raw[offset:offset+8]), binary.BigEndian.Uint64(raw[offset+8:offset+16])
	offset += 16
	copy(view.Commitment[:], raw[offset:])
	if before > 1<<63-1 || after > 1<<63-1 || before >= after || after-before > 86400 || view.NetworkID == [32]byte{} || view.InstancePublic == [32]byte{} {
		return RequestView{}, ErrInvalid
	}
	view.NotBefore, view.NotAfter = int64(before), int64(after)
	if view.Commitment != requestCommitment(view) {
		return RequestView{}, ErrInvalid
	}
	return view, nil
}

func verifyResponse(raw []byte, request RequestView) (publication.Credential, error) {
	if len(raw) != len(responseDomain)+32+credentialSize || string(raw[:len(responseDomain)]) != responseDomain {
		return publication.Credential{}, ErrInvalid
	}
	var commitment [32]byte
	copy(commitment[:], raw[len(responseDomain):len(responseDomain)+32])
	if commitment == [32]byte{} {
		return publication.Credential{}, ErrInvalid
	}
	credential := raw[len(responseDomain)+32:]
	var authority, network [32]byte
	copy(authority[:], credential[2:34])
	copy(network[:], credential[122:154])
	before := binary.BigEndian.Uint64(credential[106:114])
	if before > 1<<63-1 {
		return publication.Credential{}, ErrInvalid
	}
	verified, err := publication.VerifyCredential(credential, publication.Target(authority), network, time.Unix(int64(before), 0).UTC())
	if err != nil {
		return publication.Credential{}, ErrInvalid
	}
	value := verified.Delegation()
	if commitment != request.Commitment || value.Network != request.NetworkID || value.Instance != request.InstancePublic || value.NotBefore.Unix() != request.NotBefore || value.NotAfter.Unix() != request.NotAfter || binary.BigEndian.Uint32(credential[154:158]) != 3 {
		return publication.Credential{}, ErrUnavailable
	}
	return verified, nil
}
