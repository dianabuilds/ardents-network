package node

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const closedIntroductionDeliveryBytes = uint64(16 + 4096 + 16 + 16384 + 16 + 1)
const closedIntroductionWithdrawalBytes = uint64(16 + 4096 + 16 + 16384)

// All mutable admission state is guarded by the server slot mutex. The writer
// reservation serializes complete frames without blocking that mutex.
type closedIntroductionSlot struct {
	dispatched    [4]time.Time
	inFlight      int
	request       route.ClosedRegistrationRequest
	connection    net.Conn
	writer        chan struct{}
	done          chan struct{}
	active        bool
	next          uint32
	used, maximum uint64
	openings      [4]time.Time
	pending       map[uint32]*closedIntroductionDelivery
}
type closedIntroductionDelivery struct {
	nonce        [32]byte
	end          time.Time
	acknowledged bool
	result       chan uint8
}

func (slot *closedIntroductionSlot) write(ctx context.Context, frame route.ClosedLaneFrame, end time.Time) error {
	bounded, cancel := context.WithDeadline(ctx, end)
	defer cancel()
	select {
	case slot.writer <- struct{}{}:
	case <-bounded.Done():
		return bounded.Err()
	case <-slot.done:
		return errors.New("closed Introduction registration ended")
	}
	defer func() { <-slot.writer }()
	if bounded.Err() != nil {
		return bounded.Err()
	}
	select {
	case <-slot.done:
		return errors.New("closed Introduction registration ended")
	default:
	}
	return slot.writeReserved(bounded, frame, end)
}

func (server *closedIntroductionServer) submit(ctx context.Context, connection net.Conn, lease route.ClosedAdmission, used uint64) error {
	frame, err := route.ReadClosedLaneFrame(connection)
	if err != nil || frame.Kind != 10 || frame.Lane != 0 {
		return errors.New("closed Introduction submission required")
	}
	nonce, capsule, err := route.DecodeClosedIntroductionSubmission(frame.Body)
	if err != nil {
		return err
	}
	defer clear(capsule.Ciphertext)
	now := server.config.now()
	if !now.Before(capsule.Expiry) || capsule.Expiry.After(lease.Deadline) ||
		capsule.Expiry.After(now.Add(10*time.Second)) || !server.current() || ctx.Err() != nil {
		return errors.New("closed Introduction submission expired or unavailable")
	}
	if used+uint64(16+len(frame.Body)+16+16384) > lease.Bytes {
		return errors.New("closed Introduction Control budget exhausted")
	}
	status := server.deliver(ctx, capsule)
	if !server.current() || ctx.Err() != nil || !server.config.now().Before(capsule.Expiry) {
		return errors.New("closed Introduction submission ended before acknowledgement")
	}
	body, err := route.EncodeClosedDescriptorResult(nonce, status, nil)
	if err != nil {
		return err
	}
	return route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 11, Body: body})
}

