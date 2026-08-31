package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

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

// UserApplicationConnectionRequest contains the Connection-surface input for
// one explicit headless Application stream. Endpoint retains Target
// authentication, Route opening, one-use transport inputs, and the Local Grant
// capability; the caller receives none of those facts.
type UserApplicationConnectionRequest struct {
	Introduction                                UserIntroductionRouteRequest
	Reachability                                *UserReachabilityRouteRequest
	Principal                                   [32]byte
	BytesEachDirection, SendBytes, ReceiveBytes uint32
}

// ApplicationConnection is one authenticated, reliable, ordered byte stream
// returned by the Connection Interface. Application protocol and replay
// semantics remain entirely caller-owned.
type ApplicationConnection struct {
	stream io.ReadWriteCloser
	cancel context.CancelFunc
	done   chan applicationconnection.Outcome
	once   sync.Once
}

// OpenUserApplicationConnection opens an explicit Target-Link Application
// stream through the same Endpoint owner used by optional Adapters.
func (endpoint *endpoint) OpenUserApplicationConnection(ctx context.Context, input UserApplicationConnectionRequest) (*ApplicationConnection, error) {
	return endpoint.openUserApplicationConnection(ctx, input, nil)
}

func (endpoint *endpoint) openAlphaApplicationConnection(ctx context.Context, binding alpha.Binding, input UserApplicationConnectionRequest) (*ApplicationConnection, error) {
	if endpoint == nil || binding.Network() != endpoint.network || binding.Target() == [32]byte{} {
		return nil, ErrAlphaBindingNetwork
	}
	return endpoint.openUserApplicationConnection(ctx, input, &binding)
}

func (endpoint *endpoint) openUserApplicationConnection(ctx context.Context, input UserApplicationConnectionRequest,
	binding *alpha.Binding) (*ApplicationConnection, error) {
	if endpoint == nil || ctx == nil || input.Principal == [32]byte{} ||
		(input.BytesEachDirection == 0 && input.SendBytes == 0 && input.ReceiveBytes == 0) {
		return nil, errors.New("user Application Connection input is incomplete")
	}
	routeInput := userRouteRequest{Introduction: input.Introduction, Reachability: input.Reachability}
	if binding != nil {
		link, err := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: binding.Target()})
		if err != nil {
			return nil, err
		}
		routeInput, err = bindAlphaUserRoute(routeInput, link)
		if err != nil {
			return nil, err
		}
	}
	route, _, at, err := endpoint.openUserRoute(ctx, routeInput)
	if err != nil {
		return nil, err
	}
	failRoute := func(cause error) (*ApplicationConnection, error) {
		return nil, errors.Join(cause, route.Close())
	}
	capability, err := endpoint.Admit(input.Principal, broker.Connection)
	if err != nil {
		return failRoute(errors.New("local Application Connection admission is unavailable"))
	}
	owned, application := net.Pipe()
	lifetime, cancel := context.WithCancel(ctx)
	ready := make(chan struct{})
	done := make(chan applicationconnection.Outcome, 1)
	request := OutboundConnectionRequest{Principal: input.Principal, Capability: capability,
		Target: route.AuthenticatedTarget, AuthorityPublic: route.AuthorityPublic, Publication: route.Publication,
		Route: route.Connection, Application: owned, BytesEachDirection: input.BytesEachDirection,
		SendBytes: input.SendBytes, ReceiveBytes: input.ReceiveBytes, At: at,
		OnAuthenticated: func(authenticated [32]byte) error {
			if authenticated != route.AuthenticatedTarget {
				return errors.New("application Connection authenticated a different Target")
			}
			close(ready)
			return nil
		}}
	go func() {
		result, runErr := endpoint.Connect(lifetime, request)
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
	connection := &ApplicationConnection{stream: application, cancel: cancel, done: done}
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

func (connection *ApplicationConnection) Read(destination []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, io.ErrClosedPipe
	}
	return connection.stream.Read(destination)
}

func (connection *ApplicationConnection) Write(source []byte) (int, error) {
	if connection == nil || connection.stream == nil {
		return 0, io.ErrClosedPipe
	}
	return connection.stream.Write(source)
}

// Done carries exactly one terminal outcome and then closes.
func (connection *ApplicationConnection) Done() <-chan applicationconnection.Outcome {
	if connection == nil {
		return nil
	}
	return connection.done
}

// Close stops only this Application stream. It cannot withdraw a Service or
// mutate Network, Entry, or custody state.
func (connection *ApplicationConnection) Close() error {
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
