package ardp

import (
	"encoding/binary"
	"errors"
	"time"
)

// JoinRequest carries one channel-local request for an opposite-side match.
// Parsing these bytes grants no admission, pairing or Service authority.
type JoinRequest struct {
	Nonce, Secret, Context [32]byte
	Side                   uint8
	Deadline               time.Time
}

func validJoinRequest(request JoinRequest) bool {
	return request.Nonce != [32]byte{} && request.Secret != [32]byte{} && request.Context != [32]byte{} &&
		(request.Side == 1 || request.Side == 2) && request.Deadline.Unix() > 0 &&
		request.Deadline == request.Deadline.UTC().Truncate(time.Second)
}

// EncodeJoinRequest returns exactly one canonical 4096-byte operation 5.
// The transport owner enforces dedicated lane 1 and the original setup bounds.
func EncodeJoinRequest(request JoinRequest) ([]byte, error) {
	if !validJoinRequest(request) {
		return nil, errors.New("route JOIN request invalid")
	}
	body := make([]byte, 4096)
	body[0] = 5
	copy(body[1:33], request.Nonce[:])
	copy(body[33:65], request.Secret[:])
	body[65] = request.Side
	copy(body[66:98], request.Context[:])
	binary.BigEndian.PutUint64(body[98:106], uint64(request.Deadline.Unix()))
	return body, nil
}

// DecodeJoinRequest accepts only the selected exact grammar and zero padding.
func DecodeJoinRequest(body []byte) (JoinRequest, error) {
	if len(body) != 4096 || body[0] != 5 {
		return JoinRequest{}, errors.New("route JOIN operation invalid")
	}
	request := JoinRequest{Side: body[65]}
	copy(request.Nonce[:], body[1:33])
	copy(request.Secret[:], body[33:65])
	copy(request.Context[:], body[66:98])
	seconds := binary.BigEndian.Uint64(body[98:106])
	if seconds == 0 || seconds > 1<<63-1 {
		return JoinRequest{}, errors.New("route JOIN deadline invalid")
	}
	request.Deadline = time.Unix(int64(seconds), 0).UTC()
	if !validJoinRequest(request) {
		return JoinRequest{}, errors.New("route JOIN binding invalid")
	}
	for _, value := range body[106:] {
		if value != 0 {
			return JoinRequest{}, errors.New("route JOIN padding invalid")
		}
	}
	return request, nil
}

// EncodeJoinResult keeps acceptance and refusal at exactly 16384 bytes.
// Only the receiving pair owner may authorize an accepted result.
func EncodeJoinResult(nonce [32]byte, status uint8) ([]byte, error) {
	if nonce == [32]byte{} || status > 4 {
		return nil, errors.New("route JOIN result invalid")
	}
	body := make([]byte, 16384)
	copy(body[:32], nonce[:])
	body[32] = status
	return body, nil
}

// DecodeJoinResult binds the fixed empty result to this channel's request.
// The caller independently enforces its lane and original live bounds.
func DecodeJoinResult(body []byte, nonce [32]byte) (uint8, error) {
	if len(body) != 16384 || nonce == [32]byte{} || body[32] > 4 {
		return 0, errors.New("route JOIN result invalid")
	}
	var actual [32]byte
	copy(actual[:], body[:32])
	if actual != nonce {
		return 0, errors.New("route JOIN result nonce differs")
	}
	for _, value := range body[33:] {
		if value != 0 {
			return 0, errors.New("route JOIN result payload or padding invalid")
		}
	}
	return body[32], nil
}
