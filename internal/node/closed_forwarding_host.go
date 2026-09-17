package node

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// closedForwardingHost keeps the provider-period ledger outside Route. The
// concrete production adapter opens only the already initialized local root;
// tests may provide a bounded owner without selecting provider facts.
type closedForwardingHost interface {
	Sample(context.Context, time.Duration) (resource.HostingSample, error)
	Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (closedForwardingHostReservation, error)
	Close() error
}

type closedForwardingHostReservation interface {
	Release(context.Context) error
}

type installedClosedForwardingHost struct{ owner *resource.Hosting }

func openClosedForwardingHost(root string) (closedForwardingHost, error) {
	owner, err := resource.OpenHosting(root)
	if err != nil {
		return nil, err
	}
	return installedClosedForwardingHost{owner: owner}, nil
}

func (host installedClosedForwardingHost) Reserve(ctx context.Context, work, termination resource.HostingTraffic, end time.Time) (closedForwardingHostReservation, error) {
	return host.owner.Reserve(ctx, work, termination, end)
}

func (host installedClosedForwardingHost) Close() error { return host.owner.Close() }

func (host installedClosedForwardingHost) Sample(ctx context.Context, maximumAge time.Duration) (resource.HostingSample, error) {
	return host.owner.Sample(ctx, maximumAge)
}
