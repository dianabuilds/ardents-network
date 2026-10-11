//go:build linux

package main

import (
	"context"
	"testing"
	"time"
)

// Holder-local join cannot attest remote borrower completion. Observe actual
// receiving owners before fixture Listener Close introduces local interruption.
// Route returns these reservations only after its original physical joins.
func waitRouteIssuerReceivingBorrowers(t *testing.T, ctx context.Context, x *routeIssuerFixture) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		joined := x.bootstrapHolds.Load() == x.bootstrapReturns.Load()
		for _, budget := range x.budgets {
			observed, err := budget.Observe(ctx)
			if err != nil {
				t.Fatal("original receiving Hosting observation", err)
			}
			joined = joined && observed.ReservedBytes == 0
		}
		if joined {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("receiving borrowers did not join within the original operation bound", ctx.Err())
		}
	}
}
