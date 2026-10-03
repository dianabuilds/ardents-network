package hosting

import (
	"context"
	"errors"
	"testing"
	"time"

	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
)

type observedHost struct {
	reserved   int
	released   int
	releaseErr error
	reserveErr error
}

func (host *observedHost) Reserve(_ context.Context, work, termination hostingbudget.Traffic, end time.Time) (Reservation, error) {
	if work.Tx != 11 || termination.Rx != 7 || end.IsZero() {
		return nil, errors.New("reservation envelope changed")
	}
	if host.reserveErr != nil {
		return nil, host.reserveErr
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
	release, err := Reserve(host, hostingbudget.Traffic{Tx: 11}, hostingbudget.Traffic{Rx: 7}, time.Now().Add(time.Minute))
	if err != nil || host.reserved != 1 {
		t.Fatalf("reservation = %d, %v", host.reserved, err)
	}
	if err := release(); !errors.Is(err, releaseErr) || host.released != 1 {
		t.Fatalf("release = %d, %v", host.released, err)
	}
}

func TestReserveRefusesMissingHostOrDeadline(t *testing.T) {
	if release, err := Reserve(nil, hostingbudget.Traffic{Tx: 1}, hostingbudget.Traffic{Rx: 1}, time.Now()); err == nil || release != nil {
		t.Fatal("missing host was reserved")
	}
	if release, err := Reserve(&observedHost{}, hostingbudget.Traffic{Tx: 1}, hostingbudget.Traffic{Rx: 1}, time.Time{}); err == nil || release != nil {
		t.Fatal("missing deadline was reserved")
	}
}

func TestReserveRetainsDomainFailure(t *testing.T) {
	cause := errors.New("reservation cleanup is unresolved")
	host := &observedHost{reserveErr: cause}
	release, err := Reserve(host, hostingbudget.Traffic{Tx: 11}, hostingbudget.Traffic{Rx: 7}, time.Now().Add(time.Minute))
	if release != nil || !errors.Is(err, cause) {
		t.Fatalf("domain failure erased: %v", err)
	}
}
