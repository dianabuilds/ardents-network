//go:build linux || darwin

package connection

import (
	"os"
	"syscall"
)

// Open without following the final symlink or waiting for a non-regular file.
func openPrivateLinkFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
