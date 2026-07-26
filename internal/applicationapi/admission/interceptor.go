// Package admission owns Principal-aware admission for protected Application
// product calls. It is intentionally separate from the public wire handler so
// internal and SDK identity protobuf registries never share a package graph.
// It does not own identity state or product command behavior.
package admission

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	applicationerror "ardents/internal/applicationapi/applicationerror"
	applicationbinding "ardents/internal/applicationapi/binding"
	applicationcall "ardents/internal/applicationapi/call"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"connectrpc.com/connect"
)

const applicationSessionScheme = "ArdentsApplicationSession"

const applicationDelegationHeader = "Ardents-Delegation"

type Admitter interface {
	AdmitTarget(context.Context, identityaccess.TargetAttempt) (identityaccess.AuthorizedCall, error)
}

type successfulMutationRecorder interface {
	RecordSuccessfulMutation(identityaccess.AuthorizedCall)
}

type deniedCallRecorder interface {
	RecordDeniedCall(identityaccess.Audience, identityaccess.Action, identityaccess.DenialReason)
}

type Config struct {
	Access         Admitter
	Node           string
	FallbackPeer   [32]byte
	FallbackSource identityaccess.SourceKey
	Injector       applicationcall.Injector
	Registry       Registry
}

type interceptor struct{ config Config }

func NewInterceptor(config Config) (connect.Interceptor, error) {
	if config.Access == nil || !config.Injector.Valid() || config.Registry == nil ||
		config.FallbackPeer == [32]byte{} || config.FallbackSource == (identityaccess.SourceKey{}) {
		return nil, fmt.Errorf("protected Application admission dependencies are required")
	}
	if _, err := identityprincipal.Parse(config.Node); err != nil {
		return nil, fmt.Errorf("protected Application admission node is required")
	}
	return &interceptor{config: config}, nil
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		rule, registered := i.config.Registry.Lookup(request.Spec().Procedure)
		if !registered {
			i.recordDenied(ctx, "", identityaccess.DenialActionUnregistered)
			return nil, applicationerror.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, "application.content", "application action is forbidden", false, connect.CodePermissionDenied)
		}
		target, err := rule.Resolve(request.Any())
		if err != nil || target.Kind != rule.ResourceKind {
			i.recordDenied(ctx, rule.Action, identityaccess.DenialResourceTarget)
			if err == nil {
				err = identityaccess.ErrInvalidResourceTarget
			}
			return nil, rule.MapTargetErr(err)
		}
		delegation, err := parseDelegation(request.Header())
		if err != nil {
			i.recordDenied(ctx, rule.Action, identityaccess.DenialDelegationPresentation)
			return nil, authenticationError(rule.Action)
		}
		defer clear(delegation)
		deleteHeaderFold(request.Header(), applicationDelegationHeader)
		values := request.Header().Values("Authorization")
		if len(values) != 1 || len(values[0]) > 128 {
			i.recordDenied(ctx, rule.Action, identityaccess.DenialSessionPresentation)
			return nil, authenticationError(rule.Action)
		}
		if !strings.HasPrefix(values[0], applicationSessionScheme+" ") {
			i.recordDenied(ctx, rule.Action, identityaccess.DenialSessionPresentation)
			return nil, authenticationError(rule.Action)
		}
		return i.admitPrincipal(ctx, request, next, rule, target, delegation)
	}
}

func (i *interceptor) admitPrincipal(ctx context.Context, request connect.AnyRequest, next connect.UnaryFunc, rule ProcedureRule, target identityaccess.ResourceTarget, delegation []byte) (connect.AnyResponse, error) {
	secret, err := parseSession(request.Header())
	if err != nil {
		i.recordDenied(ctx, rule.Action, identityaccess.DenialSessionPresentation)
		return nil, authenticationError(rule.Action)
	}
	binding, _ := applicationbinding.Application(ctx, i.config.Node, i.config.FallbackPeer, i.config.FallbackSource)
	action, err := identityaccess.ParseAction(binding.Audience.Interface, rule.Action)
	if err != nil {
		i.recordDenied(ctx, rule.Action, identityaccess.DenialResourceTarget)
		return nil, applicationerror.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, rule.Action, "application action is forbidden", false, connect.CodePermissionDenied)
	}
	admitted, err := i.config.Access.AdmitTarget(ctx, identityaccess.TargetAttempt{
		SessionSecret: secret,
		Binding:       binding,
		Action:        action,
		Target:        target,
		Finalize:      rule.Finalize,
		Delegation:    delegation,
	})
	clear(delegation)
	if err != nil {
		return nil, principalError(rule.Action, err)
	}
	ctx = identityaccess.ContextWithAuthorizedCall(ctx, admitted)
	ctx, injected := i.config.Injector.WithAuthorizedCall(ctx, admitted, rule.ResourceKind, rule.OwnerRequired)
	if !injected {
		i.recordDenied(ctx, rule.Action, identityaccess.DenialResourceTarget)
		return nil, applicationerror.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, rule.Action, "application action is forbidden", false, connect.CodePermissionDenied)
	}
	response, dispatchErr := next(ctx, request)
	if dispatchErr == nil && rule.Mutating {
		if recorder, ok := i.config.Access.(successfulMutationRecorder); ok {
			recorder.RecordSuccessfulMutation(admitted)
		}
	}
	return response, dispatchErr
}

