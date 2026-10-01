//go:build linux

package installation

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type startCompletion struct {
	listener           *net.UnixListener
	root               string
	stopListenerWait   func() bool
	stopConnectionWait func() bool
}

func prepareStartCompletion(ctx context.Context, root string, intent transitionIntent, gid uint32) (*startCompletion, error) {
	if err := retainStartGuard(root, intent); err != nil {
		return nil, err
	}
	path := filepath.Join(root, "start-completion.socket")
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, errors.New("installation completion socket requires explicit recovery")
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	listener.SetUnlinkOnClose(false)
	if err := errors.Join(os.Chown(path, 0, int(gid)), os.Chmod(path, 0660)); err != nil {
		return nil, errors.Join(err, listener.Close())
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := listener.SetDeadline(deadline); err != nil {
			return nil, errors.Join(err, listener.Close())
		}
	}
	stop := context.AfterFunc(ctx, func() { _ = listener.SetDeadline(time.Now()) })
	return &startCompletion{listener: listener, root: root, stopListenerWait: stop}, nil
}

func (completion *startCompletion) close() error {
	if completion.stopConnectionWait != nil {
		completion.stopConnectionWait()
	}
	completion.stopListenerWait()
	return completion.listener.Close()
}

func completionFrame(selected selection, invocation [16]byte) ([]byte, error) {
	if invocation == [16]byte{} || !canonicalDigest(selected.GenerationDigest) || !canonicalDigest(selected.BindingDigest) {
		return nil, errors.New("installation completion binding is invalid")
	}
	return []byte(hex.EncodeToString(invocation[:]) + selected.GenerationDigest + selected.BindingDigest), nil
}

func unixPeer(connection *net.UnixConn) (*syscall.Ucred, error) {
	raw, err := connection.SyscallConn()
	if err != nil {
		return nil, err
	}
	var peer *syscall.Ucred
	var observedErr error
	err = raw.Control(func(fd uintptr) {
		peer, observedErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	return peer, errors.Join(err, observedErr)
}

func (completion *startCompletion) accept(ctx context.Context, selected selection, pid, uid uint32, invocation [16]byte) (*net.UnixConn, error) {
	connection, err := completion.listener.AcceptUnix()
	if err != nil {
		return nil, errors.Join(ctx.Err(), err)
	}
	refuse := func(err error) (*net.UnixConn, error) { return nil, errors.Join(ctx.Err(), err, connection.Close()) }
	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return refuse(err)
		}
	}
	completion.stopConnectionWait = context.AfterFunc(ctx, func() { _ = connection.SetDeadline(time.Now()) })
	peer, err := unixPeer(connection)
	if err != nil || peer == nil || peer.Pid <= 0 || uint32(peer.Pid) != pid || peer.Uid != uid {
		return refuse(errors.Join(errors.New("installation completion Endpoint peer differs"), err))
	}
	wanted, err := completionFrame(selected, invocation)
	if err != nil {
		return refuse(err)
	}
	body := make([]byte, len(wanted))
	if _, err := io.ReadFull(connection, body); err != nil {
		return refuse(err)
	}
	if !bytes.Equal(body, wanted) {
		return refuse(errors.New("installation completion invocation differs"))
	}
	return connection, nil
}

func awaitStartCompletion(ctx context.Context, root string, selected selection, invocation [16]byte) (returnedErr error) {
	if _, err := os.Lstat(filepath.Join(root, "start-guard.json")); os.IsNotExist(err) {
		return refusePendingTransition(root)
	} else if err != nil {
		return err
	}
	wanted, err := completionFrame(selected, invocation)
	if err != nil {
		return err
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", filepath.Join(root, "start-completion.socket"))
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, connection.Close()) }()
	local, ok := connection.(*net.UnixConn)
	if !ok {
		return errors.New("installation completion connection differs")
	}
	peer, err := unixPeer(local)
	if err != nil || peer == nil || peer.Uid != 0 {
		return errors.Join(errors.New("installation completion owner is not root"), err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := local.SetDeadline(deadline); err != nil {
			return err
		}
	}
	stopWait := context.AfterFunc(ctx, func() { _ = local.SetDeadline(time.Now()) })
	defer stopWait()
	if _, err := io.Copy(local, bytes.NewReader(wanted)); err != nil {
		return errors.Join(ctx.Err(), err)
	}
	body := make([]byte, len(wanted))
	if _, err := io.ReadFull(local, body); err != nil {
		return errors.Join(ctx.Err(), err)
	}
	if !bytes.Equal(body, wanted) {
		return errors.New("installation completion acknowledgement differs")
	}
	return ctx.Err()
}

func sendStartCompletion(ctx context.Context, connection *net.UnixConn, selected selection, invocation [16]byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := completionFrame(selected, invocation)
	if err != nil {
		return err
	}
	_, err = io.Copy(connection, bytes.NewReader(body))
	return errors.Join(ctx.Err(), err)
}
