//go:build !linux

package resource

import (
	"context"
	"os"
)

type hostingLease struct{}

func hostingPlatform() error { return errUnsupportedPlatform }
func acquireHostingLease(context.Context, *os.Root) (*hostingLease, error) {
	return nil, errUnsupportedPlatform
}
func tryAcquireHostingReadLease(context.Context, *os.Root) (*hostingLease, bool, error) {
	return nil, false, errUnsupportedPlatform
}
func (*hostingLease) close() error { return errUnsupportedPlatform }
func measureHosting([]string) (hostingReading, error) {
	return hostingReading{}, errUnsupportedPlatform
}
