package hosting

import (
	"context"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// Handle keeps the provider-period ledger outside Route. A receiving duty
// borrows it until its accepted work has joined.
type Handle interface {
	Sample(context.Context, time.Duration) (resource.HostingSample, error)
	Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (Reservation, error)
	Close() error
}

// sharedHostingCacheMaxTTL caps the lifetime of a cached successful
// observation for concurrent retained duties on the same root, measured from its
// At. A successful Reserve
// invalidates the cache immediately so reserved-byte growth becomes visible
// without waiting for the TTL; Release does not invalidate because shrinking
// ReservedBytes never raises pressure on any reader of the shared cache, and
// per-stream Releases would otherwise clear the cache faster than concurrent
// observations can coalesce on it. The TTL keeps an idle period cheap.
const sharedHostingCacheMaxTTL = 1 * time.Second

type Ledger struct {
	owner     *resource.Hosting
	sampler   *sharedClosedHostingSampler
	closeOnce sync.Once
	closeErr  error
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
	sample             resource.HostingSample
	err                error
	startInvalidations uint64
}

var closedHostingHandles = struct {
	sync.Mutex
	samplers map[string]*sharedClosedHostingSampler
}{samplers: make(map[string]*sharedClosedHostingSampler)}

// Open acquires one handle of an already initialized local Hosting period.
func Open(root string) (*Ledger, error) {
	owner, err := resource.OpenHosting(root)
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
	return &Ledger{owner: owner, sampler: sampler}, nil
}

func (host *Ledger) Reserve(ctx context.Context, work, termination resource.HostingTraffic, end time.Time) (Reservation, error) {
	inner, err := host.owner.Reserve(ctx, work, termination, end)
	if err != nil {
		return nil, err
	}
	// A successful Reserve changes ReservedBytes; invalidate the shared cache
	// so the next Sample observes the fresh reservation.
	host.sampler.invalidate()
	return &sharedSamplerReservation{inner: inner, sampler: host.sampler}, nil
}

func (host *Ledger) Close() error {
	host.closeOnce.Do(func() {
		host.closeErr = host.owner.Close()
		closedHostingHandles.Lock()
		sampler := host.sampler
		if sampler != nil && sampler.refs > 0 && closedHostingHandles.samplers[sampler.root] == sampler {
			sampler.refs--
			if sampler.refs == 0 {
				delete(closedHostingHandles.samplers, sampler.root)
			}
		}
		closedHostingHandles.Unlock()
	})
	return host.closeErr
}

func (host *Ledger) Sample(ctx context.Context, maximumAge time.Duration) (resource.HostingSample, error) {
	return host.sampler.sample(ctx, maximumAge, time.Now, host.owner.Sample)
}

func (sampler *sharedClosedHostingSampler) sample(ctx context.Context, maximumAge time.Duration, clock func() time.Time, read func(context.Context, time.Duration) (resource.HostingSample, error)) (resource.HostingSample, error) {
	// Invalid requests and unavailable contexts must reach the owner's refusal
	// boundary even when a successful shared observation exists.
	if maximumAge <= 0 || maximumAge > sharedHostingCacheMaxTTL || ctx == nil || ctx.Err() != nil {
		return read(ctx, maximumAge)
	}
	for {
		sampler.mu.Lock()
		now := clock()
		if cached := sampler.cachedSample; cached != nil && cached.reusable(now, maximumAge, sampler.invalidations) {
			sample := cached.sample
			sampler.mu.Unlock()
			return sample, nil
		}
		if active := sampler.active; active != nil {
			sampler.mu.Unlock()
			select {
			case <-ctx.Done():
				return resource.HostingSample{}, ctx.Err()
			case <-active.done:
				sampler.mu.Lock()
				reusable := active.reusable(clock(), maximumAge, sampler.invalidations)
				sampler.mu.Unlock()
				if ctx.Err() != nil {
					return resource.HostingSample{}, ctx.Err()
				}
				if active.err != nil {
					return active.sample, active.err
				}
				if reusable {
					return active.sample, nil
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
		active.sample, active.err = sample, err
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

type sharedSamplerReservation struct {
	inner   *resource.HostingReservation
	sampler *sharedClosedHostingSampler
}

func (r *sharedSamplerReservation) Release(ctx context.Context) error {
	// Release shrinks ReservedBytes, which never raises resource pressure on
	// any reader of the shared cache. Invalidate on Reserve only; per-stream
	// Releases would otherwise clear the cache faster than concurrent
	// observations can coalesce on it.
	return r.inner.Release(ctx)
}
