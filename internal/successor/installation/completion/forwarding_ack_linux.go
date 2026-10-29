package completion

import (
	"context"
	"os"
)

// EncodeForwardingACK preserves the original request frame and adds one exact
// history-opening byte. The caller must independently prove that choice; this
// codec supplies no semantic root, original history or runtime authority.
func EncodeForwardingACK(request [160]byte, history uint8) (reply [161]byte, err error) {
	if !canonicalFrame(request) || history > 1 {
		return reply, ErrInput
	}
	copy(reply[:160], request[:])
	reply[160] = history
	return reply, nil
}

// ForwardingExchange owns a fresh physical exchange created by the common
// native opener. It never accepts the legacy 160-byte response and never borrows
// a Source caller, live exchange or startup permission.
type ForwardingExchange struct{ original *Exchange }

func ConnectForwarding(ctx context.Context, directory string, expected os.FileInfo, gid uint32, request [160]byte) (*ForwardingExchange, error) {
	physical, err := Connect(ctx, directory, expected, gid, request)
	if err != nil {
		return nil, err
	}
	return &ForwardingExchange{original: physical}, nil
}

// Wait returns the history byte only after the entire new frame and final
// original-peer/cancellation checks. A truncated legacy reply cannot transfer it.
func (e *ForwardingExchange) Wait() (uint8, error) {
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

func (e *ForwardingExchange) Close() error {
	if e == nil {
		return nil
	}
	return e.original.Close()
}
