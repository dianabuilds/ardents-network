//go:build linux

package connection

import (
	"encoding/binary"
	"errors"
)

// DecodeRequest accepts exactly one canonical AAI3 request. Name is returned
// as a typed request; its receiver must refuse it before any network effect.
func DecodeRequest(raw []byte) (Request, error) {
	if len(raw) < len(magic)+3 || string(raw[:len(magic)]) != magic {
		return Request{}, errors.New("local text Connection request is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw[len(magic)+1:]))
	if length == 0 || length > maximumTarget || len(raw) != len(magic)+3+length {
		return Request{}, errors.New("local text Connection destination is invalid")
	}
	request := Request{Destination: DestinationTag(raw[len(magic)]), Value: string(raw[len(magic)+3:])}
	if err := validRequest(request); err != nil {
		return Request{}, err
	}
	return request, nil
}
