package hosting

import (
	"context"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"time"
)

// Handle is the local capacity boundary borrowed by receiving duties until join.
type Handle interface {
	Sample(context.Context, time.Duration) (hostingbudget.Sample, error)
	Reserve(context.Context, hostingbudget.Traffic, hostingbudget.Traffic, time.Time) (Reservation, error)
	Close() error
}

// Ledger adapts the domain handle to the receiving duty reservation interface.
type Ledger struct{ owner *hostingbudget.Shared }

func Open(root string) (*Ledger, error) {
	owner, err := hostingbudget.OpenShared(root)
	if err != nil {
		return nil, err
	}
	return &Ledger{owner: owner}, nil
}
func (host *Ledger) Sample(ctx context.Context, age time.Duration) (hostingbudget.Sample, error) {
	return host.owner.Sample(ctx, age)
}
func (host *Ledger) Reserve(ctx context.Context, work, termination hostingbudget.Traffic, end time.Time) (Reservation, error) {
	reservation, err := host.owner.Reserve(ctx, work, termination, end)
	if err != nil {
		return nil, err
	}
	return reservation, nil
}
func (host *Ledger) Close() error { return host.owner.Close() }
