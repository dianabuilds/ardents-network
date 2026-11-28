//go:build !linux && !darwin

package localsnapshot

import (
	"context"
	"errors"
)

// Read is unavailable outside the Unix-family (Linux and macOS)
// import boundary; every other platform receives the same explicit refusal.
func Read(context.Context, string) ([]byte, error) {
	return nil, errors.New("document snapshot import requires the Unix-family platform boundary")
}
