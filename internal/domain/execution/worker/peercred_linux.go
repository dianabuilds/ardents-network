//go:build linux

package worker

import (
	"errors"
	"net"
	"syscall"
)

// prepareSocket verifies the accepted socket's provenance and enables per-frame
// peer-credential authentication. SO_PEERCRED binds the root-owned listener/
// system manager, not the worker that later inherits the accepted descriptor;
// its PID is therefore not used as the worker Principal. SO_PASSCRED then makes
// every later recvmsg carry the sending worker's SCM_CREDENTIALS, which Read
// matches against the bound identity.
func prepareSocket(connection *net.UnixConn) error {
	if connection == nil {
		return errors.New("document worker socket is unavailable")
	}
	raw, err := connection.SyscallConn()
	if err != nil {
		return errors.New("document worker socket is unavailable")
	}
	var socketErr error
	err = raw.Control(func(fd uintptr) {
		peer, peerErr := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if peerErr != nil || peer.Pid != 1 || peer.Uid != 0 || peer.Gid != 0 {
			socketErr = errors.New("document worker listener is not the system manager")
			return
		}
		socketErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
	})
	if err != nil || socketErr != nil {
		return errors.New("document worker socket provenance is unavailable")
	}
	return nil
}

// peerCredentialControlSpace is the control-buffer room for the Linux per-frame
// SCM_CREDENTIALS message.
func peerCredentialControlSpace() int {
	return syscall.CmsgSpace(syscall.SizeofUcred)
}

// authenticatePeerCredential reports whether one received control message
// authenticates the bound worker identity through Linux SCM_CREDENTIALS. The
// credential PID must be positive and match the bound PID, and both UID and GID
// must equal the bound UID (the selected worker runs with uid==gid).
func authenticatePeerCredential(message *syscall.SocketControlMessage, pid, uid uint32) bool {
	credential, err := syscall.ParseUnixCredentials(message)
	return err == nil && credential.Pid > 0 && uint32(credential.Pid) == pid && credential.Uid == uid && credential.Gid == uid
}
