package completion

import (
	"context"
	"os"
)

// HistoryExchange owns one fresh physical exchange. It accepts only the full
// history reply and does not transfer another invocation or startup authority.
type HistoryExchange struct{ original *Exchange }

func ConnectHistory(ctx context.Context, directory string, expected os.FileInfo, gid uint32, request [160]byte) (*HistoryExchange, error) {
	physical, err := Connect(ctx, directory, expected, gid, request)
	if err != nil {
		return nil, err
	}
	return &HistoryExchange{original: physical}, nil
}

// Wait returns the history byte only after the entire new frame and final
// original-peer/cancellation checks. A truncated legacy reply cannot transfer it.
func (e *HistoryExchange) Wait() (uint8, error) {
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

func (e *HistoryExchange) Close() error {
	if e == nil {
		return nil
	}
	return e.original.Close()
}
