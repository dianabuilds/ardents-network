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

type historyLease struct{}

func acquireHistoryLease(string) (historyLease, error) {
	return historyLease{}, errors.New("release: unsupported lease")
}
func (*historyLease) release() error     { return errors.New("release: unsupported lease") }
func durableRename(string, string) error { return errors.New("release: unsupported publication") }
func syncDirectory(string) error         { return errors.New("release: unsupported flush") }
