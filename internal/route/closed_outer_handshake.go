package route

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

const (
	closedOuterHandshakeBytes = 4 << 10
	closedOuterLaneCredit     = 64 << 10
)

// ClosedOuterReceiver is the complete public State binding accepted by one
// successor Node Carrier before it allocates an inner TLS handshake lane.
type ClosedOuterReceiver struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest, NodeID, RecordDigest [32]byte
	DutyGeneration                                                               uint64
	RoleDomain, Subrole                                                          uint8
	Deadline                                                                     time.Time
}

// ClosedOuterHandshake accepts only the public outer HELLO and a bounded
// child handshake allocation. It has no Endpoint admission, bootstrap result,
// Target, or data-forwarding authority.
type ClosedOuterHandshake struct {
	mu       sync.Mutex
	receiver ClosedOuterReceiver
	clock    func() time.Time
	duty     *closedDutyChannel
	hello    bool
	children map[uint32]closedOuterChild
}

type closedOuterChild struct {
	deadline              time.Time
	purpose               ClosedPurpose
	bytes, credit, queued uint32
	active, eof           bool
}

// ClosedOpen names the sole receiving Node/duty for a child inner TLS
// handshake. It cannot instruct the outer carrier to dial another peer.
type ClosedOpen struct {
	NextNodeID         [32]byte
	NextDutyGeneration uint64
	Purpose            ClosedPurpose
	Deadline           time.Time
}

// NewClosedOuterHandshake creates one unauthenticated outer state machine.
func NewClosedOuterHandshake(receiver ClosedOuterReceiver, limits *ClosedDutyLimits, clock func() time.Time) (*ClosedOuterHandshake, error) {
	if clock == nil || !validClosedOuterReceiver(receiver) {
		return nil, errors.New("closed outer handshake receiver is invalid")
	}
	duty, err := limits.reserveChannel()
	if err != nil {
		return nil, errors.New("closed outer handshake receiver is unavailable")
	}
	return &ClosedOuterHandshake{receiver: receiver, clock: clock, duty: duty, children: make(map[uint32]closedOuterChild)}, nil
}

// Close releases all child and queued-byte reservations. It is required when
// the Node Carrier closes, expires or is withdrawn.
func (handshake *ClosedOuterHandshake) Close() {
	if handshake == nil {
		return
	}
	handshake.mu.Lock()
	defer handshake.mu.Unlock()
	if handshake.duty == nil {
		return
	}
	for lane, child := range handshake.children {
		if child.queued != 0 {
			handshake.duty.limits.dequeue(uint64(child.queued))
		}
		delete(handshake.children, lane)
	}
	handshake.duty.release()
	handshake.duty = nil
}

// Accept consumes one frame and returns only newly accepted opaque inner TLS
// bytes. A caller must give those bytes to the separately authenticated inner
// TLS state; this method never treats them as Application Data.
func (handshake *ClosedOuterHandshake) Accept(frame ClosedLaneFrame) ([]byte, error) {
	if handshake == nil {
		return nil, errors.New("closed outer handshake is unavailable")
	}
	handshake.mu.Lock()
	defer handshake.mu.Unlock()
	if handshake.duty == nil || !handshake.clock().UTC().Before(handshake.receiver.Deadline) {
		return nil, errors.New("closed outer handshake is unavailable")
	}
	if !handshake.hello {
		if frame.Kind != closedFrameHello || frame.Lane != 0 {
			return nil, errors.New("closed outer HELLO is required")
		}
		hello, err := DecodeClosedHello(frame.Body)
		if err != nil || !handshake.matchesHello(hello) {
			return nil, errors.New("closed outer HELLO is unavailable")
		}
		handshake.hello = true
		return nil, nil
	}
	switch frame.Kind {
	case closedFrameOpen:
		return nil, handshake.open(frame)
	case closedFrameBytes:
		return handshake.bytes(frame)
	case closedFrameEOF:
		return nil, handshake.eof(frame)
	case closedFrameClose:
		return nil, handshake.close(frame)
	default:
		return nil, errors.New("closed outer frame is unavailable")
	}
}

