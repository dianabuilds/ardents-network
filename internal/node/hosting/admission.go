package hosting

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
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
// verification and before Admission durably spends the token.
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
func Replenisher(source authority.Source, now func() time.Time, receiver route.ClosedRoleReceiver, host Host, spends *spending.Ledger, work, termination resource.HostingTraffic) route.ClosedForwardingReplenisher {
	verify := AdmissionVerifier(source, now, receiver, host, work, termination)
	return func(input route.ClosedAdmissionVerification) (func() error, error) {
		approval, err := receiving.Redeem(receiving.Redemption{Class: admission.Class(input.Class), Token: input.Token, Deadline: input.Deadline}, spends, now,
			func() (receiving.Approval, error) {
				approval, err := verify(input)
				return receiving.Approval{Window: approval.Window, Release: approval.Release}, err
			}, nil)
		return approval.Release, err
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
