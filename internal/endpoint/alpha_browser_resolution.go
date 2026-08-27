package endpoint

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// AlphaBrowserSite is one Endpoint-owned presentation opened because a
// browser requested one verified alpha Service Name. Its lifecycle belongs to
// AlphaBrowserResolution after Open returns; callers must not retain it as a
// second browser or Target authority.
type AlphaBrowserSite interface {
	Ready() <-chan ReferenceReady
	Done() <-chan ReferenceOutcome
	Close() error
}

// AlphaBrowserResolutionRequest supplies the Endpoint-local facts needed to
// turn an `.ard` browser host into one verified alpha binding and one selected
// Service presentation. Open must use the supplied binding exactly and must
// not open a browser itself: the browser request already initiated the path.
type AlphaBrowserResolutionRequest struct {
	Floor *alpha.PersistentFloor
	Clock func() time.Time
	Open  func(context.Context, alpha.Binding) (AlphaBrowserSite, error)
}

// AlphaBrowserResolution owns demand-opened bounded alpha names for one live
// Endpoint. It is not OS DNS, a general HTTP proxy, or a source of new Target
// authority: every name comes from the accepted Endpoint-local corpus floor.
type AlphaBrowserResolution struct {
	endpoint *endpoint
	floor    *alpha.PersistentFloor
	clock    func() time.Time
	open     func(context.Context, alpha.Binding) (AlphaBrowserSite, error)

	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	sites  map[string]AlphaBrowserSite
	closed bool
	once   sync.Once
}

// OpenAlphaBrowserResolution starts the Endpoint-owned resolver hook behind
// the existing loopback alpha proxy. A browser request for `name.ard` then
// resolves only `ardents-alpha://name` from the accepted floor and waits until
// its exact authenticated Service presentation has registered that host.
func (endpoint *endpoint) OpenAlphaBrowserResolution(ctx context.Context, input AlphaBrowserResolutionRequest) (*AlphaBrowserResolution, error) {
	if endpoint == nil || ctx == nil || input.Floor == nil || input.Open == nil {
		return nil, errors.New("alpha browser resolution input is incomplete")
	}
	clock := input.Clock
	if clock == nil {
		clock = time.Now
	}
	lifetime, cancel := context.WithCancel(ctx)
	resolution := &AlphaBrowserResolution{endpoint: endpoint, floor: input.Floor, clock: clock, open: input.Open,
		ctx: lifetime, cancel: cancel, sites: make(map[string]AlphaBrowserSite)}

	endpoint.alphaBrowserMu.Lock()
	if endpoint.alphaBrowserOwners != 0 {
		endpoint.alphaBrowserMu.Unlock()
		cancel()
		return nil, errors.New("alpha browser resolution is already active")
	}
	proxy, err := endpoint.ensureAlphaBrowserProxyLocked()
	if err == nil {
		err = proxy.SetRouteOpener(resolution.openName)
	}
	if err == nil {
		err = endpoint.publishAlphaBrowserProxyLocked(proxy)
	}
	if err == nil {
		endpoint.alphaBrowserOwners++
	}
	closeProxy := false
	if err != nil && endpoint.alphaBrowserRoutes == 0 && endpoint.alphaBrowserOwners == 0 && endpoint.alphaBrowserProxy == proxy {
		endpoint.alphaBrowserProxy = nil
		closeProxy = true
	}
	endpoint.alphaBrowserMu.Unlock()
	if err != nil {
		cancel()
		if closeProxy && proxy != nil {
			_ = proxy.Close()
		}
		return nil, err
	}
	return resolution, nil
}

