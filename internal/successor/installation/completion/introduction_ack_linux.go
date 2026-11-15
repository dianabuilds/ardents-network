package completion

import (
	"context"
	"os"
)

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

// Wait verifies the full reply and original root peer/caller. The returned
// selection grants no authority without the owning Startup admission.
func (e *IntroductionExchange) Wait() (uint8, error) {
	if e == nil || e.original == nil {
		return 0, ErrInput
	}
	var reply [161]byte
	if err := e.original.exchange(reply[:]); err != nil {
		return 0, err
	}
	if reply[160] > 1 {
		e.original.terminal = ErrBinding
		return 0, ErrBinding
	}
	return reply[160], nil
}

func (e *IntroductionExchange) Close() error {
	if e == nil {
		return nil
	}
	return e.original.Close()
}
