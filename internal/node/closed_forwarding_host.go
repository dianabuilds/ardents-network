package node

import (
	"context"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// closedForwardingHost keeps the provider-period ledger outside Route. The
// concrete production adapter opens only the already initialized local root;
// tests may provide a bounded owner without selecting provider facts.
type closedForwardingHost interface {
	Sample(context.Context, time.Duration) (resource.HostingSample, error)
	Reserve(context.Context, resource.HostingTraffic, resource.HostingTraffic, time.Time) (closedForwardingHostReservation, error)
	Close() error
}

type closedForwardingHostReservation interface {
	Release(context.Context) error
}

// sharedForwardingCacheMaxTTL caps the lifetime of a cached successful
// Sample for concurrent retained duties on the same root. A successful Reserve
// invalidates the cache immediately so reserved-byte growth becomes visible
// without waiting for the TTL; Release does not invalidate because shrinking
// ReservedBytes never raises pressure on any reader of the shared cache, and
// per-stream Releases would otherwise clear the cache faster than concurrent
// observations can coalesce on it. The TTL keeps an idle period cheap.
const sharedForwardingCacheMaxTTL = 1 * time.Second

type installedClosedForwardingHost struct {
	owner     *resource.Hosting
	sampler   *sharedClosedForwardingSampler
	closeOnce sync.Once
	closeErr  error
}

type sharedClosedForwardingSampler struct {
	mu            sync.Mutex
	root          string
	refs          uint64
	active        *closedForwardingSample
	cachedSample  *closedForwardingSample
	cachedAt      time.Time
	invalidations uint64
}

type closedForwardingSample struct {
	done               chan struct{}
	sample             resource.HostingSample
	err                error
	startInvalidations uint64
}

var closedForwardingHosts = struct {
	sync.Mutex
	samplers map[string]*sharedClosedForwardingSampler
}{samplers: make(map[string]*sharedClosedForwardingSampler)}

func openClosedForwardingHost(root string) (closedForwardingHost, error) {
	owner, err := resource.OpenHosting(root)
	if err != nil {
		return nil, err
	}
	closedForwardingHosts.Lock()
	sampler := closedForwardingHosts.samplers[root]
	if sampler == nil {
		sampler = &sharedClosedForwardingSampler{root: root}
		closedForwardingHosts.samplers[root] = sampler
	}
	sampler.refs++
	closedForwardingHosts.Unlock()
	return &installedClosedForwardingHost{owner: owner, sampler: sampler}, nil
}

func (host *installedClosedForwardingHost) Reserve(ctx context.Context, work, termination resource.HostingTraffic, end time.Time) (closedForwardingHostReservation, error) {
	inner, err := host.owner.Reserve(ctx, work, termination, end)
	if err != nil {
		return nil, err
	}
	// A successful Reserve changes ReservedBytes; invalidate the shared cache
	// so the next Sample observes the fresh reservation.
	host.sampler.invalidate()
	return &sharedSamplerReservation{inner: inner, sampler: host.sampler}, nil
}

func (host *installedClosedForwardingHost) Close() error {
	host.closeOnce.Do(func() {
		host.closeErr = host.owner.Close()
		closedForwardingHosts.Lock()
		sampler := host.sampler
		if sampler != nil && sampler.refs > 0 && closedForwardingHosts.samplers[sampler.root] == sampler {
			sampler.refs--
			if sampler.refs == 0 {
				delete(closedForwardingHosts.samplers, sampler.root)
			}
		}
		closedForwardingHosts.Unlock()
	})
	return host.closeErr
}

func (host *installedClosedForwardingHost) Sample(ctx context.Context, maximumAge time.Duration) (resource.HostingSample, error) {
	if maximumAge <= 0 {
		return host.owner.Sample(ctx, maximumAge)
	}
	host.sampler.mu.Lock()
	now := time.Now()
	if cached := host.sampler.cachedSample; cached != nil &&
		now.Sub(host.sampler.cachedAt) <= maximumAge &&
		now.Sub(host.sampler.cachedAt) <= sharedForwardingCacheMaxTTL {
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
	active := &closedForwardingSample{done: make(chan struct{})}
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
func (sampler *sharedClosedForwardingSampler) invalidate() {
	sampler.mu.Lock()
	sampler.cachedSample = nil
	sampler.invalidations++
	sampler.mu.Unlock()
}

type sharedSamplerReservation struct {
	inner   *resource.HostingReservation
	sampler *sharedClosedForwardingSampler
}

func (r *sharedSamplerReservation) Release(ctx context.Context) error {
	// Release shrinks ReservedBytes, which never raises resource pressure on
	// any reader of the shared cache. Invalidate on Reserve only; per-stream
	// Releases would otherwise clear the cache faster than concurrent
	// observations can coalesce on it.
	return r.inner.Release(ctx)
}
