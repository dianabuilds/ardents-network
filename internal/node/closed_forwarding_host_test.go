package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// testClosedForwardingHost supplies the bounded local provider owner needed by
// listener fixtures. Production profiles must instead open an initialized
// provider-period ledger from HostingRoot.
type testClosedForwardingHost struct{}

type testClosedForwardingReservation struct{}

func (testClosedForwardingHost) Observe(context.Context) (resource.HostingObservation, error) {
	return resource.HostingObservation{}, nil
}

func (testClosedForwardingHost) Reserve(_ context.Context, work, termination resource.HostingTraffic, end time.Time) (closedForwardingHostReservation, error) {
	if work.Tx == 0 && work.Rx == 0 || termination.Tx == 0 && termination.Rx == 0 || end.IsZero() {
		return nil, errors.New("test forwarding host reservation is invalid")
	}
	return testClosedForwardingReservation{}, nil
}

func (testClosedForwardingHost) Close() error { return nil }

func (testClosedForwardingReservation) Release(context.Context) error { return nil }
