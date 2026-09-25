package route

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

const (
	closedLaneCredit       = uint64(64 << 10)
	closedForwardChildren  = 256
	closedPrefixQueueBytes = uint64(4 << 20)
)

// ClosedForwardingAuthorizer checks one exact next public recipient before a
// lane exists. It may not dial: the caller receives a later Open event only
// after this check succeeds.
type ClosedForwardingAuthorizer func(ClosedOpen) error

// ClosedForwardingReplenisher verifies and durably spends a fresh class-2
// token after the complete parent ADMIT frame was charged to the old reserve.
// It returns an optional host-capacity release retained until the parent and
// all of its children have joined.
type ClosedForwardingReplenisher func(ClosedAdmissionVerification) (func() error, error)

// ClosedForwardingEvent describes bounded work already admitted by the local
// channel state. It intentionally has no carrier, endpoint or Target input.
type ClosedForwardingEvent struct {
	Kind        uint8
	Lane        uint32
	Open        ClosedOpen
	Restriction ClosedChildRestriction
	Bytes       []byte
}

// ClosedForwardingChannel owns Endpoint-initiated child lanes under one
// class-2 admission lease. It accounts peer input before yielding an event.
type ClosedForwardingChannel struct {
	mu           sync.Mutex
	duty         *closedDutyChannel
	deadline     time.Time
	byteLimit    uint64
	bootstrap    *ClosedBootstrapLease
	authorize    ClosedForwardingAuthorizer
	replenish    ClosedForwardingReplenisher
	hello        ardp.Hello
	exporter     [32]byte
	releases     []func() error
	clock        func() time.Time
	lastOdd      uint32
	children     map[uint32]closedForwardChild
	ready        []uint32
	controls     []ClosedForwardingEvent
	controlBytes uint32
	usedBytes    uint64
	queued       uint64
	terminated   bool
}

type closedForwardChild struct {
	reservedControl      bool
	deadline             time.Time
	credit               uint64
	reverseCredit        uint64
	reverseQueued        uint64
	reverseControlQueued uint64
	queued               uint64
	delivered            uint64
	frames               [][]byte
	ready                bool
	eof                  bool
	eofSent              bool
}

// NewReplenishableClosedForwardingChannel creates the one forwarding parent
// that can receive later lane-zero class-2 replenishment ADMIT frames.
func NewReplenishableClosedForwardingChannel(lease *ClosedAdmission, authorize ClosedForwardingAuthorizer, replenish ClosedForwardingReplenisher, clock func() time.Time) (*ClosedForwardingChannel, error) {
	if replenish == nil {
		return nil, errors.New("closed forwarding replenishment is unavailable")
	}
	return newClosedForwardingChannel(lease, authorize, replenish, clock)
}

func newClosedForwardingChannel(lease *ClosedAdmission, authorize ClosedForwardingAuthorizer, replenish ClosedForwardingReplenisher, clock func() time.Time) (*ClosedForwardingChannel, error) {
	if lease == nil || lease.Class != 2 || lease.Bytes != 32<<20 || lease.Deadline.IsZero() || !lease.claim.live() || authorize == nil || clock == nil || clock().IsZero() {
		return nil, errors.New("closed forwarding channel is invalid")
	}
	duty, release, transferred := lease.claim.transfer()
	if !transferred {
		return nil, errors.New("closed forwarding channel is unavailable")
	}
	channel := &ClosedForwardingChannel{duty: duty, deadline: lease.Deadline, byteLimit: lease.Bytes, usedBytes: closedAdmissionFrameBytes,
		authorize: authorize, replenish: replenish, hello: lease.hello, exporter: lease.exporter, clock: clock, children: make(map[uint32]closedForwardChild)}
	if release != nil {
		channel.releases = append(channel.releases, release)
	}
	return channel, nil
}

