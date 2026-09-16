package resource

import (
	"context"
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

// Sample charges the delta and returns the same committed interface reading.
func (owner *Hosting) Sample(ctx context.Context) (sample HostingSample, outcome error) {
	observation, err := owner.transact(ctx, func(state *hostingState, now time.Time) error {
		sample.At, sample.Policy, sample.Boot = now, state.Policy, state.Reading.Boot
		for _, counter := range state.Reading.Interfaces {
			sample.Interfaces = append(sample.Interfaces, HostingInterfaceSample(counter))
		}
		return nil
	})
	sample.Observation = observation
	return sample, err
}