// EncodeClosedOpen serializes one exact 49-byte OPEN body.
func EncodeClosedOpen(open ClosedOpen) ([]byte, error) {
	if open.NextNodeID == [32]byte{} || open.NextDutyGeneration == 0 || !validClosedPurpose(open.Purpose) || open.Deadline.IsZero() ||
		open.Deadline != open.Deadline.UTC().Truncate(time.Second) {
		return nil, errors.New("closed OPEN is invalid")
	}
	body := make([]byte, 0, 49)
	body = append(body, open.NextNodeID[:]...)
	body = binary.BigEndian.AppendUint64(body, open.NextDutyGeneration)
	body = append(body, byte(open.Purpose))
	return binary.BigEndian.AppendUint64(body, uint64(open.Deadline.Unix())), nil
}

// DecodeClosedOpen parses only the fixed 49-byte successor OPEN body.
func DecodeClosedOpen(body []byte) (ClosedOpen, error) {
	if len(body) != 49 {
		return ClosedOpen{}, errors.New("closed OPEN length is invalid")
	}
	open := ClosedOpen{NextDutyGeneration: binary.BigEndian.Uint64(body[32:40]), Purpose: ClosedPurpose(body[40]),
		Deadline: time.Unix(int64(binary.BigEndian.Uint64(body[41:49])), 0).UTC()}
	copy(open.NextNodeID[:], body[:32])
	if _, err := EncodeClosedOpen(open); err != nil {
		return ClosedOpen{}, err
	}
	return open, nil
}

func (handshake *ClosedOuterHandshake) matchesHello(hello ClosedHello) bool {
	return hello.NetworkID == handshake.receiver.NetworkID && hello.StateGeneration == handshake.receiver.StateGeneration &&
		hello.StateDigest == handshake.receiver.StateDigest && hello.ProfileDigest == handshake.receiver.ProfileDigest &&
		hello.RecipientNodeID == handshake.receiver.NodeID && hello.RecipientDutyGeneration == handshake.receiver.DutyGeneration &&
		hello.Purpose == ClosedPurposeForwarding && !hello.Deadline.After(handshake.receiver.Deadline)
}

func (handshake *ClosedOuterHandshake) open(frame ClosedLaneFrame) error {
	if frame.Lane == 0 || frame.Lane%2 == 0 {
		return errors.New("closed outer child lane is invalid")
	}
	open, err := DecodeClosedOpen(frame.Body)
	now := handshake.clock().UTC()
	if err != nil || open.NextNodeID != handshake.receiver.NodeID || open.NextDutyGeneration != handshake.receiver.DutyGeneration ||
		!ClosedPurposePermitsDuty(open.Purpose, handshake.receiver.RoleDomain, handshake.receiver.Subrole) || !open.Deadline.After(now) || open.Deadline.After(handshake.receiver.Deadline) ||
		open.Deadline.After(now.Add(10*time.Second)) {
		return errors.New("closed outer OPEN is unavailable")
	}
	if _, exists := handshake.children[frame.Lane]; exists || handshake.duty.reserveChild() != nil {
		return errors.New("closed outer child lane is reused")
	}
	handshake.children[frame.Lane] = closedOuterChild{deadline: open.Deadline, purpose: open.Purpose, credit: closedOuterLaneCredit}
	return nil
}

// VerifyInnerHello binds the fresh TLS channel in one allocated outer lane to
// the exact purpose that its OPEN selected. The caller invokes it only after
// the opaque bytes have completed a separately authenticated inner TLS
// handshake, and still passes that HELLO to the receiving role admission.
func (handshake *ClosedOuterHandshake) VerifyInnerHello(lane uint32, hello ClosedHello) error {
	if handshake == nil {
		return errors.New("closed inner HELLO is unavailable")
	}
	handshake.mu.Lock()
	defer handshake.mu.Unlock()
	if handshake.duty == nil || !handshake.clock().UTC().Before(handshake.receiver.Deadline) {
		return errors.New("closed inner HELLO is unavailable")
	}
	child, exists := handshake.children[lane]
	if !exists || !validClosedHello(hello) || hello.NetworkID != handshake.receiver.NetworkID || hello.StateGeneration != handshake.receiver.StateGeneration ||
		hello.StateDigest != handshake.receiver.StateDigest || hello.ProfileDigest != handshake.receiver.ProfileDigest ||
		hello.RecipientNodeID != handshake.receiver.NodeID || hello.RecipientDutyGeneration != handshake.receiver.DutyGeneration ||
		hello.Purpose != child.purpose || !hello.Deadline.After(handshake.clock().UTC()) || hello.Deadline.After(child.deadline) {
		return errors.New("closed inner HELLO is unavailable")
	}
	child.active = true
	handshake.children[lane] = child
	return nil
}

