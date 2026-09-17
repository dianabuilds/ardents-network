//go:build linux

package node

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
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

func (*sharedHostingSampleFixture) Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (closedForwardingHostReservation, error) {
	return nil, nil
}

func (*sharedHostingSampleFixture) Close() error { return nil }

type sharedHostingSampleListener struct {
	closed chan struct{}
	once   sync.Once
}

func (listener *sharedHostingSampleListener) Accept(ctx context.Context, _ time.Duration) (route.ClosedSharedCarrier, error) {
	select {
	case <-ctx.Done():
		return route.ClosedSharedCarrier{}, ctx.Err()
	case <-listener.closed:
		return route.ClosedSharedCarrier{}, context.Canceled
	}
}

func (listener *sharedHostingSampleListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return nil
}

func closedForwardingHostingRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "hosting")
	policy := resource.HostingPolicy{Provider: "test fixture", Start: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Unit: "GiB", Quantity: 1,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := resource.InitializeHosting(root, policy); err != nil {
		t.Fatal(err)
	}
	return root
}

// Periodic Node observation shares the recently committed whole-host sample.
// It must not create another exclusive writer per local duty: the qualification
// host runs several independently supervised duties against this one ledger.
func TestClosedForwardingReaperSharesRecentHostingSample(t *testing.T) {
	host := &sharedHostingSampleFixture{sampled: make(chan struct{})}
	listener := &sharedHostingSampleListener{closed: make(chan struct{})}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	server := &closedForwardingServer{host: host, listener: listener, pool: pool, stopped: make(chan struct{})}
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
