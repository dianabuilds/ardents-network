//go:build !linux

package hosting

import (
	"context"
	"errors"
	"os"
)

var errUnsupportedPlatform = errors.New("hosting is unsupported on this platform")

type hostingLease struct{}

func hostingPlatform() error { return errUnsupportedPlatform }
func acquireHostingLease(context.Context, *os.Root) (*hostingLease, error) {
	return nil, errUnsupportedPlatform
}
func tryAcquireHostingReadLease(context.Context, *os.Root) (*hostingLease, bool, error) {
	return nil, false, errUnsupportedPlatform
}
func tryAcquireHostingWriteLease(context.Context, *os.Root) (*hostingLease, bool, error) {
	return nil, false, errUnsupportedPlatform
}
func (*hostingLease) close() error { return errUnsupportedPlatform }
func measureHosting([]string) (hostingReading, error) {
	return hostingReading{}, errUnsupportedPlatform
}