// Accept accounts one child frame. It returns an event only after every local
// bound and State authorization is satisfied; a caller may then attach its
// separately owned next Carrier without a peer-selected fallback.
func (channel *ClosedForwardingChannel) Accept(frame ardp.Frame) (ClosedForwardingEvent, error) {
	if channel == nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding channel is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.terminated || !channel.clock().UTC().Before(channel.deadline) {
		return ClosedForwardingEvent{}, errors.New("closed forwarding channel is unavailable")
	}
	size := uint64(ardp.HeaderSize + len(frame.Body))
	if channel.bootstrap != nil {
		if err := channel.bootstrap.Receive(size); err != nil {
			return ClosedForwardingEvent{}, err
		}
	} else {
		if size > channel.byteLimit-channel.usedBytes {
			return ClosedForwardingEvent{}, errors.New("closed forwarding input exhausted")
		}
		// The complete frame has already arrived. A later semantic refusal
		// cannot refund its ingress or let control bypass the parent budget.
		channel.usedBytes += size
	}
	switch frame.Kind {
	case ardp.KindOpen:
		return channel.open(frame)
	case ardp.KindBytes:
		return channel.bytes(frame)
	case ardp.KindCredit:
		return channel.peerCredit(frame)
	case ardp.KindEOF:
		return channel.eof(frame)
	case ardp.KindClose:
		return channel.close(frame)
	case ardp.KindAdmit:
		return channel.admit(frame)
	default:
		return ClosedForwardingEvent{}, errors.New("closed forwarding frame is unavailable")
	}
}

func (channel *ClosedForwardingChannel) admit(frame ardp.Frame) (ClosedForwardingEvent, error) {
	if frame.Lane != 0 || channel.replenish == nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding replenishment is unavailable")
	}
	class, token, err := decodeClosedAdmit(frame.Body)
	if err != nil || class != 2 {
		return ClosedForwardingEvent{}, errors.New("closed forwarding replenishment is unavailable")
	}
	release, err := channel.replenish(ClosedAdmissionVerification{Hello: channel.hello, Class: class, Token: token, Exporter: channel.exporter, Deadline: channel.deadline})
	if err != nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding replenishment is unavailable")
	}
	if channel.usedBytes > ^uint64(0)-(32<<20) {
		if release != nil {
			_ = release()
		}
		return ClosedForwardingEvent{}, errors.New("closed forwarding replenishment is unavailable")
	}
	channel.byteLimit = channel.usedBytes + 32<<20
	if release != nil {
		channel.releases = append(channel.releases, release)
	}
	return ClosedForwardingEvent{}, nil
}

func (channel *ClosedForwardingChannel) open(frame ardp.Frame) (ClosedForwardingEvent, error) {
	if frame.Lane%2 == 0 || frame.Lane <= channel.lastOdd || len(channel.children) >= closedForwardChildren {
		return ClosedForwardingEvent{}, errors.New("closed forwarding child lane is invalid")
	}
	open, err := DecodeClosedOpen(frame.Body)
	if err != nil || !channel.clock().UTC().Before(open.Deadline) || open.Deadline.After(channel.deadline) || channel.authorize(open) != nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding OPEN is unavailable")
	}
	restriction := ClosedChildOrdinary
	if channel.bootstrap != nil {
		restriction = ClosedChildIssuerBootstrap
		// Node-outer OPEN adds one mandatory byte to the accepted role OPEN.
		if err := channel.bootstrap.Send(1); err != nil {
			return ClosedForwardingEvent{}, err
		}
	}
	reservedControl, err := channel.duty.reserveChildCapacity(closedControlPurpose(open.Purpose))
	if err != nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding child capacity is unavailable")
	}
	channel.children[frame.Lane] = closedForwardChild{reservedControl: reservedControl, deadline: open.Deadline, credit: closedLaneCredit, reverseCredit: closedLaneCredit}
	channel.lastOdd = frame.Lane
	if !channel.queueControl(ClosedForwardingEvent{Kind: ardp.KindOpen, Lane: frame.Lane, Open: open, Restriction: restriction}) {
		delete(channel.children, frame.Lane)
		channel.duty.releaseChildCapacity(reservedControl)
		return ClosedForwardingEvent{}, errors.New("closed forwarding control queue is unavailable")
	}
	return ClosedForwardingEvent{}, nil
}

