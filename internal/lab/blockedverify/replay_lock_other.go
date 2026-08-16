//go:build !linux && !windows

package blockedverify

import "errors"

type registryLease struct{}

func acquireRegistryLock(string) (*registryLease, error) {
	return nil, errors.New("replay registry locking is unsupported on this platform")
}

func (*registryLease) close() error { return nil }
func replaceRegistryFile(string, string) error {
	return errors.New("registry replacement is unsupported")
}
func syncDirectory(string) error {
	return errors.New("registry synchronization is unsupported")
}
