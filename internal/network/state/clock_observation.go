package state

import (
	"os"
	"time"
)

func fileClockObserver(path string) func() time.Time {
	return func() time.Time {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return time.Time{}
		}
		return info.ModTime().UTC()
	}
}

func trustedNow(config config, state distributionState) (time.Time, error) {
	now := config.clock().UTC()
	monotonic := config.anchorWall.Add(time.Since(config.anchorMono))
	if monotonic.After(now) {
		now = monotonic
	}
	observation := config.observe().UTC()
	if observation.IsZero() || now.Sub(observation).Abs() > 2*time.Second {
		return time.Time{}, errClockUncertain
	}
	if now.Unix()+2 < state.trustedTimeFloor {
		return time.Time{}, errClockUncertain
	}
	if now.Unix() < state.trustedTimeFloor {
		return time.Unix(state.trustedTimeFloor, 0).UTC(), nil
	}
	return now, nil
}
