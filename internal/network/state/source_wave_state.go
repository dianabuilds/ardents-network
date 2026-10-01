package state

import (
	"math"
	"time"
)

// finishWaveState applies one completed wave to the copied distribution state.
// The caller commits the resulting journal before exposing it to readers.
func finishWaveState(state *distributionState, now time.Time, outcomes [4]byte) {
	state.sequence++
	state.cycleActive = false
	state.outcomes = outcomes
	for index, status := range state.attempts {
		if status == sourceAttemptInFlight {
			state.attempts[index] = sourceAttemptFailed
		}
	}
	for index, outcome := range outcomes {
		if outcome == sourceOutcomeValid {
			state.attempts[index] = sourceAttemptCompleted
		} else if outcome != 0 {
			state.attempts[index] = sourceAttemptFailed
		}
	}
	for _, outcome := range outcomes {
		if outcome != 0 && outcome != sourceOutcomeValid {
			applyFailureBackoff(state, now)
			return
		}
	}
	state.consecutiveFailures, state.backoffLevel, state.nextAutomatic = 0, 0, 0
}

func applyFailureBackoff(state *distributionState, now time.Time) {
	if state.consecutiveFailures < math.MaxInt64 {
		state.consecutiveFailures++
	}
	level := state.consecutiveFailures - 1
	if level > 5 {
		level = 5
	}
	state.backoffLevel = byte(level)
	bases := [...]int64{60, 120, 240, 480, 960, 1800}
	base := bases[state.backoffLevel]
	state.nextAutomatic = now.Unix() + base/2 + int64(state.cycleSeed[1])*(base/2+1)/256
}
