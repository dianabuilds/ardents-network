package routeplan

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeferredPublisherStreamPreservesHalfCloseBeforeOpen(t *testing.T) {
	path := filepath.Join(os.TempDir(), "arp-half-"+time.Now().Format("150405.000000")+".sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !isClosedNetworkError(err) {
			t.Error(err)
		}
	})
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- connection
		}
	}()
	stream := &deferredUnixStream{path: path, timeout: time.Second}
	if err := stream.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan struct {
		value byte
		err   error
	}, 1)
	go func() {
		buffer := make([]byte, 1)
		_, readErr := stream.Read(buffer)
		readDone <- struct {
			value byte
			err   error
		}{buffer[0], readErr}
	}()
	peer := <-accepted
	t.Cleanup(func() {
		if err := peer.Close(); err != nil && !isClosedNetworkError(err) {
			t.Error(err)
		}
	})
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if count, err := peer.Read(make([]byte, 1)); count != 0 || err == nil {
		t.Fatalf("pending half-close was not applied: count=%d err=%v", count, err)
	}
	if _, err := peer.Write([]byte{9}); err != nil {
		t.Fatal(err)
	}
	result := <-readDone
	if result.err != nil || result.value != 9 {
		t.Fatalf("read after pending half-close: value=%d err=%v", result.value, result.err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func isClosedNetworkError(err error) bool {
	return err == nil || errors.Is(err, net.ErrClosed)
}
