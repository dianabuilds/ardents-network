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
	if maximumAge > 0 {
		state, observation, recent, err := owner.recentHostingState(ctx, maximumAge)
		if err != nil {
			return sample, err
		}
		if recent {
			return hostingSample(state, observation), nil
		}
	}
	state, observation, err := owner.transact(ctx, maximumAge, nil)
	return hostingSample(state, observation), err
}

func (owner *Hosting) recentHostingState(ctx context.Context, maximumAge time.Duration) (hostingState, HostingObservation, bool, error) {
	var empty hostingState
	unavailable := HostingObservation{Protect: true, Drain: true}
	if owner == nil || ctx == nil || ctx.Err() != nil {
		return empty, unavailable, false, errors.New("hosting operation is unavailable")
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || ctx.Err() != nil {
		return empty, unavailable, false, errors.New("hosting owner is unavailable")
	}
	lease, err := acquireHostingReadLease(ctx, owner.root)
	if err != nil {
		return empty, unavailable, false, err
	}
	state, err := readHostingState(owner.root)
	if err != nil {
		return empty, unavailable, false, errors.Join(err, lease.close())
	}
	now := owner.now()
	if now.Before(state.Observed) {
		return empty, unavailable, false, errors.Join(errors.New("hosting observation continuity is unavailable"), lease.close())
	}
	if err := lease.close(); err != nil {
		return empty, unavailable, false, err
	}
	return state, state.observation(now), now.Sub(state.Observed) <= maximumAge, nil
}

func hostingSample(state hostingState, observation HostingObservation) (sample HostingSample) {
	sample.At, sample.Policy, sample.Boot = state.Observed, state.Policy, state.Reading.Boot
	for _, counter := range state.Reading.Interfaces {
		sample.Interfaces = append(sample.Interfaces, HostingInterfaceSample(counter))
	}
	sample.Observation = observation
	return sample
}
