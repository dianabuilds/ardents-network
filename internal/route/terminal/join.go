package terminal

import (
	"encoding/binary"
	"errors"
	"time"
)

// JoinRequest is one channel-local operation. Secret and Context belong
// only to a single Rendezvous pairing, not a Service or issuer identity.
// Deadline limits setup; it never replaces the independently admitted lifetime.
type JoinRequest struct {
	Nonce, Secret, Context [32]byte
	Side                   uint8
	Deadline               time.Time
}

func DecodeJoinRequest(body []byte) (JoinRequest, error) {
	var request JoinRequest
	if len(body) != SmallBodySize || body[0] != 5 || !zeroPadding(body[106:]) {
		return request, errors.New("closed JOIN operation or padding invalid")
	}
	seconds := binary.BigEndian.Uint64(body[98:106])
	if seconds == 0 || seconds > 1<<63-1 {
		return request, errors.New("closed JOIN deadline invalid")
	}
	copy(request.Nonce[:], body[1:33])
	copy(request.Secret[:], body[33:65])
	request.Side = body[65]
	copy(request.Context[:], body[66:98])
	request.Deadline = time.Unix(int64(seconds), 0).UTC()
	if !validJoinRequest(request) {
		return JoinRequest{}, errors.New("closed JOIN binding invalid")
	}
	return request, nil
}

func validJoinRequest(request JoinRequest) bool {
	return request.Nonce != [32]byte{} && request.Secret != [32]byte{} && request.Context != [32]byte{} &&
		(request.Side == 1 || request.Side == 2) && request.Deadline.Unix() > 0 && request.Deadline.Equal(request.Deadline.UTC().Truncate(time.Second))
}

// JOIN results always carry an empty payload, including acceptance. Only the
// pairing owner can select acceptance; a valid codec result is not authority.
func EncodeJoinResult(nonce [32]byte, status uint8) ([]byte, error) {
	if nonce == [32]byte{} || status > 4 {
		return nil, errors.New("closed JOIN result invalid")
	}
	body := make([]byte, BodySize)
	copy(body[:32], nonce[:])
	body[32] = status
	return body, nil
}
