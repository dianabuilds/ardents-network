package node

import (
	"context"
	"crypto/tls"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// closedIssuerNodeHandler owns one State-authenticated outer Carrier. It
// creates no peer, route or fallback: every child terminates at this issuer.
func closedIssuerNodeHandler(config runtimeConfig, snapshot dutyFacts, certificate tls.Certificate, limits *route.ClosedDutyLimits) credential.ClosedNodeBootstrapHandler {
	return func(ctx context.Context, carrier route.ClosedSharedCarrier, serve func(context.Context, io.ReadWriter, [32]byte, route.ClosedHello) error) {
		defer carrier.Connection.Close()
		updated, err := currentFacts(config)
		if err != nil {
			return
		}
		receiver, available := closedRouteReceiver(config, updated, route.ClosedPurposeIssuer, config.now())
		if !available {
			return
		}
		deadline := config.now().UTC().Truncate(time.Second).Add(10 * time.Second)
		if receiver.NotAfter.Before(deadline) {
			deadline = receiver.NotAfter
		}
		outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration,
			StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest,
			DutyGeneration: receiver.DutyGeneration, RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: deadline}, limits, config.now)
		if err != nil {
			return
		}
		defer outer.Close()
		var writer sync.Mutex
		bridge, err := route.NewClosedOuterBridge(outer, func(frame route.ClosedLaneFrame) error {
			writer.Lock()
			defer writer.Unlock()
			return route.WriteClosedLaneFrame(carrier.Connection, frame)
		})
		if err != nil {
			return
		}
		for {
			frame, readErr := route.ReadClosedLaneFrame(carrier.Connection)
			if readErr != nil {
				return
			}
			lane, acceptErr := bridge.Accept(frame)
			if acceptErr != nil {
				return
			}
			if lane != nil {
				go serveClosedIssuerInner(ctx, lane, certificate, deadline, carrier.NodeKey, serve)
			}
		}
	}
}

func serveClosedIssuerInner(ctx context.Context, lane *route.ClosedOuterBridgeLane, certificate tls.Certificate, deadline time.Time, adjacency [32]byte, serve func(context.Context, io.ReadWriter, [32]byte, route.ClosedHello) error) {
	status := byte(1)
	defer func() { _ = lane.CloseWithStatus(status) }()
	secured, err := route.AcceptClosedRoleTLS(ctx, lane, certificate, deadline)
	if err != nil {
		return
	}
	defer secured.Close()
	if err := lane.BeginInnerHello(); err != nil {
		return
	}
	helloFrame, err := route.ReadClosedLaneFrame(secured)
	if err != nil {
		return
	}
	hello, err := route.DecodeClosedHello(helloFrame.Body)
	if err != nil || lane.Activate(hello) != nil {
		return
	}
	if serve(ctx, secured, adjacency, hello) == nil {
		status = 0
	}
}

func closedSharedPeerCurrent(config runtimeConfig, snapshot dutyFacts, key [32]byte, now time.Time) bool {
	if key == [32]byte{} || config.CurrentClosedRoute == nil || snapshot.Profile != route.ClosedRouteProfile || !snapshot.Fresh || snapshot.Conflicting {
		return false
	}
	view, available := config.CurrentClosedRoute()
	if !available || !closedRouteProfileMatchesSnapshot(view.Profile, snapshot, now) {
		return false
	}
	matched := false
	for index := uint8(0); index < view.NodeCount; index++ {
		recipient := view.Nodes[index]
		if recipient.NodeID == [32]byte{} || recipient.NodeID == snapshot.NodeID || recipient.DutyGeneration == 0 || !route.ClosedPurposePermitsDuty(route.ClosedPurposeForwarding, recipient.RoleDomain, recipient.Subrole) {
			continue
		}
		for peerIndex := uint8(0); peerIndex < snapshot.CandidateCount; peerIndex++ {
			peer := snapshot.Candidates[peerIndex]
			if peer.NodeID != recipient.NodeID || peer.PublicKey != key || peer.PublicKey == [32]byte{} || peer.RecordDigest != recipient.RecordDigest || !now.Before(peer.ValidUntil) || !now.Before(peer.AssignmentNotAfter) {
				continue
			}
			if matched {
				return false
			}
			matched = true
		}
	}
	return matched
}
