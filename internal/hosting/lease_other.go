//go:build !linux && !darwin

package hosting

import (
	"context"
	"os"
)

type hostingLease struct{}

func acquireHostingLease(context.Context, *os.Root) (*hostingLease, error) {
	return nil, ErrUnsupportedPlatform
}

func (*hostingLease) close() error { return ErrUnsupportedPlatform }
