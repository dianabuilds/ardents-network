package route

import (
	"encoding/binary"
	"errors"
	"time"
)

const (
	closedOuterHandshakeBytes = 4 << 10
)

// ClosedOuterReceiver is the complete public State binding accepted by one
// successor Node Carrier before it allocates an inner TLS handshake lane.
type ClosedOuterReceiver struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest, NodeID [32]byte
	DutyGeneration                                                 uint64
	Deadline                                                       time.Time
	AllowedPurposes                                                [8]bool
}

// ClosedOuterHandshake accepts only the public outer HELLO and a bounded
// child handshake allocation. It has no Endpoint admission, bootstrap result,
// Target, or data-forwarding authority.
type ClosedOuterHandshake struct {
	receiver ClosedOuterReceiver
	clock    func() time.Time
	hello    bool
	children map[uint32]closedOuterChild
}

type closedOuterChild struct {
	deadline time.Time
	bytes    uint32
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
func NewClosedOuterHandshake(receiver ClosedOuterReceiver, clock func() time.Time) (*ClosedOuterHandshake, error) {
	if clock == nil || !validClosedOuterReceiver(receiver) {
		return nil, errors.New("closed outer handshake receiver is invalid")
	}
	return &ClosedOuterHandshake{receiver: receiver, clock: clock, children: make(map[uint32]closedOuterChild)}, nil
}

// Accept consumes one frame and returns only newly accepted opaque inner TLS
// bytes. A caller must give those bytes to the separately authenticated inner
// TLS state; this method never treats them as Application Data.
func (handshake *ClosedOuterHandshake) Accept(frame ClosedLaneFrame) ([]byte, error) {
	if handshake == nil || !handshake.clock().UTC().Before(handshake.receiver.Deadline) {
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
		!handshake.receiver.AllowedPurposes[open.Purpose] || !open.Deadline.After(now) || open.Deadline.After(handshake.receiver.Deadline) ||
		open.Deadline.After(now.Add(10*time.Second)) {
		return errors.New("closed outer OPEN is unavailable")
	}
	if _, exists := handshake.children[frame.Lane]; exists {
		return errors.New("closed outer child lane is reused")
	}
	handshake.children[frame.Lane] = closedOuterChild{deadline: open.Deadline}
	return nil
}

func (handshake *ClosedOuterHandshake) bytes(frame ClosedLaneFrame) ([]byte, error) {
	child, exists := handshake.children[frame.Lane]
	if !exists || len(frame.Body) == 0 || !handshake.clock().UTC().Before(child.deadline) || uint64(child.bytes)+uint64(len(frame.Body)) > closedOuterHandshakeBytes {
		return nil, errors.New("closed outer handshake bytes are unavailable")
	}
	child.bytes += uint32(len(frame.Body))
	handshake.children[frame.Lane] = child
	return append([]byte(nil), frame.Body...), nil
}

func validClosedOuterReceiver(receiver ClosedOuterReceiver) bool {
	return receiver.NetworkID != [32]byte{} && receiver.StateGeneration != [32]byte{} && receiver.StateDigest != [32]byte{} &&
		receiver.ProfileDigest != [32]byte{} && receiver.NodeID != [32]byte{} && receiver.DutyGeneration != 0 &&
		!receiver.Deadline.IsZero() && receiver.Deadline == receiver.Deadline.UTC().Truncate(time.Second)
}
