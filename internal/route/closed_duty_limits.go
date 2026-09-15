package route

import (
	"errors"
	"sync"
	"time"
)

const (
	closedDutyChannels        = 1024
	closedDutyChildren        = 1024
	closedDutyQueueBytes      = 64 << 20
	closedChannelControlBytes = 16 << 10
	closedVerifyPerSecond     = 128
	closedVerifyConcurrency   = 4
)

// ClosedDutyLimits owns aggregate receiving-duty reservations. It admits no
// peer identity, route, or destination; every counter is released only by its
// owning channel lifecycle.
type ClosedDutyLimits struct {
	mu                       sync.Mutex
	clock                    func() time.Time
	channels, children       uint16
	queued                   uint64 // Data; each live channel separately reserves control within the same total.
	verificationWindow       time.Time
	verifications, verifying uint16
}

type closedDutyChannel struct {
	limits                  *ClosedDutyLimits
	reservedControlChildren uint16
	children                uint16
	controlQueued           uint64 // Protected by limits.mu, inside the channel's admission reserve.
	released                bool
}

// NewClosedDutyLimits creates the finite governor for one exact receiver
// duty. It is intentionally not process-global and cannot cross a State
// successor or spend ledger binding.
func NewClosedDutyLimits(clock func() time.Time) (*ClosedDutyLimits, error) {
	if clock == nil || clock().IsZero() {
		return nil, errors.New("closed duty limits clock is invalid")
	}
	return &ClosedDutyLimits{clock: clock, verificationWindow: clock().UTC().Truncate(time.Second)}, nil
}

// BeginVerification reserves one cheap pre-crypto check. The returned release
// must be called after verification succeeds or fails.
func (limits *ClosedDutyLimits) BeginVerification() (func(), error) {
	if limits == nil {
		return nil, errors.New("closed duty verification is unavailable")
	}
	limits.mu.Lock()
	now := limits.clock().UTC().Truncate(time.Second)
	if !now.Equal(limits.verificationWindow) {
		limits.verificationWindow, limits.verifications = now, 0
	}
	if limits.verifications >= closedVerifyPerSecond || limits.verifying >= closedVerifyConcurrency {
		limits.mu.Unlock()
		return nil, errors.New("closed duty verification is exhausted")
	}
	limits.verifications++
	limits.verifying++
	limits.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { limits.mu.Lock(); limits.verifying--; limits.mu.Unlock() }) }, nil
}

func (limits *ClosedDutyLimits) reserveChannel() (*closedDutyChannel, error) {
	if limits == nil {
		return nil, errors.New("closed duty channel is unavailable")
	}
	limits.mu.Lock()
	defer limits.mu.Unlock()
	if limits.channels >= closedDutyChannels || limits.queued > closedDutyQueueBytes-uint64(limits.channels+1)*closedChannelControlBytes {
		return nil, errors.New("closed duty channels are exhausted")
	}
	limits.channels++
	return &closedDutyChannel{limits: limits}, nil
}

func (channel *closedDutyChannel) reserveChild() error {
	_, err := channel.reserveChildCapacity(false)
	return err
}

// Control uses ordinary capacity first; only two additional reservations may
// survive when the 256 work positions are occupied.
func (channel *closedDutyChannel) reserveChildCapacity(control bool) (bool, error) {
	if channel == nil || channel.limits == nil {
		return false, errors.New("closed duty child unavailable")
	}
	limits := channel.limits
	limits.mu.Lock()
	defer limits.mu.Unlock()
	reserved := channel.children-channel.reservedControlChildren >= closedForwardChildren
	if channel.released || limits.children >= closedDutyChildren || reserved && (!control || channel.reservedControlChildren >= 2) {
		return false, errors.New("closed duty children exhausted")
	}
	channel.children++
	limits.children++
	if reserved {
		channel.reservedControlChildren++
	}
	return reserved, nil
}

func (channel *closedDutyChannel) releaseChild() { channel.releaseChildCapacity(false) }

func (channel *closedDutyChannel) releaseChildCapacity(reserved bool) {
	if channel == nil || channel.limits == nil {
		return
	}
	limits := channel.limits
	limits.mu.Lock()
	defer limits.mu.Unlock()
	if channel.released || channel.children == 0 {
		return
	}
	if reserved {
		if channel.reservedControlChildren == 0 {
			return
		}
		channel.reservedControlChildren--
	}
	channel.children--
	limits.children--
}

func (channel *closedDutyChannel) release() {
	if channel == nil || channel.limits == nil {
		return
	}
	limits := channel.limits
	limits.mu.Lock()
	defer limits.mu.Unlock()
	if channel.released {
		return
	}
	limits.children -= channel.children
	channel.children = 0
	channel.reservedControlChildren = 0
	limits.channels--
	channel.released = true
}

func (limits *ClosedDutyLimits) queue(bytes uint64) error {
	if limits == nil || bytes == 0 {
		return errors.New("closed duty queue is unavailable")
	}
	limits.mu.Lock()
	defer limits.mu.Unlock()
	available := uint64(closedDutyQueueBytes) - uint64(limits.channels)*closedChannelControlBytes - limits.queued
	if bytes > available {
		return errors.New("closed duty queue is exhausted")
	}
	limits.queued += bytes
	return nil
}

func (limits *ClosedDutyLimits) dequeue(bytes uint64) {
	if limits == nil || bytes == 0 {
		return
	}
	limits.mu.Lock()
	defer limits.mu.Unlock()
	if bytes > limits.queued {
		limits.queued = 0
		return
	}
	limits.queued -= bytes
}
