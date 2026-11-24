//go:build linux

package introduction

import (
	"errors"
	"fmt"
	"syscall"
)

// rootWatch retains the kernel's original root movement/loss evidence through
// an inotify watch on the held directory inode. The fd-anchored procfs path is
// a magic link that resolves to the exact held inode, so a pathname
// substitution cannot redirect the watch. IN_MOVE_SELF, IN_DELETE_SELF and
// IN_UNMOUNT report original move, deletion and filesystem loss; the lawful
// snapshot replacement changes directory metadata but is deliberately not
// watched, and overflow or descriptor loss refuses.
type rootWatch struct{ fd int }

func watchRoot(dirfd int) (rootWatch, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return rootWatch{}, err
	}
	anchored := fmt.Sprintf("/proc/self/fd/%d", dirfd)
	if _, err := syscall.InotifyAddWatch(fd, anchored, syscall.IN_MOVE_SELF|syscall.IN_DELETE_SELF|syscall.IN_UNMOUNT|syscall.IN_ONLYDIR); err != nil {
		return rootWatch{}, errors.Join(err, syscall.Close(fd))
	}
	return rootWatch{fd: fd}, nil
}

// drain refuses when the kernel queued any original root movement/loss event
// since the last observation, or when the watch descriptor itself was lost.
func (w rootWatch) drain() error {
	var events [4096]byte
	n, err := syscall.Read(w.fd, events[:])
	if n > 0 || err != nil && !errors.Is(err, syscall.EAGAIN) {
		return errors.Join(errors.New("introduction original root movement or watch loss"), err)
	}
	return nil
}

func (w rootWatch) close() error { return syscall.Close(w.fd) }
