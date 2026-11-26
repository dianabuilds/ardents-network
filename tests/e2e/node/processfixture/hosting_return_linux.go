package processfixture

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
)

// WaitReturnedReservations observes original receiving budgets before teardown
// can interrupt the remote borrowers and cause a different retirement.
func WaitReturnedReservations(t testing.TB, ctx context.Context, budgets []*hosting.Budget, holds, returns *atomic.Int32) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		held, returned := holds.Load(), returns.Load()
		allReturned := held == returned
		for _, budget := range budgets {
			observation, err := budget.Observe(ctx)
			if err != nil {
				t.Fatalf("observe original receiving Hosting budget: %v", err)
			}
			allReturned = allReturned && observation.ReservedBytes == 0
		}
		if allReturned {
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal(fmt.Errorf("receiving reservations did not return within the original operation bound (holds=%d returns=%d): %w", held, returned, ctx.Err()))
		}
	}
}
