//go:build linux

package daemon

import (
	"net"

	"golang.org/x/sys/unix"
)

func unixPeerIdentity(connection net.Conn) ([]byte, bool) {
	unixConnection, ok := connection.(*net.UnixConn)
	if !ok {
		return nil, false
	}
	raw, err := unixConnection.SyscallConn()
	if err != nil {
		return nil, false
	}
	var credential *unix.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		credential, socketErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil || socketErr != nil || credential == nil {
		return nil, false
	}
	return canonicalUnixPeerUID(credential.Uid), true
}
