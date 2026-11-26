package issuance

import (
	"encoding/binary"
	"errors"
	"math"
	"time"
)

var (
	ErrInvalid     = errors.New("invalid-input")
	ErrUnavailable = errors.New("storage-unavailable")
	ErrUncertain   = errors.New("storage-uncertain")
	ErrBusy        = errors.New("busy")
	ErrUnsupported = errors.New("unsupported-platform")
	ErrClosed      = errors.New("closed")
)

// Binding consists of caller-selected offline facts, not authenticated State.
type Binding struct {
	Network, Issuer, Signer [32]byte
	Start, End              time.Time
}

func (b Binding) valid() bool {
	start, end := b.Start.Unix(), b.End.Unix()
	return b.Network != [32]byte{} && b.Issuer != [32]byte{} && b.Signer != [32]byte{} &&
		b.Start.Location() == time.UTC && b.End.Location() == time.UTC && b.Start.Nanosecond() == 0 && b.End.Nanosecond() == 0 &&
		start >= 0 && start <= math.MaxInt64-21600 && start%3600 == 0 && end%3600 == 0 && end > start && end-start <= 21600 &&
		time.Unix(start, 0).UTC().Equal(b.Start) && time.Unix(end, 0).UTC().Equal(b.End)
}

func bindingBytes(b Binding) []byte {
	raw := make([]byte, 112)
	copy(raw, b.Network[:])
	copy(raw[32:], b.Issuer[:])
	copy(raw[64:], b.Signer[:])
	binary.BigEndian.PutUint64(raw[96:], uint64(b.Start.Unix()))
	binary.BigEndian.PutUint64(raw[104:], uint64(b.End.Unix()))
	return raw
}
