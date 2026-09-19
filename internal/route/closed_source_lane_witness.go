//go:build linux

package route

import "io"

// A verified peer CLOSE makes a later, unemitted CREDIT unnecessary. This
// witness never treats a raw EOF, local close, or failed parent as peer success.
func (lane *closedSourceLane) writeWitness() (uint64, bool, bool) {
	owner := lane.owner
	owner.mu.Lock()
	defer owner.mu.Unlock()
	busy := owner.active != nil && owner.active.lane == lane
	clean := !lane.closed && lane.remoteClosed && lane.failure == io.EOF && owner.terminal == nil && !lane.physicalWriteFailed
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
	payload := active && owner.active.frame.Kind != closedFrameCredit
	clean := !lane.closed && !active && lane.remoteClosed && lane.failure == io.EOF && owner.terminal == nil && !lane.physicalWriteFailed
	return lane.closePayloadEmissions, payload, clean
}
