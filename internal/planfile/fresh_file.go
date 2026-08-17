package planfile

import (
	"os"
	"time"
)

// FreshRegular returns a fail-closed observation of one recent regular file.
func FreshRegular(path string, clock func() time.Time, maximumAge time.Duration) func() bool {
	return func() bool {
		if path == "" || clock == nil || maximumAge <= 0 {
			return false
		}
		info, err := os.Lstat(path)
		return err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 &&
			clock().UTC().Sub(info.ModTime().UTC()).Abs() <= maximumAge
	}
}
