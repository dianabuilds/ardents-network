package node

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// closedIssuerNodeHandler owns one State-authenticated outer Carrier. It
// creates no peer, route or fallback: every child terminates at this issuer.
func closedIssuerNodeHandler(config runtimeConfig, certificate tls.Certificate, issuer *credential.ClosedTokenIssuer, spends *route.ClosedSpendLedger, limits *route.ClosedDutyLimits) credential.ClosedNodeBootstrapHandler {
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
		deadline := receiver.NotAfter
		outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration,
			StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest,
			DutyGeneration: receiver.DutyGeneration, RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: deadline}, limits, config.now)
		if err != nil {
			return
		}
		serveClosedOuter(ctx, carrier.Connection, outer, func(childContext context.Context, lane *route.ClosedOuterBridgeLane) {
			admitted := func(connection net.Conn, hello route.ClosedLaneFrame) error {
				exporter, err := route.ClosedRoleTLSExporter(connection)
				if err != nil {
					return err
				}
				channel, err := route.NewClosedAdmissionChannel(receiver, spends, limits, exporter, closedRoleTokenVerifier(config, receiver), config.now)
				if err != nil {
					return err
				}
				return issuer.ServeAdmittedAfterHello(childContext, connection, channel, hello, lane)
			}
			serveClosedIssuerInner(childContext, lane, certificate, deadline, carrier.NodeKey, serve, admitted)
		})
	}
}

func serveClosedIssuerInner(ctx context.Context, lane *route.ClosedOuterBridgeLane, certificate tls.Certificate, deadline time.Time, adjacency [32]byte, serve func(context.Context, io.ReadWriter, [32]byte, route.ClosedHello) error, admitted func(net.Conn, route.ClosedLaneFrame) error) {
	status := byte(1)
	defer func() { _ = lane.CloseWithStatus(status) }()
	secured, err := route.AcceptClosedRoleTLS(ctx, lane, certificate, deadline)
	if err != nil {
		return
	}
	defer func() {
		if secured.CloseWrite() != nil {
			status = 1
		}
	}()
	if err := lane.BeginInnerHello(); err != nil {
		return
	}
	helloFrame, err := route.ReadClosedLaneFrame(secured)
	if err != nil {
		return
	}
	if helloFrame.Kind != 1 || helloFrame.Lane != 0 {
		return
	}
	hello, err := route.DecodeClosedHello(helloFrame.Body)
	if err != nil || lane.Activate(hello) != nil {
		return
	}
	switch lane.Restriction() {
	case route.ClosedChildIssuerBootstrap:
		if serve(ctx, secured, adjacency, hello) == nil {
			status = 0
		}
	case route.ClosedChildOrdinary:
		if admitted(secured, helloFrame) == nil {
			status = 0
		}
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
