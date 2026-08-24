package endpoint

import (
	"context"
	"errors"
	"sync"
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
// Target Link path and verifies its descriptor before Entry work; the retained
// Introduction form exists only for direct controlled evidence. It does not
// perform private lookup, discover peers, retry a route, or expose a raw
// carrier to its caller.
func (endpoint *endpoint) OpenUserReferenceSite(ctx context.Context, input UserReferenceSiteRequest) (*UserReferenceSite, error) {
	if endpoint == nil || ctx == nil || input.Principal == [32]byte{} || input.Capability == [32]byte{} ||
		(input.BytesEachDirection == 0 && input.SendBytes == 0 && input.ReceiveBytes == 0) {
		return nil, errors.New("User Reference Site input is incomplete")
	}
	if input.Reachability != nil && input.Introduction.TargetLink != "" {
		return nil, errors.New("User Reference Site received two route authorities")
	}
	var (
		route      *UserIntroductionRoute
		targetLink string
		at         = input.Introduction.At
		err        error
	)
	if input.Reachability != nil {
		route, err = endpoint.OpenUserReachabilityRoute(ctx, *input.Reachability)
		targetLink, at = input.Reachability.TargetLink, input.Reachability.At
	} else {
		route, err = endpoint.OpenUserIntroductionRoute(ctx, input.Introduction)
		targetLink = input.Introduction.TargetLink
	}
	if err != nil {
		return nil, err
	}
	closeRoute := func(cause error) (*UserReferenceSite, error) {
		return nil, errors.Join(cause, route.Close())
	}
	reference, err := endpoint.StartReferenceConnection(ctx, ReferenceConnectionRequest{TargetLink: targetLink,
		Routes: input.Routes, Browser: input.Browser, Connection: OutboundConnectionRequest{
			Principal: input.Principal, Capability: input.Capability, Target: route.AuthenticatedTarget,
			AuthorityPublic: route.AuthorityPublic, Publication: route.Publication, Route: route.Connection,
			BytesEachDirection: input.BytesEachDirection, SendBytes: input.SendBytes, ReceiveBytes: input.ReceiveBytes,
			At: at}})
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
