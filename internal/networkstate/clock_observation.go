package networkstate

import (
	"os"
	"time"
)

func fileClockObserver(path string) func() time.Time {
	return func() time.Time {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return time.Time{}
		}
		return info.ModTime().UTC()
	}
}
