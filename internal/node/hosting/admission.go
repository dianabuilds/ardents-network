package hosting

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// Reservation belongs to one admitted operation until its lease releases it.
type Reservation interface {
	Release(context.Context) error
}

// Host exposes only the reservation operation of the shared ledger.
type Host interface {
	Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (Reservation, error)
}

// AdmissionVerifier reserves the declared interface envelope after token
// verification and before Route burns the token.
func AdmissionVerifier(source authority.Source, now func() time.Time, receiver route.ClosedRoleReceiver, host Host, work, termination resource.HostingTraffic) route.ClosedAdmissionVerifier {
	verify := source.TokenVerifier(receiver, now)
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		approval, err := verify(input)
		if err != nil || input.Class != 2 {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		release, err := Reserve(host, work, termination, input.Deadline)
		if err != nil {
			return route.ClosedAdmissionApproval{}, err
		}
		approval.Release = release
		return approval, nil
	}
}

// Replenisher commits the same envelope before burning a fresh token. The
// parent supplies its original immutable deadline.
func Replenisher(source authority.Source, now func() time.Time, receiver route.ClosedRoleReceiver, host Host, spends *replay.Ledger, work, termination resource.HostingTraffic) route.ClosedForwardingReplenisher {
	verify := source.TokenVerifier(receiver, now)
	return func(input route.ClosedAdmissionVerification) (func() error, error) {
		approval, err := verify(input)
		if err != nil || input.Class != 2 {
			return nil, errors.New("closed forwarding token is unavailable")
		}
		release, err := Reserve(host, work, termination, input.Deadline)
		if err != nil {
			return nil, err
		}
		if err := spends.Spend(input.Token, approval.Window, now().UTC()); err != nil {
			return nil, errors.Join(err, release())
		}
		return release, nil
	}
}

// Reserve bounds each Hosting call and returns the exact release result.
func Reserve(host Host, work, termination resource.HostingTraffic, deadline time.Time) (func() error, error) {
	if host == nil || deadline.IsZero() {
		return nil, errors.New("closed forwarding host allowance is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reservation, err := host.Reserve(ctx, work, termination, deadline)
	if err != nil {
		return nil, errors.New("closed forwarding host allowance is unavailable")
	}
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return reservation.Release(ctx)
	}, nil
}
