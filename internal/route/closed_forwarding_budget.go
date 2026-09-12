package route

import (
	"errors"
	"time"
)

// NewClosedBootstrapForwardingChannel transfers an actual target-free
// bootstrap reservation into the forwarding owner. It never constructs an
// ADMIT or class-2 lease. The authorizer must restrict each OPEN to the current
// public-evidence/issuer path; private purposes remain unavailable.
func NewClosedBootstrapForwardingChannel(lease *ClosedBootstrapLease, limits *ClosedDutyLimits, authorize ClosedForwardingAuthorizer, clock func() time.Time) (*ClosedForwardingChannel, error) {
	if lease == nil || authorize == nil || clock == nil || clock().IsZero() {
		return nil, errors.New("closed bootstrap forwarding is invalid")
	}
	duty, err := limits.reserveChannel()
	if err != nil {
		return nil, err
	}
	owned, err := lease.transferForwarding()
	if err != nil {
		duty.release()
		return nil, err
	}
	return &ClosedForwardingChannel{duty: duty, deadline: owned.deadline, byteLimit: closedBootstrapLaneBytes,
		bootstrap: owned, authorize: func(open ClosedOpen) error {
			if open.Purpose != ClosedPurposeForwarding && open.Purpose != ClosedPurposeIssuer {
				return errors.New("closed bootstrap private purpose is unavailable")
			}
			return authorize(open)
		}, clock: clock, children: make(map[uint32]closedForwardChild)}, nil
}

// transferForwarding prevents a retained source lease from releasing or
// duplicating a transferred reservation. Adjacency/duty pressure is unchanged.
func (lease *ClosedBootstrapLease) transferForwarding() (*ClosedBootstrapLease, error) {
	if lease == nil || lease.controller == nil {
		return nil, errors.New("closed bootstrap reservation is unavailable")
	}
	controller := lease.controller
	controller.mu.Lock()
	defer controller.mu.Unlock()
	controller.reap(controller.clock().UTC())
	if _, live := controller.leases[lease]; !live || lease.released {
		return nil, errors.New("closed bootstrap reservation is unavailable")
	}
	owned := *lease
	delete(controller.leases, lease)
	controller.leases[&owned] = struct{}{}
	lease.released = true
	return &owned, nil
}

func (channel *ClosedForwardingChannel) reserveQueue(size uint64) error {
	if err := channel.duty.limits.queue(size); err != nil {
		return err
	}
	if channel.bootstrap != nil {
		if err := channel.bootstrap.Queue(size); err != nil {
			channel.duty.limits.dequeue(size)
			return err
		}
	}
	return nil
}

func (channel *ClosedForwardingChannel) releaseQueue(size uint64) {
	if size == 0 {
		return
	}
	channel.duty.limits.dequeue(size)
	if channel.bootstrap != nil {
		_ = channel.bootstrap.Dequeue(size)
	}
}
