package state

import (
	"os"
	"time"

	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
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
	observation, err := trustedObservation(config, state)
	return observation.Instant(), err
}

func trustedObservation(config config, state distributionState) (networkdomain.TrustedTime, error) {
	if config.clock == nil || config.observe == nil {
		return networkdomain.TrustedTime{}, errClockUncertain
	}
	evidence := networkdomain.ClockEvidence{Wall: config.clock(), Monotonic: config.anchorWall.Add(time.Since(config.anchorMono)), Independent: config.observe()}
	observation, err := networkdomain.ConfirmTime(evidence, state.trustedTimeFloor)
	if err != nil {
		return networkdomain.TrustedTime{}, errClockUncertain
	}
	return observation, nil
}
