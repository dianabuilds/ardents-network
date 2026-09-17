//go:build linux

package route

import (
	"context"
	"errors"
	"time"
)

const closedRefillThreshold = 8 << 20

// Replenish refills only channels that consumed actual traffic. The original
// HELLO, peer and deadline remain fixed; Endpoint supplies a fresh spent token.
func (prefix *ClosedSourcePrefix) Replenish(ctx context.Context, present ClosedTokenPresenter) error {
	if prefix == nil || ctx == nil || present == nil || ctx.Err() != nil {
		return errors.New("forwarding refill unavailable")
	}
	prefix.refillMu.Lock()
	defer prefix.refillMu.Unlock()
	if prefix.channels == nil {
		return errors.New("forwarding refill channels unavailable")
	}
	// Completed child work can cross the refill threshold immediately before
	// its lane retires. The accounted bytes, rather than a coincident live lane,
	// decide whether this retained prefix needs more authority. An unused prefix
	// remains below the threshold and therefore spends no token below.
	if err := prefix.plan.current(prefix.source, prefix.selection); err != nil {
		return err
	}
	if prefix.child != nil {
		child := prefix.child
		child.mu.Lock()
		used, base, terminal := child.transferred, child.refillBase, child.terminal
		child.mu.Unlock()
		if terminal != nil {
			return terminal
		}
		if used-base >= closedRefillThreshold {
			frame, err := closedRefillFrame(prefix.hellos[0], present)
			if err != nil {
				return err
			}
			err = child.replenish(ctx, frame)
			clear(frame.Body)
			if err != nil {
				return err
			}
			child.mu.Lock()
			child.refillBase = used
			child.mu.Unlock()
		}
	}
	return prefix.channels.replenish(ctx, prefix.hellos[1], present)
}

func closedRefillFrame(hello ClosedHello, present ClosedTokenPresenter) (ClosedLaneFrame, error) {
	if hello.ChannelNonce == [32]byte{} || !time.Now().Before(hello.Deadline) {
		return ClosedLaneFrame{}, errors.New("refill original authority expired")
	}
	token, err := present(hello, 2)
	defer clear(token)
	if err != nil {
		return ClosedLaneFrame{}, err
	}
	if len(token) != 354 {
		return ClosedLaneFrame{}, errors.New("refill token invalid")
	}
	return ClosedLaneFrame{Kind: closedFrameAdmit, Lane: 0, Body: append([]byte{2}, token...)}, nil
}

func (owner *closedSourceChannels) replenish(ctx context.Context, hello ClosedHello, present ClosedTokenPresenter) error {
	owner.mu.Lock()
	used, base, terminal := owner.transferred, owner.refillBase, owner.terminal
	owner.mu.Unlock()
	if terminal != nil {
		return terminal
	}
	if used-base < closedRefillThreshold {
		return nil
	}
	frame, err := closedRefillFrame(hello, present)
	if err != nil {
		return err
	}
	defer clear(frame.Body)
	if err := ctx.Err(); err != nil {
		return err
	}
	result := make(chan error, 1)
	owner.mu.Lock()
	if owner.terminal != nil || owner.refill != nil {
		err := owner.terminal
		owner.mu.Unlock()
		return errors.Join(err, errors.New("closed source refill is unavailable"))
	}
	owner.refill = result
	owner.mu.Unlock()
	control := &closedSourceLane{owner: owner, id: 0, end: owner.end, writeEnd: owner.end, opened: true, active: true}
	if err := control.send(frame, time.Time{}); err != nil {
		owner.mu.Lock()
		if owner.refill == result {
			owner.refill = nil
		}
		owner.mu.Unlock()
		return err
	}
	select {
	case err := <-result:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		owner.fail(ctx.Err())
		return ctx.Err()
	case <-owner.done:
		owner.mu.Lock()
		err := owner.terminal
		owner.mu.Unlock()
		return errors.Join(err, errors.New("closed source refill ended"))
	}
	owner.mu.Lock()
	// In-flight receipt after this snapshot stays charged conservatively.
	owner.refillBase = used
	owner.mu.Unlock()
	return nil
}

// Replenish preserves the same logical JOIN and its original receiver binding.
func (stream *ClosedJoinedStream) Replenish(ctx context.Context, present ClosedTokenPresenter) error {
	if stream == nil || ctx == nil || present == nil {
		return errors.New("JOIN refill unavailable")
	}
	stream.refillMu.Lock()
	defer stream.refillMu.Unlock()
	if stream.refillRetired() {
		return nil
	}
	err := stream.channels.replenish(ctx, stream.hello, present)
	// Service owns and reports the terminal outcome. A CLOSE that races a
	// refill does not create a second workload failure or revive that JOIN.
	if err != nil && stream.refillRetired() {
		return nil
	}
	return err
}

func (stream *ClosedJoinedStream) refillRetired() bool {
	stream.channels.mu.Lock()
	defer stream.channels.mu.Unlock()
	return stream.closedSourceLane.closed || stream.closedSourceLane.remoteClosed
}
