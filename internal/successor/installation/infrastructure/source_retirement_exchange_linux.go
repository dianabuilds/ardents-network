package infrastructure

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

const sourceRetirementPrefix = "ardents-source-domain-joined-v1\n"
const sourceRetirementFrameSize = len(sourceRetirementPrefix) + 160 + 32

func sourceRetirementFrame(start [160]byte, challenge [32]byte) [sourceRetirementFrameSize]byte {
	var frame [sourceRetirementFrameSize]byte
	copy(frame[:], sourceRetirementPrefix)
	copy(frame[len(sourceRetirementPrefix):], start[:])
	copy(frame[len(sourceRetirementPrefix)+len(start):], challenge[:])
	return frame
}

// ReportRetirement is a one-use completion report from the original opaque
// Startup after every Startup input/observer has physically closed. Composition
// must additionally supply its complete joined State/Guard/key/output result.
// It cannot reconstruct a live right or accept a restart. Root independently
// checks the original peer, fresh challenge, process exit and kernel-scope join.
// No explicit stop endpoint means ordinary retirement, with no restart proof.
func (s *SourceStartup) ReportRetirement(ctx context.Context, result error) (reported bool, err error) {
	if s == nil || s.original == nil || ctx == nil {
		return false, ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return false, ErrInput
	}
	o := s.original
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
	if !sourceJoinedCancellation(result) || !sourceJoinedCancellation(o.closeErr) {
		return false, ErrBinding
	}
	if err = ctx.Err(); err != nil {
		return false, err
	}
	if o.completionDirectoryInfo == nil || o.selection == nil {
		return false, ErrBinding
	}
	gid := o.selection.binding.GID
	directory, original, err := openSourceDirectory(o.completionDirectory, 0750, 0, gid)
	if err != nil {
		return false, err
	}
	defer func() {
		err = errors.Join(err, directory.Close(), ctx.Err())
		if err != nil {
			reported = false
		}
	}()
	if !os.SameFile(original, o.completionDirectoryInfo) {
		return false, ErrBinding
	}
	path := filepath.Join(o.completionDirectory, "stop-completion.socket")
	socket, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || !sameSourceSocket(socket, socket, 0660, gid) {
		return false, errors.Join(ErrBinding, err)
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return false, err
	}
	defer func() {
		err = errors.Join(err, connection.Close())
		if err != nil {
			reported = false
		}
	}()
	closed := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { closed <- connection.Close() })
	defer func() {
		if !stop() {
			err = errors.Join(err, <-closed)
			reported = false
		}
	}()
	peer, ok := connection.(*net.UnixConn)
	if !ok {
		return false, ErrBinding
	}
	deadline, _ := ctx.Deadline()
	if err = peer.SetDeadline(deadline); err != nil {
		return false, err
	}
	check := func() error {
		if err := observeSourceDirectory(o.completionDirectory, directory, original); err != nil {
			return err
		}
		named, err := os.Lstat(path)
		if err != nil || !sameSourceSocket(socket, named, 0660, gid) {
			return errors.Join(ErrBinding, err)
		}
		credentials, err := sourceUnixPeer(peer)
		if err != nil || credentials.Uid != 0 || credentials.Gid != 0 || credentials.Pid <= 0 {
			return errors.Join(ErrBinding, err)
		}
		return ctx.Err()
	}
	if err = check(); err != nil {
		return false, err
	}
	var challenge [32]byte
	if _, err = io.ReadFull(peer, challenge[:]); err != nil || challenge == [32]byte{} {
		return false, errors.Join(ErrBinding, err)
	}
	if err = check(); err != nil {
		return false, err
	}
	frame := sourceRetirementFrame(o.completionFrame, challenge)
	count, err := io.Copy(peer, bytes.NewReader(frame[:]))
	if err != nil || count != int64(len(frame)) {
		return false, errors.Join(io.ErrShortWrite, err)
	}
	if err = check(); err != nil {
		return false, err
	}
	return true, nil
}

func sourceUnixPeer(connection *net.UnixConn) (syscall.Ucred, error) {
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

func sourceJoinedCancellation(err error) bool {
	if err == nil || err == context.Canceled {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if child == nil || !sourceJoinedCancellation(child) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		child := wrapped.Unwrap()
		return child != nil && sourceJoinedCancellation(child)
	}
	return false
}