func (server *closedIntroductionServer) deliver(ctx context.Context, capsule route.ClosedIntroductionCapsule) uint8 {
	// A request nonce belongs to one TLS channel. Only the sealed envelope
	// crosses this hop unchanged; the source nonce is answered on its channel.
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return 1
	}
	operation, err := route.EncodeClosedIntroductionSubmission(nonce, capsule)
	if err != nil {
		return 1
	}
	defer clear(operation)
	server.slotsMu.Lock()
	slot := server.slots[capsule.Slot]
	now := server.config.now()
	if slot == nil || !slot.active || slot.request.Revision != capsule.Revision || capsule.Expiry.After(slot.request.Expiry) ||
		!now.Before(capsule.Expiry) || slot.inFlight >= 16 || slot.used+closedIntroductionDeliveryBytes > slot.maximum ||
		now.Before(slot.openings[3]) || now.Before(slot.openings[0].Add(time.Second)) {
		server.slotsMu.Unlock()
		return 1
	}
	slot.inFlight++
	slot.used += closedIntroductionDeliveryBytes
	copy(slot.openings[:3], slot.openings[1:])
	slot.openings[3] = now
	server.slotsMu.Unlock()
	var lane uint32
	defer func() {
		server.slotsMu.Lock()
		slot.inFlight--
		delete(slot.pending, lane)
		server.slotsMu.Unlock()
	}()
	bounded, cancel := context.WithDeadline(ctx, capsule.Expiry)
	defer cancel()
	// Allocate the even ID only after owning the writer. Concurrent admitted
	// submissions therefore cannot put lane 4 before lane 2 on the wire.
	select {
	case slot.writer <- struct{}{}:
	case <-slot.done:
		return 1
	case <-bounded.Done():
		return 1
	}
	server.slotsMu.Lock()
	now = server.config.now()
	if !slot.active || bounded.Err() != nil || now.Before(slot.dispatched[3]) || now.Before(slot.dispatched[0].Add(time.Second)) {
		server.slotsMu.Unlock()
		<-slot.writer
		return 1
	}
	copy(slot.dispatched[:3], slot.dispatched[1:])
	slot.dispatched[3] = now
	slot.next += 2
	lane = slot.next
	delivery := &closedIntroductionDelivery{nonce: nonce, end: capsule.Expiry, result: make(chan uint8, 1)}
	slot.pending[lane] = delivery
	server.slotsMu.Unlock()
	err = slot.writeReserved(bounded, route.ClosedLaneFrame{Kind: 10, Lane: lane, Body: operation}, capsule.Expiry)
	<-slot.writer
	if err != nil {
		server.interruptSlot(slot)
		return 1
	}
	var status uint8
	select {
	case status = <-delivery.result:
	case <-slot.done:
		return 1
	case <-bounded.Done():
		server.interruptSlot(slot)
		return 1
	}
	if !server.current() || !server.config.now().Before(capsule.Expiry) {
		status = 1
	}
	if err := slot.write(bounded, route.ClosedLaneFrame{Kind: 9, Lane: lane, Body: []byte{status}}, capsule.Expiry); err != nil {
		server.interruptSlot(slot)
		return 1
	}
	return status
}
func (server *closedIntroductionServer) interruptSlot(slot *closedIntroductionSlot) {
	server.retireSlot(slot)
	_ = slot.connection.SetDeadline(server.config.now())
}

func (server *closedIntroductionServer) serveRegistration(ctx context.Context, slot *closedIntroductionSlot) error {
	for {
		operation, err := route.ReadClosedLaneFrame(slot.connection)
		if err != nil {
			return err
		}
		if ctx.Err() != nil || !server.current() || !server.config.now().Before(slot.request.Expiry) {
			return errors.New("closed Introduction registration expired")
		}
		if operation.Kind == 11 && operation.Lane != 0 && operation.Lane%2 == 0 {
			server.slotsMu.Lock()
			pending := slot.pending[operation.Lane]
			if pending == nil || pending.acknowledged || !server.config.now().Before(pending.end) {
				server.slotsMu.Unlock()
				return errors.New("closed Introduction delivery acknowledgement unavailable")
			}
			status, proof, err := route.DecodeClosedDescriptorResult(operation.Body, pending.nonce)
			if err != nil || len(proof) != 0 {
				server.slotsMu.Unlock()
				return errors.New("closed Introduction delivery acknowledgement invalid")
			}
			pending.acknowledged = true
			pending.result <- status
			server.slotsMu.Unlock()
			continue
		}
		if operation.Kind != 10 || operation.Lane != 0 {
			return errors.New("closed Introduction owning withdrawal required")
		}
		withdraw, err := route.DecodeClosedRegistrationRequest(operation.Body)
		if err != nil || !withdraw.Withdraw || withdraw.Nonce == slot.request.Nonce ||
			withdraw.Slot != slot.request.Slot || withdraw.Revision != slot.request.Revision {
			return errors.New("closed Introduction withdrawal binding invalid")
		}
		server.retireSlot(slot)
		body, err := route.EncodeClosedDescriptorResult(withdraw.Nonce, 0, nil)
		if err != nil {
			return err
		}
		// Withdrawal itself consumes the termination reservation held at REGISTER.
		// Exclude concurrent delivery writers before the owning result and EOF.
		bounded, cancel := context.WithDeadline(ctx, slot.request.Expiry)
		defer cancel()
		select {
		case slot.writer <- struct{}{}:
		case <-bounded.Done():
			return bounded.Err()
		}
		defer func() { <-slot.writer }()
		if err := slot.connection.SetWriteDeadline(slot.request.Expiry); err != nil {
			return err
		}
		return route.WriteClosedLaneFrame(slot.connection, route.ClosedLaneFrame{Kind: 11, Body: body})
	}
}

func (slot *closedIntroductionSlot) writeReserved(ctx context.Context, frame route.ClosedLaneFrame, end time.Time) (outcome error) {
	if err := slot.connection.SetWriteDeadline(end); err != nil {
		return err
	}
	interrupted := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		interruptErr = slot.connection.SetWriteDeadline(time.Now())
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, ctx.Err(), interruptErr)
	}()
	return route.WriteClosedLaneFrame(slot.connection, frame)
}
