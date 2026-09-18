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

func TestInstalledClosedForwardingHostsShareOneLocalSampler(t *testing.T) {
	root := closedForwardingHostingRoot(t)
	firstHost, err := openClosedForwardingHost(root)
	if err != nil {
		t.Fatal(err)
	}
	first := firstHost.(*installedClosedForwardingHost)
	secondHost, err := openClosedForwardingHost(root)
	if err != nil {
		t.Fatal(err)
	}
	second := secondHost.(*installedClosedForwardingHost)
	if first.owner == second.owner || first.sampler != second.sampler {
		t.Fatal("same hosting period did not retain independent owners and one sampler")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Sample(t.Context(), time.Second); err != nil {
		t.Fatalf("one handle closed the shared hosting owner: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedHost, err := openClosedForwardingHost(root)
	if err != nil {
		t.Fatal(err)
	}
	reopened := reopenedHost.(*installedClosedForwardingHost)
	t.Cleanup(func() { _ = reopened.Close() })
	if reopened.sampler == first.sampler {
		t.Fatal("last close retained the shared hosting sampler")
	}
}

// TestInstalledClosedForwardingHostSampleCachesAndInvalidatesOnReserve
// asserts the cache contract used by concurrent retained duties: idle Samples
// coalesce to the last successful observation, and a successful Reserve
// invalidates the cache so the next Sample must re-read. Release intentionally
// does not invalidate: shrinking ReservedBytes never raises pressure on any
// reader of the shared cache, and per-stream Releases would clear the cache
// faster than concurrent observations can coalesce on it.
func TestInstalledClosedForwardingHostSampleCachesAndInvalidatesOnReserve(t *testing.T) {
	root := closedForwardingHostingRoot(t)
	host, err := openClosedForwardingHost(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = host.Close() })

	first, err := host.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	second, err := host.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !sampleEqual(first, second) {
		t.Fatalf("consecutive Sample did not coalesce via cache: first=%+v second=%+v", first, second)
	}

	reservation, err := host.Reserve(t.Context(), resource.HostingTraffic{Tx: 1 << 16}, resource.HostingTraffic{Rx: 1 << 16}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	afterReserve, err := host.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if afterReserve.Observation.ReservedBytes <= second.Observation.ReservedBytes {
		t.Fatalf("Sample after Reserve did not observe ReservedBytes growth: before=%+v after=%+v", second, afterReserve)
	}

	if err := reservation.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	afterRelease, err := host.Sample(t.Context(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if afterRelease.Observation.ReservedBytes != afterReserve.Observation.ReservedBytes {
		t.Fatalf("Release must not invalidate the shared cache: afterReserve=%+v afterRelease=%+v", afterReserve, afterRelease)
	}
}

func sampleEqual(a, b resource.HostingSample) bool {
	if !a.At.Equal(b.At) {
		return false
	}
	if a.Boot != b.Boot {
		return false
	}
	if a.Observation.UsedBytes != b.Observation.UsedBytes ||
		a.Observation.ReservedBytes != b.Observation.ReservedBytes ||
		a.Observation.RemainingBytes != b.Observation.RemainingBytes ||
		a.Observation.Protect != b.Observation.Protect ||
		a.Observation.Drain != b.Observation.Drain {
		return false
	}
	return true
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
	_, cancel := context.WithCancel(context.Background())
	server := &closedForwardingServer{host: host, listener: listener, pool: pool, stopped: make(chan struct{}), cancel: cancel}
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
