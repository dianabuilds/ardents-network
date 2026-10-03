package hosting

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
)

func TestLifetimeDefersCloseUntilJoined(t *testing.T) {
	host := &lifetimeHost{}
	lifetime := NewLifetime(host)
	joined := make(chan struct{})
	if !lifetime.DeferCloseUntil(joined) {
		t.Fatal("did not transfer Hosting close to late join")
	}
	if err := lifetime.Close(); err != nil {
		t.Fatalf("deferred close = %v", err)
	}
	if host.closed.Load() != 0 {
		t.Fatalf("Hosting closed before child join: %d", host.closed.Load())
	}
	close(joined)
	deadline := time.After(time.Second)
	for host.closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("Hosting did not close after child join")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if host.closed.Load() != 1 {
		t.Fatalf("Hosting close calls = %d", host.closed.Load())
	}
}

func TestLifetimeRetainsLateCloseFailure(t *testing.T) {
	closeErr := errors.New("late Hosting close failed")
	host := &lifetimeHost{err: closeErr}
	lifetime := NewLifetime(host)
	joined := make(chan struct{})
	if !lifetime.DeferCloseUntil(joined) {
		t.Fatal("did not transfer Hosting close")
	}
	if err := lifetime.Close(); err != nil {
		t.Fatalf("bounded close before join = %v", err)
	}
	close(joined)
	deadline := time.After(time.Second)
	for host.closed.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("late Hosting close did not run")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if err := lifetime.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("retained late close result = %v", err)
	}
}

type lifetimeHost struct {
	closed atomic.Int32
	err    error
}

func (*lifetimeHost) Sample(context.Context, time.Duration) (hostingbudget.Sample, error) {
	return hostingbudget.Sample{}, nil
}

func (*lifetimeHost) Reserve(context.Context, hostingbudget.Traffic, hostingbudget.Traffic, time.Time) (Reservation, error) {
	return nil, nil
}

func (host *lifetimeHost) Close() error {
	host.closed.Add(1)
	return host.err
}
