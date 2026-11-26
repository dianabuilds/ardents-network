//go:build darwin

package worker

import (
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestDocumentWorkerDarwinPeerCredentialIsConnectTimeOnly is the runtime
// security-equivalence evidence for the macOS refusal. macOS AF_UNIX exposes the
// peer's identity only at connection time (LOCAL_PEERCRED / LOCAL_PEERPID, the
// getpeereid family); it has no SO_PASSCRED / SCM_CREDENTIALS per-frame
// credential passing. The worker attachment must authenticate the bound worker
// on EVERY frame, so the macOS port refuses rather than admitting unauthenticated
// bytes (ADR-0129: a missing required protection is an explicit unavailable
// outcome, never a weaker accepting path).
func TestDocumentWorkerDarwinPeerCredentialIsConnectTimeOnly(t *testing.T) {
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	var connections [2]*net.UnixConn
	for index, descriptor := range pair {
		file := os.NewFile(uintptr(descriptor), "text-worker-darwin-test")
		connection, err := net.FileConn(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		connections[index] = connection.(*net.UnixConn)
		t.Cleanup(func() { _ = connection.Close() })
		if err := connection.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	// Connect-time peer identity IS observable through LOCAL_PEERCRED and
	// LOCAL_PEERPID: this is the getpeereid-family mechanism macOS provides.
	raw, err := connections[0].SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var cred *unix.Xucred
	var peerPID int
	var controlErr error
	if err := raw.Control(func(fd uintptr) {
		cred, controlErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if controlErr != nil {
			return
		}
		peerPID, controlErr = unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
	}); err != nil || controlErr != nil {
		t.Fatalf("connect-time peer credential: %v %v", err, controlErr)
	}
	if cred.Uid != uint32(os.Getuid()) || cred.Groups[0] != uint32(os.Getgid()) || peerPID != os.Getpid() {
		t.Fatalf("connect-time peer identity mismatch: uid=%d gid=%d pid=%d", cred.Uid, cred.Groups[0], peerPID)
	}
	// But it cannot authenticate individual frames, so prepareSocket refuses with
	// the explicit typed unavailable outcome rather than enabling a weaker path.
	if err := prepareSocket(connections[0]); err != errPeerCredentialUnavailable {
		t.Fatalf("macOS peer-credential preparation must refuse, got %v", err)
	}
	// A bound attachment delivers no unauthenticated bytes: the peer writes a
	// plain frame and Read refuses it (returned count 0) because no per-frame
	// credential binds the worker. This matches the Linux contract exactly — a
	// refused read returns 0 bytes even though recvmsg filled the scratch buffer.
	attachment := &Attachment{connection: connections[0], pid: uint32(os.Getpid()), uid: uint32(os.Getuid())}
	if _, err := connections[1].Write([]byte("unauthenticated")); err != nil {
		t.Fatal(err)
	}
	n, err := attachment.Read(make([]byte, 32))
	if n != 0 || err == nil {
		t.Fatalf("macOS attachment admitted unauthenticated bytes: n=%d err=%v", n, err)
	}
}
