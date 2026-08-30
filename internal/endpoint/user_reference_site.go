package endpoint

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// UserReferenceSiteRequest contains the User-local authority needed for one
// selected C-2 route and Reference Site. It deliberately has no caller-owned
// Route, Application, Target, Publication, or recovery opener: those facts
// stay bound to the exact Link and State-selected Introduction request.
type UserReferenceSiteRequest struct {
	Introduction                                UserIntroductionRouteRequest
	Reachability                                *UserReachabilityRouteRequest
	Routes                                      map[string]string
	Principal                                   [32]byte
	Capability                                  [32]byte
	BytesEachDirection, SendBytes, ReceiveBytes uint32
	Browser                                     ReferenceBrowser
}

// AlphaUserReferenceSiteRequest carries a single resolved alpha binding into
// the existing selected C-2 route. Route must not carry a caller-supplied
// Target Link: Endpoint derives the private-lookup target internally from the
// verified binding and presents only its provisional exact alpha `.ard` HTTP
// origin through the Endpoint-owned Browser Entry proxy.
type AlphaUserReferenceSiteRequest struct {
	Binding alpha.Binding
	Route   UserReferenceSiteRequest
}

// AlphaTransparentUserReferenceSiteRequest carries one verified alpha binding
// into the selected C-2 route for the payload-neutral HTTP bridge. It does not
// add a different route, Target, or content-specific Publisher integration.
type AlphaTransparentUserReferenceSiteRequest struct {
	Binding alpha.Binding
	Route   UserReferenceSiteRequest
}

// UserReferenceSite owns one User Introduction route and the Reference Site
// connection that consumes it. It withdraws both on Close or terminal Service
// Connection result; its browser surface remains the scoped ReferenceConnection
// rather than a generic local proxy.
type UserReferenceSite struct {
	reference *ReferenceConnection
	route     *UserIntroductionRoute

	once     sync.Once
	closeErr error
}

// OpenUserReferenceSite composes one exact State-selected C-2 route with one
// authenticated static Reference Site. A Reachability request is the selected
// Target Link path either performs the selected private lookup or verifies its
// supplied controlled-evidence descriptor before C-2 Entry work; the retained
// Introduction form exists only for direct controlled evidence. It does not
// discover peers, retry a route, or expose a raw carrier to its caller.
func (endpoint *endpoint) OpenUserReferenceSite(ctx context.Context, input UserReferenceSiteRequest) (*UserReferenceSite, error) {
	return endpoint.openUserReferenceSite(ctx, input, nil, false)
}

// OpenAlphaUserReferenceSite composes one verified alpha binding with the
// existing exact C-2 delivery. It does not accept an alpha string, resolve a
// name, or allow a caller to substitute a Target Link. The internal Target
// Link exists only to drive the already-bounded private reachability protocol;
// it is never returned as an alpha resolution result or browser address.
func (endpoint *endpoint) OpenAlphaUserReferenceSite(ctx context.Context, input AlphaUserReferenceSiteRequest) (*UserReferenceSite, error) {
	return endpoint.openAlphaUserReferenceSite(ctx, input.Binding, input.Route, false)
}

// OpenAlphaTransparentUserReferenceSite composes one verified alpha binding
// with the selected C-2 delivery and its generic HTTP bridge.
func (endpoint *endpoint) OpenAlphaTransparentUserReferenceSite(ctx context.Context, input AlphaTransparentUserReferenceSiteRequest) (*UserReferenceSite, error) {
	return endpoint.openAlphaUserReferenceSite(ctx, input.Binding, input.Route, true)
}

func (endpoint *endpoint) openAlphaUserReferenceSite(ctx context.Context, binding alpha.Binding, input UserReferenceSiteRequest, transparent bool) (*UserReferenceSite, error) {
	if endpoint == nil || binding.Network() != endpoint.network {
		return nil, ErrAlphaBindingNetwork
	}
	return endpoint.openUserReferenceSite(ctx, input, &binding, transparent)
}

