package stock

import (
	"context"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/token"
)

// Coordination alone supplies no permission or successful authority fixture.
func TestPreparationRetainsOriginalReservationAndRejectsCopiedRetirement(t *testing.T) {
	o := &Owner{}
	ctx, cancel := context.WithCancel(t.Context())
	p, err := o.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := o.Prepare(t.Context()); err == nil || other != nil {
		t.Fatal("concurrent opening acquired the same stock")
	}
	copyOfPreparation := *p
	if missing, err := copyOfPreparation.Missing([]token.ClosedTokenContext{{}}); err == nil || len(missing) != 0 {
		t.Fatal("copied preparation inspected original Stock", missing, err)
	}
	if copyOfPreparation.Close() == nil {
		t.Fatal("copied preparation retired original coordination")
	}
	if other, err := o.Prepare(t.Context()); err == nil || other != nil {
		t.Fatal("copied Close released the original opening")
	}
	cancel()
	if missing, err := p.Missing([]token.ClosedTokenContext{{}}); err == nil || len(missing) != 0 {
		t.Fatal("canceled preparation supplied reusable Stock", missing, err)
	}
	if _, err := p.Begin(IssuanceIntent{}); err == nil {
		t.Fatal("canceled reservation supplied batch authority")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := o.Prepare(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
}
