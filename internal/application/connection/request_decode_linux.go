//go:build linux

package connection

import (
	"encoding/binary"
	"errors"
)

// DecodeRequest accepts exactly one canonical local request. Name is returned
// as a typed request; its receiver must refuse it before any network effect.
func DecodeRequest(raw []byte) (Request, error) {
	const magicSize = len(localMagic)
	if len(raw) < magicSize+3 {
		return Request{}, errors.New("local Application Connection request is invalid")
	}
	var application Application
	switch string(raw[:magicSize]) {
	case localMagic:
		application = Document
	case streamMagic:
		application = ByteStream
	default:
		return Request{}, errors.New("local Application Connection request is invalid")
	}
	length := int(binary.BigEndian.Uint16(raw[magicSize+1:]))
	if length == 0 || length > maximumTarget || len(raw) != magicSize+3+length {
		return Request{}, errors.New("local Application Connection destination is invalid")
	}
	request := Request{Application: application, Destination: DestinationTag(raw[magicSize]), Value: string(raw[magicSize+3:])}
	if err := validRequest(request); err != nil {
		return Request{}, err
	}
	return request, nil
}
