package completion

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// Exchange retains its own independently opened root and connected socket.
// A matching reply is only the local byte exchange, never runtime authority.
type Exchange struct {
	ctx                   context.Context
	path                  string
	root                  *os.Root
	identity              os.FileInfo
	guard, record, socket os.FileInfo
	connection            *net.UnixConn
	peer                  syscall.Ucred
	gid                   uint32
	frame                 [160]byte
	attempted             bool
	terminal              error
}

func Connect(ctx context.Context, directory string, expected os.FileInfo, gid uint32, frame [160]byte) (result *Exchange, returnedErr error) {
	if ctx == nil || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || directory == "/" || gid == 0 || gid == ^uint32(0) || !rootDirectory(expected, gid) || !canonicalFrame(frame) {
		return nil, ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e := &Exchange{ctx: ctx, path: directory, gid: gid, frame: frame}
	defer func() {
		if returnedErr != nil {
			e.terminal = returnedErr
			returnedErr = e.Close()
			result = nil
		}
	}()
	var err error
	e.root, err = os.OpenRoot(directory)
	if err != nil {
		return nil, err
	}
	e.identity, err = e.root.Stat(".")
	if err != nil || !sameDirectory(expected, e.identity, gid) {
		return nil, errors.Join(ErrBinding, err)
	}
	e.guard, err = e.root.Lstat("start-guard.json")
	if err != nil || !privateRecord(e.guard, 128<<10) {
		return nil, errors.Join(ErrBinding, err)
	}
	e.record, err = e.root.Lstat("start-socket.json")
	if err != nil || !privateRecord(e.record, 4<<10) {
		return nil, errors.Join(ErrBinding, err)
	}
	e.socket, err = e.root.Lstat("start-completion.socket")
	if err != nil || !socketIdentity(e.socket, gid) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := e.observeFiles(); err != nil {
		return nil, err
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", filepath.Join(directory, "start-completion.socket"))
	if err != nil {
		return nil, err
	}
	var ok bool
	e.connection, ok = connection.(*net.UnixConn)
	if !ok {
		return nil, errors.Join(ErrBinding, connection.Close())
	}
	deadline, _ := ctx.Deadline()
	if err := e.connection.SetDeadline(deadline); err != nil {
		return nil, err
	}
	e.peer, err = rootPeer(e.connection)
	if err != nil {
		return nil, err
	}
	if err := e.observeFiles(); err != nil {
		return nil, err
	}
	return e, ctx.Err()
}

// Wait sends and receives the single exact 160-byte frame under the opening's
// original absolute deadline. Root may retire its private guard/socket after
// sending the reply; subsequent runtime admission belongs to Installation.
func (e *Exchange) Wait() (returnedErr error) {
	if e == nil || e.ctx == nil || e.root == nil || e.connection == nil || e.attempted {
		return ErrInput
	}
	if e.terminal != nil {
		return e.terminal
	}
	e.attempted = true
	defer func() {
		returnedErr = errors.Join(returnedErr, e.ctx.Err())
		if returnedErr != nil {
			e.terminal = returnedErr
		}
	}()
	if err := e.observeFiles(); err != nil {
		return err
	}
	if err := e.observePeer(); err != nil {
		return err
	}
	// The original connection remains owned even after cancellation. A physical
	// Close interrupts blocked I/O; joining this callback retains its actual error.
	connection := e.connection
	closed := make(chan error, 1)
	stop := context.AfterFunc(e.ctx, func() { closed <- connection.Close() })
	defer func() {
		if !stop() {
			returnedErr = errors.Join(returnedErr, <-closed)
		}
	}()
	if err := e.ctx.Err(); err != nil {
		return err
	}
	if _, err := io.Copy(connection, bytes.NewReader(e.frame[:])); err != nil {
		return err
	}
	var reply [160]byte
	if _, err := io.ReadFull(connection, reply[:]); err != nil {
		return err
	}
	if reply != e.frame {
		return ErrBinding
	}
	if err := e.observePeer(); err != nil {
		return err
	}
	return e.ctx.Err()
}

func (e *Exchange) observeFiles() error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	current, err := e.root.Stat(".")
	path, pathErr := os.Lstat(e.path)
	if err != nil || pathErr != nil || !sameDirectory(e.identity, current, e.gid) || !sameDirectory(e.identity, path, e.gid) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	for _, entry := range []struct {
		name     string
		original os.FileInfo
		maximum  int64
	}{
		{"start-guard.json", e.guard, 128 << 10}, {"start-socket.json", e.record, 4 << 10},
	} {
		current, err := e.root.Lstat(entry.name)
		if err != nil || !privateRecord(current, entry.maximum) || !sameRecord(entry.original, current) {
			return errors.Join(ErrBinding, err)
		}
	}
	socket, err := e.root.Lstat("start-completion.socket")
	if err != nil || !socketIdentity(socket, e.gid) || !sameRecord(e.socket, socket) {
		return errors.Join(ErrBinding, err)
	}
	return e.ctx.Err()
}

func (e *Exchange) observePeer() error {
	peer, err := rootPeer(e.connection)
	if err != nil || peer != e.peer {
		return errors.Join(ErrBinding, err)
	}
	return e.ctx.Err()
}

func (e *Exchange) Close() error {
	if e == nil {
		return nil
	}
	if e.connection != nil {
		err := e.connection.Close()
		if !errors.Is(err, net.ErrClosed) {
			e.terminal = errors.Join(e.terminal, err)
		}
		e.connection = nil
	}
	if e.root != nil {
		e.terminal = errors.Join(e.terminal, e.root.Close())
		e.root = nil
	}
	return e.terminal
}

func rootPeer(connection *net.UnixConn) (syscall.Ucred, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return syscall.Ucred{}, err
	}
	var peer *syscall.Ucred
	var peerErr error
	err = raw.Control(func(fd uintptr) {
		peer, peerErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil || peerErr != nil || peer == nil || peer.Pid <= 0 || peer.Uid != 0 || peer.Gid != 0 {
		return syscall.Ucred{}, errors.Join(ErrBinding, err, peerErr)
	}
	return *peer, nil
}

func rootDirectory(info os.FileInfo, gid uint32) bool {
	if info == nil || info.Mode() != os.ModeDir|0750 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == gid && native.Dev != 0 && native.Ino != 0
}

func sameDirectory(original, current os.FileInfo, gid uint32) bool {
	return rootDirectory(original, gid) && rootDirectory(current, gid) && os.SameFile(original, current)
}

func privateRecord(info os.FileInfo, maximum int64) bool {
	if info == nil || info.Mode() != 0600 || info.Size() <= 0 || info.Size() > maximum {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == 0 && native.Nlink == 1
}

func sameRecord(original, current os.FileInfo) bool {
	if original == nil || current == nil || !os.SameFile(original, current) || original.Size() != current.Size() || !original.ModTime().Equal(current.ModTime()) {
		return false
	}
	a, aOK := original.Sys().(*syscall.Stat_t)
	b, bOK := current.Sys().(*syscall.Stat_t)
	return aOK && bOK && a.Ctim == b.Ctim
}

func socketIdentity(info os.FileInfo, gid uint32) bool {
	if info == nil || info.Mode() != os.ModeSocket|0660 {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Uid == 0 && native.Gid == gid && native.Nlink == 1 && native.Dev != 0 && native.Ino != 0
}
