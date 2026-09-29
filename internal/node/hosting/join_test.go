package hosting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type joinObservedHandle struct {
	reserved int
	closed   int
	closeErr error
	age      time.Duration
	sample   resource.HostingSample
}

func (host *joinObservedHandle) Sample(_ context.Context, age time.Duration) (resource.HostingSample, error) {
	host.age = age
	return host.sample, nil
}

func (host *joinObservedHandle) Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (Reservation, error) {
	host.reserved++
	return nil, errors.New("unexpected reservation")
}

func (host *joinObservedHandle) Close() error {
	host.closed++
	return host.closeErr
}

func TestJoinHandleDelegatesSampleAndOwnedClose(t *testing.T) {
	closeErr := errors.New("host close failed")
	host := &joinObservedHandle{sample: resource.HostingSample{Observation: resource.HostingObservation{Drain: true}}, closeErr: closeErr}
	lease := NewJoinHandle(host, authority.Source{}, time.Now)
	got, err := lease.Sample(context.Background(), 3*time.Second)
	if err != nil || !got.Observation.Drain || host.age != 3*time.Second {
		t.Fatalf("sample = %+v, %v; age = %v", got, err, host.age)
	}
	if err := lease.Close(); !errors.Is(err, closeErr) || host.closed != 1 {
		t.Fatalf("close = %v; calls = %d", err, host.closed)
	}
}

func TestJoinHandleRejectsUnauthenticatedWorkBeforeReservation(t *testing.T) {
	host := &joinObservedHandle{}
	lease := NewJoinHandle(host, authority.Source{}, time.Now)
	input := route.ClosedAdmissionVerification{Class: 2}
	if _, err := lease.AdmissionVerifier(route.ClosedRoleReceiver{})(input); err == nil {
		t.Fatal("unauthenticated JOIN admission succeeded")
	}
	if _, err := lease.Replenisher(route.ClosedRoleReceiver{}, nil)(input); err == nil {
		t.Fatal("unauthenticated JOIN replenishment succeeded")
	}
	if host.reserved != 0 {
		t.Fatalf("unauthenticated JOIN reserved %d times", host.reserved)
	}
}

func TestJoinEnvelopeRetainsOriginalLifetimeBounds(t *testing.T) {
	work, termination := joinEnvelope()
	if work.Tx != 64<<20 || work.Rx != 64<<20 || termination.Tx != 1<<20 || termination.Rx != 1<<20 {
		t.Fatalf("JOIN envelope = %+v, %+v", work, termination)
	}
}
