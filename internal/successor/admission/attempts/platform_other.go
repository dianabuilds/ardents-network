//go:build !linux

package attempts

import (
	"errors"
	"time"
)

// Journal is unavailable without the selected Linux durability implementation.
type Journal struct{}
type Attempt struct {
	Profile, Receiver [32]byte
	Duty              uint64
	Window            time.Time
	Class             uint8
	Nonce             [32]byte
}

func Open(string, [32]byte, func() time.Time) (*Journal, error) {
	return nil, errors.New("presentation journal requires Linux")
}
func (*Journal) Mark([]byte, Attempt) error { return errors.New("presentation journal requires Linux") }
func (*Journal) Close() error               { return nil }
