//go:build linux

package hosting

import (
	"testing"
	"time"
)

func TestCopiedHostingReservationCannotReleaseAnotherOwnersBudget(t *testing.T) {
	root, reading, now := hostingFixture(t)
	owner := openHostingFixture(t, root, reading, now)
	first, err := owner.Reserve(t.Context(), Traffic{Tx: 100}, Traffic{Rx: 10}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	second, err := owner.Reserve(t.Context(), Traffic{Tx: 200}, Traffic{Rx: 20}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	copyOfFirst := *first
	if err := first.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := copyOfFirst.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
	observation, err := owner.Observe(t.Context())
	if err != nil || observation.ReservedBytes != 220 {
		t.Fatalf("copied reservation refunded another owner's budget: %+v / %v", observation, err)
	}
	if err := second.Release(t.Context()); err != nil {
		t.Fatal(err)
	}
}
