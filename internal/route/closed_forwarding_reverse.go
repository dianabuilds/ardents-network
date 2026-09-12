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
	if channel.queued+size > closedPrefixQueueBytes {
		return errors.New("closed forwarding reverse queue is unavailable")
	}
	switch frame.Kind {
	case closedFrameAccept, closedFrameCredit, closedFrameEOF, closedFrameClose:
	case closedFrameBytes:
		if uint64(len(frame.Body)) > child.reverseCredit {
			return errors.New("closed forwarding reverse credit exceeded")
		}
	default:
		return errors.New("closed forwarding reverse frame is unavailable")
	}
	if err := channel.reserveQueue(size); err != nil {
		return err
	}
	if frame.Kind == closedFrameBytes {
		child.reverseCredit -= uint64(len(frame.Body))
	}
	child.reverseQueued += size
	channel.queued += size
	channel.children[frame.Lane] = child
	return nil
}

// ReleaseReverse releases only work whose writer has returned. A terminated
// child has already released its complete reservation and cannot be revived by
// a late completion from the shared Carrier reader.
func (channel *ClosedForwardingChannel) ReleaseReverse(lane uint32, size uint64) {
	if channel == nil {
		return
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	child, found := channel.children[lane]
	if !found || size == 0 || size > child.reverseQueued {
		return
	}
	child.reverseQueued -= size
	channel.queued -= size
	channel.releaseQueue(size)
	channel.children[lane] = child
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
	if uint64(len(frame.Body)) > channel.byteLimit-channel.received {
		return errors.New("closed forwarding output exhausted")
	}
	channel.received += uint64(len(frame.Body))
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
