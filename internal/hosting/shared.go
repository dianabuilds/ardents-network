package hosting

import (
	"context"
	"errors"
	"sync"
	"time"
)

// sharedHostingCacheMaxTTL caps the lifetime of a cached successful
// observation for concurrent retained duties on the same root, measured from its
// At. Every Reserve attempt
// invalidates the cache before returning so growth or a storage refusal becomes visible
// without waiting for the TTL; Release does not invalidate because shrinking
// ReservedBytes never raises pressure on any reader of the shared cache, and
// per-stream Releases would otherwise clear the cache faster than concurrent
// observations can coalesce on it. The TTL keeps an idle period cheap.
const sharedHostingCacheMaxTTL = 1 * time.Second

// Shared owns one independently closable handle with root-wide bounded sampling.
// Copies share lifecycle state; closing a copy cannot release another handle.
type Shared struct{ *sharedHandle }

type sharedHandle struct {
	owner    *Ledger
	sampler  *sharedClosedHostingSampler
	gate     chan struct{}
	closed   bool
	closeErr error
}

type sharedClosedHostingSampler struct {
	mu            sync.Mutex
	root          string
	refs          uint64
	active        *closedHostingSample
	cachedSample  *closedHostingSample
	invalidations uint64
}

type closedHostingSample struct {
	done               chan struct{}
	sample             Sample
	err                error
	startInvalidations uint64
}

var closedHostingHandles = struct {
	sync.Mutex
	samplers map[string]*sharedClosedHostingSampler
}{samplers: make(map[string]*sharedClosedHostingSampler)}

// OpenShared acquires one handle of an already initialized local Hosting period.
func OpenShared(root string) (*Shared, error) {
	owner, err := Open(root)
	if err != nil {
		return nil, err
	}
	closedHostingHandles.Lock()
	sampler := closedHostingHandles.samplers[root]
	if sampler == nil {
		sampler = &sharedClosedHostingSampler{root: root}
		closedHostingHandles.samplers[root] = sampler
	}
	sampler.refs++
	closedHostingHandles.Unlock()
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return &Shared{&sharedHandle{owner: owner, sampler: sampler, gate: gate}}, nil
}

func (host *Shared) Reserve(ctx context.Context, work, termination Traffic, end time.Time) (*Reservation, error) {
	if err := host.enter(ctx); err != nil {
		return nil, err
	}
	defer host.leave()
	// A failed transaction may have committed a debit or exposed ambiguous storage.
	// Neither old healthy facts nor a flight elected before this result may survive.
	defer host.sampler.invalidate()
	handle, err := host.owner.Reserve(ctx, work, termination, end)
	if err != nil {
		return nil, err
	}
	// A successful release may keep a conservative cached debit. A failure may
	// mean storage continuity is unavailable, which cached success must not hide.
	handle.state.invalidateFailure = host.sampler.invalidate
	return handle, nil
}

func (host *Shared) Close() error {
	if host == nil || host.sharedHandle == nil {
		return nil
	}
	<-host.gate
	defer host.leave()
	if host.closed {
		return host.closeErr
	}
	host.closed = true
	host.closeErr = host.owner.Close()
	closedHostingHandles.Lock()
	sampler := host.sampler
	if sampler.refs > 0 && closedHostingHandles.samplers[sampler.root] == sampler {
		sampler.refs--
		if sampler.refs == 0 {
			delete(closedHostingHandles.samplers, sampler.root)
		}
	}
	closedHostingHandles.Unlock()
	return host.closeErr
}

func (host *Shared) enter(ctx context.Context) error {
	if host == nil || host.sharedHandle == nil || ctx == nil {
		return errors.New("hosting handle is unavailable")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-host.gate:
	}
	if host.closed || ctx.Err() != nil {
		host.leave()
		return errors.New("hosting handle is unavailable")
	}
	return nil
}
func (host *Shared) leave() { host.gate <- struct{}{} }

func (host *Shared) Sample(ctx context.Context, maximumAge time.Duration) (Sample, error) {
	if err := host.enter(ctx); err != nil {
		return Sample{}, err
	}
	defer host.leave()
	return host.sampler.sample(ctx, maximumAge, time.Now, host.owner.Sample)
}

// cloneSample keeps both the elected reader and cache consumers from acquiring
// mutable ownership of the shared observation's slice backing arrays.
func cloneSample(sample Sample) Sample {
	sample.Policy.Interfaces = append([]string(nil), sample.Policy.Interfaces...)
	sample.Interfaces = append([]InterfaceSample(nil), sample.Interfaces...)
	return sample
}

func (sampler *sharedClosedHostingSampler) sample(ctx context.Context, maximumAge time.Duration, clock func() time.Time, read func(context.Context, time.Duration) (Sample, error)) (Sample, error) {
	// Invalid requests and unavailable contexts must reach the owner's refusal
	// boundary even when a successful shared observation exists.
	if maximumAge <= 0 || maximumAge > sharedHostingCacheMaxTTL || ctx == nil || ctx.Err() != nil {
		return read(ctx, maximumAge)
	}
	for {
		sampler.mu.Lock()
		now := clock()
		if cached := sampler.cachedSample; cached != nil && cached.reusable(now, maximumAge, sampler.invalidations) {
			sample := cloneSample(cached.sample)
			sampler.mu.Unlock()
			return sample, nil
		}
		if active := sampler.active; active != nil {
			sampler.mu.Unlock()
			select {
			case <-ctx.Done():
				return Sample{}, ctx.Err()
			case <-active.done:
				sampler.mu.Lock()
				reusable := active.reusable(clock(), maximumAge, sampler.invalidations)
				sampler.mu.Unlock()
				if ctx.Err() != nil {
					return Sample{}, ctx.Err()
				}
				if active.err != nil {
					return cloneSample(active.sample), active.err
				}
				if reusable {
					return cloneSample(active.sample), nil
				}
				// A flight elected by a looser caller cannot satisfy this caller's
				// freshness or a reservation made during that flight. Elect/join again.
				continue
			}
		}
		active := &closedHostingSample{done: make(chan struct{}), startInvalidations: sampler.invalidations}
		sampler.active = active
		sampler.mu.Unlock()

		sample, err := read(ctx, maximumAge)
		sampler.mu.Lock()
		active.sample, active.err = cloneSample(sample), err
		if err == nil && active.reusable(clock(), maximumAge, sampler.invalidations) {
			sampler.cachedSample = active
		}
		sampler.active = nil
		close(active.done)
		sampler.mu.Unlock()
		// The elected reader retains the resource owner's current decision, including
		// Drain after period expiry. Only reuse by other calls requires a live period.
		return sample, err
	}
}

func (sample *closedHostingSample) reusable(now time.Time, maximumAge time.Duration, invalidations uint64) bool {
	return sample.err == nil && sample.startInvalidations == invalidations &&
		!sample.sample.At.IsZero() && !now.Before(sample.sample.At) &&
		now.Sub(sample.sample.At) <= maximumAge && now.Sub(sample.sample.At) <= sharedHostingCacheMaxTTL &&
		!now.Before(sample.sample.Policy.Start) && now.Before(sample.sample.Policy.End)
}

// invalidate drops the cached Sample and bumps the invalidation counter so an
// in-flight Sample can detect that ReservedBytes may have changed while it was
// reading. Counter, not the sample itself, is what the flight compares.
func (sampler *sharedClosedHostingSampler) invalidate() {
	sampler.mu.Lock()
	sampler.cachedSample = nil
	sampler.invalidations++
	sampler.mu.Unlock()
}
