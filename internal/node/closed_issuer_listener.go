package node

import (
	"context"
	"encoding/hex"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func startClosedIssuer(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	local := config.ClosedIssuer
	if err := validateClosedIssuerProfile(local, config, snapshot, config.now()); err != nil {
		return nil, err
	}
	current := func() (state.ClosedProfileView, bool) {
		updated, err := currentFacts(config)
		if err != nil {
			return state.ClosedProfileView{}, false
		}
		return closedIssuerStateProfile(config, updated, config.now())
	}
	issuer, err := credential.OpenClosedTokenIssuer(credential.ClosedTokenIssuerConfig{Root: local.Root, NetworkID: snapshot.NetworkID,
		CurrentProfile: current, Clock: config.now})
	if err != nil {
		return nil, err
	}
	receiver, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeIssuer, config.now())
	if !available {
		return nil, errors.Join(errors.New("closed issuer receiver is unavailable"), issuer.Close())
	}
	spends, err := route.OpenClosedSpendLedger(local.AdmissionRoot, route.ClosedSpendBinding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, errors.Join(err, issuer.Close())
	}
	limits, err := route.NewClosedDutyLimits(config.now)
	if err != nil {
		return nil, errors.Join(err, spends.Close(), issuer.Close())
	}
	shared, err := route.ListenClosedSharedCarrier(route.CarrierProfile(snapshot.CarrierProfile), snapshot.ProbeEndpoint, local.Certificate, func(key [32]byte) bool {
		updated, currentErr := currentFacts(config)
		return currentErr == nil && closedSharedPeerCurrent(config, updated, key, config.now())
	}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, spends.Close(), issuer.Close())
	}
	listener, err := credential.StartClosedTokenListener(context.Background(), credential.ClosedTokenListenerConfig{Issuer: issuer,
		SharedListener: shared, NodeHandler: closedIssuerNodeHandler(config, local.Certificate, issuer, spends, limits),
		ConnectionLimit: local.ConnectionLimit, Clock: config.now})
	if err != nil {
		return nil, errors.Join(err, spends.Close(), issuer.Close())
	}
	return &probeServer{Done: listener.Done(), Protect: func(bool) {}, Usage: func() (uint64, uint64, uint64) {
		return uint64(listener.Active()), uint64(listener.Active()), 0
	}, Stop: func() { _ = listener.Stop() }, Drain: func(ctx context.Context) error {
		drain, cancel := context.WithTimeout(ctx, local.DrainTimeout)
		defer cancel()
		if err := listener.Drain(drain); err != nil {
			// An incomplete join cannot release durable owners to a successor.
			return err
		}
		return errors.Join(spends.Close(), issuer.Close())
	}}, nil
}

func validateClosedIssuerProfile(local ClosedIssuerProfile, config runtimeConfig, snapshot dutyFacts, now time.Time) error {
	if local.Root == "" || !filepath.IsAbs(local.Root) || filepath.Clean(local.Root) != local.Root || local.Certificate.PrivateKey == nil ||
		local.AdmissionRoot == "" || !filepath.IsAbs(local.AdmissionRoot) || filepath.Clean(local.AdmissionRoot) != local.AdmissionRoot || local.Root == local.AdmissionRoot ||
		local.ConnectionLimit == 0 || local.ConnectionLimit > 16 || local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute ||
		!literalNodeEndpoint(snapshot.ProbeEndpoint) || route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierTCP && route.CarrierProfile(snapshot.CarrierProfile) != route.ClosedCarrierQUIC {
		return errors.New("closed issuer local profile is incomplete")
	}
	if _, available := closedIssuerStateProfile(config, snapshot, now); !available {
		return errors.New("closed issuer State profile is unavailable")
	}
	return nil
}

func closedIssuerStateProfile(config runtimeConfig, snapshot dutyFacts, now time.Time) (state.ClosedProfileView, bool) {
	if config.CurrentClosedProfile == nil || snapshot.Profile != route.ClosedRouteProfile || !snapshot.Fresh || snapshot.Conflicting ||
		snapshot.NodeID == [32]byte{} || !now.Before(snapshot.ValidUntil) || !now.Before(snapshot.RecordValidUntil) {
		return state.ClosedProfileView{}, false
	}
	profile, available := config.CurrentClosedProfile()
	if !available || profile.NetworkID != snapshot.NetworkID || profile.StateDigest != snapshot.Digest || profile.Epoch != snapshot.Epoch ||
		profile.IssuerNodeID != snapshot.NodeID || profile.IssuerDutyGeneration == 0 || profile.NotBefore.After(now) || !now.Before(profile.NotAfter) ||
		profile.NotBefore.Before(snapshot.EpochValidFrom) || profile.NotAfter.After(snapshot.ValidUntil) || profile.NotAfter.After(snapshot.RecordValidUntil) ||
		!closedStateGenerationMatches(profile.StateGeneration, snapshot.Generation) {
		return state.ClosedProfileView{}, false
	}
	receiver, available := closedRouteReceiver(config, snapshot, route.ClosedPurposeIssuer, now)
	if !available || receiver.NetworkID != profile.NetworkID || receiver.StateGeneration != profile.StateGeneration || receiver.StateDigest != profile.StateDigest ||
		receiver.ProfileDigest != profile.Digest || receiver.NodeID != profile.IssuerNodeID || receiver.DutyGeneration != profile.IssuerDutyGeneration ||
		receiver.NotAfter != profile.NotAfter {
		return state.ClosedProfileView{}, false
	}
	return profile, true
}

func closedStateGenerationMatches(generation [32]byte, encoded string) bool {
	if len(encoded) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != len(generation) || hex.EncodeToString(decoded) != encoded {
		return false
	}
	for index := range generation {
		if decoded[index] != generation[index] {
			return false
		}
	}
	return true
}
