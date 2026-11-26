package state

import (
	"os"
	"time"

	networkdomain "github.com/dianabuilds/ardents-network/internal/domain/network"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
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

func trustedNow(config config, state distribution.Record) (time.Time, error) {
	observation, err := trustedObservation(config, state)
	if err != nil {
		return time.Time{}, err
	}
	return observation.Instant(), nil
}

func trustedObservation(config config, state distribution.Record) (networkdomain.TrustedTime, error) {
	if config.clock == nil || config.observe == nil {
		return networkdomain.TrustedTime{}, errClockUncertain
	}
	return confirmClockEvidence(sampleClockEvidence(config), state)
}

func sampleClockEvidence(config config) networkdomain.ClockEvidence {
	return networkdomain.ClockEvidence{Wall: config.clock(), Monotonic: config.anchorWall.Add(time.Since(config.anchorMono)), Independent: config.observe()}
}

func confirmClockEvidence(evidence networkdomain.ClockEvidence, state distribution.Record) (networkdomain.TrustedTime, error) {
	observation, err := networkdomain.ConfirmTime(evidence, state.TrustedTimeFloor)
	if err != nil {
		return networkdomain.TrustedTime{}, errClockUncertain
	}
	return observation, nil
}
