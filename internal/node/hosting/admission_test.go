package hosting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

type observedHost struct {
	reserved   int
	released   int
	releaseErr error
}

func (host *observedHost) Reserve(_ context.Context, work, termination resource.HostingTraffic, end time.Time) (Reservation, error) {
	if work.Tx != 11 || termination.Rx != 7 || end.IsZero() {
		return nil, errors.New("reservation envelope changed")
	}
	host.reserved++
	return observedReservation{host}, nil
}

type observedReservation struct{ host *observedHost }

func (reservation observedReservation) Release(context.Context) error {
	reservation.host.released++
	return reservation.host.releaseErr
}

func TestReserveTransfersExactEnvelopeAndReleaseFailure(t *testing.T) {
	releaseErr := errors.New("hosting release failed")
	host := &observedHost{releaseErr: releaseErr}
	release, err := Reserve(host, resource.HostingTraffic{Tx: 11}, resource.HostingTraffic{Rx: 7}, time.Now().Add(time.Minute))
	if err != nil || host.reserved != 1 {
		t.Fatalf("reservation = %d, %v", host.reserved, err)
	}
	if err := release(); !errors.Is(err, releaseErr) || host.released != 1 {
		t.Fatalf("release = %d, %v", host.released, err)
	}
}

func TestReserveRefusesMissingHostOrDeadline(t *testing.T) {
	if release, err := Reserve(nil, resource.HostingTraffic{Tx: 1}, resource.HostingTraffic{Rx: 1}, time.Now()); err == nil || release != nil {
		t.Fatal("missing host was reserved")
	}
	if release, err := Reserve(&observedHost{}, resource.HostingTraffic{Tx: 1}, resource.HostingTraffic{Rx: 1}, time.Time{}); err == nil || release != nil {
		t.Fatal("missing deadline was reserved")
	}
}
