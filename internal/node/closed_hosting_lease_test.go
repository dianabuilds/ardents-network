//go:build linux

package node

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func closedHostingHandleingRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "hosting")
	policy := resource.HostingPolicy{Provider: "test fixture", Start: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Unit: "GiB", Quantity: 1,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := resource.InitializeHosting(root, policy); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInstalledClosedHostingHandlesShareOneLocalSampler(t *testing.T) {
	root := closedHostingHandleingRoot(t)
	firstHost, err := openClosedHostingHandle(root)
	if err != nil {
		t.Fatal(err)
	}
	first := firstHost.(*installedClosedHostingHandle)
	secondHost, err := openClosedHostingHandle(root)
	if err != nil {
		t.Fatal(err)
	}
	second := secondHost.(*installedClosedHostingHandle)
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
	reopenedHost, err := openClosedHostingHandle(root)
	if err != nil {
		t.Fatal(err)
	}
	reopened := reopenedHost.(*installedClosedHostingHandle)
	t.Cleanup(func() { _ = reopened.Close() })
	if reopened.sampler == first.sampler {
		t.Fatal("last close retained the shared hosting sampler")
	}
}

// TestInstalledClosedHostingSampleCachesAndInvalidatesOnReserve
// asserts the cache contract used by concurrent retained duties: idle Samples
// coalesce to the last successful observation, and a successful Reserve
// invalidates the cache so the next Sample must re-read. Release intentionally
// does not invalidate: shrinking ReservedBytes never raises pressure on any
// reader of the shared cache, and per-stream Releases would clear the cache
// faster than concurrent observations can coalesce on it.
func TestInstalledClosedHostingSampleCachesAndInvalidatesOnReserve(t *testing.T) {
	root := closedHostingHandleingRoot(t)
	host, err := openClosedHostingHandle(root)
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
