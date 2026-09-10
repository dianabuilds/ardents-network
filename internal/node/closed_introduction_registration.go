package node

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func (server *closedIntroductionServer) current() bool {
	snapshot, err := currentFacts(server.config)
	if err != nil {
		return false
	}
	receiver, ok := closedRouteReceiver(server.config, snapshot, route.ClosedPurposeIntroduction, server.config.now())
	return ok && receiver == server.receiver
}

func (server *closedIntroductionServer) serveOuter(ctx context.Context, carrier route.ClosedSharedCarrier) {
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

func (server *closedIntroductionServer) serveInner(ctx context.Context, lane *route.ClosedOuterBridgeLane) {
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
	if err != nil || (hello.Purpose != route.ClosedPurposeIntroduction && hello.Purpose != route.ClosedPurposeSubmission) || lane.Activate(hello) != nil {
		return
	}
	if server.serveAdmitted(ctx, secured, lane, frame) == nil {
		status = 0
	}
}

func (server *closedIntroductionServer) serveAdmitted(ctx context.Context, connection net.Conn, lane *route.ClosedOuterBridgeLane, hello route.ClosedLaneFrame) error {
	exporter, err := route.ClosedRoleTLSExporter(connection)
	if err != nil {
		return err
	}
	facts, err := route.DecodeClosedHello(hello.Body)
	if err != nil {
		return err
	}
	receiver := server.receiver
	receiver.ExpectedPurpose = facts.Purpose
	class := byte(3)
	if facts.Purpose == route.ClosedPurposeSubmission {
		class = 1
	} else if facts.Purpose != route.ClosedPurposeIntroduction {
		return errors.New("closed Introduction purpose unavailable")
	}
	channel, err := route.NewClosedAdmissionChannel(receiver, server.spends, server.limits, exporter,
		closedRoleTokenVerifier(server.config, receiver), server.config.now)
	if err != nil {
		return err
	}
	if _, err := channel.Accept(hello); err != nil {
		return err
	}
	frame, err := route.ReadClosedLaneFrame(connection)
	if err != nil || frame.Kind != 2 || frame.Lane != 0 || len(frame.Body) != 355 || frame.Body[0] != class {
		return errors.New("closed Introduction requires Publication admission")
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
	if class == 1 {
		return server.submit(ctx, connection, lease, used)
	}
	return server.register(ctx, connection, lease, used)
}

func (server *closedIntroductionServer) register(ctx context.Context, connection net.Conn, lease route.ClosedAdmission, used uint64) error {
	operation, err := route.ReadClosedLaneFrame(connection)
	if err != nil || operation.Kind != 10 || operation.Lane != 0 {
		return errors.New("closed Introduction registration required")
	}
	request, err := route.DecodeClosedRegistrationRequest(operation.Body)
	now := server.config.now()
	if err != nil || request.Withdraw || !now.Before(request.Expiry) || request.Expiry.After(lease.Deadline) || request.Expiry.After(now.Add(600*time.Second)) ||
		ctx.Err() != nil || !server.current() {
		return errors.New("closed Introduction registration invalid")
	}
	used += uint64(16 + len(operation.Body) + 16 + (16 << 10))
	if used > lease.Bytes {
		return errors.New("closed Introduction budget exhausted")
	}
	slot := &closedIntroductionSlot{request: request, connection: connection, writer: make(chan struct{}, 1),
		done: make(chan struct{}), pending: make(map[uint32]*closedIntroductionDelivery), used: used + closedIntroductionWithdrawalBytes, maximum: lease.Bytes}
	if !server.reserveSlot(slot) {
		result, err := route.EncodeClosedDescriptorResult(request.Nonce, 1, nil)
		if err != nil {
			return err
		}
		return route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 11, Body: result})
	}
	defer server.retireSlot(slot)
	if err := connection.SetDeadline(request.Expiry); err != nil {
		return err
	}
	if ctx.Err() != nil || !server.current() || !server.config.now().Before(request.Expiry) {
		return errors.New("closed Introduction ended before registration acknowledgement")
	}
	result, err := route.EncodeClosedDescriptorResult(request.Nonce, 0, nil)
	if err != nil {
		return err
	}
	slot.writer <- struct{}{}
	server.slotsMu.Lock()
	slot.active = true
	server.slotsMu.Unlock()
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 11, Body: result}); err != nil {
		server.retireSlot(slot)
		<-slot.writer
		return err
	}
	<-slot.writer
	return server.serveRegistration(ctx, slot)
}

func (server *closedIntroductionServer) reserveSlot(slot *closedIntroductionSlot) bool {
	server.slotsMu.Lock()
	defer server.slotsMu.Unlock()
	now := server.config.now()
	for id, retained := range server.slots {
		if !now.Before(retained.request.Expiry) {
			delete(server.slots, id)
		}
	}
	// The same whole-duty 1024-channel ceiling bounds retained slot metadata.
	// Closing channels cannot open an unbounded tombstone allocation path.
	if _, exists := server.slots[slot.request.Slot]; exists || len(server.slots) >= 1024 {
		return false
	}
	if err := server.slotFloor.Claim(slot.request.Slot, slot.request.Expiry, now); err != nil {
		return false
	}
	server.slots[slot.request.Slot] = slot
	return true
}

func (server *closedIntroductionServer) retireSlot(slot *closedIntroductionSlot) {
	server.slotsMu.Lock()
	defer server.slotsMu.Unlock()
	if server.slots[slot.request.Slot] == slot {
		slot.active = false
		select {
		case <-slot.done:
		default:
			close(slot.done)
		}
	}
}
