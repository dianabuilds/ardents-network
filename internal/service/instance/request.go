package instance

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

const requestDomain = "ardents-service-instance-request-v1\x00"

const requestSize = len(requestDomain) + 32 + 32 + 32 + 8 + 8 + 32

func encodeRequest(view RequestView) []byte {
	raw := make([]byte, requestSize)
	offset := copy(raw, requestDomain)
	offset += copy(raw[offset:], view.NetworkID[:])
	offset += copy(raw[offset:], view.InstancePublic[:])
	offset += copy(raw[offset:], view.IntroductionPublic[:])
	binary.BigEndian.PutUint64(raw[offset:offset+8], uint64(view.NotBefore))
	offset += 8
	binary.BigEndian.PutUint64(raw[offset:offset+8], uint64(view.NotAfter))
	offset += 8
	copy(raw[offset:], view.Commitment[:])
	return raw
}

func requestCommitment(view RequestView) [32]byte {
	view.Commitment = [32]byte{}
	raw := encodeRequest(view)
	return sha256.Sum256(raw[:len(raw)-32])
}

// ParseRequest accepts only the one fixed canonical request grammar.
func ParseRequest(raw []byte) (RequestView, error) {
	if len(raw) != requestSize || !bytes.Equal(raw[:len(requestDomain)], []byte(requestDomain)) {
		return RequestView{}, ErrInvalid
	}
	var view RequestView
	offset := len(requestDomain)
	offset += copy(view.NetworkID[:], raw[offset:offset+32])
	offset += copy(view.InstancePublic[:], raw[offset:offset+32])
	offset += copy(view.IntroductionPublic[:], raw[offset:offset+32])
	view.NotBefore = int64(binary.BigEndian.Uint64(raw[offset : offset+8]))
	offset += 8
	view.NotAfter = int64(binary.BigEndian.Uint64(raw[offset : offset+8]))
	offset += 8
	copy(view.Commitment[:], raw[offset:offset+32])
	if view.NetworkID == ([32]byte{}) || view.InstancePublic == ([32]byte{}) ||
		view.IntroductionPublic == ([32]byte{}) || view.NotAfter <= view.NotBefore ||
		view.Commitment == ([32]byte{}) || view.Commitment != requestCommitment(view) {
		return RequestView{}, ErrInvalid
	}
	return view, nil
}
