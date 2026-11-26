package processguard

import "time"

const (
	maximumObservationSpan    = time.Second
	maximumObservationAge     = time.Second
	memoryHighBytes           = 768 << 20
	rssLimitBytes             = 768 << 20
	goMemoryLimitBytes        = 512 << 20
	fileDescriptorLimit       = 1024
	threadLimit               = 256
	trendPressureLimitPercent = 1
)

// Observation projects actual native readings only. Route timer/queue counts
// are not supplied by this owner and cannot be inferred from these fields.
// CPUFraction is measured over the original declared elapsed interval.
type Observation struct {
	Started, Completed, CPUWindowStarted                 time.Time
	CPUUsec, MemoryBytes, RSSBytes, GoMemoryBytes        uint64
	FDs, Threads                                         uint64
	CPUFraction, CPUPressure, MemoryPressure, IOPressure float64
	HighEvents, EmergencyEvents, PIDEvents               uint64
}

func validateObservation(observation Observation, now time.Time) error {
	if !observation.isCurrent(now) {
		return &admissionFailure{reason: "observation-stale"}
	}
	// The owning development contract admits brief measured CPU/I/O stalls;
	// established trend pressure and every hard/event boundary still retire.
	if observation.HighEvents != 0 || observation.EmergencyEvents != 0 || observation.PIDEvents != 0 {
		return &admissionFailure{reason: "kernel-limit-event"}
	}
	if observation.MemoryBytes >= memoryHighBytes || observation.RSSBytes >= rssLimitBytes ||
		observation.GoMemoryBytes >= goMemoryLimitBytes || observation.FDs >= fileDescriptorLimit ||
		observation.Threads >= threadLimit {
		return &admissionFailure{reason: "resource-bound"}
	}
	if observation.MemoryPressure > 0 {
		return &admissionFailure{reason: "memory-pressure"}
	}
	if observation.CPUPressure >= trendPressureLimitPercent {
		return &admissionFailure{reason: "cpu-pressure"}
	}
	if observation.IOPressure >= trendPressureLimitPercent {
		return &admissionFailure{reason: "io-pressure"}
	}
	return nil
}

func (observation Observation) isCurrent(now time.Time) bool {
	started, completed := observation.Started, observation.Completed
	if started.IsZero() || completed.IsZero() {
		return false
	}
	if completed.Before(started) || now.Before(completed) {
		return false
	}
	duration := completed.Sub(started)
	age := now.Sub(completed)
	return duration <= maximumObservationSpan && age <= maximumObservationAge
}
