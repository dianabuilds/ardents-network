package route

import (
	"encoding/binary"
	"errors"
	"time"
)

// ClosedRegistrationRequest contains a channel-owned slot action, never
// Service Authority, a Target or a Publisher callback address.
type ClosedRegistrationRequest struct {
	Nonce, Slot [32]byte
	Revision    uint64
	Expiry      time.Time
	Withdraw    bool
}

func DecodeClosedRegistrationRequest(body []byte) (ClosedRegistrationRequest, error) {
	var request ClosedRegistrationRequest
	if len(body) != closedSmallTerminalOperation || body[0] != 3 && body[0] != 7 {
		return request, errors.New("closed registration operation invalid")
	}
	copy(request.Nonce[:], body[1:33])
	copy(request.Slot[:], body[33:65])
	request.Revision = binary.BigEndian.Uint64(body[65:73])
	request.Withdraw = body[0] == 7
	offset := 73
	if !request.Withdraw {
		seconds := binary.BigEndian.Uint64(body[73:81])
		if seconds == 0 || seconds > 1<<63-1 {
			return request, errors.New("closed registration expiry invalid")
		}
		request.Expiry = time.Unix(int64(seconds), 0).UTC()
		offset = 81
	}
	if request.Nonce == [32]byte{} || request.Slot == [32]byte{} || request.Revision == 0 || !closedDescriptorPadding(body[offset:]) {
		return request, errors.New("closed registration binding or padding invalid")
	}
	return request, nil
}
