package introduction

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	nodeouter "github.com/dianabuilds/ardents-network/internal/node/outer"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

func (server *Server) current() bool {
	snapshot, err := server.config.CurrentDuty()
	if err != nil {
		return false
	}
	receiver, ok := server.config.Authority.Receiver(snapshot, ardp.PurposeIntroduction, server.config.Now())
	return ok && receiver == server.receiver
}

func (server *Server) serveOuter(ctx context.Context, carrier routecarrier.ClosedSharedCarrier) {
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

func (server *Server) serveInner(ctx context.Context, lane *route.ClosedOuterBridgeLane) (cleanupErr error) {
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
	if err != nil || (hello.Purpose != ardp.PurposeIntroduction && hello.Purpose != ardp.PurposeSubmission) || lane.Activate(hello) != nil {
		return
	}
	var operationErr error
	admission, operationErr = server.serveAdmitted(ctx, secured, lane, frame)
	if operationErr == nil {
		status = 0
	}
	return
}

func (server *Server) serveAdmitted(ctx context.Context, connection net.Conn, lane *route.ClosedOuterBridgeLane, hello ardp.Frame) (*route.ClosedAdmission, error) {
	exporter, err := routecarrier.ClosedRoleTLSExporter(connection)
	if err != nil {
		return nil, err
	}
	facts, err := ardp.DecodeHello(hello.Body)
	if err != nil {
		return nil, err
	}
	receiver := server.receiver
	receiver.ExpectedPurpose = facts.Purpose
	class := byte(3)
	if facts.Purpose == ardp.PurposeSubmission {
		class = 1
	} else if facts.Purpose != ardp.PurposeIntroduction {
		return nil, errors.New("closed Introduction purpose unavailable")
	}
	channel, err := route.NewClosedAdmissionChannel(receiver, server.spends, server.limits, exporter,
		server.config.VerifyAdmission(receiver), server.config.Now)
	if err != nil {
		return nil, err
	}
	if _, err := channel.Accept(hello); err != nil {
		return nil, err
	}
	frame, err := ardp.ReadFrame(connection)
	if err != nil || frame.Kind != 2 || frame.Lane != 0 || len(frame.Body) != 355 || frame.Body[0] != class {
		return nil, errors.New("closed Introduction requires Publication admission")
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
	if class == 1 {
		return &lease, server.submit(ctx, connection, lease, used)
	}
	return &lease, server.register(ctx, connection, lease, used)
}

func (server *Server) register(ctx context.Context, connection net.Conn, lease route.ClosedAdmission, used uint64) error {
	operation, err := ardp.ReadFrame(connection)
	if err != nil || operation.Kind != 10 || operation.Lane != 0 {
		return errors.New("closed Introduction registration required")
	}
	request, err := terminal.DecodeRegistrationRequest(operation.Body)
	now := server.config.Now()
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
		result, err := terminal.EncodeDescriptorResult(request.Nonce, 1, nil)
		if err != nil {
			return err
		}
		return ardp.WriteFrame(connection, ardp.Frame{Kind: 11, Body: result})
	}
	defer server.retireSlot(slot)
	if err := connection.SetDeadline(request.Expiry); err != nil {
		return err
	}
	if ctx.Err() != nil || !server.current() || !server.config.Now().Before(request.Expiry) {
		return errors.New("closed Introduction ended before registration acknowledgement")
	}
	result, err := terminal.EncodeDescriptorResult(request.Nonce, 0, nil)
	if err != nil {
		return err
	}
	slot.writer <- struct{}{}
	server.slotsMu.Lock()
	slot.active = true
	server.slotsMu.Unlock()
	if err := ardp.WriteFrame(connection, ardp.Frame{Kind: 11, Body: result}); err != nil {
		server.retireSlot(slot)
		<-slot.writer
		return err
	}
	<-slot.writer
	return server.serveRegistration(ctx, slot)
}

func (server *Server) reserveSlot(slot *closedIntroductionSlot) bool {
	server.slotsMu.Lock()
	defer server.slotsMu.Unlock()
	now := server.config.Now()
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

func (server *Server) retireSlot(slot *closedIntroductionSlot) {
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
