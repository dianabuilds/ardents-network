package state

import (
	"time"
)

// finishWaveState translates a Network transition into the same copied journal.
// No outcome becomes visible until the application commits it.
func finishWaveState(state *distributionState, now time.Time, outcomes [4]byte) error {
	history, err := acquisitionHistory(*state)
	if err != nil {
		return err
	}
	next, err := history.Finish(now, outcomes)
	if err != nil {
		return err
	}
	applyAcquisition(state, next)
	state.sequence++
	return nil
}
