package route

import (
	"encoding/binary"
	"errors"
)

// ErrClosedForwardingChildRetired distinguishes an expected late completion
// from a violation on a live lane. It never authorizes a replacement lane.
var ErrClosedForwardingChildRetired = errors.New("closed forwarding child is retired")

// QueueReverse reserves the next Carrier's complete frame before its shared
// reader queues it. Both directions share the prefix and receiving-duty bounds;
// bootstrap also shares its smaller aggregate queue and output budgets.
func (channel *ClosedForwardingChannel) QueueReverse(frame ClosedLaneFrame) error {
	if channel == nil {
		return errors.New("closed forwarding reverse owner is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	child, found := channel.children[frame.Lane]
	size := uint64(closedLaneHeaderSize + len(frame.Body))
	if !found {
		return ErrClosedForwardingChildRetired
	}
	now := channel.clock().UTC()
	if channel.terminated || !now.Before(channel.deadline) || !now.Before(child.deadline) {
		return ErrClosedForwardingChildRetired
	}
	switch frame.Kind {
	case closedFrameAccept, closedFrameCredit, closedFrameEOF, closedFrameClose:
		if err := channel.reserveControlQueue(size); err != nil {
			return err
		}
		child.reverseControlQueued += size
	case closedFrameBytes:
		if uint64(len(frame.Body)) > child.reverseCredit || size > closedPrefixQueueBytes-channel.queued {
			return errors.New("closed forwarding reverse credit or queue exceeded")
		}
		if err := channel.reserveQueue(size); err != nil {
			return err
		}
		child.reverseCredit -= uint64(len(frame.Body))
		child.reverseQueued += size
		channel.queued += size
	default:
		return errors.New("closed forwarding reverse frame is unavailable")
	}
	channel.children[frame.Lane] = child
	return nil
}

// ReleaseReverse releases the original queued frame only after its writer has
// returned. Its kind keeps control and data reservations distinct, including
// failed writes. Child retirement already releases both classes; a late writer
// cannot release another child's reserve or revive the retired lane.
func (channel *ClosedForwardingChannel) ReleaseReverse(frame ClosedLaneFrame) {
	if channel == nil {
		return
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	child, found := channel.children[frame.Lane]
	if !found {
		return
	}
	size := uint64(closedLaneHeaderSize + len(frame.Body))
	switch frame.Kind {
	case closedFrameAccept, closedFrameCredit, closedFrameEOF, closedFrameClose:
		if size > child.reverseControlQueued {
			return
		}
		child.reverseControlQueued -= size
		channel.releaseControlQueue(size)
	case closedFrameBytes:
		if size > child.reverseQueued {
			return
		}
		child.reverseQueued -= size
		channel.queued -= size
		channel.releaseQueue(size)
	default:
		return
	}
	channel.children[frame.Lane] = child
}

// AccountOutput charges actual framed output before the serialized writer.
// It never refunds a failed write or turns bootstrap success into admission.
func (channel *ClosedForwardingChannel) AccountOutput(frame ClosedLaneFrame) error {
	if channel == nil {
		return errors.New("closed forwarding output is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.terminated || !channel.clock().UTC().Before(channel.deadline) {
		return errors.New("closed forwarding output is unavailable")
	}
	if frame.Lane != 0 {
		if _, found := channel.children[frame.Lane]; !found {
			return ErrClosedForwardingChildRetired
		}
	}
	if channel.bootstrap != nil {
		return channel.bootstrap.Send(uint64(closedLaneHeaderSize + len(frame.Body)))
	}
	size := uint64(closedLaneHeaderSize + len(frame.Body))
	if size > channel.byteLimit-channel.usedBytes {
		return errors.New("closed forwarding output exhausted")
	}
	channel.usedBytes += size
	return nil
}

func (channel *ClosedForwardingChannel) peerCredit(frame ClosedLaneFrame) (ClosedForwardingEvent, error) {
	child, found := channel.children[frame.Lane]
	if !found || len(frame.Body) != 4 {
		return ClosedForwardingEvent{}, errors.New("closed forwarding peer credit is invalid")
	}
	increment := uint64(binary.BigEndian.Uint32(frame.Body))
	if increment == 0 || increment > closedLaneCredit-child.reverseCredit {
		return ClosedForwardingEvent{}, errors.New("closed forwarding peer credit exceeds consumption")
	}
	if !channel.queueControl(ClosedForwardingEvent{Kind: closedFrameCredit, Lane: frame.Lane, Bytes: append([]byte(nil), frame.Body...)}) {
		return ClosedForwardingEvent{}, errors.New("closed forwarding peer credit queue is exhausted")
	}
	child.reverseCredit += increment
	channel.children[frame.Lane] = child
	return ClosedForwardingEvent{}, nil
}

// ReverseRetired reads the actual child retirement state without reserving a
// queue slot. Shared readers use it before treating a full queue as overload;
// A local child or parent deadline also retires receive authority. Late frames
// are discarded before queue reservation, not treated as a shared peer failure.
func (channel *ClosedForwardingChannel) ReverseRetired(lane uint32) bool {
	if channel == nil {
		return true
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	child, found := channel.children[lane]
	now := channel.clock().UTC()
	return channel.terminated || !found || !now.Before(channel.deadline) || !now.Before(child.deadline)
}