func (channel *ClosedForwardingChannel) bytes(frame ardp.Frame) (ClosedForwardingEvent, error) {
	child, found := channel.children[frame.Lane]
	if !found || child.eof || !channel.clock().UTC().Before(child.deadline) || uint64(len(frame.Body)) > child.credit || channel.queued+uint64(len(frame.Body)) > closedPrefixQueueBytes {
		return ClosedForwardingEvent{}, errors.New("closed forwarding bytes are unavailable")
	}
	bytes := uint64(len(frame.Body))
	if err := channel.reserveQueue(bytes); err != nil {
		return ClosedForwardingEvent{}, errors.New("closed forwarding queue is unavailable")
	}
	child.credit -= bytes
	child.queued += bytes
	child.frames = append(child.frames, append([]byte(nil), frame.Body...))
	if !child.ready {
		child.ready = true
		channel.ready = append(channel.ready, frame.Lane)
	}
	channel.children[frame.Lane] = child
	channel.queued += bytes
	return ClosedForwardingEvent{}, nil
}

func (channel *ClosedForwardingChannel) eof(frame ardp.Frame) (ClosedForwardingEvent, error) {
	child, found := channel.children[frame.Lane]
	if !found || child.eof {
		return ClosedForwardingEvent{}, errors.New("closed forwarding EOF is unavailable")
	}
	child.eof = true
	channel.children[frame.Lane] = child
	return ClosedForwardingEvent{}, nil
}

func (channel *ClosedForwardingChannel) close(frame ardp.Frame) (ClosedForwardingEvent, error) {
	child, found := channel.children[frame.Lane]
	if !found {
		return ClosedForwardingEvent{}, errors.New("closed forwarding close is unavailable")
	}
	event := ClosedForwardingEvent{Kind: ardp.KindClose, Lane: frame.Lane, Bytes: append([]byte(nil), frame.Body...)}
	if !channel.queueControl(event) {
		return ClosedForwardingEvent{}, errors.New("closed forwarding control queue is unavailable")
	}
	channel.releaseQueue(child.queued + child.reverseQueued)
	channel.releaseControlQueue(child.reverseControlQueued)
	channel.queued -= child.queued + child.reverseQueued
	delete(channel.children, frame.Lane)
	channel.duty.releaseChildCapacity(child.reservedControl)
	return ClosedForwardingEvent{}, nil
}

