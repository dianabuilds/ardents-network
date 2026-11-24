//go:build linux

package durablefile

import "syscall"

// inodeTimes reads the Linux Stat_t change/modification time field spelling.
func inodeTimes(native *syscall.Stat_t) (ctime, mtime syscall.Timespec) {
	return native.Ctim, native.Mtim
}
