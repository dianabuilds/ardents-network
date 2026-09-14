package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// closedForwardingAdmissionVerifier reserves the installed host's declared
// interface envelope after token verification but before Route burns the token.
func closedForwardingAdmissionVerifier(config runtimeConfig, receiver route.ClosedRoleReceiver, host closedForwardingHost, local ClosedForwardingProfile) route.ClosedAdmissionVerifier {
	verify := closedRoleTokenVerifier(config, receiver)
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		approval, err := verify(input)
		if err != nil || input.Class != 2 {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		release, err := reserveClosedForwarding(host, local, input.Deadline)
		if err != nil {
			return route.ClosedAdmissionApproval{}, err
		}
		approval.Release = release
		return approval, nil
	}
}

// closedForwardingReplenisher commits the same host envelope before it burns
// a fresh token. The parent supplies its original immutable deadline.
func closedForwardingReplenisher(config runtimeConfig, receiver route.ClosedRoleReceiver, host closedForwardingHost, spends *route.ClosedSpendLedger, local ClosedForwardingProfile) route.ClosedForwardingReplenisher {
	verify := closedRoleTokenVerifier(config, receiver)
	return func(input route.ClosedAdmissionVerification) (func() error, error) {
		approval, err := verify(input)
		if err != nil || input.Class != 2 {
			return nil, errors.New("closed forwarding token is unavailable")
		}
		release, err := reserveClosedForwarding(host, local, input.Deadline)
		if err != nil {
			return nil, err
		}
		if err := spends.Spend(input.Token, approval.Window, config.now().UTC()); err != nil {
			return nil, errors.Join(err, release())
		}
		return release, nil
	}
}

func reserveClosedForwarding(host closedForwardingHost, local ClosedForwardingProfile, deadline time.Time) (func() error, error) {
	if host == nil || deadline.IsZero() {
		return nil, errors.New("closed forwarding host allowance is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reservation, err := host.Reserve(ctx, local.AdmissionTraffic, local.TerminationTraffic, deadline)
	if err != nil {
		return nil, errors.New("closed forwarding host allowance is unavailable")
	}
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return reservation.Release(ctx)
	}, nil
}
