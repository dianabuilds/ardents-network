package route

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ClosedJoinPairs owns matching and activation within one receiving duty.
// Reservations remain held until each handler joins its I/O and closes its side.
// Close cancels all sides, then waits for those explicit ownership returns.
type ClosedJoinPairs struct {
	mu        sync.Mutex
	receiver  ClosedRoleReceiver
	limits    *ClosedDutyLimits
	entries   map[[32]byte]*closedJoinPair
	closed    bool
	drained   chan struct{}
	drainOnce sync.Once
	timers    sync.WaitGroup
}

type closedJoinPair struct {
	secret, context, profile  [32]byte
	sides                     [2]*ClosedJoinSide
	setup, setupWall          time.Time
	timer                     *closedJoinTimer
	ready, dataReady, done    chan struct{}
	paired, stopped, graceful bool
}

// ClosedJoinSide retains one exact request and its original receiving reserve.
// Result is local to this side. ConfirmResult follows a successful write of that
// RESULT; WaitData prevents forwarding until both sides confirm their writes.
type ClosedJoinSide struct {
	owner                          *ClosedJoinPairs
	pair                           *closedJoinPair
	duty                           *closedDutyChannel
	stream                         *closedJoinStream
	used                           uint64
	nonce                          [32]byte
	deadline, wallDeadline         time.Time
	timer                          *closedJoinTimer
	resultTaken, confirmed, closed bool
}

func NewClosedJoinPairs(receiver ClosedRoleReceiver, limits *ClosedDutyLimits) (*ClosedJoinPairs, error) {
	if !validClosedRoleReceiver(receiver) || receiver.ExpectedPurpose != ClosedPurposeDataJoin ||
		receiver.RoleDomain != closedRoleDomainRendezvous || receiver.Subrole != closedDutyDataJoin || limits == nil || limits.clock == nil || limits.clock().IsZero() {
		return nil, errors.New("closed JOIN duty unavailable")
	}
	return &ClosedJoinPairs{receiver: receiver, limits: limits, entries: make(map[[32]byte]*closedJoinPair), drained: make(chan struct{})}, nil
}

