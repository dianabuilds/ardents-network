package node

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
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
func closedForwardingReplenisher(config runtimeConfig, receiver route.ClosedRoleReceiver, host closedForwardingHost, spends *replay.Ledger, local ClosedForwardingProfile) route.ClosedForwardingReplenisher {
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

func closedRoleTokenVerifier(config runtimeConfig, receiver route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
	return func(input route.ClosedAdmissionVerification) (route.ClosedAdmissionApproval, error) {
		if len(input.Token) != 354 || input.Class < 1 || input.Class > 3 || config.CurrentClosedProfile == nil {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		profile, available := config.CurrentClosedProfile()
		now := config.now().UTC()
		if !available || profile.NetworkID != receiver.NetworkID || profile.StateGeneration != receiver.StateGeneration || profile.StateDigest != receiver.StateDigest ||
			profile.Digest != receiver.ProfileDigest || profile.NotBefore.After(now) || !now.Before(profile.NotAfter) {
			return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
		}
		var keyID [32]byte
		copy(keyID[:], input.Token[66:98])
		for index := uint8(0); index < profile.TokenKeyCount; index++ {
			key := profile.TokenKeys[index]
			if key.Class != input.Class || key.WindowStart != now.Truncate(time.Hour) || sha256.Sum256(key.SPKI[:]) != keyID {
				continue
			}
			context := credential.ClosedTokenContext{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID,
				IssuerNodeID: profile.IssuerNodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: input.Class, WindowStart: key.WindowStart}
			if credential.VerifyClosedToken(context, key.SPKI[:], input.Token) == nil {
				return route.ClosedAdmissionApproval{Window: key.WindowStart}, nil
			}
		}
		return route.ClosedAdmissionApproval{}, errors.New("closed forwarding token is unavailable")
	}
}
