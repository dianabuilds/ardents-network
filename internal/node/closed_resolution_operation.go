package node

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func (server *closedResolutionServer) current() bool {
	snapshot, err := currentFacts(server.config)
	if err != nil {
		return false
	}
	receiver, ok := closedRouteReceiver(server.config, snapshot, route.ClosedPurposeReachability, server.config.now())
	return ok && receiver == server.receiver
}

func (server *closedResolutionServer) serveOuter(ctx context.Context, carrier route.ClosedSharedCarrier) {
	if !server.current() {
		return
	}
	receiver := server.receiver
	outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID,
		StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest, DutyGeneration: receiver.DutyGeneration,
		RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: receiver.NotAfter}, server.limits, server.config.now)
	if err != nil {
		return
	}
	serveClosedOuter(ctx, carrier.Connection, outer, server.serveInner)
}

func (server *closedResolutionServer) serveInner(ctx context.Context, lane *route.ClosedOuterBridgeLane) {
	status := byte(1)
	defer func() { _ = lane.CloseWithStatus(status) }()
	if lane.Restriction() != route.ClosedChildOrdinary || !server.current() {
		return
	}
	secured, err := route.AcceptClosedRoleTLS(ctx, lane, server.certificate, server.receiver.NotAfter)
	if err != nil {
		return
	}
	defer func() {
		if secured.CloseWrite() != nil {
			status = 1
		}
	}()
	if lane.BeginInnerHello() != nil {
		return
	}
	frame, err := route.ReadClosedLaneFrame(secured)
	if err != nil || frame.Kind != 1 || frame.Lane != 0 {
		return
	}
	hello, err := route.DecodeClosedHello(frame.Body)
	if err != nil || hello.Purpose != route.ClosedPurposeReachability || lane.Activate(hello) != nil {
		return
	}
	if server.serveAdmitted(ctx, secured, lane, frame) == nil {
		status = 0
	}
}

func (server *closedResolutionServer) serveAdmitted(ctx context.Context, connection net.Conn, lane *route.ClosedOuterBridgeLane, hello route.ClosedLaneFrame) error {
	exporter, err := route.ClosedRoleTLSExporter(connection)
	if err != nil {
		return err
	}
	channel, err := route.NewClosedAdmissionChannel(server.receiver, server.spends, server.limits, exporter,
		closedRoleTokenVerifier(server.config, server.receiver), server.config.now)
	if err != nil {
		return err
	}
	if _, err := channel.Accept(hello); err != nil {
		return err
	}
	frame, err := route.ReadClosedLaneFrame(connection)
	if err != nil || frame.Kind != 2 || frame.Lane != 0 || len(frame.Body) != 355 || frame.Body[0] != 1 {
		return errors.New("closed resolution requires Control admission")
	}
	lease, err := channel.Accept(frame)
	if err != nil {
		return err
	}
	defer lease.Release()
	if err := lane.Admit(&lease, connection); err != nil {
		return err
	}
	if err := connection.SetDeadline(lease.Deadline); err != nil {
		return err
	}
	accepted, err := route.ClosedAcceptFrame(0, 64<<10)
	if err != nil {
		return err
	}
	used := uint64(16 + len(hello.Body) + 16 + len(frame.Body) + 16 + len(accepted.Body))
	if err := route.WriteClosedLaneFrame(connection, accepted); err != nil {
		return err
	}
	operation, err := route.ReadClosedLaneFrame(connection)
	if err != nil || operation.Kind != 10 || operation.Lane != 0 {
		return errors.New("closed resolution operation is invalid")
	}
	used += uint64(16 + len(operation.Body) + 16 + (16 << 10))
	if used > lease.Bytes || ctx.Err() != nil || !server.config.now().Before(lease.Deadline) || !server.current() {
		return errors.New("closed resolution admission ended")
	}
	request, err := route.DecodeClosedDescriptorRequest(operation.Body)
	if err != nil {
		return err
	}
	result, err := server.resolve(request, server.config.now())
	if err != nil {
		return err
	}
	if ctx.Err() != nil || !server.current() || !server.config.now().Before(lease.Deadline) {
		return errors.New("closed resolution ended before acknowledgement")
	}
	return route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 11, Body: result})
}

func (server *closedResolutionServer) resolve(request route.ClosedDescriptorRequest, now time.Time) ([]byte, error) {
	status := uint8(1)
	var proof []byte
	if len(request.Descriptor) != 0 {
		verified, err := reachability.VerifyPrivatePublication(request.Descriptor, server.receiver.NetworkID, server.receiver.ProfileDigest, now)
		if err == nil && server.currentIntroduction(verified.Descriptor.Private, now) {
			result, err := server.store.PublishPrivate(request.Descriptor, server.receiver.ProfileDigest, now)
			if err == nil && (result.Class == reachability.StoreAccepted || result.Class == reachability.StoreAlreadyCurrent) {
				status = 0
			}
			if result.Class == reachability.StoreStale || result.Class == reachability.StoreConflicting {
				status = 3
			}
		}
	} else {
		raw, class, err := server.store.LookupPrivate(request.Target, server.receiver.ProfileDigest, now)
		if err == nil {
			verified, err := reachability.VerifyPrivate(raw, request.Target, server.receiver.NetworkID, server.receiver.ProfileDigest, now)
			if err == nil && server.currentIntroduction(verified.Descriptor.Private, now) {
				status, proof = 0, raw
			}
		} else if class == reachability.StoreStale || class == reachability.StoreConflicting {
			status = 3
		}
	}
	return route.EncodeClosedDescriptorResult(request.Nonce, status, proof)
}

func (server *closedResolutionServer) currentIntroduction(introduction reachability.PrivateIntroduction, now time.Time) bool {
	if !server.current() || introduction.NotAfter.After(server.receiver.NotAfter) {
		return false
	}
	snapshot, err := currentFacts(server.config)
	if err != nil {
		return false
	}
	view, ok := server.config.CurrentClosedRoute()
	if !ok || !closedRouteProfileMatchesSnapshot(view.Profile, snapshot, now) {
		return false
	}
	matches := 0
	for index := uint8(0); index < view.NodeCount; index++ {
		node := view.Nodes[index]
		if node.NodeID != introduction.NodeID || !route.ClosedPurposePermitsDuty(route.ClosedPurposeIntroduction, node.RoleDomain, node.Subrole) {
			continue
		}
		for peerIndex := uint8(0); peerIndex < snapshot.CandidateCount; peerIndex++ {
			peer := snapshot.Candidates[peerIndex]
			if peer.NodeID == node.NodeID && peer.RecordDigest == node.RecordDigest && !now.Before(peer.ValidFrom) &&
				!introduction.NotAfter.After(peer.ValidUntil) && !introduction.NotAfter.After(peer.AssignmentNotAfter) {
				matches++
			}
		}
	}
	return matches == 1
}
