package endpointstream

import (
	"encoding/binary"
	"errors"
	"unicode/utf8"
)

// PublishByteStreamRequest is the exact Administration upgrade request.
const PublishByteStreamRequest = "stream!!\n"

const (
	localMagic    = "AAI4"
	maximumTarget = 512
)

// DestinationTag identifies the only selected destination form. Name is
// deliberately parsed as a reserved value so it cannot be mistaken for a
// Target Link or gain a fallback route.
type DestinationTag uint8

const (
	Name       DestinationTag = 1
	TargetLink DestinationTag = 2
)

// Request carries one bounded local destination. It carries no Route, peer,
// grant, key, filesystem path, or Application-selected authority.
type Request struct {
	Destination DestinationTag
	Value       string
}

// EncodeRequest returns the canonical request for the selected Application.
func EncodeRequest(request Request) ([]byte, error) {
	if err := validRequest(request); err != nil {
		return nil, err
	}
	magic := localMagic
	raw := make([]byte, len(magic)+1+2+len(request.Value))
	copy(raw, magic)
	raw[len(magic)] = byte(request.Destination)
	binary.BigEndian.PutUint16(raw[len(magic)+1:], uint16(len(request.Value)))
	copy(raw[len(magic)+3:], request.Value)
	return raw, nil
}

func validRequest(request Request) error {
	if (request.Destination != Name && request.Destination != TargetLink) || len(request.Value) == 0 ||
		len(request.Value) > maximumTarget || !utf8.ValidString(request.Value) {
		return errors.New("local Application Connection destination is invalid")
	}
	return nil
}
