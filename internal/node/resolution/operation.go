package resolution

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/authority"
	nodeouter "github.com/dianabuilds/ardents-network/internal/node/outer"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func (server *closedResolutionServer) current() bool {
	snapshot, err := server.config.CurrentDuty()
	if err != nil {
		return false
	}
	receiver, ok := server.config.Authority.Receiver(snapshot, ardp.PurposeReachability, server.config.Now())
	return ok && receiver == server.receiver
}

func (server *closedResolutionServer) serveOuter(ctx context.Context, carrier routecarrier.ClosedSharedCarrier) {
	if !server.current() {
		return
	}
	receiver := server.receiver
	outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID,
		StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest, DutyGeneration: receiver.DutyGeneration,
		RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: receiver.NotAfter}, server.limits, server.config.Now)
	if err != nil {
		return
	}
	// Bound retained child failures over this Carrier's lifetime. Publish the
	// complete first child outcome together with the joined physical outcome;
	// a secondary TLS failure must not hide its causal physical writer failure.
	var childMu sync.Mutex
	var childCleanup error
	outerErr := nodeouter.Serve(ctx, carrier.Connection, outer, func(childContext context.Context, lane *route.ClosedOuterBridgeLane) {
		err := server.serveInner(childContext, lane)
		childMu.Lock()
		if childCleanup == nil {
			childCleanup = err
		}
		childMu.Unlock()
	})
	server.recordCleanup(errors.Join(carrierCleanupError(outerErr), childCleanup))
}

func (server *closedResolutionServer) serveInner(ctx context.Context, lane *route.ClosedOuterBridgeLane) (cleanupErr error) {
	status := byte(1)
	var admission *route.ClosedAdmission
	// The child retains its reservation through inner TLS and Outer termination.
	defer func() { cleanupErr = errors.Join(cleanupErr, admission.Release()) }()
	defer func() { cleanupErr = errors.Join(cleanupErr, carrierCleanupError(lane.CloseWithStatus(status))) }()
	if lane.Restriction() != route.ClosedChildOrdinary || !server.current() {
		return
	}
	secured, err := routecarrier.AcceptClosedRoleTLS(ctx, lane, server.certificate, server.receiver.NotAfter)
	if err != nil {
		return
	}
	defer func() {
		if err := secured.CloseWrite(); err != nil {
			cleanupErr = errors.Join(cleanupErr, carrierCleanupError(err))
			status = 1
		}
	}()
	if lane.BeginInnerHello() != nil {
		return
	}
	frame, err := ardp.ReadFrame(secured)
	if err != nil || frame.Kind != 1 || frame.Lane != 0 {
		return
	}
	hello, err := ardp.DecodeHello(frame.Body)
	if err != nil || hello.Purpose != ardp.PurposeReachability || lane.Activate(hello) != nil {
		return
	}
	var operationErr error
	admission, operationErr = server.serveAdmitted(ctx, secured, lane, frame)
	if operationErr == nil {
		status = 0
	}
	return
}

func (server *closedResolutionServer) serveAdmitted(ctx context.Context, connection net.Conn, lane *route.ClosedOuterBridgeLane, hello ardp.Frame) (*route.ClosedAdmission, error) {
	exporter, err := routecarrier.ClosedRoleTLSExporter(connection)
	if err != nil {
		return nil, err
	}
	channel, err := route.NewClosedAdmissionChannel(server.receiver, server.spends, server.limits, exporter,
		server.config.VerifyAdmission(server.receiver), server.config.Now)
	if err != nil {
		return nil, err
	}
	if _, err := channel.Accept(hello); err != nil {
		return nil, err
	}
	frame, err := ardp.ReadFrame(connection)
	if err != nil || frame.Kind != 2 || frame.Lane != 0 || len(frame.Body) != 355 || frame.Body[0] != 1 {
		return nil, errors.New("closed resolution requires Control admission")
	}
	lease, err := channel.Accept(frame)
	if err != nil {
		return nil, err
	}
	if err := lane.Admit(&lease, connection); err != nil {
		return &lease, err
	}
	if err := connection.SetDeadline(lease.Deadline); err != nil {
		return &lease, err
	}
	accepted, err := ardp.AcceptFrame(0, 64<<10)
	if err != nil {
		return &lease, err
	}
	used := uint64(16 + len(hello.Body) + 16 + len(frame.Body) + 16 + len(accepted.Body))
	if err := ardp.WriteFrame(connection, accepted); err != nil {
		return &lease, err
	}
	operation, err := ardp.ReadFrame(connection)
	if err != nil || operation.Kind != 10 || operation.Lane != 0 {
		return &lease, errors.New("closed resolution operation is invalid")
	}
	used += uint64(16 + len(operation.Body) + 16 + (16 << 10))
	if used > lease.Bytes || ctx.Err() != nil || !server.config.Now().Before(lease.Deadline) || !server.current() {
		return &lease, errors.New("closed resolution admission ended")
	}
	request, err := terminal.DecodeDescriptorRequest(operation.Body)
	if err != nil {
		return &lease, err
	}
	result, err := server.resolve(request, server.config.Now())
	if err != nil {
		return &lease, err
	}
	if ctx.Err() != nil || !server.current() || !server.config.Now().Before(lease.Deadline) {
		return &lease, errors.New("closed resolution ended before acknowledgement")
	}
	return &lease, ardp.WriteFrame(connection, ardp.Frame{Kind: 11, Body: result})
}

func (server *closedResolutionServer) resolve(request terminal.DescriptorRequest, now time.Time) ([]byte, error) {
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
	return terminal.EncodeDescriptorResult(request.Nonce, status, proof)
}

func (server *closedResolutionServer) currentIntroduction(introduction reachability.PrivateIntroduction, now time.Time) bool {
	if !server.current() || introduction.NotAfter.After(server.receiver.NotAfter) {
		return false
	}
	snapshot, err := server.config.CurrentDuty()
	if err != nil {
		return false
	}
	view, err := server.config.Authority.CurrentRoute()
	if err != nil || !authority.ProfileMatchesSnapshot(view.Profile, snapshot, now) {
		return false
	}
	matches := 0
	for index := uint8(0); index < view.NodeCount; index++ {
		node := view.Nodes[index]
		if node.NodeID != introduction.NodeID || !route.ClosedPurposePermitsDuty(ardp.PurposeIntroduction, node.RoleDomain, node.Subrole) {
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
