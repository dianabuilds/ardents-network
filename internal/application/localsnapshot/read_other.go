//go:build !linux && !darwin

package localsnapshot

import (
	"context"
	"errors"
	"io"
)

// Read is unavailable outside the Unix-family (Linux and macOS)
// import boundary; every other platform receives the same explicit refusal.
func Read(context.Context, string) ([]byte, error) {
	return nil, errUnavailable
}

var errUnavailable = errors.New("document snapshot import requires the Unix-family platform boundary")

// CopyVerified refuses before selecting an output on an unsupported host.
func CopyVerified(context.Context, string, int64, func(int64) (io.Writer, error)) error {
	return errUnavailable
}
