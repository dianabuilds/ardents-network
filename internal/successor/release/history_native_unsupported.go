//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package release

import (
	"errors"
	"os"
)

const historyPlatformSupported = false

type historyLease struct{}

func checkHistoryRoot(os.FileInfo) error { return errors.New("release: unsupported history") }
func acquireHistoryLease(string) (historyLease, error) {
	return historyLease{}, errors.New("release: unsupported lease")
}
func (*historyLease) release() error     { return errors.New("release: unsupported lease") }
func durableRename(string, string) error { return errors.New("release: unsupported publication") }
func syncDirectory(string) error         { return errors.New("release: unsupported flush") }
