//go:build !linux && !windows

package reachability

import "errors"

const storePlatformSupported = false

type storeLease struct{}

func acquireStoreLease(string) (storeLease, error) {
	return storeLease{}, errors.New("reachability Store platform unavailable")
}
func (*storeLease) release() error    { return nil }
func syncStoreDirectory(string) error { return errors.New("reachability Store platform unavailable") }
