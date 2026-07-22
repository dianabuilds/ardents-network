package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	localauth "ardents/internal/localapi/auth"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

type Admitter interface {
	AdmitTarget(context.Context, identityaccess.TargetAttempt) (identityaccess.AuthorizedCall, error)
}

type ResourceCanonicalizer func(string, any) (identityaccess.ResourceTarget, error)

type OperatorInterceptorConfig struct {
	Access         Admitter
	Node           string
	FallbackPeer   [32]byte
	FallbackSource identityaccess.SourceKey
	Canonicalize   ResourceCanonicalizer
}

type operatorInterceptor struct{ config OperatorInterceptorConfig }

func NewOperatorInterceptor(config OperatorInterceptorConfig) (connect.Interceptor, error) {
	if config.Access == nil || config.Node == "" || config.Canonicalize == nil {
		return nil, fmt.Errorf("Operator Principal interceptor dependencies are required")
	}
	return &operatorInterceptor{config: config}, nil
}

func (i *operatorInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if hasUnknownFields(request.Any()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator request"))
		}
		if request.Spec().Procedure == ardentsv1connect.ContentServicePublishBlobProcedure {
			call, err := i.admitDeferred(ctx, request.Spec().Procedure, request.Header(), func() (identityaccess.ResourceTarget, error) {
				return i.config.Canonicalize(request.Spec().Procedure, request.Any())
			})
			if err != nil {
				return nil, err
			}
			return next(rpc.WithAuthorizedCall(ctx, call), request)
		}
		target, err := i.config.Canonicalize(request.Spec().Procedure, request.Any())
		if err != nil {
			return nil, operatorTargetError(err)
		}
		call, err := i.admit(ctx, request.Spec().Procedure, request.Header(), target)
		if err != nil {
			return nil, err
		}
		return next(rpc.WithAuthorizedCall(ctx, call), request)
	}
}

func (i *operatorInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *operatorInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, connection connect.StreamingHandlerConn) error {
		request, err := receiveOperatorStreamRequest(connection)
		if err != nil {
			return err
		}
		target, err := i.config.Canonicalize(connection.Spec().Procedure, request)
		if err != nil {
			return operatorTargetError(err)
		}
		call, err := i.admit(ctx, connection.Spec().Procedure, connection.RequestHeader(), target)
		if err != nil {
			return err
		}
		return next(rpc.WithAuthorizedCall(ctx, call), &prefetchedConn{StreamingHandlerConn: connection, request: request})
	}
}

type prefetchedConn struct {
	connect.StreamingHandlerConn
	request  proto.Message
	consumed bool
}

func (c *prefetchedConn) Receive(message any) error {
	if c.consumed {
		return c.StreamingHandlerConn.Receive(message)
	}
	destination, ok := message.(proto.Message)
	if !ok {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator request"))
	}
	proto.Reset(destination)
	proto.Merge(destination, c.request)
	c.consumed = true
	return nil
}

func receiveOperatorStreamRequest(connection connect.StreamingHandlerConn) (proto.Message, error) {
	if connection.Spec().Procedure != ardentsv1connect.NodeServiceStreamNodeEventsProcedure {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("operator action is not registered"))
	}
	request := &protocol.StreamNodeEventsRequest{}
	if err := connection.Receive(request); err != nil {
		return nil, err
	}
	if hasUnknownFields(request) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator request"))
	}
	return request, nil
}

func (i *operatorInterceptor) admit(ctx context.Context, procedure string, header http.Header, target identityaccess.ResourceTarget) (identityaccess.AuthorizedCall, error) {
	return i.admitAttempt(ctx, procedure, header, target, nil)
}

func (i *operatorInterceptor) admitDeferred(ctx context.Context, procedure string, header http.Header, resolve func() (identityaccess.ResourceTarget, error)) (identityaccess.AuthorizedCall, error) {
	return i.admitAttempt(ctx, procedure, header, identityaccess.ResourceTarget{}, resolve)
}

func (i *operatorInterceptor) admitAttempt(ctx context.Context, procedure string, header http.Header, target identityaccess.ResourceTarget, resolve func() (identityaccess.ResourceTarget, error)) (identityaccess.AuthorizedCall, error) {
	rule, known := localauth.RuleForProcedure(procedure)
	if !known || rule.Action == "" || rule.ResourceKind == "" {
		return identityaccess.AuthorizedCall{}, connect.NewError(connect.CodePermissionDenied, errors.New("operator action is not registered"))
	}
	secret, err := parseOperatorSession(header)
	if err != nil {
		return identityaccess.AuthorizedCall{}, connect.NewError(connect.CodeUnauthenticated, errInvalidSessionHeader)
	}
	binding, _ := OperatorBinding(ctx, i.config.Node, i.config.FallbackPeer, i.config.FallbackSource)
	action, err := identityaccess.ParseAction(binding.Audience.Interface, rule.Action)
	if err != nil {
		return identityaccess.AuthorizedCall{}, connect.NewError(connect.CodePermissionDenied, errors.New("operator action is not registered"))
	}
	if resolve == nil && target.Kind != identityaccess.ResourceKind(rule.ResourceKind) {
		return identityaccess.AuthorizedCall{}, connect.NewError(connect.CodePermissionDenied, errors.New("operator access denied"))
	}
	if resolve != nil {
		canonicalize := resolve
		resolve = func() (identityaccess.ResourceTarget, error) {
			resolved, resolveErr := canonicalize()
			if resolveErr != nil || resolved.Kind != identityaccess.ResourceKind(rule.ResourceKind) {
				return identityaccess.ResourceTarget{}, identityaccess.ErrInvalidResourceTarget
			}
			return resolved, nil
		}
	}
	call, err := i.config.Access.AdmitTarget(ctx, identityaccess.TargetAttempt{
		SessionSecret: secret,
		Binding:       binding,
		Action:        action,
		Target:        target,
		ResolveTarget: resolve,
		Finalize:      finalizeOperatorTarget,
	})
	if err != nil {
		return identityaccess.AuthorizedCall{}, operatorAdmissionError(err)
	}
	return call, nil
}

func operatorTargetError(err error) error {
	if errors.Is(err, localauth.ErrUnknownProcedure) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("operator action is not registered"))
	}
	return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator request"))
}

func finalizeOperatorTarget(target identityaccess.ResourceTarget, audience identityaccess.Audience, _, effective string) (identityaccess.ResourceRef, error) {
	if audience.Interface != identityprotocol.Interface_INTERFACE_OPERATOR {
		return identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
	}
	contract, known := identitycontract.LookupResourceKind(string(target.Kind))
	if !known {
		return identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
	}
	owner := ""
	if contract.OwnerRequired {
		owner = effective
	}
	return identityaccess.NewResourceRef(audience.Node, owner, string(target.Kind), target.ID)
}

func operatorAdmissionError(err error) error {
	switch {
	case errors.Is(err, identityaccess.ErrUnauthenticated):
		return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication failed"))
	case errors.Is(err, identityaccess.ErrInvalidResourceTarget):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid operator request"))
	case errors.Is(err, identityaccess.ErrPermissionDenied), errors.Is(err, identityaccess.ErrInvalidArgument):
		return connect.NewError(connect.CodePermissionDenied, errors.New("operator access denied"))
	default:
		return connect.NewError(connect.CodeUnavailable, errors.New("operator access unavailable"))
	}
}
