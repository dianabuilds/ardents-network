package node

import (
	"errors"
	"net"
	"sync"
)

type terminalCleanup struct {
	mu  sync.Mutex
	err error
}

func (cleanup *terminalCleanup) record(err error) {
	err = terminalCleanupError(err)
	if err == nil {
		return
	}
	cleanup.mu.Lock()
	if cleanup.err == nil {
		cleanup.err = err
	}
	cleanup.mu.Unlock()
}

func (cleanup *terminalCleanup) result() error {
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	return cleanup.err
}

// terminalCleanupError discards an expected already-closed sentinel without
// hiding another failure joined or wrapped with it.
func terminalCleanupError(err error) error {
	if err == nil || err == net.ErrClosed {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var result error
		for _, part := range joined.Unwrap() {
			result = errors.Join(result, terminalCleanupError(part))
		}
		return result
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok && errors.Is(err, net.ErrClosed) {
		return terminalCleanupError(wrapped.Unwrap())
	}
	return err
}
