//go:build darwin

package worker

import (
	"errors"
	"net"
	"syscall"
)

// errPeerCredentialUnavailable is the explicit typed refusal of the macOS
// peer-credential capability. macOS AF_UNIX exposes the connecting peer's
// identity only at connection time (LOCAL_PEERCRED / LOCAL_PEERPID, the
// getpeereid family); it has no SO_PASSCRED / SCM_CREDENTIALS per-message
// credential passing. The worker attachment's security guarantee authenticates
// the bound worker identity on EVERY frame, so a connect-time-only peer check
// cannot satisfy it: a process that later shares the descriptor could inject
// frames indistinguishably from the selected worker. Per ADR-0129 the missing
// required protection is an explicit unavailable outcome, never a weaker
// accepting path, so the macOS attachment refuses to authenticate frames rather
// than admitting unauthenticated bytes. peercred_darwin_test.go exercises the
// connect-time LOCAL_PEERCRED observation and the resulting refusal at runtime.
var errPeerCredentialUnavailable = errors.New("document worker peer-credential authentication is unavailable on this platform")

// prepareSocket refuses: macOS cannot enable per-frame peer-credential
// authentication on the accepted AF_UNIX socket.
func prepareSocket(connection *net.UnixConn) error {
	return errPeerCredentialUnavailable
}

// peerCredentialControlSpace is zero: macOS carries no per-frame credential
// control message.
func peerCredentialControlSpace() int {
	return 0
}

// authenticatePeerCredential authenticates no frame: macOS AF_UNIX has no
// per-message peer credential, so no control message can bind the worker
// identity. The Unix-family transport therefore rejects every frame that must
// present a credential and admits no unauthenticated bytes.
func authenticatePeerCredential(message *syscall.SocketControlMessage, pid, uid uint32) bool {
	return false
}
