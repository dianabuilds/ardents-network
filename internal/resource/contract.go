package resource

import (
	"errors"
	"runtime"
	"runtime/metrics"
	"time"
)

// Sample is one bounded process/cgroup observation.
type Sample struct {
	CPUUsageUsec, MemoryBytes, GoMemoryBytes, SocketMemoryBytes uint64
	FDs, Goroutines, Threads, Timers, QueueItems, QueueBytes    uint64
	CPUPressure, MemoryPressure, IOPressure                     float64
	HighEvents, EmergencyEvents                                 uint64
}

// Check verifies the fixed runtime and OS placement for this guard's profile.
func (guard *Guard) Check() error {
	if runtime.GOMAXPROCS(0) != guard.profile.goMaxProcs {
		return errors.New("resource guard GOMAXPROCS does not match its profile")
	}
	samples := []metrics.Sample{{Name: "/gc/gomemlimit:bytes"}}
	metrics.Read(samples)
	if samples[0].Value.Kind() != metrics.KindUint64 {
		return errors.New("resource guard Go memory limit is unavailable")
	}
	limit := samples[0].Value.Uint64()
	if guard.profile.exactGoMemory && limit != guard.profile.goMemory || !guard.profile.exactGoMemory && limit > guard.profile.goMemory {
		return errors.New("resource guard Go memory limit does not match its profile")
	}
	deadline := time.Now().Add(guard.profile.placementWait)
	for {
		if err := checkPlacement(guard.profile); err == nil {
			return nil
		} else if guard.profile.placementWait == 0 || time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Config selects one fixed H3 profile and its measurement Adapter.
type Config struct {
	Profile  string
	Interval time.Duration
	Measure  func() (Sample, error)
}

// Observation is the complete externally relevant governor decision.
type Observation struct {
	Protect bool
	Drain   bool
}

// Guard owns one process's placement and pressure state.
type Guard struct {
	profile profile
	monitor monitor
	measure func() (Sample, error)
}

// New constructs one fixed-profile resource guard.
func New(config Config) (*Guard, error) {
	selected, ok := profiles[config.Profile]
	if !ok || config.Interval <= 0 || config.Interval > time.Second {
		return nil, errors.New("resource guard profile or interval is invalid")
	}
	measure := config.Measure
	if measure == nil {
		measure = func() (Sample, error) { return sampleProcess(selected) }
	}
	return &Guard{profile: selected, monitor: monitor{interval: config.Interval}, measure: measure}, nil
}

// Observe measures current use and advances the finite pressure state.
func (guard *Guard) Observe(timers, queueItems, queueBytes uint64) (Observation, error) {
	sample, err := guard.measure()
	if err != nil {
		return Observation{Protect: true, Drain: true}, err
	}
	sample.Timers, sample.QueueItems, sample.QueueBytes = timers, queueItems, queueBytes
	level := guard.monitor.observe(guard.profile, sample)
	return Observation{Protect: level != levelNormal, Drain: level == levelDrain}, nil
}
