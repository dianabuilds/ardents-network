//go:build linux || darwin

package node

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

// checkLocal reobserves the original Guard and participation producer without
// borrowing State. It is shared by the independent Node supervisor and the full
// effect observation. No successful authority or previous measurement is cached.
func (f *forwarding) checkLocal() (processguard.Observation, error) {
	f.mu.Lock()
	denied := f.denied
	f.mu.Unlock()
	if denied || f.ctx.Err() != nil {
		return processguard.Observation{}, f.retired
	}
	observation, err := f.process.Check()
	if err != nil {
		return processguard.Observation{}, f.failedObservation(err, "process")
	}
	if err := f.producer.Check(); err != nil {
		return processguard.Observation{}, f.failedObservation(err, "participation")
	}
	if time.Since(observation.Completed) > time.Second {
		return processguard.Observation{}, f.failedObservation(expiredMeasurement(observation), "process")
	}
	f.mu.Lock()
	denied = f.denied
	f.mu.Unlock()
	if denied || f.ctx.Err() != nil {
		return processguard.Observation{}, f.retired
	}
	return observation, nil
}
