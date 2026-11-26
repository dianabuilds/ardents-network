//go:build linux || darwin

package worker

import (
	"errors"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
)

// Attachment admits bytes only from the selected worker process on
// the exact accepted AF_UNIX stream. This is one launch check, not a Grant.
// Every later frame also goes through Read, so SCM_RIGHTS cannot be smuggled
// after a successful readiness exchange.
//
// The AF_UNIX/SCM_RIGHTS transport is Unix-family (Linux and macOS). The
// per-frame peer-credential authentication that binds the worker identity is a
// capability port at this boundary: peercred_linux.go authenticates every frame
// through SO_PASSCRED/SCM_CREDENTIALS, while peercred_darwin.go refuses because
// macOS AF_UNIX exposes only connect-time LOCAL_PEERCRED, not per-frame
// credentials. A missing required protection is an explicit unavailable outcome
// on every platform, never a weaker accepting path (ADR-0129).
type Attachment struct {
	connection *net.UnixConn
	pid        uint32
	uid        uint32
	closeOnce  sync.Once
	closeErr   error
}

// SetDeadline moves the bound socket's I/O deadline; the zero time releases it.
func (attachment *Attachment) SetDeadline(deadline time.Time) error {
	if attachment == nil || attachment.connection == nil {
		return errors.New("document worker attachment is unavailable")
	}
	return attachment.connection.SetDeadline(deadline)
}

func (attachment *Attachment) Read(body []byte) (int, error) {
	if attachment == nil || attachment.connection == nil || attachment.pid == 0 {
		return 0, errors.New("document worker attachment is unavailable")
	}
	if len(body) == 0 {
		return 0, nil
	}
	// Linux permits at most 253 SCM_RIGHTS descriptors in one message; size the
	// control buffer for all of them so a rejected message can release each
	// received descriptor, plus the platform's per-frame credential message.
	control := make([]byte, syscall.CmsgSpace(253*4)+peerCredentialControlSpace())
	n, controlN, flags, _, err := attachment.connection.ReadMsgUnix(body, control)
	controlErr := checkControl(control[:controlN], flags, attachment.pid, attachment.uid, n > 0)
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

func checkControl(control []byte, flags int, pid, uid uint32, hasBytes bool) error {
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
		if authenticatePeerCredential(&message, pid, uid) {
			credentials++
		} else {
			invalid = true
		}
	}
	if invalid || hasBytes && credentials != 1 || !hasBytes && credentials > 1 {
		return errors.New("document worker attachment credentials or descriptors are invalid")
	}
	return nil
}

func (attachment *Attachment) Write(body []byte) (int, error) {
	if attachment == nil || attachment.connection == nil {
		return 0, errors.New("document worker attachment is unavailable")
	}
	return attachment.connection.Write(body)
}

func (attachment *Attachment) Close() error {
	if attachment == nil || attachment.connection == nil {
		return nil
	}
	attachment.closeOnce.Do(func() { attachment.closeErr = attachment.connection.Close() })
	return attachment.closeErr
}
