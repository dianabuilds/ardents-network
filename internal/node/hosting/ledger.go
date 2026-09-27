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
// Sample for concurrent retained duties on the same root. A successful Reserve
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
	cachedAt      time.Time
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
	if maximumAge <= 0 {
		return host.owner.Sample(ctx, maximumAge)
	}
	host.sampler.mu.Lock()
	now := time.Now()
	if cached := host.sampler.cachedSample; cached != nil &&
		now.Sub(host.sampler.cachedAt) <= maximumAge &&
		now.Sub(host.sampler.cachedAt) <= sharedHostingCacheMaxTTL {
		sample := cached.sample
		err := cached.err
		host.sampler.mu.Unlock()
		return sample, err
	}
	if active := host.sampler.active; active != nil {
		host.sampler.mu.Unlock()
		select {
		case <-ctx.Done():
			return resource.HostingSample{}, ctx.Err()
		case <-active.done:
			return active.sample, active.err
		}
	}
	active := &closedHostingSample{done: make(chan struct{})}
	active.startInvalidations = host.sampler.invalidations
	host.sampler.active = active
	host.sampler.mu.Unlock()

	sample, err := host.owner.Sample(ctx, maximumAge)
	host.sampler.mu.Lock()
	active.sample = sample
	active.err = err
	// Cache only when no Reserve/Release happened during the flight; the
	// observed ReservedBytes would otherwise still reflect the pre-reserve
	// state and violate the invalidation contract on the next Sample.
	if err == nil && host.sampler.invalidations == active.startInvalidations {
		host.sampler.cachedSample = active
		host.sampler.cachedAt = now
	}
	host.sampler.active = nil
	close(active.done)
	host.sampler.mu.Unlock()
	return sample, err
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
