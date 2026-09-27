package probe

import (
	"errors"
	"net"
	"sync"
)

// cleanupResult retains the first unexpected physical close error for this
// listener while accepting concurrent connection completion.
type cleanupResult struct {
	mu  sync.Mutex
	err error
}

func (cleanup *cleanupResult) record(err error) {
	err = cleanupError(err)
	if err == nil {
		return
	}
	cleanup.mu.Lock()
	if cleanup.err == nil {
		cleanup.err = err
	}
	cleanup.mu.Unlock()
}

func (cleanup *cleanupResult) result() error {
	cleanup.mu.Lock()
	defer cleanup.mu.Unlock()
	return cleanup.err
}

func cleanupError(err error) error {
	if err == nil || err == net.ErrClosed {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var result error
		for _, part := range joined.Unwrap() {
			result = errors.Join(result, cleanupError(part))
		}
		return result
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok && errors.Is(err, net.ErrClosed) {
		return cleanupError(wrapped.Unwrap())
	}
	return err
}
