package node

import (
	"context"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (server *closedDataJoinServer) current() bool {
	snapshot, err := currentFacts(server.config)
	if err != nil {
		return false
	}
	receiver, ok := closedRouteReceiver(server.config, snapshot, route.ClosedPurposeDataJoin, server.config.now())
	return ok && receiver == server.receiver
}

func (server *closedDataJoinServer) serveOuter(ctx context.Context, carrier route.ClosedSharedCarrier) {
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

func (server *closedDataJoinServer) serveInner(ctx context.Context, lane *route.ClosedOuterBridgeLane) {
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
	if err != nil || hello.Purpose != route.ClosedPurposeDataJoin || lane.Activate(hello) != nil {
		return
	}
	if server.serveAdmitted(ctx, secured, lane, frame) == nil {
		status = 0
	}
}

func (server *closedDataJoinServer) serveAdmitted(ctx context.Context, connection net.Conn, lane *route.ClosedOuterBridgeLane, hello route.ClosedLaneFrame) error {
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
	if err != nil || frame.Kind != 2 || frame.Lane != 0 || len(frame.Body) != 355 || frame.Body[0] != 2 {
		return errors.New("closed JOIN requires Data admission")
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
	if err := route.WriteClosedLaneFrame(connection, accepted); err != nil {
		return err
	}
	if ctx.Err() != nil || !server.current() {
		return errors.New("closed JOIN State ended")
	}
	return server.pairs.AcceptStream(ctx, &lease, connection)
}
