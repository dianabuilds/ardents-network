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
	mu        sync.Mutex
	handshake *ClosedOuterHandshake
	write     func(ClosedLaneFrame) error
	lanes     map[uint32]*closedOuterBridgeLane
	closed    bool
}

// ClosedOuterBridgeLane is one locally owned inner TLS byte stream. The owner
// must call Activate only after its TLS peer has supplied the inner HELLO.
type ClosedOuterBridgeLane struct{ lane *closedOuterBridgeLane }

type closedOuterBridgeLane struct {
	bridge                             *ClosedOuterBridge
	id                                 uint32
	mu                                 sync.Mutex
	buffer                             []byte
	notify                             chan struct{}
	innerHello, active, inputEOF, dead bool
	unaccounted                        uint32
	outboundCredit                     uint32
	readDeadline                       time.Time
}

// NewClosedOuterBridge binds one outer state machine to one serialized ARDP
// writer. The writer must reject partial frames and must not call back into the
// bridge while it holds its own transport lock.
func NewClosedOuterBridge(handshake *ClosedOuterHandshake, write func(ClosedLaneFrame) error) (*ClosedOuterBridge, error) {
	if handshake == nil || write == nil {
		return nil, errors.New("closed outer bridge is invalid")
	}
	return &ClosedOuterBridge{handshake: handshake, write: write, lanes: make(map[uint32]*closedOuterBridgeLane)}, nil
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
		return nil, bridge.write(accepted)
	case closedFrameOpen:
		lane := &closedOuterBridgeLane{bridge: bridge, id: frame.Lane, notify: make(chan struct{}, 1), outboundCredit: closedOuterLaneCredit}
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
			lane.closeInput()
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
	if inner.dead || inner.active || len(inner.buffer) != 0 {
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
				if err := inner.bridge.write(credit); err != nil {
					return 0, err
				}
			}
			return count, nil
		}
		if inner.dead || inner.inputEOF {
			inner.mu.Unlock()
			return 0, io.EOF
		}
		deadline := inner.readDeadline
		inner.mu.Unlock()
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

func (lane *ClosedOuterBridgeLane) Write(value []byte) (int, error) {
	if lane == nil || lane.lane == nil || len(value) == 0 {
		return 0, errors.New("closed outer bridge write is invalid")
	}
	inner := lane.lane
	inner.mu.Lock()
	defer inner.mu.Unlock()
	if inner.dead || uint64(len(value)) > uint64(inner.outboundCredit) {
		return 0, errors.New("closed outer bridge write is unavailable")
	}
	for remaining := value; len(remaining) != 0; {
		count := len(remaining)
		if count > closedLaneMaximum {
			count = closedLaneMaximum
		}
		if err := inner.bridge.write(ClosedLaneFrame{Kind: closedFrameBytes, Lane: inner.id, Body: append([]byte(nil), remaining[:count]...)}); err != nil {
			return 0, err
		}
		remaining = remaining[count:]
	}
	inner.outboundCredit -= uint32(len(value))
	return len(value), nil
}

func (lane *ClosedOuterBridgeLane) Close() error {
	return lane.CloseWithStatus(1)
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
	return lane.SetReadDeadline(deadline)
}
func (lane *ClosedOuterBridgeLane) SetWriteDeadline(time.Time) error { return nil }
func (lane *ClosedOuterBridgeLane) SetReadDeadline(deadline time.Time) error {
	if lane == nil || lane.lane == nil {
		return errors.New("closed outer bridge deadline is invalid")
	}
	lane.lane.mu.Lock()
	lane.lane.readDeadline = deadline
	lane.lane.mu.Unlock()
	return nil
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
	delete(bridge.lanes, lane.id)
	lane.closeInput()
	return bridge.write(ClosedLaneFrame{Kind: closedFrameClose, Lane: lane.id, Body: []byte{status}})
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

func (lane *closedOuterBridgeLane) closeInput() {
	lane.mu.Lock()
	lane.dead = true
	lane.buffer = nil
	lane.mu.Unlock()
	select {
	case lane.notify <- struct{}{}:
	default:
	}
}

type closedOuterBridgeAddr struct{}

func (closedOuterBridgeAddr) Network() string { return "ardp" }
func (closedOuterBridgeAddr) String() string  { return "closed-outer-lane" }

var _ net.Conn = (*ClosedOuterBridgeLane)(nil)