// parseDelegation accepts the one frozen Application presentation form. Header
// names are case-insensitive, including map entries inserted without net/http's
// canonicalization, so differently-cased duplicates cannot bypass the count.
// The encoded and decoded bounds are enforced before the access service parses
// the protobuf artifact.
func parseDelegation(header http.Header) ([]byte, error) {
	var value string
	count := 0
	for name, values := range header {
		if !strings.EqualFold(name, applicationDelegationHeader) {
			continue
		}
		count += len(values)
		if len(values) == 1 {
			value = values[0]
		}
	}
	if count == 0 {
		return nil, nil
	}
	maxEncoded := base64.RawURLEncoding.EncodedLen(identitycontract.MaxArtifactBytes)
	if count != 1 || value == "" || len(value) > maxEncoded {
		return nil, identityaccess.ErrUnauthenticated
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || !identitycontract.ValidArtifactSize(len(raw)) || base64.RawURLEncoding.EncodeToString(raw) != value {
		clear(raw)
		return nil, identityaccess.ErrUnauthenticated
	}
	return raw, nil
}

func deleteHeaderFold(header http.Header, target string) {
	for name := range header {
		if strings.EqualFold(name, target) {
			delete(header, name)
		}
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, _ connect.StreamingHandlerConn) error {
		i.recordDenied(ctx, "", identityaccess.DenialActionUnregistered)
		return connect.NewError(connect.CodePermissionDenied, errors.New("application streaming action is not registered"))
	}
}

func (i *interceptor) recordDenied(ctx context.Context, actionName string, reason identityaccess.DenialReason) {
	recorder, ok := i.config.Access.(deniedCallRecorder)
	if !ok {
		return
	}
	binding, _ := applicationbinding.Application(ctx, i.config.Node, i.config.FallbackPeer, i.config.FallbackSource)
	action := identityaccess.Action("")
	if parsed, err := identityaccess.ParseAction(binding.Audience.Interface, actionName); err == nil {
		action = parsed
	}
	recorder.RecordDeniedCall(binding.Audience, action, reason)
}

func parseSession(header http.Header) (identityaccess.SessionSecret, error) {
	var secret identityaccess.SessionSecret
	values := header.Values("Authorization")
	if len(values) != 1 || len(values[0]) > 128 {
		return secret, identityaccess.ErrUnauthenticated
	}
	prefix := applicationSessionScheme + " "
	if !strings.HasPrefix(values[0], prefix) || strings.Count(values[0], " ") != 1 {
		return secret, identityaccess.ErrUnauthenticated
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(values[0], prefix))
	if err != nil || len(raw) != len(secret) {
		return secret, identityaccess.ErrUnauthenticated
	}
	copy(secret[:], raw)
	return secret, nil
}

func authenticationError(operation string) error {
	return applicationerror.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, operation, "application authentication required", false, connect.CodeUnauthenticated)
}

func principalError(operation string, err error) error {
	switch {
	case errors.Is(err, identityaccess.ErrUnauthenticated):
		return authenticationError(operation)
	case errors.Is(err, identityaccess.ErrPermissionDenied), errors.Is(err, identityaccess.ErrInvalidArgument), errors.Is(err, identityaccess.ErrInvalidResourceTarget):
		return applicationerror.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, operation, "application action is forbidden", false, connect.CodePermissionDenied)
	default:
		return applicationerror.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE, operation, "application authorization unavailable", true, connect.CodeUnavailable)
	}
}
