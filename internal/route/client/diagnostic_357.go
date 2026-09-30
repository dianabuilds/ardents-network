//go:build linux

package client

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
)

var diagnostic357Counter atomic.Uint64

// Temporary observation only: first committed parent terminal under owner.mu.
func (owner *closedSourceChannels) diagnostic357Terminal(err error) {
	timedOut := false
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) {
		timedOut = timeout.Timeout()
	}
	fmt.Fprintf(os.Stderr, "[DEBUG-357-source] at=%s id=%d boundary=parent-terminal eof=%t timeout=%t stopped=%t cleanup=%t joined=%t lanes=%d\n", time.Now().UTC().Format(time.RFC3339Nano), owner.diagnostic357ID, errors.Is(err, io.EOF), timedOut, errors.Is(err, ErrClosedSourceStopped), errors.Is(err, ErrClosedSourceCleanup), owner.retainClosedRead, len(owner.lanes))
}
func (prefix *ClosedSourcePrefix) Diagnostic357ID() uint64 {
	if prefix == nil || prefix.channels == nil {
		return 0
	}
	return prefix.channels.diagnostic357ID
}
