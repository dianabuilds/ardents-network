//go:build linux || darwin

package worker

import (
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

// unixAttachmentPair returns a bound worker attachment and its peer over a plain
// AF_UNIX socketpair without enabling per-frame credentials. It exercises the
// Unix-family transport mechanics shared by Linux and macOS; per-frame
// credential authentication is covered by the Linux port tests and the macOS
// refusal evidence test.
func unixAttachmentPair(t *testing.T) (*Attachment, *net.UnixConn) {
	t.Helper()
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
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
	return &Attachment{connection: connections[0], pid: uint32(os.Getpid()), uid: uint32(os.Getuid())}, connections[1]
}

func TestDocumentWorkerAttachmentWriteReachesPeer(t *testing.T) {
	attachment, peer := unixAttachmentPair(t)
	if _, err := attachment.Write([]byte("outbound")); err != nil {
		t.Fatal(err)
	}
	actual := make([]byte, 8)
	if _, err := io.ReadFull(peer, actual); err != nil || string(actual) != "outbound" {
		t.Fatalf("outbound message: %q %v", actual, err)
	}
}

func TestDocumentWorkerAttachmentUnavailableGuards(t *testing.T) {
	var nilAttachment *Attachment
	if err := nilAttachment.SetDeadline(time.Time{}); err == nil {
		t.Fatal("nil attachment accepted a deadline")
	}
	if n, err := nilAttachment.Read(make([]byte, 1)); n != 0 || err == nil {
		t.Fatal("nil attachment admitted a read")
	}
	if n, err := nilAttachment.Write([]byte("x")); n != 0 || err == nil {
		t.Fatal("nil attachment admitted a write")
	}
	if err := nilAttachment.Close(); err != nil {
		t.Fatal("nil attachment invented a cleanup failure")
	}
	empty := &Attachment{}
	if err := empty.SetDeadline(time.Time{}); err == nil {
		t.Fatal("unbound attachment accepted a deadline")
	}
	if n, err := empty.Read(make([]byte, 1)); n != 0 || err == nil {
		t.Fatal("unbound attachment admitted a read")
	}
}

func TestDocumentWorkerControlClosesEveryReceivedDescriptor(t *testing.T) {
	// Dup owns independent fds here, exactly as recvmsg would. Verify every one
	// closes even when truncation has already invalidated the read. This is the
	// Unix-family SCM_RIGHTS rejection, independent of per-frame credentials.
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
	control := syscall.UnixRights(first, second)
	if checkControl(control, syscall.MSG_CTRUNC, uint32(os.Getpid()), uint32(os.Getuid()), true) == nil {
		t.Fatal("descriptor passing accepted")
	}
	for _, descriptor := range []int{first, second} {
		var stat syscall.Stat_t
		if err := syscall.Fstat(descriptor, &stat); err != syscall.EBADF {
			_ = syscall.Close(descriptor)
			t.Fatal("received descriptor leaked")
		}
	}
}

func TestDocumentWorkerAttachmentCloseIsJoinedAndIdempotent(t *testing.T) {
	attachment, _ := unixAttachmentPair(t)
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
		t.Fatal("Close did not unblock the bound read")
	}
}
