package publication

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const requestDomain = "ardents-service-instance-request-v3\x00"
const responseDomain = "ardents-service-instance-response-v3\x00"
const requestSize = len(requestDomain) + 112

var (
	// ErrRequestInvalid rejects malformed public request or response bytes.
	ErrRequestInvalid = errors.New("service Instance input invalid")
	// ErrResponseMismatch rejects a signed response for different public input.
	ErrResponseMismatch = errors.New("service Instance unavailable")
)

// RequestView is copied public approval input, never private Instance authority.
type RequestView struct {
	NetworkID, InstancePublic [32]byte
	NotBefore, NotAfter       int64
	Commitment                [32]byte
}

// EncodeServiceRequest encodes copied public fields; ParseServiceRequest alone
// validates them. Encoding grants no approval or Instance authority.
func EncodeServiceRequest(view RequestView) []byte {
	raw := append([]byte(requestDomain), view.NetworkID[:]...)
	raw = append(raw, view.InstancePublic[:]...)
	raw = binary.BigEndian.AppendUint64(raw, uint64(view.NotBefore))
	raw = binary.BigEndian.AppendUint64(raw, uint64(view.NotAfter))
	return append(raw, view.Commitment[:]...)
}

// ServiceRequestCommitment hashes the request body before its embedded digest.
// Operator approval instead binds SHA-256 of the entire exact raw request.
func ServiceRequestCommitment(view RequestView) [32]byte {
	raw := EncodeServiceRequest(view)
	return sha256.Sum256(raw[:len(raw)-32])
}

// ParseServiceRequest accepts exactly the selected v3 public approval grammar.
func ParseServiceRequest(raw []byte) (RequestView, error) {
	if len(raw) != requestSize || string(raw[:len(requestDomain)]) != requestDomain {
		return RequestView{}, ErrRequestInvalid
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
		return RequestView{}, ErrRequestInvalid
	}
	view.NotBefore, view.NotAfter = int64(before), int64(after)
	if view.Commitment != ServiceRequestCommitment(view) {
		return RequestView{}, ErrRequestInvalid
	}
	return view, nil
}

// VerifyServiceResponse authenticates the exact public request binding, without
// claiming present validity, private signing authority or accepting readiness.
// The independent live consumer still checks its own time and authority.
func VerifyServiceResponse(raw []byte, request RequestView) (Credential, error) {
	if _, err := ParseServiceRequest(EncodeServiceRequest(request)); err != nil {
		return Credential{}, ErrRequestInvalid
	}
	if len(raw) != len(responseDomain)+32+credentialSize || string(raw[:len(responseDomain)]) != responseDomain {
		return Credential{}, ErrRequestInvalid
	}
	var commitment [32]byte
	copy(commitment[:], raw[len(responseDomain):len(responseDomain)+32])
	if commitment == [32]byte{} {
		return Credential{}, ErrRequestInvalid
	}
	credential := raw[len(responseDomain)+32:]
	var authority, network [32]byte
	copy(authority[:], credential[2:34])
	copy(network[:], credential[122:154])
	before := binary.BigEndian.Uint64(credential[106:114])
	if before > 1<<63-1 {
		return Credential{}, ErrRequestInvalid
	}
	verified, err := VerifyCredential(credential, Target(authority), network, time.Unix(int64(before), 0).UTC())
	if err != nil {
		return Credential{}, ErrRequestInvalid
	}
	value := verified.Delegation()
	if commitment != request.Commitment || value.Network != request.NetworkID || value.Instance != request.InstancePublic || value.NotBefore.Unix() != request.NotBefore || value.NotAfter.Unix() != request.NotAfter || binary.BigEndian.Uint32(credential[154:158]) != 3 {
		return Credential{}, ErrResponseMismatch
	}
	return verified, nil
}

// ServiceCredentialTranscript constructs only the accepted fixed-purpose v3
// public bytes. Custody separately admits approval, validity, generation and
// durable effects before signing; this function exposes no private capability.
func ServiceCredentialTranscript(request RequestView, authority [32]byte, generation uint64) ([]byte, error) {
	if _, err := ParseServiceRequest(EncodeServiceRequest(request)); err != nil || authority == [32]byte{} || generation == 0 {
		return nil, ErrRequestInvalid
	}
	raw := binary.BigEndian.AppendUint16(nil, 3)
	raw = append(raw, authority[:]...)
	target := Target(authority)
	raw = append(raw, target[:]...)
	raw = append(raw, request.InstancePublic[:]...)
	raw = binary.BigEndian.AppendUint64(raw, generation)
	raw = binary.BigEndian.AppendUint64(raw, uint64(request.NotBefore))
	raw = binary.BigEndian.AppendUint64(raw, uint64(request.NotAfter))
	raw = append(raw, request.NetworkID[:]...)
	return binary.BigEndian.AppendUint32(raw, publishCapability|connectCapability), nil
}

// BuildServiceResponse frames an independently signed Credential after exact
// request verification. It never signs or confers present-time authority.
func BuildServiceResponse(requestRaw, credentialRaw []byte) ([]byte, error) {
	request, err := ParseServiceRequest(requestRaw)
	if err != nil {
		return nil, err
	}
	raw := append([]byte(responseDomain), request.Commitment[:]...)
	raw = append(raw, credentialRaw...)
	if _, err := VerifyServiceResponse(raw, request); err != nil {
		return nil, err
	}
	return raw, nil
}
