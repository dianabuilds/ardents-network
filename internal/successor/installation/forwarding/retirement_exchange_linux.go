package forwarding

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

const retirementPrefix = "ardents-forwarding-domain-joined-v1\n"
const retirementFrameSize = len(retirementPrefix) + 160 + 32

func retirementFrame(start [160]byte, challenge [32]byte) [retirementFrameSize]byte {
	var frame [retirementFrameSize]byte
	copy(frame[:], retirementPrefix)
	copy(frame[len(retirementPrefix):], start[:])
	copy(frame[len(retirementPrefix)+len(start):], challenge[:])
	return frame
}

// Only exact ordinary cancellation leaves and multi-error joins can be clean.
// A single-error physical wrapper is retained even if its cause is cancellation;
// errors.Is(cancel) alone cannot classify actual cleanup as successful.
func joinedCancellation(err error) bool {
	if err == nil || err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if child == nil || !joinedCancellation(child) {
				return false
			}
		}
		return true
	}
	return false
}

// ReportRetirement is one use of this original admitted Startup AFTER every
// input/native observer has physically closed. Command composition must supply
// its complete joined Node/State/Guard/key/output result. This report is not
// native exit, domain authority, a restart proof or an accepting Startup right.
func (h *Startup) ReportRetirement(ctx context.Context, result error) (reported bool, returned error) {
	if h == nil || h.original == nil || ctx == nil {
		return false, ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return false, ErrInput
	}
	o := h.original
	o.mu.Lock()
	if !o.ready || !o.closed || o.reportAttempted || o.closeDone == nil {
		o.mu.Unlock()
		return false, ErrBinding
	}
	select {
	case <-o.closeDone:
	default:
		o.mu.Unlock()
		return false, ErrBinding
	}
	o.reportAttempted = true
	o.mu.Unlock()
	if !joinedCancellation(result) || !joinedCancellation(o.closeErr) {
		return false, ErrBinding
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if o.selection == nil || o.directoryInfo == nil {
		return false, ErrBinding
	}
	b := o.selection.binding
	if os.Geteuid() != int(b.UID) || os.Getegid() != int(b.GID) {
		return false, ErrBinding
	}
	path := o.attempt.retirement(b.Request.InstallationRoot)
	parents := &inputs{ctx: ctx, byPath: map[string]*inputDirectory{}}
	defer func() {
		returned = errors.Join(returned, (Inputs{original: parents}).Close(), ctx.Err())
		if returned != nil {
			reported = false
		}
	}()
	parent, err := parents.parents(path)
	if err != nil {
		return false, err
	}
	originalParent := o.selection.files.dirs[b.Request.InstallationRoot]
	if originalParent == nil || !os.SameFile(originalParent.info, parent.info) {
		return false, ErrBinding
	}
	directory, err := openInputAt(parent, filepath.Base(path), true)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} // ordinary retirement has no explicit root stop endpoint
	if err != nil {
		return false, err
	}
	defer func() {
		returned = errors.Join(returned, directory.Close())
		if returned != nil {
			reported = false
		}
	}()
	info, err := directory.Stat()
	if err != nil || !nodeAccess(info, 0750, 0, b.GID, true) {
		return false, errors.Join(ErrBinding, err)
	}
	pin := &inputDirectory{path: path, name: filepath.Base(path), file: directory, info: info, parent: parent}
	if err := runtimeDirectoryMount(pin, true); err != nil {
		return false, err
	}
	socketPath := filepath.Join(path, "stop-completion.socket")
	socket, err := os.Lstat(socketPath)
	if err != nil || !sameStartSocket(socket, socket, 0660, b.GID) {
		return false, errors.Join(ErrBinding, err)
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	if err != nil {
		return false, err
	}
	defer func() {
		returned = errors.Join(returned, connection.Close())
		if returned != nil {
			reported = false
		}
	}()
	peer, ok := connection.(*net.UnixConn)
	if !ok {
		return false, ErrBinding
	}
	deadline, _ := ctx.Deadline()
	if err := peer.SetDeadline(deadline); err != nil {
		return false, err
	}
	closed := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { closed <- peer.Close() })
	defer func() {
		if !stop() {
			returned = errors.Join(returned, <-closed)
			reported = false
		}
	}()
	check := func() error {
		if err := parents.links(); err != nil {
			return err
		}
		current, err := os.Lstat(path)
		if err != nil || !os.SameFile(info, current) || !nodeAccess(current, 0750, 0, b.GID, true) {
			return errors.Join(ErrBinding, err)
		}
		if err := runtimeDirectoryMount(pin, true); err != nil {
			return err
		}
		named, err := os.Lstat(socketPath)
		if err != nil || !sameStartSocket(socket, named, 0660, b.GID) {
			return errors.Join(ErrBinding, err)
		}
		credentials, err := retirementPeer(peer)
		if err != nil || credentials.Pid <= 0 || credentials.Uid != 0 || credentials.Gid != 0 {
			return errors.Join(ErrBinding, err)
		}
		return ctx.Err()
	}
	if err := check(); err != nil {
		return false, err
	}
	var challenge [32]byte
	if _, err := io.ReadFull(peer, challenge[:]); err != nil || challenge == [32]byte{} {
		return false, errors.Join(ErrBinding, err)
	}
	if err := check(); err != nil {
		return false, err
	}
	frame := retirementFrame(o.frame, challenge)
	count, err := io.Copy(peer, bytes.NewReader(frame[:]))
	if err != nil || count != int64(len(frame)) {
		return false, errors.Join(io.ErrShortWrite, err)
	}
	if err := check(); err != nil {
		return false, err
	}
	return true, nil
}

func retirementPeer(connection *net.UnixConn) (syscall.Ucred, error) {
	if connection == nil {
		return syscall.Ucred{}, ErrInput
	}
	raw, err := connection.SyscallConn()
	if err != nil {
		return syscall.Ucred{}, err
	}
	var peer *syscall.Ucred
	var native error
	err = raw.Control(func(fd uintptr) {
		peer, native = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if err != nil || native != nil || peer == nil {
		return syscall.Ucred{}, errors.Join(ErrBinding, err, native)
	}
	return *peer, nil
}
