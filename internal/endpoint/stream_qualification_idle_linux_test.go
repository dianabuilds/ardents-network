//go:build linux

package endpoint

import (
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestStreamQualificationIdleTraffic(t *testing.T) {
	for _, window := range []time.Duration{1, 86399, 86400, 86401, time.Millisecond, time.Second, 18 * time.Second, 19 * time.Second, time.Minute, streamQualificationIdleWindow - 1, streamQualificationIdleWindow} {
		work, termination := streamQualificationIdleTraffic(window)
		exact := new(big.Int).Mul(big.NewInt(int64(window)), big.NewInt(1_000_000_000))
		exact.Quo(exact, big.NewInt(int64(24*time.Hour))).Add(exact, big.NewInt(1))
		if work.Tx != exact.Uint64() || work.Rx != exact.Uint64() {
			t.Fatalf("window %s: work %+v, exact %s", window, work, exact)
		}
		if termination.Tx != 8<<20 || termination.Rx != 8<<20 {
			t.Fatalf("window %s: termination %+v", window, termination)
		}
	}
	work, _ := streamQualificationIdleTraffic(10 * time.Minute)
	if work.Tx != 6_944_445 || work.Rx != 6_944_445 {
		t.Fatalf("ten-minute work: %+v", work)
	}
}

func TestStreamQualificationIdleHostingAdmission(t *testing.T) {
	for _, direction := range []string{"tx", "rx", "tx+rx"} {
		t.Run(direction, func(t *testing.T) {
			const oneDirection = uint64(6_944_445 + 8<<20)
			want := oneDirection
			if direction == "tx+rx" {
				want *= 2
			}
			now := time.Now().UTC().Truncate(time.Second)
			root := filepath.Join(t.TempDir(), "hosting")
			policy := resource.HostingPolicy{
				Provider: "NET-32 admission fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
				Unit: "B", Quantity: want + 1, Directions: direction, Interfaces: []string{"lo"}, LowWatermarkBytes: 1,
			}
			if err := resource.InitializeHosting(root, policy); err != nil {
				t.Fatal(err)
			}
			host, err := resource.OpenHosting(root)
			if err != nil {
				t.Fatal(err)
			}
			defer host.Close()
			work, termination := streamQualificationIdleTraffic(streamQualificationIdleWindow)
			deadline := now.Add(streamQualificationIdleWindow + 2*time.Minute)
			held, err := host.Reserve(t.Context(), work, termination, deadline)
			if err != nil {
				t.Fatal(err)
			}
			view, err := host.Observe(t.Context())
			if err != nil || view.ReservedBytes != want {
				t.Fatalf("reservation %+v: %v; want %d", view, err, want)
			}
			if _, err := host.Reserve(t.Context(), work, termination, deadline); err == nil {
				t.Fatal("admitted duplicate work without available allowance")
			}
			if err := held.Release(t.Context()); err != nil {
				t.Fatal(err)
			}
			// Enough room for termination and the old wrapped work, but not the
			// corrected work: real admission must refuse this amount.
			shortRoot := filepath.Join(t.TempDir(), "short-hosting")
			policy.Quantity = want - 1
			if err := resource.InitializeHosting(shortRoot, policy); err != nil {
				t.Fatal(err)
			}
			shortHost, err := resource.OpenHosting(shortRoot)
			if err != nil {
				t.Fatal(err)
			}
			defer shortHost.Close()
			if _, err := shortHost.Reserve(t.Context(), work, termination, deadline); err == nil {
				t.Fatal("admitted work without its full work and termination capacity")
			}
		})
	}
}
