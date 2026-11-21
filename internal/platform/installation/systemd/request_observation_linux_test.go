package systemd

import (
	"context"
	"errors"
	"net"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// These controls use actual sockets but produce only refusals, never a fake
// manager reply or an accepting Systemd/Startup/domain observation.
func requestObservationSocket(t *testing.T) *startupBus {
	t.Helper()
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sockets [2]*net.UnixConn
	for i, fd := range pair {
		file := os.NewFile(uintptr(fd), "request-observation")
		conn, err := net.FileConn(file)
		closed := file.Close()
		if err != nil || closed != nil {
			t.Fatal(err, closed)
		}
		var ok bool
		sockets[i], ok = conn.(*net.UnixConn)
		if !ok {
			conn.Close()
			t.Fatal("actual Unix connection required")
		}
		t.Cleanup(func() { _ = sockets[i].Close() })
	}
	return &startupBus{transport: &referenceTransport{socket: sockets[0]}, interruptedIO: &atomic.Bool{}}
}

func TestRequestObservationKeepsOriginalRefusalAndBound(t *testing.T) {
	client := requestObservationSocket(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	end, _ := ctx.Deadline()
	refusal := errors.New("read-only observation refused")
	var borrowed context.Context
	var marker *requestObservation
	err := client.observeRequests(ctx, func(observed context.Context) error {
		borrowed = observed
		var ok bool
		marker, ok = observed.Value(requestObservationKey{}).(*requestObservation)
		if !ok || marker.client != client || !marker.active.Load() {
			t.Fatal("original physical frame not retained")
		}
		if deadline, ok := observed.Deadline(); !ok || deadline.After(end) {
			t.Fatal("original deadline extended")
		}
		foreign := requestObservationSocket(t)
		if err := foreign.transport.socket.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := foreign.request(observed, "", "", "unavailable"); !errors.Is(err, net.ErrClosed) {
			t.Fatal("foreign client borrowed a frame", err)
		}
		return refusal
	})
	if err != refusal {
		t.Fatal("original refusal identity changed", err)
	}
	if marker == nil || marker.active.Load() {
		t.Fatal("original physical frame survived return")
	}
	if !errors.Is(borrowed.Err(), context.Canceled) {
		t.Fatal("borrowed context outlived original observation")
	}
	if _, err := client.request(borrowed, "", "", "unavailable"); !errors.Is(err, context.Canceled) {
		t.Fatal("late request borrowed old physical deadline", err)
	}
}

func TestRequestObservationCancellationJoinsActualSocketInterruption(t *testing.T) {
	client := requestObservationSocket(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered := make(chan struct{})
	finished := make(chan error, 1)
	refusal := errors.New("held physical observation refused")
	go func() {
		finished <- client.observeRequests(ctx, func(context.Context) error {
			close(entered)
			var data [1]byte
			_, err := client.transport.socket.Read(data[:])
			return errors.Join(refusal, err)
		})
	}()
	<-entered
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, refusal) || !errors.Is(err, net.ErrClosed) || !errors.Is(err, context.Canceled) {
			t.Fatal("original interruption or independent refusal lost", err)
		}
		if !client.interruptedIO.Load() {
			t.Fatal("returned before original transport interruption")
		}
	case <-time.After(5 * time.Second):
		_ = client.transport.socket.Close()
		<-finished
		t.Fatal("original physical interruption not joined")
	}
}
