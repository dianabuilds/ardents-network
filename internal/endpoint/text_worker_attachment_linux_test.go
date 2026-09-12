//go:build linux

package endpoint

import (
	"bytes"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

func textAttachmentPair(t *testing.T) (*textWorkerAttachment, *net.UnixConn) {
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
	return &textWorkerAttachment{connection: connections[0], pid: uint32(os.Getpid()), uid: uint32(os.Getuid())}, connections[1]
}

func TestTextWorkerAttachmentChecksEveryReadAndRejectsForeignPID(t *testing.T) {
	attachment, peer := textAttachmentPair(t)
	for _, body := range []string{"first", "later"} {
		if _, err := peer.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
		actual := make([]byte, len(body))
		if _, err := io.ReadFull(attachment, actual); err != nil || string(actual) != body {
			t.Fatalf("bound message: %q %v", actual, err)
		}
	}
	attachment.pid++
	if _, err := peer.Write([]byte("foreign")); err != nil {
		t.Fatal(err)
	}
	if n, err := attachment.Read(make([]byte, 32)); n != 0 || err == nil {
		t.Fatal("foreign credential delivered bytes")
	}
}

func TestTextWorkerAttachmentRejectsDescriptorPassingAfterOrdinaryBytes(t *testing.T) {
	attachment, peer := textAttachmentPair(t)
	file, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := peer.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(attachment, make([]byte, 2)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := peer.WriteMsgUnix([]byte("bad"), syscall.UnixRights(int(file.Fd())), nil); err != nil {
		t.Fatal(err)
	}
	if n, err := attachment.Read(make([]byte, 16)); n != 0 || err == nil {
		t.Fatal("SCM_RIGHTS delivered application bytes")
	}
}

func TestTextWorkerControlClosesEveryUnwantedDescriptor(t *testing.T) {
	// Dup owns independent fds here, exactly as recvmsg would. Verify every one
	// closes even when credentials/truncation have already invalidated the read.
	file, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	first, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	second, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		_ = syscall.Close(first)
		t.Fatal(err)
	}
	credential := syscall.UnixCredentials(&syscall.Ucred{Pid: int32(os.Getpid()), Uid: uint32(os.Getuid()), Gid: uint32(os.Getgid())})
	control := bytes.Join([][]byte{credential, syscall.UnixRights(first, second)}, nil)
	if checkTextWorkerControl(control, syscall.MSG_CTRUNC, uint32(os.Getpid()), uint32(os.Getuid()), true) == nil {
		t.Fatal("descriptor passing accepted")
	}
	for _, descriptor := range []int{first, second} {
		var stat syscall.Stat_t
		if err := syscall.Fstat(descriptor, &stat); err != syscall.EBADF {
			_ = syscall.Close(descriptor)
			t.Fatal("received descriptor leaked")
		}
	}
	for _, invalid := range [][]byte{nil, append(append([]byte{}, credential...), credential...)} {
		if checkTextWorkerControl(invalid, 0, uint32(os.Getpid()), uint32(os.Getuid()), true) == nil {
			t.Fatal("missing or duplicate credentials accepted")
		}
	}
}

func TestTextWorkerAttachmentCloseIsJoinedAndIdempotent(t *testing.T) {
	attachment, _ := textAttachmentPair(t)
	completed := make(chan error, 1)
	joined := make(chan struct{})
	go func() { defer close(joined); _, err := attachment.Read(make([]byte, 1)); completed <- err }()
	t.Cleanup(func() {
		_ = attachment.Close()
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("socket read did not join")
		}
	})
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
	if err := attachment.Close(); err != nil {
		t.Fatal("repeated Close invented a cleanup failure")
	}
	select {
	case err := <-completed:
		if err == nil {
			t.Fatal("closed attachment admitted a read")
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock credential-aware read")
	}
}
