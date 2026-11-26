package state

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

// finishWaveState translates a Network transition into the same copied journal.
// No outcome becomes visible until the application commits it.
func finishWaveState(state *distribution.Record, now time.Time, outcomes [4]byte) error {
	history, err := acquisitionHistory(*state)
	if err != nil {
		return err
	}
	next, err := history.Finish(now, outcomes)
	if err != nil {
		return err
	}
	applyAcquisition(state, next)
	state.Sequence++
	return nil
}
