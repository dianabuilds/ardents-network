package endpoint

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
)

func TestNET32IdleHostingReservationKeepsProviderAccounting(t *testing.T) {
	const oneDirection = uint64(6_944_445 + 8<<20)
	for _, provider := range []struct {
		directions string
		counted    uint64
	}{{"tx", oneDirection}, {"rx", oneDirection}, {"tx+rx", 2 * oneDirection}} {
		t.Run(provider.directions, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			work, termination := net32IdleTraffic(10 * time.Minute)
			deadline := now.Add(12 * time.Minute)
			request := hosting.ReservationRequest{Work: work, Termination: termination, WorkUntil: deadline, HoldUntil: deadline}
			policy := hosting.Policy{
				Provider: "NET-32 qualification fixture", Start: now.Add(-time.Hour), End: now.Add(time.Hour),
				Unit: "B", Directions: provider.directions, Interfaces: []string{"lo"}, LowWatermarkBytes: 1,
			}
			for _, amount := range []struct {
				name     string
				quantity uint64
			}{{"exact", provider.counted + 1}, {"short", provider.counted}} {
				t.Run(amount.name, func(t *testing.T) {
					policy.Quantity = amount.quantity
					root := filepath.Join(t.TempDir(), "hosting")
					if err := hosting.Initialize(root, policy); err != nil {
						t.Fatal(err)
					}
					budget, err := hosting.Open(root)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := budget.Close(); err != nil {
							t.Error(err)
						}
					})
					held, err := budget.Reserve(t.Context(), request)
					if amount.quantity == provider.counted {
						if !errors.Is(err, hosting.ErrCapacity) {
							t.Fatalf("one byte short: %v, want capacity refusal", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					observation, err := budget.Observe(t.Context())
					if err != nil || observation.ReservedBytes != provider.counted {
						t.Fatalf("held %+v / %v, want %d counted bytes", observation, err, provider.counted)
					}
					if _, err := budget.Reserve(t.Context(), request); !errors.Is(err, hosting.ErrCapacity) {
						t.Fatalf("second reservation: %v, want capacity refusal", err)
					}
					if err := held.Release(t.Context()); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
