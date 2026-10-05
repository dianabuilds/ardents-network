package transport

// This native control distinguishes Winsock WSAECONNRESET from Go's portable
// errno values. Common error-tree and real adapter tests have no Windows tag;
// their portable reset values cannot prove recognition of this native cause.

import (
	"errors"
	"net"
	"syscall"
	"testing"
)

func TestWinsockRetirementDoesNotClassifyLocalOrMixedFailure(t *testing.T) {
	reset := &net.OpError{Op: "write", Net: "tcp", Err: syscall.WSAECONNRESET}
	if !IsPeerRetirementCause(reset) {
		t.Fatal("native peer reset unavailable to common classification")
	}
	local := &net.OpError{Op: "write", Net: "tcp", Err: net.ErrClosed}
	for _, err := range []error{local, errors.Join(reset, local)} {
		if IsPeerRetirementCause(err) {
			t.Fatal("peer reset hid independent local shutdown", err)
		}
	}
}
