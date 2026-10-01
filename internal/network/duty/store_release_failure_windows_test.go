//go:build windows

package duty

import (
	"syscall"
	"testing"
)

func invalidReleaseLease(t *testing.T) rootLease {
	t.Helper()
	return rootLease{handle: syscall.Handle(0xdeadbeef)}
}