// Credit returns receive credit only after the caller's actual consumer has
// removed bytes. It never exceeds the fixed 64 KiB window or parent lease.
func (channel *ClosedForwardingChannel) Credit(lane uint32, bytes uint32) (ardp.Frame, error) {
	if channel == nil {
		return ardp.Frame{}, errors.New("closed forwarding credit is unavailable")
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.terminated || bytes == 0 || !channel.clock().UTC().Before(channel.deadline) {
		return ardp.Frame{}, errors.New("closed forwarding credit is unavailable")
	}
	child, found := channel.children[lane]
	if !found {
		return ardp.Frame{}, ErrClosedForwardingChildRetired
	}
	if uint64(bytes) > closedLaneCredit-child.credit || uint64(bytes) > child.delivered {
		return ardp.Frame{}, errors.New("closed forwarding credit is unavailable")
	}
	if uint64(bytes) > child.queued {
		return ardp.Frame{}, errors.New("closed forwarding credit is unavailable")
	}
	child.credit += uint64(bytes)
	child.queued -= uint64(bytes)
	child.delivered -= uint64(bytes)
	channel.releaseQueue(uint64(bytes))
	channel.queued -= uint64(bytes)
	channel.children[lane] = child
	body := make([]byte, 4)
	binary.BigEndian.PutUint32(body, bytes)
	return ardp.Frame{Kind: ardp.KindCredit, Lane: lane, Body: body}, nil
}

// NextAvailable returns bounded work whose consumer is currently available.
// Rejected work remains in this channel's accounted control or prefix queue.
func (channel *ClosedForwardingChannel) NextAvailable(available func(ClosedForwardingEvent) bool) (ClosedForwardingEvent, bool) {
	if channel == nil {
		return ClosedForwardingEvent{}, false
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	if channel.terminated || !channel.clock().UTC().Before(channel.deadline) {
		return ClosedForwardingEvent{}, false
	}
	for index, event := range channel.controls {
		if available != nil && !available(event) {
			continue
		}
		channel.controls = append(channel.controls[:index], channel.controls[index+1:]...)
		channel.controlBytes -= closedForwardControlSize(event)
		channel.releaseControlQueue(uint64(closedForwardControlSize(event)))
		return event, true
	}
	ready := len(channel.ready)
	for range ready {
		lane := channel.ready[0]
		channel.ready = channel.ready[1:]
		child, found := channel.children[lane]
		if !found || len(child.frames) == 0 {
			continue
		}
		event := ClosedForwardingEvent{Kind: ardp.KindBytes, Lane: lane, Bytes: child.frames[0]}
		if available != nil && !available(event) {
			channel.ready = append(channel.ready, lane)
			continue
		}
		child.frames = child.frames[1:]
		child.delivered += uint64(len(event.Bytes))
		if len(child.frames) > 0 {
			channel.ready = append(channel.ready, lane)
		} else {
			child.ready = false
			if child.eof && !child.eofSent {
				child.eofSent = true
				if !channel.queueControl(ClosedForwardingEvent{Kind: ardp.KindEOF, Lane: lane}) {
					channel.terminated = true
					channel.children[lane] = child
					return ClosedForwardingEvent{}, false
				}
			}
		}
		channel.children[lane] = child
		return event, true
	}
	for lane, child := range channel.children {
		if child.eof && !child.eofSent {
			event := ClosedForwardingEvent{Kind: ardp.KindEOF, Lane: lane}
			if available != nil && !available(event) {
				continue
			}
			child.eofSent = true
			channel.children[lane] = child
			return event, true
		}
	}
	return ClosedForwardingEvent{}, false
}

func (channel *ClosedForwardingChannel) queueControl(event ClosedForwardingEvent) bool {
	size := closedForwardControlSize(event)
	if size > closedChannelControlBytes || channel.controlBytes+size > closedChannelControlBytes {
		return false
	}
	if err := channel.reserveControlQueue(uint64(size)); err != nil {
		return false
	}
	channel.controls = append(channel.controls, event)
	channel.controlBytes += size
	return true
}

func closedForwardControlSize(event ClosedForwardingEvent) uint32 {
	if event.Kind == ardp.KindOpen {
		return ardp.HeaderSize + 50
	}
	return ardp.HeaderSize + uint32(len(event.Bytes))
}

// Cancel terminates every child before a caller joins its owned carriers.
// It is idempotent and makes future OPEN/bytes unavailable.
func (channel *ClosedForwardingChannel) Cancel() error {
	if channel == nil {
		return nil
	}
	channel.mu.Lock()
	defer channel.mu.Unlock()
	channel.terminated = true
	for _, child := range channel.children {
		channel.releaseQueue(child.queued + child.reverseQueued)
		channel.releaseControlQueue(child.reverseControlQueued)
		channel.queued -= child.queued + child.reverseQueued
	}
	clear(channel.children)
	for _, event := range channel.controls {
		channel.releaseControlQueue(uint64(closedForwardControlSize(event)))
	}
	channel.controls = nil
	channel.ready = nil
	channel.controlBytes = 0
	channel.duty.release()
	var result error
	for _, release := range channel.releases {
		result = errors.Join(result, release())
	}
	channel.releases = nil
	if channel.bootstrap != nil {
		channel.bootstrap.Release()
	}
	return result
}
