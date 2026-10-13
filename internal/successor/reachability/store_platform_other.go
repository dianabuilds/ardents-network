//go:build !linux && !windows

package reachability

import (
	"errors"
	"os"
)

const storePlatformSupported = false

func retainStoreSyncDirectory(*os.Root) (*os.File, error) {
	return nil, errors.New("reachability Store platform unavailable")
}

type storeLease struct{}

func (*storeLease) check(os.FileInfo) error {
	return errors.New("reachability Store platform unavailable")
}

func acquireStoreLease(*os.Root) (storeLease, error) {
	return storeLease{}, errors.New("reachability Store platform unavailable")
}
func (*storeLease) release() error    { return nil }
func syncStoreDirectory(string) error { return errors.New("reachability Store platform unavailable") }
