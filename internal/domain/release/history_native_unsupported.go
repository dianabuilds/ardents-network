//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package release

import (
	"errors"
	"os"
)

const historyPlatformSupported = false

type historyRoot struct{}

func openHistoryRoot(string, bool) (*historyRoot, error) { return nil, ErrTrustUnavailable }
func (*historyRoot) check(os.FileInfo) error             { return ErrTrustUnavailable }
func (*historyRoot) release() error                      { return nil }
func checkHistoryObject(*os.File, bool) error            { return ErrTrustUnavailable }

// New history objects use the host's private creation semantics.
func createHistoryFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}
func createHistoryDirectory(path string) error { return os.Mkdir(path, 0700) }
func createHistoryTempDirectory(parent, prefix string) (string, error) {
	return os.MkdirTemp(parent, prefix)
}
func createHistoryTempFile(parent, prefix string) (*os.File, error) {
	return os.CreateTemp(parent, prefix)
}

type historyLease struct{}

func acquireHistoryLease(string) (historyLease, error) {
	return historyLease{}, errors.New("release: unsupported lease")
}
func (*historyLease) release() error     { return errors.New("release: unsupported lease") }
func durableRename(string, string) error { return errors.New("release: unsupported publication") }
func syncDirectory(string) error         { return errors.New("release: unsupported flush") }
