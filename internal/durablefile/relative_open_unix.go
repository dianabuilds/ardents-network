//go:build linux || darwin

package durablefile

import "golang.org/x/sys/unix"

// Openat opens one entry relative to a retained directory descriptor without
// following a final symlink when the caller supplies O_NOFOLLOW. It is the
// Unix-family (Linux and macOS) spelling of the directory-relative open the
// durable-store inspection idiom uses; the caller keeps every flag decision
// (O_NOFOLLOW, O_CLOEXEC, O_DIRECTORY, O_NONBLOCK) and owns the returned
// descriptor. The reviewed x/sys/unix dependency is admitted only here.
func Openat(dirfd int, name string, flags int, mode uint32) (int, error) {
	return unix.Openat(dirfd, name, flags, mode)
}
