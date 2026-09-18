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
		state, observation, err = owner.refreshHostingState(ctx, maximumAge)
		return hostingSample(state, observation), err
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
	if err := owner.enter(ctx); err != nil {
		return empty, unavailable, false, err
	}
	defer owner.leave()
	if owner.closed || ctx.Err() != nil {
		return empty, unavailable, false, errors.New("hosting owner is unavailable")
	}
	lease, available, err := tryAcquireHostingReadLease(ctx, owner.root)
	if err != nil {
		return empty, unavailable, false, err
	}
	var state hostingState
	if available {
		state, err = readHostingState(owner.root)
	} else {
		// A current exclusive writer proves period.pending belongs to an
		// in-flight transaction. The atomically replaced period.json remains
		// the last complete committed observation and may be shared only while
		// it still satisfies the caller's existing freshness bound.
		state, err = readCommittedHostingState(owner.root)
	}
	if err != nil {
		if available {
			err = errors.Join(err, lease.close())
		}
		return empty, unavailable, false, err
	}
	now := owner.now()
	if now.Before(state.Observed) {
		err = errors.New("hosting observation continuity is unavailable")
		if available {
			err = errors.Join(err, lease.close())
		}
		return empty, unavailable, false, err
	}
	if available {
		if err := lease.close(); err != nil {
			return empty, unavailable, false, err
		}
	}
	return state, state.observation(now), now.Sub(state.Observed) <= maximumAge, nil
}

// refreshHostingState elects at most one expired-sample writer. Contenders do
// not queue as writers: they observe the active writer's atomically replaced
// committed state and return as soon as it satisfies the requested freshness.
func (owner *Hosting) refreshHostingState(ctx context.Context, maximumAge time.Duration) (hostingState, HostingObservation, error) {
	var empty hostingState
	unavailable := HostingObservation{Protect: true, Drain: true}
	if err := owner.enter(ctx); err != nil {
		return empty, unavailable, err
	}
	defer owner.leave()
	if owner.closed {
		return empty, unavailable, errors.New("hosting owner is unavailable")
	}
	retryDelay := time.Millisecond
	for {
		if err := ctx.Err(); err != nil {
			return empty, unavailable, err
		}
		lease, elected, err := tryAcquireHostingWriteLease(ctx, owner.root)
		if err != nil {
			return empty, unavailable, err
		}
		if elected {
			state, err := readHostingState(owner.root)
			if err != nil {
				return empty, unavailable, errors.Join(err, lease.close())
			}
			now := owner.now()
			if now.Before(state.Observed) {
				return empty, unavailable, errors.Join(errors.New("hosting observation continuity is unavailable"), lease.close())
			}
			if now.Sub(state.Observed) > maximumAge {
				reading, measureErr := owner.measure(state.Policy.Interfaces)
				if measureErr == nil {
					measureErr = state.observe(reading, now)
				}
				if measureErr != nil {
					return empty, unavailable, errors.Join(measureErr, lease.close())
				}
				if err := ctx.Err(); err != nil {
					return empty, unavailable, errors.Join(err, lease.close())
				}
				if err := writeHostingState(owner.root, state); err != nil {
					return empty, unavailable, errors.Join(err, lease.close())
				}
			}
			observation := state.observation(now)
			if err := lease.close(); err != nil {
				return empty, unavailable, err
			}
			return state, observation, nil
		}

		// period.json is replaced atomically before its directory commit. While
		// the elected writer retains the lease it is the only valid progress
		// source, so a verified fresh replacement is safe to share immediately.
		state, readErr := readCommittedHostingState(owner.root)
		if readErr == nil {
			now := owner.now()
			if now.Before(state.Observed) {
				return empty, unavailable, errors.New("hosting observation continuity is unavailable")
			}
			if now.Sub(state.Observed) <= maximumAge {
				return state, state.observation(now), nil
			}
		}
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return empty, unavailable, ctx.Err()
		case <-timer.C:
		}
		if retryDelay < 32*time.Millisecond {
			retryDelay *= 2
		}
	}
}

func hostingSample(state hostingState, observation HostingObservation) (sample HostingSample) {
	sample.At, sample.Policy, sample.Boot = state.Observed, state.Policy, state.Reading.Boot
	for _, counter := range state.Reading.Interfaces {
		sample.Interfaces = append(sample.Interfaces, HostingInterfaceSample(counter))
	}
	sample.Observation = observation
	return sample
}
