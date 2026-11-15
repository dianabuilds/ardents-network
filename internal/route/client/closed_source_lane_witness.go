//go:build linux

package client

import (
	"io"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// A verified peer CLOSE makes a later, unemitted CREDIT unnecessary. The
// caller also requires the attempted write itself to have returned EOF, so a
// concurrent local close cannot manufacture success after this snapshot.
func (lane *closedSourceLane) writeWitness() (uint64, bool, bool) {
	owner := lane.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	busy := owner.active != nil && owner.active.lane == lane
	clean := lane.remoteClosed && lane.failure == io.EOF && owner.terminal == nil && !lane.physicalWriteFailed
	return lane.emissions, busy, clean
}

// Count every non-CREDIT physical attempt across cleanup. The final witness is
// valid only while the exact lower lane and its owner are still live: peer
// CLOSE(0) cannot be inferred from local retirement or a failed parent.
func (lane *closedSourceLane) closeWriteWitness() (uint64, bool, bool) {
	owner := lane.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	active := owner.active != nil && owner.active.lane == lane
	payload := active && owner.active.frame.Kind != ardp.KindCredit
	clean := !lane.closed && !active && lane.remoteClosed && lane.failure == io.EOF && owner.terminal == nil && !lane.physicalWriteFailed
	return lane.closePayloadEmissions, payload, clean
}

// A peer CLOSE can be decoded while the original CREDIT is still completing
// its physical write. That CREDIT is not payload from the nested CLOSE, but its
// actual result must be known before a clean no-payload witness is possible.
// The caller already applied its original cleanup deadline to this lane. Join
// outside the state lock and reobserve the complete witness afterward; this
// method alone grants no clean result and never clears a physical failure.
func (lane *closedSourceLane) joinCloseCredit() {
	owner := lane.owner
	owner.mu.Lock()
	var pending <-chan struct{}
	if !lane.closed && lane.remoteClosed && lane.failure == io.EOF && owner.terminal == nil && !lane.physicalWriteFailed &&
		owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind == ardp.KindCredit {
		pending = owner.active.done
	}
	owner.mu.Unlock()
	if pending != nil {
		<-pending
	}
}
