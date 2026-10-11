package tls

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
)

type observedSocket struct {
	net.Conn
	writes  atomic.Int32
	entered chan struct{}
}

func (socket *observedSocket) Write(value []byte) (int, error) {
	socket.writes.Add(1)
	if socket.entered != nil {
		close(socket.entered)
	}
	return socket.Conn.Write(value)
}

func TestNativeSocketClosedBeforeRecordStartsNoSocketWrite(t *testing.T) {
	local, peer := net.Pipe()
	t.Cleanup(func() { _ = local.Close(); _ = peer.Close() })
	observed := &observedSocket{Conn: local}
	socket := &nativeSocket{Conn: observed}
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	n, err := socket.Write([]byte("record"))
	if n != 0 || err == nil {
		t.Fatalf("closed record outcome: %d %v", n, err)
	}
	if calls := observed.writes.Load(); calls != 0 {
		t.Fatalf("record selected before retirement entered native output after close: %d calls", calls)
	}
	observedState := socket.observe()
	if !observedState.closed || observedState.attempts != 0 || observedState.active != 0 || observedState.failed {
		t.Fatalf("closed socket has no unchanged native-write witness: %+v", observedState)
	}
}

func TestNativeSocketCloseJoinsAlreadyStartedRecordFailure(t *testing.T) {
	local, peer := net.Pipe()
	t.Cleanup(func() { _ = local.Close(); _ = peer.Close() })
	observed := &observedSocket{Conn: local, entered: make(chan struct{})}
	socket := &nativeSocket{Conn: observed}
	result := make(chan error, 1)
	go func() { _, err := socket.Write([]byte("record")); result <- err }()
	<-observed.entered
	if err := socket.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("started native output lost its failure: %v", err)
	}
	if calls := observed.writes.Load(); calls != 1 {
		t.Fatalf("native output attempts changed: %d", calls)
	}
	observedState := socket.observe()
	if !observedState.closed || observedState.attempts != 1 || observedState.active != 0 || !observedState.failed {
		t.Fatalf("started native failure was not retained: %+v", observedState)
	}
}
