//go:build !windows

package durable

import (
	"os"
	"testing"
)

func invalidReleaseLease(t *testing.T) rootLease {
	t.Helper()
	file := os.NewFile(uintptr(1<<30), "invalid state-root lease")
	if file == nil {
		t.Fatal("could not construct an invalid lease file")
	}
	return rootLease{file: file}
}
