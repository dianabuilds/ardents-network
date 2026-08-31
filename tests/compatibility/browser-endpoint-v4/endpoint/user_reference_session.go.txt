//go:build browsercompat

package endpoint

import (
	"context"
	"sync"
)

// UserReferenceState is one non-topological User-visible state of an exact
// Target Link attempt. It does not expose a peer, Route, or transport detail.
type UserReferenceState string

const (
	UserReferenceStarting    UserReferenceState = "starting"
	UserReferenceReady       UserReferenceState = "ready"
	UserReferenceUnavailable UserReferenceState = "unavailable"
	UserReferenceStopped     UserReferenceState = "stopped"
)

// UserReferenceEvent reports one lifecycle transition for a C-2 Reference
// Site attempt. Ready is populated only after exact Target authentication and
// local-origin creation. Class and Reason retain the bounded Endpoint result
// vocabulary; they never carry a Route error or other topology diagnostic.
type UserReferenceEvent struct {
	State  UserReferenceState
	Ready  ReferenceReady
	Class  string
	Reason string
}

// UserReferenceSession owns an asynchronous User C-2 Reference Site attempt.
// It makes early delivery failures visible through Events rather than requiring
// a caller to interpret an unstructured setup error. It does not add discovery,
// retries, a local proxy, or a second browser authority.
type UserReferenceSession struct {
	endpoint *endpoint
	cancel   context.CancelFunc
	events   chan UserReferenceEvent

	mu      sync.Mutex
	site    *UserReferenceSite
	closing bool
	once    sync.Once
}

// StartUserReferenceSite starts the same exact C-2 route as
// OpenUserReferenceSite, but reports its complete user-facing lifecycle. The
// initial Starting event is always first. Events closes after an Unavailable or
// Stopped event; callers must not infer success from a closed channel.
func (endpoint *endpoint) StartUserReferenceSite(ctx context.Context, input UserReferenceSiteRequest) *UserReferenceSession {
	return endpoint.startUserReferenceSite(ctx, func(ctx context.Context) (*UserReferenceSite, error) {
		return endpoint.OpenUserReferenceSite(ctx, input)
	})
}

// StartAlphaUserReferenceSite starts one already-resolved alpha destination
// through the same bounded C-2 lifecycle as StartUserReferenceSite. Its
// Ready event carries the exact alpha `.ard` HTTP origin and its local Browser
// Entry proxy input, never an internal Target Link.
func (endpoint *endpoint) StartAlphaUserReferenceSite(ctx context.Context, input AlphaUserReferenceSiteRequest) *UserReferenceSession {
	return endpoint.startUserReferenceSite(ctx, func(ctx context.Context) (*UserReferenceSite, error) {
		return endpoint.OpenAlphaUserReferenceSite(ctx, input)
	})
}

func (endpoint *endpoint) startUserReferenceSite(ctx context.Context, open func(context.Context) (*UserReferenceSite, error)) *UserReferenceSession {
	session := &UserReferenceSession{endpoint: endpoint, events: make(chan UserReferenceEvent, 3)}
	session.emit(UserReferenceEvent{State: UserReferenceStarting})
	if ctx == nil {
		session.emitUnavailable()
		close(session.events)
		return session
	}
	lifetime, cancel := context.WithCancel(ctx)
	session.cancel = cancel
	go session.run(lifetime, open)
	return session
}

func (session *UserReferenceSession) run(ctx context.Context, opener func(context.Context) (*UserReferenceSite, error)) {
	defer close(session.events)
	if session.endpoint == nil || opener == nil {
		session.emitUnavailable()
		return
	}
	site, err := opener(ctx)
	if err != nil {
		if session.isClosing() {
			session.emitStopped(RuntimeResult{Class: "local timeout or cancellation", Reason: "Target connection was stopped locally"})
		} else {
			session.emitUnavailable()
		}
		return
	}
	session.mu.Lock()
	session.site = site
	closing := session.closing
	session.mu.Unlock()
	if closing {
		_ = site.Close()
	}
	ready, open := <-site.Ready()
	if open {
		session.emit(UserReferenceEvent{State: UserReferenceReady, Ready: ready})
	}
	outcome, open := <-site.Done()
	if !open {
		outcome = ReferenceOutcome{Result: RuntimeResult{Class: "service unavailable", Reason: "Target connection ended without a result"}}
	}
	if session.isClosing() || outcome.Result.Class == "clean service connection close" {
		session.emitStopped(outcome.Result)
		return
	}
	if outcome.Result.Class == "" {
		session.emitUnavailable()
		return
	}
	session.emit(UserReferenceEvent{State: UserReferenceUnavailable, Class: outcome.Result.Class, Reason: outcome.Result.Reason})
}

// Events carries the ordered, bounded lifecycle projection for this attempt.
func (session *UserReferenceSession) Events() <-chan UserReferenceEvent {
	if session == nil {
		return nil
	}
	return session.events
}

// Close withdraws the scoped local origin and cancels a pending C-2 attempt.
// It never closes a caller-owned Browser or changes ordinary browser traffic.
func (session *UserReferenceSession) Close() error {
	if session == nil {
		return nil
	}
	var result error
	session.once.Do(func() {
		session.mu.Lock()
		session.closing = true
		site := session.site
		cancel := session.cancel
		session.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if site != nil {
			result = site.Close()
		}
	})
	return result
}

func (session *UserReferenceSession) emit(value UserReferenceEvent) {
	if session != nil {
		session.events <- value
	}
}

func (session *UserReferenceSession) emitUnavailable() {
	session.emit(UserReferenceEvent{State: UserReferenceUnavailable, Class: "service unavailable", Reason: "Target connection could not be established"})
}

func (session *UserReferenceSession) emitStopped(result RuntimeResult) {
	class, reason := result.Class, result.Reason
	if class == "" {
		class = "clean service connection close"
	}
	if reason == "" {
		reason = "Target connection stopped"
	}
	session.emit(UserReferenceEvent{State: UserReferenceStopped, Class: class, Reason: reason})
}

func (session *UserReferenceSession) isClosing() bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.closing
}
