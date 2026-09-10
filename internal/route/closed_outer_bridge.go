package route

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// ClosedOuterBridge owns one authenticated outer Carrier's framing boundary.
// It exposes each accepted child as an opaque inner net.Conn; it never parses
// TLS or role bytes itself.
type ClosedOuterBridge struct {
	retired             [closedForwardChildren]uint32
	retiredNext         uint32
	mu                  sync.Mutex
	handshake           *ClosedOuterHandshake
	write               func(ClosedLaneFrame, func() time.Time) error
	updateWriteDeadline func(uint32, time.Time) error
	lanes               map[uint32]*closedOuterBridgeLane
	closed              bool
}

// ClosedOuterBridgeLane is one locally owned inner TLS byte stream. The owner
// must call Activate only after its TLS peer has supplied the inner HELLO.
type ClosedOuterBridgeLane struct{ lane *closedOuterBridgeLane }

type closedOuterBridgeLane struct {
	bridge                             *ClosedOuterBridge
	id                                 uint32
	restriction                        ClosedChildRestriction
	mu                                 sync.Mutex
	buffer                             []byte
	notify                             chan struct{}
	innerHello, active, inputEOF, dead bool
	unaccounted                        uint32
	successfulClose                    bool
	outboundCredit                     uint32
	outboundWriter                     sync.Mutex
	outboundChanged                    chan struct{}
	deadlineMu                         sync.Mutex
	writeUpdate                        sync.Mutex
	readDeadline                       time.Time
	writeDeadline                      time.Time
	authorizedUntil                    time.Time
	hardDeadline                       time.Time
}

// NewClosedOuterBridge binds one outer state machine to one serialized ARDP
// writer. The writer must reject partial frames and must not call back into the
// bridge while it holds its own transport lock.
func NewClosedOuterBridge(handshake *ClosedOuterHandshake, update func(uint32, time.Time) error, write func(ClosedLaneFrame, func() time.Time) error) (*ClosedOuterBridge, error) {
	if handshake == nil || write == nil || update == nil {
		return nil, errors.New("closed outer bridge is invalid")
	}
	return &ClosedOuterBridge{handshake: handshake, write: write, updateWriteDeadline: update, lanes: make(map[uint32]*closedOuterBridgeLane)}, nil
}

