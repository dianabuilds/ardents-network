//go:build linux

package node

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

type releaseFailureHost struct {
	release  error
	reserved atomic.Int32
	released atomic.Int32
}

func (*releaseFailureHost) Sample(context.Context, time.Duration) (resource.HostingSample, error) {
	return resource.HostingSample{}, nil
}

func (*releaseFailureHost) Close() error { return nil }

func (host *releaseFailureHost) Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (hosting.Reservation, error) {
	host.reserved.Add(1)
	return releaseFailureReservation{host: host}, nil
}

type releaseFailureReservation struct{ host *releaseFailureHost }

func (reservation releaseFailureReservation) Release(context.Context) error {
	reservation.host.released.Add(1)
	return reservation.host.release
}
