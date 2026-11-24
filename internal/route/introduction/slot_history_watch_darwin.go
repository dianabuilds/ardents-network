//go:build darwin

package introduction

import (
	"errors"
	"syscall"
)

// rootWatch retains the kernel's original root movement/loss evidence through
// a kqueue EVFILT_VNODE watch registered on the held directory descriptor
// itself, so a pathname substitution cannot redirect the watch. NOTE_RENAME,
// NOTE_DELETE and NOTE_REVOKE are the macOS equivalents of inotify
// IN_MOVE_SELF, IN_DELETE_SELF and IN_UNMOUNT (rename, deletion and filesystem
// revoke/unmount of the watched vnode). The lawful snapshot replacement changes
// directory contents but is deliberately not watched (no NOTE_WRITE); a queued
// movement event is retained across a move-and-restore transaction and refuses.
type rootWatch struct{ fd int }

func watchRoot(dirfd int) (rootWatch, error) {
	kq, err := syscall.Kqueue()
	if err != nil {
		return rootWatch{}, err
	}
	// kqueue() has no creation flags on macOS; set close-on-exec explicitly to
	// match the Linux IN_CLOEXEC watch descriptor.
	if _, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(kq), syscall.F_SETFD, syscall.FD_CLOEXEC); errno != 0 {
		return rootWatch{}, errors.Join(errno, syscall.Close(kq))
	}
	change := syscall.Kevent_t{
		Ident:  uint64(dirfd),
		Filter: syscall.EVFILT_VNODE,
		Flags:  syscall.EV_ADD | syscall.EV_ENABLE | syscall.EV_CLEAR,
		Fflags: syscall.NOTE_DELETE | syscall.NOTE_RENAME | syscall.NOTE_REVOKE,
	}
	for {
		_, err := syscall.Kevent(kq, []syscall.Kevent_t{change}, nil, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return rootWatch{}, errors.Join(err, syscall.Close(kq))
		}
		break
	}
	return rootWatch{fd: kq}, nil
}

// drain refuses when the kernel queued any original root movement/loss event
// since the last observation, or when the watch descriptor itself was lost.
func (w rootWatch) drain() error {
	events := make([]syscall.Kevent_t, 8)
	var zero syscall.Timespec
	for {
		n, err := syscall.Kevent(w.fd, nil, events, &zero)
		if err == syscall.EINTR {
			// A signal interrupted the non-blocking poll; it is not a root
			// movement or watch loss. Retry the observation.
			continue
		}
		if err != nil {
			return errors.Join(errors.New("introduction original root movement or watch loss"), err)
		}
		if n > 0 {
			return errors.New("introduction original root movement or watch loss")
		}
		return nil
	}
}

func (w rootWatch) close() error { return syscall.Close(w.fd) }