func (endpoint *endpoint) openUserReferenceSite(ctx context.Context, input UserReferenceSiteRequest, alphaBinding *alpha.Binding, transparent bool) (*UserReferenceSite, error) {
	if endpoint == nil || ctx == nil || input.Principal == [32]byte{} || input.Capability == [32]byte{} ||
		(input.BytesEachDirection == 0 && input.SendBytes == 0 && input.ReceiveBytes == 0) {
		return nil, errors.New("user Reference Site input is incomplete")
	}
	routeInput := userRouteRequest{Introduction: input.Introduction, Reachability: input.Reachability}
	var err error
	if alphaBinding != nil {
		targetLink, encodeErr := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: alphaBinding.Target()})
		if encodeErr != nil {
			return nil, encodeErr
		}
		routeInput, err = bindAlphaUserRoute(routeInput, targetLink)
		if err != nil {
			return nil, err
		}
	}
	route, targetLink, at, err := endpoint.openUserRoute(ctx, routeInput)
	if err != nil {
		return nil, err
	}
	closeRoute := func(cause error) (*UserReferenceSite, error) {
		return nil, errors.Join(cause, route.Close())
	}
	connection := OutboundConnectionRequest{
		Principal: input.Principal, Capability: input.Capability, Target: route.AuthenticatedTarget,
		AuthorityPublic: route.AuthorityPublic, Publication: route.Publication, Route: route.Connection,
		BytesEachDirection: input.BytesEachDirection, SendBytes: input.SendBytes, ReceiveBytes: input.ReceiveBytes,
		At: at}
	var reference *ReferenceConnection
	if alphaBinding == nil {
		reference, err = endpoint.StartReferenceConnection(ctx, ReferenceConnectionRequest{TargetLink: targetLink,
			Routes: input.Routes, Browser: input.Browser, Connection: connection})
	} else if transparent {
		reference, err = endpoint.StartAlphaTransparentConnection(ctx, AlphaTransparentConnectionRequest{Binding: *alphaBinding,
			Browser: input.Browser, Connection: connection})
	} else {
		reference, err = endpoint.StartAlphaReferenceConnection(ctx, AlphaReferenceConnectionRequest{Binding: *alphaBinding,
			Routes: input.Routes, Browser: input.Browser, Connection: connection})
	}
	if err != nil {
		return closeRoute(err)
	}
	running := &UserReferenceSite{reference: reference, route: route}
	go func() {
		<-reference.terminated()
		_ = route.Close()
	}()
	return running, nil
}

type userRouteRequest struct {
	Introduction UserIntroductionRouteRequest
	Reachability *UserReachabilityRouteRequest
}

func (endpoint *endpoint) openUserRoute(ctx context.Context, input userRouteRequest) (*UserIntroductionRoute, string, time.Time, error) {
	if input.Reachability != nil && input.Introduction.TargetLink != "" {
		return nil, "", time.Time{}, errors.New("user Application received two route authorities")
	}
	if input.Reachability != nil {
		route, err := endpoint.OpenUserReachabilityRoute(ctx, *input.Reachability)
		return route, input.Reachability.TargetLink, input.Reachability.At, err
	}
	route, err := endpoint.OpenUserIntroductionRoute(ctx, input.Introduction)
	return route, input.Introduction.TargetLink, input.Introduction.At, err
}

func bindAlphaUserRoute(input userRouteRequest, targetLink string) (userRouteRequest, error) {
	if input.Reachability != nil {
		if input.Reachability.TargetLink != "" || input.Introduction.TargetLink != "" {
			return userRouteRequest{}, errors.New("alpha Application route must not supply a Target Link")
		}
		reachability := *input.Reachability
		reachability.TargetLink = targetLink
		input.Reachability = &reachability
		return input, nil
	}
	if input.Introduction.TargetLink != "" {
		return userRouteRequest{}, errors.New("alpha Application route must not supply a Target Link")
	}
	input.Introduction.TargetLink = targetLink
	return input, nil
}

// Ready reports the exact local origin only after C-2 delivery and Target
// authentication completed.
func (site *UserReferenceSite) Ready() <-chan ReferenceReady {
	if site == nil || site.reference == nil {
		return nil
	}
	return site.reference.Ready()
}

// Done reports the underlying classified Service Connection outcome.
func (site *UserReferenceSite) Done() <-chan ReferenceOutcome {
	if site == nil || site.reference == nil {
		return nil
	}
	return site.reference.Done()
}

// Close withdraws the browser origin. Its Entry-owned C-2 route remains live
// until the native Service Connection has reported its terminal outcome, so a
// local presentation close cannot race its final authenticated stream records.
func (site *UserReferenceSite) Close() error {
	if site == nil {
		return nil
	}
	site.once.Do(func() {
		if site.reference != nil {
			site.closeErr = errors.Join(site.closeErr, site.reference.Close())
			return
		}
		if site.route != nil {
			site.closeErr = errors.Join(site.closeErr, site.route.Close())
		}
	})
	return site.closeErr
}
