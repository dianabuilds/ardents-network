package route

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

const (
	closedLaneCredit      = uint64(64 << 10)
	closedForwardChildren = 256
)

// ClosedForwardingAuthorizer checks one exact next public recipient before a
// lane exists. It may not dial: the caller receives a later Open event only
// after this check succeeds.
type ClosedForwardingAuthorizer func(ClosedOpen) error

// ClosedForwardingEvent describes bounded work already admitted by the local
// channel state. It intentionally has no carrier, endpoint or Target input.
type ClosedForwardingEvent struct {
	Kind  uint8
	Lane  uint32
	Open  ClosedOpen
	Bytes []byte
}

// ClosedForwardingChannel owns Endpoint-initiated child lanes under one
// class-2 admission lease. It accounts peer input before yielding an event.
type ClosedForwardingChannel struct {
	mu         sync.Mutex
	lease      ClosedAdmission
	authorize  ClosedForwardingAuthorizer
	clock      func() time.Time
	lastOdd    uint32
	children   map[uint32]closedForwardChild
	received   uint64
	terminated bool
}

type closedForwardChild struct {
	deadline time.Time
	credit   uint64
	queued   uint64
	eof      bool
}

// NewClosedForwardingChannel creates only a class-2 forwarding owner. Control
// and publication admission cannot silently become arbitrary forwarding.
func NewClosedForwardingChannel(lease ClosedAdmission, authorize ClosedForwardingAuthorizer, clock func() time.Time) (*ClosedForwardingChannel, error) {
	if lease.Class != 2 || lease.Bytes != 32<<20 || lease.Deadline.IsZero() || lease.duty == nil || authorize == nil || clock == nil || clock().IsZero() {
		return nil, errors.New("closed forwarding channel is invalid")
	}
	return &ClosedForwardingChannel{lease: lease, authorize: authorize, clock: clock, children: make(map[uint32]closedForwardChild)}, nil
}

// Accept accounts one child frame. It returns an event only after every local
// bound and State authorization is satisfied; a caller may then attach its
// separately owned next Carrier without a peer-selected fallback.
func (channel *ClosedForwardingChannel) Accept(frame ClosedLaneFrame) (ClosedForwardingEvent, error) {
	if channel == nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding channel is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.terminated || !channel.clock().UTC().Before(channel.lease.Deadline) {
		return ClosedForwardingEvent{}, errors.New("closed forwarding channel is unavailable")
	}
	switch frame.Kind {
	case closedFrameOpen:
		return channel.open(frame)
	case closedFrameBytes:
		return channel.bytes(frame)
	case closedFrameEOF:
		return channel.eof(frame)
	case closedFrameClose:
		return channel.close(frame)
	default:
		return ClosedForwardingEvent{}, errors.New("closed forwarding frame is unavailable")
	}
}

func (channel *ClosedForwardingChannel) open(frame ClosedLaneFrame) (ClosedForwardingEvent, error) {
	if frame.Lane%2 == 0 || frame.Lane <= channel.lastOdd || len(channel.children) >= closedForwardChildren {
		return ClosedForwardingEvent{}, errors.New("closed forwarding child lane is invalid")
	}
	open, err := DecodeClosedOpen(frame.Body)
	if err != nil || !channel.clock().UTC().Before(open.Deadline) || open.Deadline.After(channel.lease.Deadline) || channel.authorize(open) != nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding OPEN is unavailable")
	}
	if err := channel.lease.duty.reserveChild(); err != nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding child capacity is unavailable")
	}
	channel.children[frame.Lane] = closedForwardChild{deadline: open.Deadline, credit: closedLaneCredit}
	channel.lastOdd = frame.Lane
	return ClosedForwardingEvent{Kind: closedFrameOpen, Lane: frame.Lane, Open: open}, nil
}

func (channel *ClosedForwardingChannel) bytes(frame ClosedLaneFrame) (ClosedForwardingEvent, error) {
	child, found := channel.children[frame.Lane]
	if !found || child.eof || !channel.clock().UTC().Before(child.deadline) || uint64(len(frame.Body)) > child.credit || channel.received+uint64(len(frame.Body)) > channel.lease.Bytes {
		return ClosedForwardingEvent{}, errors.New("closed forwarding bytes are unavailable")
	}
	bytes := uint64(len(frame.Body))
	if err := channel.lease.duty.limits.queue(bytes); err != nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding queue is unavailable")
	}
	child.credit -= bytes
	child.queued += bytes
	channel.children[frame.Lane] = child
	channel.received += bytes
	return ClosedForwardingEvent{Kind: closedFrameBytes, Lane: frame.Lane, Bytes: append([]byte(nil), frame.Body...)}, nil
}

func (channel *ClosedForwardingChannel) eof(frame ClosedLaneFrame) (ClosedForwardingEvent, error) {
	child, found := channel.children[frame.Lane]
	if !found || child.eof {
		return ClosedForwardingEvent{}, errors.New("closed forwarding EOF is unavailable")
	}
	child.eof = true
	channel.children[frame.Lane] = child
	return ClosedForwardingEvent{Kind: closedFrameEOF, Lane: frame.Lane}, nil
}

func (channel *ClosedForwardingChannel) close(frame ClosedLaneFrame) (ClosedForwardingEvent, error) {
	child, found := channel.children[frame.Lane]
	if !found {
		return ClosedForwardingEvent{}, errors.New("closed forwarding close is unavailable")
	}
	channel.lease.duty.limits.dequeue(child.queued)
	delete(channel.children, frame.Lane)
	channel.lease.duty.releaseChild()
	return ClosedForwardingEvent{Kind: closedFrameClose, Lane: frame.Lane, Bytes: append([]byte(nil), frame.Body...)}, nil
}

// Credit returns receive credit only after the caller's actual consumer has
// removed bytes. It never exceeds the fixed 64 KiB window or parent lease.
func (channel *ClosedForwardingChannel) Credit(lane uint32, bytes uint32) (ClosedLaneFrame, error) {
	if channel == nil {
		return ClosedLaneFrame{}, errors.New("closed forwarding credit is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.terminated || bytes == 0 || !channel.clock().UTC().Before(channel.lease.Deadline) {
		return ClosedLaneFrame{}, errors.New("closed forwarding credit is unavailable")
	}
	child, found := channel.children[lane]
	if !found || child.eof || uint64(bytes) > closedLaneCredit-child.credit {
		return ClosedLaneFrame{}, errors.New("closed forwarding credit is unavailable")
	}
	if uint64(bytes) > child.queued {
		return ClosedLaneFrame{}, errors.New("closed forwarding credit is unavailable")
	}
	child.credit += uint64(bytes)
	child.queued -= uint64(bytes)
	channel.lease.duty.limits.dequeue(uint64(bytes))
	channel.children[lane] = child
	body := make([]byte, 4)
	binary.BigEndian.PutUint32(body, bytes)
	return ClosedLaneFrame{Kind: closedFrameCredit, Lane: lane, Body: body}, nil
}

// Cancel terminates every child before a caller joins its owned carriers.
// It is idempotent and makes future OPEN/bytes unavailable.
func (channel *ClosedForwardingChannel) Cancel() {
	if channel == nil {
		return
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	channel.terminated = true
	for _, child := range channel.children {
		channel.lease.duty.limits.dequeue(child.queued)
	}
	clear(channel.children)
	channel.lease.duty.release()
}
