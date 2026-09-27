package issuer

import (
	"context"
	"crypto/tls"
	"errors"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// Profile contains only the issuer's local roots, certificate and limits.
type Profile struct {
	Root            string
	AdmissionRoot   string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
}

// Config supplies the issuer's selected duty and borrowed State projections.
// VerifyAdmission retains Node's Hosting reservation policy.
type Config struct {
	Profile         Profile
	Snapshot        state.NodeDuty
	Authority       authority.Source
	CurrentDuty     func() (state.NodeDuty, error)
	VerifyAdmission func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier
	Now             func() time.Time
	ListenAddress   string
}

// Handle exposes only supervision and joined shutdown to the Node process.
type Handle struct {
	Done   <-chan error
	Joined <-chan struct{}
	Usage  func() (uint64, uint64, uint64)
	Stop   func()
	Drain  func(context.Context) error
}

func Start(config Config) (*Handle, error) {
	local, snapshot := config.Profile, config.Snapshot
	releases := &releaseErrors{}
	current := func() (state.ClosedProfileView, bool) {
		updated, err := config.CurrentDuty()
		if err != nil {
			return state.ClosedProfileView{}, false
		}
		return currentProfile(config.Authority, updated, config.Now())
	}
	issuer, err := credential.OpenClosedTokenIssuer(credential.ClosedTokenIssuerConfig{Root: local.Root, NetworkID: snapshot.NetworkID,
		CurrentProfile: current, Clock: config.Now})
	if err != nil {
		return nil, err
	}
	receiver, available := config.Authority.Receiver(snapshot, ardp.PurposeIssuer, config.Now())
	if !available {
		return nil, errors.Join(errors.New("closed issuer receiver is unavailable"), issuer.Close())
	}
	spends, err := replay.Open(local.AdmissionRoot, replay.Binding{NetworkID: receiver.NetworkID,
		ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		return nil, errors.Join(err, issuer.Close())
	}
	limits, err := route.NewClosedDutyLimits(config.Now)
	if err != nil {
		return nil, errors.Join(err, spends.Close(), issuer.Close())
	}
	shared, err := carrier.ListenClosedSharedCarrier(carrier.CarrierProfile(snapshot.CarrierProfile), config.ListenAddress, local.Certificate, func(key [32]byte) bool {
		updated, currentErr := config.CurrentDuty()
		return currentErr == nil && config.Authority.PeerCurrent(updated, key, config.Now())
	}, local.ConnectionLimit)
	if err != nil {
		return nil, errors.Join(err, spends.Close(), issuer.Close())
	}
	listener, err := credential.StartClosedTokenListener(context.Background(), credential.ClosedTokenListenerConfig{Issuer: issuer,
		SharedListener: shared, NodeHandler: nodeHandler(config, local.Certificate, issuer, spends, limits, releases.record),
		ConnectionLimit: local.ConnectionLimit, Clock: config.Now})
	if err != nil {
		return nil, errors.Join(err, spends.Close(), issuer.Close())
	}
	server := &closedIssuerServer{listener: listener, spends: spends, issuer: issuer, releases: releases,
		done: make(chan error, 1), drained: make(chan struct{})}
	go server.run()
	return &Handle{Done: server.done, Joined: server.drained, Usage: func() (uint64, uint64, uint64) {
		return uint64(listener.Active()), uint64(listener.Active()), 0
	}, Stop: func() { _ = server.listener.Stop() }, Drain: func(ctx context.Context) error {
		return server.drain(ctx, local.DrainTimeout)
	}}, nil
}

// closedIssuerServer is the sole late owner of the opened issuer key root and
// the admission spend root. It forwards the listener terminal cause to Node
// supervision, then joins every accepted child without the caller's short
// drain deadline and closes both roots only after the last borrower finished.
type closedIssuerServer struct {
	listener *credential.ClosedTokenListener
	spends   *replay.Ledger
	issuer   *credential.ClosedTokenIssuer
	releases *releaseErrors
	done     chan error
	drained  chan struct{}
	drainErr error
}

func (server *closedIssuerServer) run() {
	cause := <-server.listener.Done()
	_ = server.listener.Stop()
	server.done <- cause
	// No timeout releases roots while an accepted child still borrows them.
	joined := server.listener.Drain(context.Background())
	server.drainErr = errors.Join(joined, server.releases.result(), server.spends.Close(), server.issuer.Close())
	close(server.drained)
}

// drain reports the recorded finalization result once every worker joined. On
// a bounded timeout it reports the unproven cleanup without closing anything;
// later waits return the same recorded result and never close a root twice.
func (server *closedIssuerServer) drain(ctx context.Context, timeout time.Duration) error {
	_ = server.listener.Stop()
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-server.drained:
		return server.drainErr
	case <-bounded.Done():
		return bounded.Err()
	}
}

func Validate(local Profile, source authority.Source, snapshot state.NodeDuty, now time.Time, endpointLiteral bool) error {
	if local.Root == "" || !filepath.IsAbs(local.Root) || filepath.Clean(local.Root) != local.Root || local.Certificate.PrivateKey == nil ||
		local.AdmissionRoot == "" || !filepath.IsAbs(local.AdmissionRoot) || filepath.Clean(local.AdmissionRoot) != local.AdmissionRoot || local.Root == local.AdmissionRoot ||
		local.ConnectionLimit == 0 || local.ConnectionLimit > 16 || local.DrainTimeout <= 0 || local.DrainTimeout > time.Minute ||
		!endpointLiteral || carrier.CarrierProfile(snapshot.CarrierProfile) != carrier.ClosedCarrierTCP && carrier.CarrierProfile(snapshot.CarrierProfile) != carrier.ClosedCarrierQUIC {
		return errors.New("closed issuer local profile is incomplete")
	}
	if _, available := currentProfile(source, snapshot, now); !available {
		return errors.New("closed issuer State profile is unavailable")
	}
	return nil
}

func currentProfile(source authority.Source, snapshot state.NodeDuty, now time.Time) (state.ClosedProfileView, bool) {
	if source.CurrentProfile == nil || snapshot.Profile != carrier.ClosedRouteProfile || !snapshot.Fresh || snapshot.Conflicting ||
		snapshot.NodeID == [32]byte{} || !now.Before(snapshot.ValidUntil) || !now.Before(snapshot.RecordValidUntil) {
		return state.ClosedProfileView{}, false
	}
	profile, available := source.CurrentProfile()
	if !available || profile.NetworkID != snapshot.NetworkID || profile.StateDigest != snapshot.Digest || profile.Epoch != snapshot.Epoch ||
		profile.IssuerNodeID != snapshot.NodeID || profile.IssuerDutyGeneration == 0 || profile.NotBefore.After(now) || !now.Before(profile.NotAfter) ||
		profile.NotBefore.Before(snapshot.EpochValidFrom) || profile.NotAfter.After(snapshot.ValidUntil) || profile.NotAfter.After(snapshot.RecordValidUntil) ||
		!authority.StateGenerationMatches(profile.StateGeneration, snapshot.Generation) {
		return state.ClosedProfileView{}, false
	}
	receiver, available := source.Receiver(snapshot, ardp.PurposeIssuer, now)
	if !available || receiver.NetworkID != profile.NetworkID || receiver.StateGeneration != profile.StateGeneration || receiver.StateDigest != profile.StateDigest ||
		receiver.ProfileDigest != profile.Digest || receiver.NodeID != profile.IssuerNodeID || receiver.DutyGeneration != profile.IssuerDutyGeneration ||
		receiver.NotAfter != profile.NotAfter {
		return state.ClosedProfileView{}, false
	}
	return profile, true
}
