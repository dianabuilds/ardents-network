//go:build darwin

package durablefile

import "syscall"

// inodeTimes reads the macOS Stat_t change/modification time field spelling.
func inodeTimes(native *syscall.Stat_t) (ctime, mtime syscall.Timespec) {
	return native.Ctimespec, native.Mtimespec
}
