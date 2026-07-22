// Package admission owns Principal-aware admission for protected Application
// product calls. It is intentionally separate from the public wire handler so
// internal and SDK identity protobuf registries never share a package graph.
package admission

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	applicationbinding "ardents/internal/applicationapi/binding"
	applicationcall "ardents/internal/applicationapi/call"
	applicationcontent "ardents/internal/applicationapi/content"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"

	"connectrpc.com/connect"
)

const applicationSessionScheme = "ArdentsApplicationSession"

const applicationDelegationHeader = "Ardents-Delegation"

type Admitter interface {
	AdmitTarget(context.Context, identityaccess.TargetAttempt) (identityaccess.AuthorizedCall, error)
}

type Config struct {
	Access         Admitter
	Node           string
	FallbackPeer   [32]byte
	FallbackSource identityaccess.SourceKey
	Injector       applicationcall.Injector
}

type interceptor struct{ config Config }

func NewInterceptor(config Config) (connect.Interceptor, error) {
	if config.Access == nil || !config.Injector.Valid() ||
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
		rule, err := applicationcontent.RuleForProcedure(request.Spec().Procedure)
		if err != nil {
			return nil, applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, "application.content", "application action is forbidden", false, connect.CodePermissionDenied)
		}
		target, err := applicationcontent.CanonicalizeResource(request.Spec().Procedure, request.Any())
		if err != nil {
			return nil, targetError(rule.Action, err)
		}
		delegation, err := parseDelegation(request.Header())
		if err != nil {
			return nil, authenticationError(rule.Action)
		}
		defer clear(delegation)
		deleteHeaderFold(request.Header(), applicationDelegationHeader)
		values := request.Header().Values("Authorization")
		if len(values) != 1 || len(values[0]) > 128 {
			return nil, authenticationError(rule.Action)
		}
		if !strings.HasPrefix(values[0], applicationSessionScheme+" ") {
			return nil, authenticationError(rule.Action)
		}
		return i.admitPrincipal(ctx, request, next, rule, target, delegation)
	}
}

func (i *interceptor) admitPrincipal(ctx context.Context, request connect.AnyRequest, next connect.UnaryFunc, rule applicationcontent.ProcedureRule, target applicationcontent.ResourceTarget, delegation []byte) (connect.AnyResponse, error) {
	secret, err := parseSession(request.Header())
	if err != nil {
		return nil, authenticationError(rule.Action)
	}
	binding, _ := applicationbinding.Application(ctx, i.config.Node, i.config.FallbackPeer, i.config.FallbackSource)
	action, err := identityaccess.ParseAction(binding.Audience.Interface, rule.Action)
	if err != nil || target.Kind != rule.ResourceKind {
		return nil, applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, rule.Action, "application action is forbidden", false, connect.CodePermissionDenied)
	}
	admitted, err := i.config.Access.AdmitTarget(ctx, identityaccess.TargetAttempt{
		SessionSecret: secret,
		Binding:       binding,
		Action:        action,
		Target: identityaccess.ResourceTarget{
			Kind: identityaccess.ResourceKind(target.Kind), ID: target.ID,
		},
		Finalize:   finalizeTarget,
		Delegation: delegation,
	})
	clear(delegation)
	if err != nil {
		return nil, principalError(rule.Action, err)
	}
	ctx = identityaccess.ContextWithAuthorizedCall(ctx, admitted)
	ctx = i.config.Injector.WithAuthorizedCall(ctx, admitted)
	return next(ctx, request)
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
	return func(context.Context, connect.StreamingHandlerConn) error {
		return connect.NewError(connect.CodePermissionDenied, errors.New("application streaming action is not registered"))
	}
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

func finalizeTarget(target identityaccess.ResourceTarget, audience identityaccess.Audience, _, effective string) (identityaccess.ResourceRef, error) {
	if audience.Interface != identityprotocol.Interface_INTERFACE_APPLICATION || effective == "" {
		return identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
	}
	return identityaccess.NewResourceRef(audience.Node, effective, string(target.Kind), target.ID)
}

func targetError(operation string, err error) error {
	if errors.Is(err, applicationcontent.ErrPayloadTooLarge) {
		return applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_RESOURCE_EXHAUSTED, operation, "content payload exceeds the unary limit", false, connect.CodeResourceExhausted)
	}
	return applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_INVALID_ARGUMENT, operation, "invalid application content request", false, connect.CodeInvalidArgument)
}

func authenticationError(operation string) error {
	return applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, operation, "application authentication required", false, connect.CodeUnauthenticated)
}

func principalError(operation string, err error) error {
	switch {
	case errors.Is(err, identityaccess.ErrUnauthenticated):
		return authenticationError(operation)
	case errors.Is(err, identityaccess.ErrPermissionDenied), errors.Is(err, identityaccess.ErrInvalidArgument), errors.Is(err, identityaccess.ErrInvalidResourceTarget):
		return applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_FORBIDDEN, operation, "application action is forbidden", false, connect.CodePermissionDenied)
	default:
		return applicationcontent.ProtocolError(applicationv1.ErrorCode_ERROR_CODE_UNAVAILABLE, operation, "application authorization unavailable", true, connect.CodeUnavailable)
	}
}
