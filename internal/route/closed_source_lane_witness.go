//go:build linux

package route

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
