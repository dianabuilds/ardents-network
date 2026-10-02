//go:build !linux

package hosting

import (
	"context"
	"os"
)

type hostingLease struct{}

func hostingPlatform() error { return ErrUnsupportedPlatform }
func acquireHostingLease(context.Context, *os.Root) (*hostingLease, error) {
	return nil, ErrUnsupportedPlatform
}
func (*hostingLease) close() error { return ErrUnsupportedPlatform }
func measureHosting([]string) (hostingReading, error) {
	return hostingReading{}, ErrUnsupportedPlatform
}
