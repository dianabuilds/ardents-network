package socketfile

import (
	"errors"
	"net"
	"os"
	"path/filepath"
)

var (
	ErrOccupied = errors.New("local Application socket path already exists")
	ErrChanged  = errors.New("original local Application socket entry changed")
)

// Entry retains the parent and identity of one bound Unix socket.
type Entry struct {
	parent   *os.Root
	name     string
	identity os.FileInfo
}

// Listen binds a new socket and records its original entry before handing
// the listener to its owner.
func Listen(path string) (_ *net.UnixListener, _ *Entry, result error) {
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, parent.Close())
		}
	}()

	name := filepath.Base(path)
	if _, err := parent.Lstat(name); !errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrOccupied
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, nil, err
	}
	listener.SetUnlinkOnClose(false)
	identity, err := parent.Lstat(name)
	if err != nil || identity.Mode().Type() != os.ModeSocket {
		return nil, nil, errors.Join(ErrChanged, err, listener.Close())
	}
	entry := &Entry{parent: parent, name: name, identity: identity}
	if err := parent.Chmod(name, 0600); err != nil {
		return nil, nil, errors.Join(err, listener.Close(), entry.remove())
	}
	if err := entry.check(); err != nil {
		return nil, nil, errors.Join(err, listener.Close())
	}
	return listener, entry, nil
}

func (entry *Entry) check() error {
	now, err := entry.parent.Lstat(entry.name)
	if err != nil {
		return err
	}
	if now.Mode().Type() != os.ModeSocket || !os.SameFile(entry.identity, now) {
		return ErrChanged
	}
	return nil
}

func (entry *Entry) remove() error {
	if err := entry.check(); err != nil {
		return err
	}
	return entry.parent.Remove(entry.name)
}

// Retire removes the original entry after its owner joins the listener and
// its accepted clients. A changed entry remains in place.
func (entry *Entry) Retire() error {
	return errors.Join(entry.remove(), entry.parent.Close())
}
