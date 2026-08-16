package routeplan

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClientStreamRetryBridgesOneRecoveryPublicationGap(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("arp-retry-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Error(err)
		}
	})
	type dialResult struct {
		connection net.Conn
		err        error
	}
	started := make(chan struct{})
	result := make(chan dialResult, 1)
	go func() {
		close(started)
		connection, err := dialClientStream(path, 5*time.Second, 5*time.Second)
		result <- dialResult{connection: connection, err: err}
	}()
	<-started
	time.Sleep(40 * time.Millisecond)
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			accepted <- connection
		}
	}()
	dialed := <-result
	if dialed.err != nil {
		t.Fatal(dialed.err)
	}
	connection := dialed.connection
	defer connection.Close()
	peer := <-accepted
	defer peer.Close()
}

func TestClientStreamRetryRemainsBounded(t *testing.T) {
	started := time.Now()
	if _, err := dialClientStream(filepath.Join(t.TempDir(), "absent.sock"), 80*time.Millisecond,
		replacementStreamWait); err == nil {
		t.Fatal("absent recovery stream was accepted")
	}
	if elapsed := time.Since(started); elapsed < 60*time.Millisecond || elapsed > 250*time.Millisecond {
		t.Fatalf("bounded recovery stream wait took %s", elapsed)
	}
}

func TestClientStreamRetryRejectsInvalidOrExpiredDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.sock")
	for _, deadline := range []time.Duration{0, time.Nanosecond} {
		connection, err := dialClientStream(path, deadline, replacementStreamWait)
		if err == nil || connection != nil {
			t.Fatalf("deadline %s returned connection=%v error=%v", deadline, connection, err)
		}
	}
}
