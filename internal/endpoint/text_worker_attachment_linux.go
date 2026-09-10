//go:build linux

package endpoint

import (
	"errors"
	"io"
	"net"
	"sync"
	"syscall"
)

// textWorkerAttachment admits bytes only from the selected worker process on
// the exact accepted AF_UNIX stream. This is one launch check, not a Grant.
// Every later frame also goes through Read, so SCM_RIGHTS cannot be smuggled
// after a successful readiness exchange.
type textWorkerAttachment struct {
	connection *net.UnixConn
	pid        uint32
	uid        uint32
	closeOnce  sync.Once
	closeErr   error
}

func prepareTextWorkerSocket(connection *net.UnixConn) error {
	if connection == nil {
		return errors.New("text worker socket is unavailable")
	}
	raw, err := connection.SyscallConn()
	if err != nil {
		return errors.New("text worker socket is unavailable")
	}
	var socketErr error
	err = raw.Control(func(fd uintptr) {
		// SO_PEERCRED binds the root-owned listener/system manager, not the worker
		// that later inherits the accepted descriptor. Its PID is therefore not
		// used as the worker Principal.
		peer, peerErr := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if peerErr != nil || peer.Pid != 1 || peer.Uid != 0 || peer.Gid != 0 {
			socketErr = errors.New("text worker listener is not the system manager")
			return
		}
		socketErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
	})
	if err != nil || socketErr != nil {
		return errors.New("text worker socket provenance is unavailable")
	}
	return nil
}

func (attachment *textWorkerAttachment) Read(body []byte) (int, error) {
	if attachment == nil || attachment.connection == nil || attachment.pid == 0 || attachment.uid == 0 {
		return 0, errors.New("text worker attachment is unavailable")
	}
	if len(body) == 0 {
		return 0, nil
	}
	// Linux permits at most 253 SCM_RIGHTS descriptors in one message. Account
	// for all of them so a rejected message can release each received descriptor.
	// Go's Linux ReadMsgUnix uses MSG_CMSG_CLOEXEC before publishing any fd.
	control := make([]byte, syscall.CmsgSpace(253*4)+syscall.CmsgSpace(syscall.SizeofUcred))
	n, controlN, flags, _, err := attachment.connection.ReadMsgUnix(body, control)
	controlErr := checkTextWorkerControl(control[:controlN], flags, attachment.pid, attachment.uid, n > 0)
	if controlErr != nil {
		return 0, controlErr
	}
	if err != nil {
		return n, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func checkTextWorkerControl(control []byte, flags int, pid, uid uint32, hasBytes bool) error {
	messages, err := syscall.ParseSocketControlMessage(control)
	invalid := err != nil || flags&(syscall.MSG_TRUNC|syscall.MSG_CTRUNC) != 0
	credentials := 0
	for _, message := range messages {
		if message.Header.Level == syscall.SOL_SOCKET && message.Header.Type == syscall.SCM_RIGHTS {
			// Never retain or act on a received fd, even when another control item
			// already made the message invalid.
			descriptors, parseErr := syscall.ParseUnixRights(&message)
			if parseErr == nil {
				for _, fd := range descriptors {
					_ = syscall.Close(fd)
				}
			}
			invalid = true
			continue
		}
		credential, parseErr := syscall.ParseUnixCredentials(&message)
		if parseErr != nil || credential.Pid <= 0 || uint32(credential.Pid) != pid || credential.Uid != uid || credential.Gid != uid {
			invalid = true
		} else {
			credentials++
		}
	}
	if invalid || hasBytes && credentials != 1 || !hasBytes && credentials > 1 {
		return errors.New("text worker attachment credentials or descriptors are invalid")
	}
	return nil
}

func (attachment *textWorkerAttachment) Write(body []byte) (int, error) {
	if attachment == nil || attachment.connection == nil {
		return 0, errors.New("text worker attachment is unavailable")
	}
	return attachment.connection.Write(body)
}

func (attachment *textWorkerAttachment) Close() error {
	if attachment == nil || attachment.connection == nil {
		return nil
	}
	attachment.closeOnce.Do(func() { attachment.closeErr = attachment.connection.Close() })
	return attachment.closeErr
}