// Accept consumes one outer frame. A successful OPEN returns the fresh child
// lane; callers give that lane only to the selected inner TLS/role owner.
func (bridge *ClosedOuterBridge) Accept(frame ClosedLaneFrame) (*ClosedOuterBridgeLane, error) {
	if bridge == nil {
		return nil, errors.New("closed outer bridge is unavailable")
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.closed {
		return nil, errors.New("closed outer bridge is unavailable")
	}
	if bridge.retiredFrame(frame) {
		return nil, nil
	}
	if frame.Kind == closedFrameCredit {
		return nil, bridge.credit(frame)
	}
	bytes, err := bridge.handshake.Accept(frame)
	if err != nil {
		return nil, err
	}
	switch frame.Kind {
	case closedFrameHello:
		accepted, err := ClosedAcceptFrame(0, closedOuterLaneCredit)
		if err != nil {
			return nil, err
		}
		return nil, bridge.write(accepted, func() time.Time {
			end := bridge.handshake.clock().UTC().Add(10 * time.Second)
			if bridge.handshake.receiver.Deadline.Before(end) {
				end = bridge.handshake.receiver.Deadline
			}
			return end
		})
	case closedFrameOpen:
		open, _, _ := DecodeClosedNodeOpen(frame.Body) // Already checked by the handshake.
		pending := bridge.handshake.clock().UTC().Add(10 * time.Second)
		if open.Deadline.Before(pending) {
			pending = open.Deadline
		}
		lane := &closedOuterBridgeLane{bridge: bridge, id: frame.Lane, restriction: ClosedChildRestriction(frame.Body[49]), notify: make(chan struct{}, 1), outboundCredit: closedOuterLaneCredit, outboundChanged: make(chan struct{}), hardDeadline: open.Deadline, readDeadline: pending, writeDeadline: pending, authorizedUntil: pending}
		bridge.lanes[frame.Lane] = lane
		return &ClosedOuterBridgeLane{lane: lane}, nil
	case closedFrameBytes:
		lane := bridge.lanes[frame.Lane]
		if lane == nil {
			return nil, errors.New("closed outer bridge lane is unavailable")
		}
		if err := lane.feed(bytes); err != nil {
			return nil, err
		}
	case closedFrameEOF:
		if lane := bridge.lanes[frame.Lane]; lane != nil {
			lane.finishInput()
		}
	case closedFrameClose:
		if lane := bridge.lanes[frame.Lane]; lane != nil {
			if err := lane.closeInput(); err != nil {
				return nil, err
			}
			bridge.rememberRetired(frame.Lane)
			delete(bridge.lanes, frame.Lane)
		}
	}
	return nil, nil
}

// Activate verifies the encrypted inner HELLO and turns the child into the
// ordinary credit-controlled post-TLS lane.
func (lane *ClosedOuterBridgeLane) Activate(hello ClosedHello) error {
	if lane == nil || lane.lane == nil {
		return errors.New("closed outer bridge lane is unavailable")
	}
	inner := lane.lane
	inner.mu.Lock()
	defer inner.mu.Unlock()
	if inner.dead || inner.active {
		return errors.New("closed outer bridge lane is unavailable")
	}
	if err := inner.bridge.handshake.VerifyInnerHello(inner.id, hello); err != nil {
		return err
	}
	inner.active = true
	return nil
}

func (lane *ClosedOuterBridgeLane) BeginInnerHello() error {
	if lane == nil || lane.lane == nil {
		return errors.New("closed outer bridge lane is unavailable")
	}
	inner := lane.lane
	// Accept accounts a frame before feeding its bytes. Keep the transition
	// and buffer snapshot in that same critical section.
	inner.bridge.mu.Lock()
	defer inner.bridge.mu.Unlock()
	inner.mu.Lock()
	defer inner.mu.Unlock()
	if inner.dead || inner.innerHello || inner.active {
		return errors.New("closed outer bridge lane is unavailable")
	}
	if err := inner.bridge.handshake.BeginInnerHello(inner.id); err != nil {
		return err
	}
	inner.innerHello = true
	inner.unaccounted = uint32(len(inner.buffer))
	return nil
}

func (lane *ClosedOuterBridgeLane) Read(value []byte) (int, error) {
	if lane == nil || lane.lane == nil || len(value) == 0 {
		return 0, errors.New("closed outer bridge read is invalid")
	}
	inner := lane.lane
	for {
		inner.mu.Lock()
		if len(inner.buffer) != 0 {
			count := copy(value, inner.buffer)
			inner.buffer = inner.buffer[count:]
			active := inner.innerHello
			credited := uint32(count)
			if credited <= inner.unaccounted {
				inner.unaccounted -= credited
				credited = 0
			} else {
				credited -= inner.unaccounted
				inner.unaccounted = 0
			}
			inner.mu.Unlock()
			if active && credited != 0 {
				credit, err := inner.bridge.handshake.ConsumeInnerBytes(inner.id, credited)
				if err != nil {
					return 0, err
				}
				if err := inner.bridge.write(credit, inner.currentWriteDeadline); err != nil {
					return 0, err
				}
			}
			return count, nil
		}
		if inner.dead || inner.inputEOF {
			inner.mu.Unlock()
			return 0, io.EOF
		}
		inner.mu.Unlock()
		inner.deadlineMu.Lock()
		deadline := inner.readDeadline
		inner.deadlineMu.Unlock()
		if deadline.IsZero() {
			<-inner.notify
			continue
		}
		wait := time.NewTimer(time.Until(deadline))
		select {
		case <-inner.notify:
			if !wait.Stop() {
				<-wait.C
			}
		case <-wait.C:
			return 0, errors.New("closed outer bridge read deadline exceeded")
		}
	}
}

func (lane *ClosedOuterBridgeLane) Close() error {
	status := byte(1)
	if lane != nil && lane.lane != nil {
		lane.lane.mu.Lock()
		if lane.lane.successfulClose {
			status = 0
		}
		lane.lane.mu.Unlock()
	}
	return lane.CloseWithStatus(status)
}

// CloseWithStatus terminates one locally owned lane and immediately releases
// its receiver reservations without closing unrelated Carrier children.
func (lane *ClosedOuterBridgeLane) CloseWithStatus(status byte) error {
	if lane == nil || lane.lane == nil {
		return nil
	}
	if status > 6 {
		return errors.New("closed outer bridge close status is invalid")
	}
	return lane.lane.bridge.closeLocal(lane.lane, status)
}

func (lane *ClosedOuterBridgeLane) LocalAddr() net.Addr  { return closedOuterBridgeAddr{} }
func (lane *ClosedOuterBridgeLane) RemoteAddr() net.Addr { return closedOuterBridgeAddr{} }
func (lane *ClosedOuterBridgeLane) SetDeadline(deadline time.Time) error {
	if err := lane.SetWriteDeadline(deadline); err != nil {
		return err
	}
	return lane.SetReadDeadline(deadline)
}
func (lane *ClosedOuterBridgeLane) SetWriteDeadline(deadline time.Time) error {
	if lane == nil || lane.lane == nil {
		return errors.New("closed outer bridge deadline is invalid")
	}
	inner := lane.lane
	inner.writeUpdate.Lock()
	defer inner.writeUpdate.Unlock()
	inner.deadlineMu.Lock()
	inner.writeDeadline = inner.boundDeadline(deadline)
	current := inner.writeDeadline
	inner.deadlineMu.Unlock()
	inner.mu.Lock()
	inner.signalOutboundLocked()
	inner.mu.Unlock()
	return inner.bridge.updateWriteDeadline(inner.id, current)
}
func (lane *ClosedOuterBridgeLane) SetReadDeadline(deadline time.Time) error {
	if lane == nil || lane.lane == nil {
		return errors.New("closed outer bridge deadline is invalid")
	}
	inner := lane.lane
	inner.deadlineMu.Lock()
	inner.readDeadline = inner.boundDeadline(deadline)
	inner.deadlineMu.Unlock()
	select {
	case inner.notify <- struct{}{}:
	default:
	}
	return nil
}
func (lane *closedOuterBridgeLane) boundDeadline(deadline time.Time) time.Time {
	ceiling := lane.authorizedUntil
	if lane.hardDeadline.Before(ceiling) {
		ceiling = lane.hardDeadline
	}
	if deadline.IsZero() || deadline.After(ceiling) {
		return ceiling
	}
	return deadline
}
func (lane *closedOuterBridgeLane) currentWriteDeadline() time.Time {
	lane.deadlineMu.Lock()
	defer lane.deadlineMu.Unlock()
	return lane.writeDeadline
}
func (bridge *ClosedOuterBridge) credit(frame ClosedLaneFrame) error {
	if frame.Lane == 0 || len(frame.Body) != 4 {
		return errors.New("closed outer bridge credit is invalid")
	}
	lane := bridge.lanes[frame.Lane]
	if lane == nil {
		return errors.New("closed outer bridge credit is unavailable")
	}
	bytes := binary.BigEndian.Uint32(frame.Body)
	lane.mu.Lock()
	defer lane.mu.Unlock()
	if !lane.active || lane.dead || bytes == 0 || bytes > closedOuterLaneCredit-lane.outboundCredit {
		return errors.New("closed outer bridge credit is unavailable")
	}
	lane.outboundCredit += bytes
	lane.signalOutboundLocked()
	return nil
}

func (bridge *ClosedOuterBridge) closeLocal(lane *closedOuterBridgeLane, status byte) error {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	if bridge.closed || bridge.lanes[lane.id] != lane {
		return nil
	}
	if _, err := bridge.handshake.Accept(ClosedLaneFrame{Kind: closedFrameClose, Lane: lane.id, Body: []byte{status}}); err != nil {
		return err
	}
	bridge.rememberRetired(lane.id)
	delete(bridge.lanes, lane.id)
	if err := lane.closeInput(); err != nil {
		return err
	}
	// Retirement may follow SetDeadline(now) used to interrupt child I/O.
	// The terminal control frame gets its own finite write bound; it carries
	// no child payload. It remains sendable after payload authority expires.
	return bridge.write(ClosedLaneFrame{Kind: closedFrameClose, Lane: lane.id, Body: []byte{status}}, func() time.Time { return time.Now().Add(time.Second) })
}

func (lane *closedOuterBridgeLane) feed(value []byte) error {
	lane.mu.Lock()
	defer lane.mu.Unlock()
	limit := closedOuterHandshakeBytes
	if lane.innerHello {
		limit = closedOuterLaneCredit
	}
	if lane.dead || lane.inputEOF || len(lane.buffer)+len(value) > limit {
		return errors.New("closed outer bridge input is unavailable")
	}
	lane.buffer = append(lane.buffer, value...)
	select {
	case lane.notify <- struct{}{}:
	default:
	}
	return nil
}

func (lane *closedOuterBridgeLane) finishInput() {
	lane.mu.Lock()
	lane.inputEOF = true
	lane.mu.Unlock()
	select {
	case lane.notify <- struct{}{}:
	default:
	}
}

func (lane *closedOuterBridgeLane) closeInput() error {
	lane.mu.Lock()
	lane.dead = true
	lane.buffer = nil
	lane.signalOutboundLocked()
	lane.mu.Unlock()
	select {
	case lane.notify <- struct{}{}:
	default:
	}
	return (&ClosedOuterBridgeLane{lane: lane}).SetWriteDeadline(time.Now())
}

type closedOuterBridgeAddr struct{}

func (closedOuterBridgeAddr) Network() string { return "ardp" }
func (closedOuterBridgeAddr) String() string  { return "closed-outer-lane" }

var _ net.Conn = (*ClosedOuterBridgeLane)(nil)

// A terminal sender may still receive in-flight opaque bytes or CREDIT for
// its just-closed child. Remember the last 256 exact allocated IDs in fixed
// storage, so those completions cannot tear down sibling lanes. An unknown,
// older-than-retention, malformed or reused OPEN still fails the usual parser.
func (bridge *ClosedOuterBridge) rememberRetired(id uint32) {
	bridge.retired[bridge.retiredNext%uint32(len(bridge.retired))] = id
	bridge.retiredNext++
}

func (bridge *ClosedOuterBridge) retiredFrame(frame ClosedLaneFrame) bool {
	if frame.Lane == 0 || !validClosedFrame(frame) {
		return false
	}
	if frame.Kind != closedFrameBytes && frame.Kind != closedFrameCredit && frame.Kind != closedFrameEOF && frame.Kind != closedFrameClose {
		return false
	}
	for _, id := range bridge.retired {
		if id == frame.Lane {
			return true
		}
	}
	return false
}