// Reserve accepts exactly one lane-1 JOIN after actual class-2 admission. It
// transfers the original reservation only on success. A refused side retains
// its caller-owned lease and cannot replace or cancel an existing reservation.
func (owner *ClosedJoinPairs) Reserve(lease *ClosedAdmission, frame ClosedLaneFrame) (*ClosedJoinSide, error) {
	if owner == nil || lease == nil || frame.Kind != closedFrameOperation || frame.Lane != 1 {
		return nil, errors.New("closed JOIN lane unavailable")
	}
	request, err := DecodeClosedJoinRequest(frame.Body)
	if err != nil {
		return nil, err
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	now, wall := owner.limits.clock().UTC(), time.Now()
	r, h := owner.receiver, lease.hello
	if owner.closed || lease.duty == nil || lease.duty.limits != owner.limits || lease.Class != 2 || lease.Bytes != closedClassBytes(2) ||
		h.Purpose != ClosedPurposeDataJoin || h.NetworkID != r.NetworkID || h.StateGeneration != r.StateGeneration || h.StateDigest != r.StateDigest ||
		h.ProfileDigest != r.ProfileDigest || h.RecipientNodeID != r.NodeID || h.RecipientDutyGeneration != r.DutyGeneration ||
		!now.Before(lease.Deadline) || lease.Deadline.After(r.NotAfter) || !now.Before(request.Deadline) || request.Deadline.After(lease.Deadline) {
		return nil, errors.New("closed JOIN admission unavailable")
	}
	pair := owner.entries[request.Secret]
	if pair != nil {
		owner.expireLocked(pair, now, wall)
		if pair.stopped || pair.paired || pair.context != request.Context || pair.profile != h.ProfileDigest || pair.sides[request.Side-1] != nil {
			return nil, errors.New("closed JOIN counterpart unavailable")
		}
	} else if len(owner.entries) >= closedDutyChannels {
		return nil, errors.New("closed JOIN capacity exhausted")
	}
	if err := lease.duty.reserveChild(); err != nil {
		return nil, err
	}
	if pair == nil {
		end := now.Add(10 * time.Second)
		if request.Deadline.Before(end) {
			end = request.Deadline
		}
		pair = &closedJoinPair{secret: request.Secret, context: request.Context, profile: h.ProfileDigest, setup: end,
			setupWall: wall.Add(end.Sub(now)), ready: make(chan struct{}), dataReady: make(chan struct{}), done: make(chan struct{})}
		owner.entries[request.Secret] = pair
		pair.timer = owner.schedule(end.Sub(now), func() {
			owner.mu.Lock()
			defer owner.mu.Unlock()
			if !pair.paired {
				owner.stopLocked(pair)
			}
		})
	}
	side := &ClosedJoinSide{owner: owner, pair: pair, duty: lease.duty, nonce: request.Nonce, used: 3*closedLaneHeaderSize + 209 + 355 + 5 + closedLaneHeaderSize + 4096, deadline: lease.Deadline, wallDeadline: wall.Add(lease.Deadline.Sub(now))}
	lease.duty = nil
	pair.sides[request.Side-1] = side
	side.timer = owner.schedule(side.deadline.Sub(now), func() { owner.mu.Lock(); defer owner.mu.Unlock(); owner.stopLocked(pair) })
	if pair.sides[0] != nil && pair.sides[1] != nil {
		pair.paired = true
		pair.timer.Stop()
		close(pair.ready)
	}
	return side, nil
}

func (owner *ClosedJoinPairs) expireLocked(pair *closedJoinPair, now, wall time.Time) {
	if !pair.paired && (!now.Before(pair.setup) || !wall.Before(pair.setupWall)) {
		owner.stopLocked(pair)
	}
	for _, side := range pair.sides {
		if side != nil && (!now.Before(side.deadline) || !wall.Before(side.wallDeadline)) {
			owner.stopLocked(pair)
		}
	}
}

func (owner *ClosedJoinPairs) stopLocked(pair *closedJoinPair) {
	if pair.stopped {
		return
	}
	pair.stopped = true
	pair.timer.Stop()
	for _, side := range pair.sides {
		if side != nil && side.timer != nil {
			side.timer.Stop()
		}
	}
	close(pair.done)
}

func (side *ClosedJoinSide) wait(ctx context.Context, data bool) error {
	if side == nil || ctx == nil {
		return errors.New("closed JOIN wait unavailable")
	}
	side.owner.mu.Lock()
	side.owner.expireLocked(side.pair, side.owner.limits.clock().UTC(), time.Now())
	side.owner.mu.Unlock()
	ready := side.pair.ready
	if data {
		ready = side.pair.dataReady
	}
	select {
	case <-ctx.Done():
		side.Abort()
		return ctx.Err()
	case <-side.pair.done:
		return errors.New("closed JOIN ended")
	case <-ready:
	}
	side.owner.mu.Lock()
	defer side.owner.mu.Unlock()
	side.owner.expireLocked(side.pair, side.owner.limits.clock().UTC(), time.Now())
	if err := ctx.Err(); err != nil {
		side.owner.stopLocked(side.pair)
		return err
	}
	if side.closed || side.pair.stopped {
		return errors.New("closed JOIN ended")
	}
	return nil
}

func (side *ClosedJoinSide) WaitPair(ctx context.Context) error { return side.wait(ctx, false) }
func (side *ClosedJoinSide) WaitData(ctx context.Context) error { return side.wait(ctx, true) }

func (side *ClosedJoinSide) Result() ([]byte, error) {
	if side == nil {
		return nil, errors.New("closed JOIN result unavailable")
	}
	owner := side.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
	if side.closed || side.pair.stopped || !side.pair.paired || side.resultTaken {
		return nil, errors.New("closed JOIN result unavailable")
	}
	side.resultTaken = true
	return EncodeClosedJoinResult(side.nonce, 0)
}

func (side *ClosedJoinSide) ConfirmResult() error {
	if side == nil {
		return errors.New("closed JOIN confirmation unavailable")
	}
	owner := side.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.expireLocked(side.pair, owner.limits.clock().UTC(), time.Now())
	if side.closed || side.pair.stopped || !side.resultTaken || side.confirmed {
		return errors.New("closed JOIN confirmation unavailable")
	}
	side.confirmed = true
	pair := side.pair
	if pair.sides[0].confirmed && pair.sides[1].confirmed {
		close(pair.dataReady)
	}
	return nil
}

// Done signals termination, not release of the held admission. Handlers must
// interrupt and join their actual I/O before returning that reserve with Close.
func (side *ClosedJoinSide) Done() <-chan struct{} { return side.pair.done }

func (side *ClosedJoinSide) Abort() {
	if side == nil {
		return
	}
	side.owner.mu.Lock()
	defer side.owner.mu.Unlock()
	side.owner.stopLocked(side.pair)
}

func (side *ClosedJoinSide) Close() {
	if side == nil {
		return
	}
	side.owner.mu.Lock()
	side.owner.stopLocked(side.pair)
	stream := side.stream
	side.owner.mu.Unlock()
	if stream != nil {
		<-stream.finished
	}
	side.release()
}

func (side *ClosedJoinSide) release() {
	if side == nil {
		return
	}
	owner := side.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if side.closed {
		return
	}
	owner.stopLocked(side.pair)
	side.closed = true
	side.duty.release()
	side.duty = nil
	clear(side.nonce[:])
	pair := side.pair
	for _, other := range pair.sides {
		if other != nil && !other.closed {
			return
		}
	}
	delete(owner.entries, pair.secret)
	clear(pair.secret[:])
	clear(pair.context[:])
	clear(pair.profile[:])
	if owner.closed && len(owner.entries) == 0 {
		owner.drainOnce.Do(func() { close(owner.drained) })
	}
}

func (owner *ClosedJoinPairs) Close() {
	if owner == nil {
		return
	}
	owner.mu.Lock()
	owner.closed = true
	for _, pair := range owner.entries {
		owner.stopLocked(pair)
	}
	if len(owner.entries) == 0 {
		owner.drainOnce.Do(func() { close(owner.drained) })
	}
	owner.mu.Unlock()
	<-owner.drained
	owner.timers.Wait()
}

// Each scheduled callback returns its ownership exactly once, whether stopped
// before execution or joined after it starts. Scheduling holds owner.mu; Close
// first bars new reservations and only waits outside that mutex.
type closedJoinTimer struct {
	timer    *time.Timer
	finished sync.Once
	owner    *ClosedJoinPairs
}

func (owner *ClosedJoinPairs) schedule(delay time.Duration, run func()) *closedJoinTimer {
	tracked := &closedJoinTimer{owner: owner}
	owner.timers.Add(1)
	tracked.timer = time.AfterFunc(delay, func() {
		defer tracked.finished.Do(owner.timers.Done)
		run()
	})
	return tracked
}

func (timer *closedJoinTimer) Stop() {
	if timer.timer.Stop() {
		timer.finished.Do(timer.owner.timers.Done)
	}
}
