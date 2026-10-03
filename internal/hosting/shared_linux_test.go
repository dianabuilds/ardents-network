//go:build linux

package hosting

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func hostingFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "hosting")
	policy := Policy{Provider: "test fixture", Start: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), Unit: "GiB", Quantity: 1,
		Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1 << 20}
	if err := Initialize(root, policy); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInstalledClosedHostingHandlesShareOneLocalSampler(t *testing.T) {
	root := hostingFixtureRoot(t)
	firstHost, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	first := firstHost
	secondHost, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	second := secondHost
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
	reopenedHost, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	reopened := reopenedHost
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
	root := hostingFixtureRoot(t)
	host, err := OpenShared(root)
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

	reservation, err := host.Reserve(t.Context(), Traffic{Tx: 1 << 16}, Traffic{Rx: 1 << 16}, time.Now().Add(time.Minute))
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

func sampleEqual(a, b Sample) bool {
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

func TestClosedSharedHandleCannotReuseSample(t *testing.T) {
	root := hostingFixtureRoot(t)
	first, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := first.Sample(t.Context(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Sample(t.Context(), time.Second); err == nil {
		t.Fatal("closed handle reused cached success")
	}
	if _, err := second.Sample(t.Context(), time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestSharedSampleOwnsReturnedSlices(t *testing.T) {
	host, err := OpenShared(hostingFixtureRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	for range 2 {
		sample, err := host.Sample(t.Context(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if sample.Policy.Interfaces[0] != "lo" || sample.Interfaces[0].Name != "lo" {
			t.Fatal("caller mutated shared cached facts")
		}
		sample.Policy.Interfaces[0] = "altered"
		sample.Interfaces[0].Name = "altered"
	}
}

func TestCopiedSharedHandleClosesOnlyItsOwnReference(t *testing.T) {
	root := hostingFixtureRoot(t)
	first, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	copied := *first
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copied.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	defer third.Close()
	if third.sampler != second.sampler {
		t.Fatal("copied close retired another handle's sampler")
	}
}

func TestFailedReservationInvalidatesPreviouslyHealthySample(t *testing.T) {
	root := hostingFixtureRoot(t)
	host, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	if _, err := host.Sample(t.Context(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "period.pending"), []byte("uncertain transaction"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Reserve(t.Context(), Traffic{Tx: 10}, Traffic{Rx: 10}, time.Now().Add(time.Minute)); err == nil {
		t.Fatal("ambiguous storage accepted reservation")
	}
	if _, err := host.Sample(t.Context(), time.Second); err == nil {
		t.Fatal("failed reservation left healthy cached observation")
	}
}

func TestFailedReleaseInvalidatesPreviouslyHealthySample(t *testing.T) {
	root := hostingFixtureRoot(t)
	host, err := OpenShared(root)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	reservation, err := host.Reserve(t.Context(), Traffic{Tx: 10}, Traffic{Rx: 10}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := host.Sample(t.Context(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "period.pending"), []byte("uncertain transaction"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := reservation.Release(t.Context()); err == nil {
		t.Fatal("ambiguous storage accepted release")
	}
	if _, err := host.Sample(t.Context(), time.Second); err == nil {
		t.Fatal("failed release left healthy cached observation")
	}
}
