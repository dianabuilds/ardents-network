package introduction

import (
	"encoding/binary"
	"errors"
	"time"
)

// Request identifies one action on an exact channel-owned registration. It
// grants no Service, publication revision or recipient authority.
type Request struct {
	Nonce, Slot [32]byte
	Revision    uint64
	Expiry      time.Time
	Withdraw    bool
}

// EncodeRequest returns the fixed canonical REGISTER or WITHDRAW body.
func EncodeRequest(request Request) ([]byte, error) {
	if request.Nonce == [32]byte{} || request.Slot == [32]byte{} || request.Revision == 0 ||
		(request.Withdraw && !request.Expiry.IsZero()) || (!request.Withdraw && (request.Expiry.Unix() <= 0 || request.Expiry != request.Expiry.UTC().Truncate(time.Second))) {
		return nil, errors.New("introduction request invalid")
	}
	body := make([]byte, 4096)
	body[0] = 3
	if request.Withdraw {
		body[0] = 7
	}
	copy(body[1:33], request.Nonce[:])
	copy(body[33:65], request.Slot[:])
	binary.BigEndian.PutUint64(body[65:73], request.Revision)
	if !request.Withdraw {
		binary.BigEndian.PutUint64(body[73:81], uint64(request.Expiry.Unix()))
	}
	return body, nil
}

// DecodeRequest accepts exactly one fixed operation and canonical zero padding.
// The channel owner independently checks its purpose, lane and nonce freshness.
func DecodeRequest(body []byte) (Request, error) {
	var request Request
	if len(body) != 4096 || (body[0] != 3 && body[0] != 7) {
		return request, errors.New("introduction operation invalid")
	}
	copy(request.Nonce[:], body[1:33])
	copy(request.Slot[:], body[33:65])
	request.Revision = binary.BigEndian.Uint64(body[65:73])
	request.Withdraw = body[0] == 7
	offset := 73
	if !request.Withdraw {
		seconds := binary.BigEndian.Uint64(body[73:81])
		if seconds == 0 || seconds > 1<<63-1 {
			return Request{}, errors.New("introduction expiry invalid")
		}
		request.Expiry = time.Unix(int64(seconds), 0).UTC()
		offset = 81
	}
	if request.Nonce == [32]byte{} || request.Slot == [32]byte{} || request.Revision == 0 || !zeroPadding(body[offset:]) {
		return Request{}, errors.New("introduction binding or padding invalid")
	}
	return request, nil
}

// EncodeResult keeps every registration outcome at 16384 bytes with an empty
// payload. A successful transport slot ACK is not Publication readiness.
func EncodeResult(nonce [32]byte, status uint8) ([]byte, error) {
	if nonce == [32]byte{} || status > 4 {
		return nil, errors.New("introduction result invalid")
	}
	body := make([]byte, 16384)
	copy(body[:32], nonce[:])
	body[32] = status
	return body, nil
}

// DecodeResult binds an empty result to this channel's exact request nonce.
func DecodeResult(body []byte, nonce [32]byte) (uint8, error) {
	if len(body) != 16384 || nonce == [32]byte{} || body[32] > 4 {
		return 0, errors.New("introduction result invalid")
	}
	var actual [32]byte
	copy(actual[:], body[:32])
	if actual != nonce || !zeroPadding(body[33:]) {
		return 0, errors.New("introduction result binding or payload invalid")
	}
	return body[32], nil
}

func zeroPadding(raw []byte) bool {
	for _, value := range raw {
		if value != 0 {
			return false
		}
	}
	return true
}
