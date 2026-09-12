//go:build linux

package route

import (
	"encoding/binary"
	"errors"
	"time"
)

func EncodeClosedRegistrationRequest(request ClosedRegistrationRequest) ([]byte, error) {
	if request.Nonce == [32]byte{} || request.Slot == [32]byte{} || request.Revision == 0 ||
		request.Withdraw && !request.Expiry.IsZero() || !request.Withdraw && (request.Expiry.Unix() <= 0 || !request.Expiry.Equal(request.Expiry.UTC().Truncate(time.Second))) {
		return nil, errors.New("closed registration request invalid")
	}
	body := make([]byte, closedSmallTerminalOperation)
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
