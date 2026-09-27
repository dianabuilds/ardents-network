package hosting

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
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

type lifetimeHost struct{ closed atomic.Int32 }

func (*lifetimeHost) Sample(context.Context, time.Duration) (resource.HostingSample, error) {
	return resource.HostingSample{}, nil
}

func (*lifetimeHost) Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (Reservation, error) {
	return nil, nil
}

func (host *lifetimeHost) Close() error {
	host.closed.Add(1)
	return nil
}
