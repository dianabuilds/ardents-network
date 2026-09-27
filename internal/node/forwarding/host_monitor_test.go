//go:build linux

package forwarding

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"sync"
	"testing"
	"time"
)

type sharedHostingSampleFixture struct {
	sampled chan struct{}
	once    sync.Once
}

func (host *sharedHostingSampleFixture) Observe(context.Context) (resource.HostingObservation, error) {
	return resource.HostingObservation{Protect: true, Drain: true}, context.DeadlineExceeded
}

func (host *sharedHostingSampleFixture) Sample(context.Context, time.Duration) (resource.HostingSample, error) {
	host.once.Do(func() { close(host.sampled) })
	return resource.HostingSample{}, nil
}

func (*sharedHostingSampleFixture) Close() error { return nil }

type sharedHostingSampleListener struct {
	closed chan struct{}
	once   sync.Once
}

func (listener *sharedHostingSampleListener) Accept(ctx context.Context, _ time.Duration) (carrier.ClosedSharedCarrier, error) {
	select {
	case <-ctx.Done():
		return carrier.ClosedSharedCarrier{}, ctx.Err()
	case <-listener.closed:
		return carrier.ClosedSharedCarrier{}, context.Canceled
	}
}

func (listener *sharedHostingSampleListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return nil
}

// Periodic Node observation shares the recently committed whole-host sample.
// It must not create another exclusive writer per local duty: the qualification
// host runs several independently supervised duties against this one ledger.
func TestClosedForwardingReaperSharesRecentHostingSample(t *testing.T) {
	host := &sharedHostingSampleFixture{sampled: make(chan struct{})}
	listener := &sharedHostingSampleListener{closed: make(chan struct{})}
	pool, err := carrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, cancel := context.WithCancel(context.Background())
	server := &forwardServer{host: host, listener: listener, pool: pool, stopped: make(chan struct{}), cancel: cancel}
	server.workers.Add(1)
	joined := make(chan struct{})
	go func() {
		server.reap()
		close(joined)
	}()
	select {
	case <-host.sampled:
	case <-server.stopped:
		t.Fatal("periodic shared-host observation stopped the Node listener")
	case <-time.After(3 * time.Second):
		t.Fatal("periodic shared-host observation did not run")
	}
	select {
	case <-server.stopped:
		t.Fatal("fresh shared-host sample stopped the Node listener")
	default:
	}
	server.Stop()
	select {
	case <-joined:
	case <-time.After(2 * time.Second):
		t.Fatal("periodic shared-host observer did not join")
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
}
