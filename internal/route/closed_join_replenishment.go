package route

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// NewReplenishableClosedJoinPairs retains receiver-side admission ownership
// for class-2 JOIN channels that carry long-lived authenticated Service bytes.
func NewReplenishableClosedJoinPairs(receiver ClosedRoleReceiver, limits *ClosedDutyLimits, replenish ClosedForwardingReplenisher) (*ClosedJoinPairs, error) {
	if replenish == nil {
		return nil, errors.New("JOIN replenishment owner unavailable")
	}
	owner, err := NewClosedJoinPairs(receiver, limits)
	if err != nil {
		return nil, err
	}
	owner.replenish = replenish
	return owner, nil
}

func (side *ClosedJoinSide) replenish(frame ardp.Frame) error {
	if side == nil || side.owner == nil {
		return errors.New("JOIN replenishment unavailable")
	}
	class, token, err := decodeClosedAdmit(frame.Body)
	if err != nil || frame.Kind != ardp.KindAdmit || frame.Lane != 0 || class != 2 {
		return errors.New("JOIN replenishment control invalid")
	}
	side.controlMu.Lock()
	defer side.controlMu.Unlock()
	owner := side.owner
	owner.mu.Lock()
	owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
	if !side.refillLiveLocked() || owner.replenish == nil || side.used > ^uint64(0)-(32<<20) {
		owner.mu.Unlock()
		return errors.New("JOIN replenishment unavailable")
	}
	input := ClosedAdmissionVerification{Hello: side.hello, Exporter: side.exporter, Class: 2, Token: token, Deadline: side.deadline}
	owner.mu.Unlock()
	// readFrame has already charged the complete ADMIT to the old allowance.
	// Verification, host reservation and durable spend happen outside the shared
	// pairing lock; only this side's release/refill control remains serialized.
	release, err := owner.replenish(input)
	if err != nil {
		return err
	}
	owner.mu.Lock()
	owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
	live := side.refillLiveLocked() && side.used <= ^uint64(0)-(32<<20)
	if live {
		side.byteLimit = side.used + (32 << 20)
		if release != nil {
			side.releases = append(side.releases, release)
		}
	}
	owner.mu.Unlock()
	if !live {
		if release != nil {
			err = release()
		}
		return errors.Join(errors.New("JOIN ended during replenishment"), err)
	}
	return nil
}

func (side *ClosedJoinSide) refillLiveLocked() bool {
	peer := side.peerLocked()
	return !side.owner.closed && !side.closed && !side.pair.stopped &&
		side.pair.paired && side.confirmed && peer != nil && peer.confirmed &&
		side.stream != nil && peer.stream != nil && side.used <= side.byteLimit
}
