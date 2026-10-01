package issuer

import (
	"errors"
	"net"
	"sync"
)

// releaseErrors retains the first unexpected child release failure until the
// listener has joined every child. Expected already-closed errors are ignored.
type releaseErrors struct {
	mu  sync.Mutex
	err error
}

func (releases *releaseErrors) record(err error) {
	err = unexpectedReleaseError(err)
	if err == nil {
		return
	}
	releases.mu.Lock()
	if releases.err == nil {
		releases.err = err
	}
	releases.mu.Unlock()
}

func (releases *releaseErrors) result() error {
	releases.mu.Lock()
	defer releases.mu.Unlock()
	return releases.err
}

// unexpectedReleaseError removes an expected already-closed sentinel without
// hiding another failure joined or wrapped with it.
func unexpectedReleaseError(err error) error {
	if err == nil || err == net.ErrClosed {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var result error
		for _, part := range joined.Unwrap() {
			result = errors.Join(result, unexpectedReleaseError(part))
		}
		return result
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok && errors.Is(err, net.ErrClosed) {
		return unexpectedReleaseError(wrapped.Unwrap())
	}
	return err
}
