package node

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestListenerFailureIsTerminal(t *testing.T) {
	want := errors.New("listener failed")
	server := &probeListener{listener: failingListener{err: want}, open: make(chan struct{}, 1),
		stop: make(chan struct{}), terminal: make(chan error, 1), connections: make(map[net.Conn]struct{})}
	server.work.Add(1)
	go server.accept()
	select {
	case got := <-server.terminal:
		if !errors.Is(got, want) {
			t.Fatalf("terminal error = %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("listener failure was hidden")
	}
	server.work.Wait()
}

type failingListener struct{ err error }

func (listener failingListener) Accept() (net.Conn, error) { return nil, listener.err }
func (failingListener) Close() error                       { return nil }
func (failingListener) Addr() net.Addr                     { return testAddress("probe") }

type testAddress string

func (address testAddress) Network() string { return string(address) }
func (address testAddress) String() string  { return string(address) }
