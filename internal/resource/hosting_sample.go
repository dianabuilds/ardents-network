package resource

import (
	"context"
	"errors"
	"time"
)

// HostingSample preserves the actual provider policy and separate counters.
// It describes the shared host; it does not invent per-process attribution.
type HostingSample struct {
	At          time.Time
	Policy      HostingPolicy
	Boot        string
	Interfaces  []HostingInterfaceSample
	Observation HostingObservation
}
type HostingInterfaceSample struct {
	Name  string
	Index uint64
	Tx    uint64
	Rx    uint64
}

// Sample charges an unobserved delta, or shares a recent committed host
// observation across local owners. Reservations never use this coalescing path.
func (owner *Hosting) Sample(ctx context.Context, maximumAge time.Duration) (sample HostingSample, outcome error) {
	if maximumAge < 0 || maximumAge > time.Second {
		return sample, errors.New("hosting sample freshness is invalid")
	}
	state, observation, err := owner.transact(ctx, maximumAge, nil)
	sample.At, sample.Policy, sample.Boot = state.Observed, state.Policy, state.Reading.Boot
	for _, counter := range state.Reading.Interfaces {
		sample.Interfaces = append(sample.Interfaces, HostingInterfaceSample(counter))
	}
	sample.Observation = observation
	return sample, err
}
