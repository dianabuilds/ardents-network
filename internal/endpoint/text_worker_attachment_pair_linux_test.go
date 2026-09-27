//go:build linux

package endpoint

import (
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
)

// textAttachmentPair binds a same-process credentialed attachment to a raw
// peer socket. The peer stands in for the not-yet-verified worker process in
// readiness and lifetime tests; it grants no host verdict.
func textAttachmentPair(t *testing.T) (*worker.Attachment, *net.UnixConn) {
	t.Helper()
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	var connections [2]*net.UnixConn
	for index, descriptor := range pair {
		file := os.NewFile(uintptr(descriptor), "text-worker-test")
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
	raw, err := connections[0].SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		optionErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_PASSCRED, 1)
	}); err != nil || optionErr != nil {
		t.Fatalf("credentials option: %v %v", err, optionErr)
	}
	return worker.NewAttachment(connections[0], uint32(os.Getpid()), uint32(os.Getuid())), connections[1]
}
