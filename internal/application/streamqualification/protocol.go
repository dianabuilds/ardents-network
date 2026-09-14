package streamqualification

import (
	"errors"
	"io"
)

const (
	initMagic  = "ARDTQP01"
	readyMagic = "ARDTQR01"
	initBytes  = len(initMagic) + 1 + 1 + 32 + 32
	readyBytes = len(readyMagic) + 32
)

// Init is the complete fixed worker-launch exchange. The Endpoint creates the
// nonce after the installed unit is verified; the runner supplies neither it
// nor a worker Principal, Grant, destination or executable.
type Init struct {
	Role    Role
	Profile Profile
	Nonce   [32]byte
	Seed    [32]byte
}

// ReadInit receives and validates one exact initialization record.
func ReadInit(reader io.Reader, expected Role) (Init, error) {
	if reader == nil || (expected != ReaderRole && expected != PublisherRole) {
		return Init{}, errors.New("qualification worker initialization is unavailable")
	}
	body := make([]byte, initBytes)
	if _, err := io.ReadFull(reader, body); err != nil {
		return Init{}, errors.New("qualification worker initialization is incomplete")
	}
	if string(body[:len(initMagic)]) != initMagic {
		return Init{}, errors.New("qualification worker initialization magic is invalid")
	}
	init := Init{Role: Role(body[len(initMagic)]), Profile: Profile(body[len(initMagic)+1])}
	copy(init.Nonce[:], body[len(initMagic)+2:])
	copy(init.Seed[:], body[len(initMagic)+2+len(init.Nonce):])
	if init.Role != expected || init.Nonce == [32]byte{} || init.Seed == [32]byte{} {
		return Init{}, errors.New("qualification worker initialization identity is invalid")
	}
	if _, err := init.Profile.Definition(init.Role); err != nil {
		return Init{}, err
	}
	return init, nil
}

// WriteReady acknowledges exactly the verified Endpoint-generated nonce.
func WriteReady(writer io.Writer, nonce [32]byte) error {
	if writer == nil || nonce == [32]byte{} {
		return errors.New("qualification worker readiness is unavailable")
	}
	body := make([]byte, readyBytes)
	copy(body, readyMagic)
	copy(body[len(readyMagic):], nonce[:])
	_, err := writer.Write(body)
	return err
}
