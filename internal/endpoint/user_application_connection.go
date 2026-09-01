package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

type userRouteRequest struct {
	Introduction userIntroductionRouteRequest
	Reachability *userReachabilityRouteRequest
}

func (endpoint *endpoint) openUserRoute(ctx context.Context, input userRouteRequest) (*userIntroductionRoute, string, time.Time, error) {
	if input.Reachability != nil && input.Introduction.TargetLink != "" {
		return nil, "", time.Time{}, errors.New("user Application received two route authorities")
	}
	if input.Reachability != nil {
		route, err := endpoint.openUserReachabilityRoute(ctx, *input.Reachability)
		return route, input.Reachability.TargetLink, input.Reachability.At, err
	}
	route, err := endpoint.openUserIntroductionRoute(ctx, input.Introduction)
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

// UserApplicationConnectionRequest contains the Connection-surface input for
// one explicit headless Application stream. Endpoint retains Target
// authentication, Route opening, one-use transport inputs, and the Local Grant
// capability; the caller receives none of those facts.
type userApplicationConnectionRequest struct {
	Introduction                                userIntroductionRouteRequest
	Reachability                                *userReachabilityRouteRequest
	Principal                                   [32]byte
	BytesEachDirection, SendBytes, ReceiveBytes uint32
}

// ApplicationConnection is one authenticated, reliable, ordered byte stream
// returned by the Connection Interface. Application protocol and replay
// semantics remain entirely caller-owned.
type applicationConnection struct {
	stream io.ReadWriteCloser
	cancel context.CancelFunc
	done   chan applicationconnection.Outcome
	once   sync.Once
}

// openApplicationConnection opens an explicit Target-Link Application
// stream through the same Endpoint owner used by optional Adapters.
func (endpoint *endpoint) openApplicationConnection(ctx context.Context, input userApplicationConnectionRequest) (*applicationConnection, error) {
	if endpoint == nil || ctx == nil || input.Principal == [32]byte{} {
		return nil, errors.New("user Application Connection input is incomplete")
	}
	session, err := endpoint.beginApplicationSession(ctx, input.Principal)
	if err != nil {
		return nil, err
	}
	return endpoint.openUserApplicationConnection(session.Context(), input, nil, session)
}

func (endpoint *endpoint) openAlphaApplicationConnection(ctx context.Context, binding alpha.Binding, input userApplicationConnectionRequest,
	session *applicationSession) (*applicationConnection, error) {
	if endpoint == nil || binding.Network() != endpoint.network || binding.Target() == [32]byte{} {
		session.Release()
		return nil, ErrAlphaBindingNetwork
	}
	return endpoint.openUserApplicationConnection(ctx, input, &binding, session)
}

func (endpoint *endpoint) openUserApplicationConnection(ctx context.Context, input userApplicationConnectionRequest,
	binding *alpha.Binding, session *applicationSession) (*applicationConnection, error) {
	if endpoint == nil || ctx == nil || input.Principal == [32]byte{} ||
		(input.BytesEachDirection == 0 && input.SendBytes == 0 && input.ReceiveBytes == 0) || session == nil {
		session.Release()
		return nil, errors.New("user Application Connection input is incomplete")
	}
	routeInput := userRouteRequest{Introduction: input.Introduction, Reachability: input.Reachability}
	if binding != nil {
		link, err := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: binding.Target()})
		if err != nil {
			session.Release()
			return nil, err
		}
		routeInput, err = bindAlphaUserRoute(routeInput, link)
		if err != nil {
			session.Release()
			return nil, err
		}
	}
	route, _, at, err := endpoint.openUserRoute(ctx, routeInput)
	if err != nil {
		session.Release()
		return nil, err
	}
	owned, application := net.Pipe()
	lifetime, cancel := context.WithCancel(ctx)
	ready := make(chan struct{})
	done := make(chan applicationconnection.Outcome, 1)
	request := connectionInput{Principal: input.Principal, Target: route.AuthenticatedTarget,
		AuthorityPublic: route.AuthorityPublic, Publication: route.Publication, Route: route.Connection,
		Application: owned, BytesEachDirection: input.BytesEachDirection, SendBytes: input.SendBytes, ReceiveBytes: input.ReceiveBytes, At: at,
		OnAuthenticated: func(authenticated [32]byte) error {
			if authenticated != route.AuthenticatedTarget {
				return errors.New("application Connection authenticated a different Target")
			}
			close(ready)
			return nil
		}}
	go func() {
		defer session.Release()
		result, runErr := endpoint.connectAuthorized(lifetime, request, session.receipt)
		_ = owned.Close()
		closeErr := route.Close()
		if result.Class == "" {
			result.Class = "indeterminate failure"
		}
		if result.Reason == "" && runErr != nil {
			result.Reason = "Application Connection ended without a classified reason"
		}
		if closeErr != nil && runErr == nil {
			result.Class, result.Reason = "indeterminate failure", "Application Connection cleanup failed"
		}
		done <- applicationconnection.Outcome{Class: applicationconnection.OutcomeClass(result.Class), Reason: result.Reason}
		close(done)
	}()
	connection := &applicationConnection{stream: application, cancel: cancel, done: done}
	select {
	case <-ready:
		return connection, nil
	case outcome := <-done:
		_ = connection.Close()
		return nil, errors.New(string(outcome.Class) + ": " + outcome.Reason)
	case <-ctx.Done():
		_ = connection.Close()
		return nil, ctx.Err()
	}
}

func (connection *applicationConnection) Read(destination []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, io.ErrClosedPipe
	}
	return connection.stream.Read(destination)
}

func (connection *applicationConnection) Write(source []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, io.ErrClosedPipe
	}
	return connection.stream.Write(source)
}

// Done carries exactly one terminal outcome and then closes.
func (connection *applicationConnection) Done() <-chan applicationconnection.Outcome {
	if connection == nil {
		return nil
	}
	return connection.done
}

// Close stops only this Application stream. It cannot withdraw a Service or
// mutate Network, Entry, or custody state.
func (connection *applicationConnection) Close() error {
	if connection == nil {
		return nil
	}
	var err error
	connection.once.Do(func() {
		connection.cancel()
		err = connection.stream.Close()
	})
	return err
}