func (resolution *AlphaBrowserResolution) openName(ctx context.Context, hostname string) error {
	if resolution == nil || ctx == nil || resolution.ctx.Err() != nil {
		return errors.New("alpha browser resolution is unavailable")
	}
	link, err := alphaBrowserServiceLink(hostname)
	if err != nil {
		return err
	}
	at := resolution.clock()
	if at.IsZero() {
		return errors.New("alpha browser resolution clock is unavailable")
	}
	binding, err := resolution.endpoint.ResolveAcceptedAlpha(resolution.floor, link.String(), at)
	if err != nil {
		return alphaBrowserRouteFailure{cause: err, status: alphaBrowserResolutionStatus(err)}
	}
	site, err := resolution.open(resolution.ctx, binding)
	if err != nil || site == nil {
		return alphaBrowserRouteFailure{cause: errors.Join(err, errors.New("alpha browser Service could not open")), status: 502}
	}
	resolution.mu.Lock()
	if resolution.closed {
		resolution.mu.Unlock()
		_ = site.Close()
		return errors.New("alpha browser resolution is unavailable")
	}
	resolution.sites[hostname] = site
	resolution.mu.Unlock()

	select {
	case ready, open := <-site.Ready():
		if !open || ready.URL != "http://"+hostname+"/" || ready.AuthenticatedTarget != binding.Target() {
			resolution.removeSite(hostname, site)
			_ = site.Close()
			return alphaBrowserRouteFailure{cause: errors.New("alpha browser Service did not authenticate its resolved name"), status: 502}
		}
	case <-ctx.Done():
		resolution.removeSite(hostname, site)
		_ = site.Close()
		return alphaBrowserRouteFailure{cause: ctx.Err(), status: 503}
	case <-resolution.ctx.Done():
		resolution.removeSite(hostname, site)
		_ = site.Close()
		return alphaBrowserRouteFailure{cause: errors.New("alpha browser resolution is unavailable"), status: 503}
	}
	go resolution.watchSite(hostname, site)
	return nil
}

type alphaBrowserRouteFailure struct {
	cause  error
	status int
}

func (failure alphaBrowserRouteFailure) Error() string {
	if failure.cause == nil {
		return "alpha browser name is unavailable"
	}
	return failure.cause.Error()
}

func (failure alphaBrowserRouteFailure) Unwrap() error {
	return failure.cause
}

// AlphaRouteHTTPStatus is the only browser-safe failure projection accepted
// by reference.AlphaProxy. It carries no binding, Target, Route, or corpus
// detail.
func (failure alphaBrowserRouteFailure) AlphaRouteHTTPStatus() int {
	return failure.status
}

func alphaBrowserResolutionStatus(err error) int {
	switch {
	case alpha.HasFailure(err, alpha.FailureWithdrawn):
		return 410
	case alpha.HasFailure(err, alpha.FailureExpired), alpha.HasFailure(err, alpha.FailureNotYetValid):
		return 503
	case alpha.HasFailure(err, alpha.FailureConflict):
		return 409
	default:
		return 404
	}
}

func (resolution *AlphaBrowserResolution) watchSite(hostname string, site AlphaBrowserSite) {
	if site == nil {
		return
	}
	<-site.Done()
	resolution.removeSite(hostname, site)
}

func (resolution *AlphaBrowserResolution) removeSite(hostname string, site AlphaBrowserSite) {
	if resolution == nil {
		return
	}
	resolution.mu.Lock()
	if resolution.sites[hostname] == site {
		delete(resolution.sites, hostname)
	}
	resolution.mu.Unlock()
}

// Close stops future demand opens and closes every presentation it owns. It
// never changes browser-wide networking; when the final named presentation has
// ended, the Endpoint withdraws its loopback proxy and Browser Entry state.
func (resolution *AlphaBrowserResolution) Close() error {
	if resolution == nil {
		return nil
	}
	var result error
	resolution.once.Do(func() {
		resolution.cancel()
		resolution.mu.Lock()
		resolution.closed = true
		sites := make([]AlphaBrowserSite, 0, len(resolution.sites))
		for _, site := range resolution.sites {
			sites = append(sites, site)
		}
		clear(resolution.sites)
		resolution.mu.Unlock()
		for _, site := range sites {
			result = errors.Join(result, site.Close())
		}

		endpoint := resolution.endpoint
		endpoint.alphaBrowserMu.Lock()
		if endpoint.alphaBrowserOwners > 0 {
			endpoint.alphaBrowserOwners--
		}
		var closeProxy *reference.AlphaProxy
		if endpoint.alphaBrowserOwners == 0 && endpoint.alphaBrowserRoutes == 0 {
			closeProxy = endpoint.alphaBrowserProxy
			endpoint.alphaBrowserProxy = nil
			if endpoint.browserEntry != nil {
				result = errors.Join(result, endpoint.browserEntry.Clear())
			}
		}
		endpoint.alphaBrowserMu.Unlock()
		if closeProxy != nil {
			result = errors.Join(result, closeProxy.Close())
		}
	})
	return result
}

func alphaBrowserServiceLink(hostname string) (alpha.ServiceLink, error) {
	const suffix = ".ard"
	if !strings.HasSuffix(hostname, suffix) || len(hostname) <= len(suffix) {
		return alpha.ServiceLink{}, errors.New("alpha browser host is invalid")
	}
	return alpha.ParseServiceLink("ardents-alpha://" + strings.TrimSuffix(hostname, suffix))
}
