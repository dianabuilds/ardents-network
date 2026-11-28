package document

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// openPresentationStage retains uncommitted result bytes in an anonymous
// owner-only file. It has no directory entry to recover after process exit.
func openPresentationStage() (*os.File, error) {
	flags := unix.O_TMPFILE | unix.O_EXCL | unix.O_RDWR | unix.O_CLOEXEC
	fd, err := unix.Open(os.TempDir(), flags, 0o600)
	if err != nil {
		return nil, errors.New("private document presentation storage unavailable")
	}
	return os.NewFile(uintptr(fd), "private presentation stage"), nil
}
