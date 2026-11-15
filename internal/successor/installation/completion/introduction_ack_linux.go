package completion

import (
	"context"
	"os"
)

// EncodeIntroductionACK preserves the original request and appends the fixed
// first-start byte zero. It grants no retained history or runtime authority.
func EncodeIntroductionACK(request [160]byte) (reply [161]byte, err error) {
	if !canonicalFrame(request) {
		return reply, ErrInput
	}
	copy(reply[:160], request[:])
	reply[160] = 0
	return reply, nil
}

// IntroductionExchange owns a fresh physical exchange created by the common
// native opener. It never accepts the legacy 160-byte response and never borrows
// a Source caller, live exchange or startup permission.
type IntroductionExchange struct{ original *Exchange }

func ConnectIntroduction(ctx context.Context, directory string, expected os.FileInfo, gid uint32, request [160]byte) (*IntroductionExchange, error) {
	physical, err := Connect(ctx, directory, expected, gid, request)
	if err != nil {
		return nil, err
	}
	return &IntroductionExchange{original: physical}, nil
}

// Wait verifies the full first-start reply and original root peer/caller.
// Truncated replies and retained-history selectors refuse.
func (e *IntroductionExchange) Wait() error {
	if e == nil || e.original == nil {
		return ErrInput
	}
	var reply [161]byte
	if err := e.original.exchange(reply[:]); err != nil {
		return err
	}
	if reply[160] != 0 {
		e.original.terminal = ErrBinding
		return ErrBinding
	}
	return nil
}

func (e *IntroductionExchange) Close() error {
	if e == nil {
		return nil
	}
	return e.original.Close()
}