func (handshake *ClosedOuterHandshake) bytes(frame ClosedLaneFrame) ([]byte, error) {
	child, exists := handshake.children[frame.Lane]
	if !exists || len(frame.Body) == 0 || child.eof || !handshake.clock().UTC().Before(child.deadline) {
		return nil, errors.New("closed outer handshake bytes are unavailable")
	}
	bytes := uint32(len(frame.Body))
	if !child.active {
		if child.bytes+bytes > closedOuterHandshakeBytes {
			return nil, errors.New("closed outer handshake bytes are unavailable")
		}
		child.bytes += bytes
	} else if bytes > child.credit || child.queued+bytes > closedOuterLaneCredit || handshake.duty.limits.queue(uint64(bytes)) != nil {
		return nil, errors.New("closed outer lane bytes are unavailable")
	} else {
		child.credit -= bytes
		child.queued += bytes
	}
	handshake.children[frame.Lane] = child
	return append([]byte(nil), frame.Body...), nil
}

func (handshake *ClosedOuterHandshake) eof(frame ClosedLaneFrame) error {
	child, exists := handshake.children[frame.Lane]
	if !exists || !child.active || child.eof || len(frame.Body) != 0 {
		return errors.New("closed outer lane EOF is unavailable")
	}
	child.eof = true
	handshake.children[frame.Lane] = child
	return nil
}

func (handshake *ClosedOuterHandshake) close(frame ClosedLaneFrame) error {
	child, exists := handshake.children[frame.Lane]
	if !exists || !child.active || len(frame.Body) != 1 || frame.Body[0] > 6 {
		return errors.New("closed outer lane close is unavailable")
	}
	if child.queued != 0 {
		handshake.duty.limits.dequeue(uint64(child.queued))
	}
	handshake.duty.releaseChild()
	delete(handshake.children, frame.Lane)
	return nil
}

// ConsumeInnerBytes releases receive credit only after the receiving inner
// TLS/role consumer has taken the opaque bytes from its bounded lane queue.
func (handshake *ClosedOuterHandshake) ConsumeInnerBytes(lane uint32, bytes uint32) (ClosedLaneFrame, error) {
	if handshake == nil {
		return ClosedLaneFrame{}, errors.New("closed outer lane credit is unavailable")
	}
	handshake.mu.Lock()
	defer handshake.mu.Unlock()
	if handshake.duty == nil || bytes == 0 || !handshake.clock().UTC().Before(handshake.receiver.Deadline) {
		return ClosedLaneFrame{}, errors.New("closed outer lane credit is unavailable")
	}
	child, exists := handshake.children[lane]
	if !exists || !child.active || child.eof || bytes > child.queued || bytes > closedOuterLaneCredit-child.credit {
		return ClosedLaneFrame{}, errors.New("closed outer lane credit is unavailable")
	}
	child.queued -= bytes
	child.credit += bytes
	handshake.duty.limits.dequeue(uint64(bytes))
	handshake.children[lane] = child
	return ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane, Body: binary.BigEndian.AppendUint32(nil, bytes)}, nil
}

func validClosedOuterReceiver(receiver ClosedOuterReceiver) bool {
	return receiver.NetworkID != [32]byte{} && receiver.StateGeneration != [32]byte{} && receiver.StateDigest != [32]byte{} &&
		receiver.ProfileDigest != [32]byte{} && receiver.NodeID != [32]byte{} && receiver.RecordDigest != [32]byte{} && receiver.DutyGeneration != 0 &&
		validClosedDutyAssignment(receiver.RoleDomain, receiver.Subrole) && !receiver.Deadline.IsZero() && receiver.Deadline == receiver.Deadline.UTC().Truncate(time.Second)
}
