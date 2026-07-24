package identity

import (
	"context"
	"fmt"

	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type attemptContextKey struct{}

type interceptor struct {
	binding func(context.Context) (identityaccess.AuthenticationBinding, identityaccess.SourceKey)
	audit   deniedCallRecorder
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := request.Spec().Procedure
		rule, known := procedureCatalog[procedure]
		if hasUnknownFields(request.Any()) {
			if known && rule.class != accessPublicBounded {
				i.recordDenied(ctx, rule, identityaccess.DenialMalformedRequest)
			}
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid identity operation"))
		}
		if !known {
			i.recordDenied(ctx, procedureRule{}, identityaccess.DenialActionUnregistered)
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("identity procedure is not registered"))
		}
		if rule.class == accessPublicBounded {
			return next(ctx, request)
		}
		secret, err := parseOperatorSession(request.Header())
		if err != nil {
			i.recordDenied(ctx, rule, identityaccess.DenialSessionPresentation)
			return nil, connect.NewError(connect.CodeUnauthenticated, errInvalidSessionHeader)
		}
		binding, _ := i.binding(ctx)
		if rule.class == accessSessionLifecycle {
			attempt := identityaccess.Attempt{SessionSecret: secret, Binding: binding}
			return next(context.WithValue(ctx, attemptContextKey{}, attempt), request)
		}
		action, resource, err := deriveAttempt(binding, procedure, rule.action, rule.resourceKind, request.Any())
		if err != nil {
			i.recordDenied(ctx, rule, identityaccess.DenialResourceTarget)
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid identity operation"))
		}
		attempt := identityaccess.Attempt{SessionSecret: secret, Binding: binding, Action: action, Resource: resource}
		return next(context.WithValue(ctx, attemptContextKey{}, attempt), request)
	}
}

func (i *interceptor) recordDenied(ctx context.Context, rule procedureRule, reason identityaccess.DenialReason) {
	if i.audit == nil {
		return
	}
	binding, _ := i.binding(ctx)
	action := identityaccess.Action("")
	if parsed, err := identityaccess.ParseAction(binding.Audience.Interface, rule.action); err == nil {
		action = parsed
	}
	i.audit.RecordDeniedCall(binding.Audience, action, reason)
}

func hasUnknownFields(value any) bool {
	message, ok := value.(interface{ ProtoReflect() protoreflect.Message })
	if !ok {
		return true
	}
	return messageHasUnknown(message.ProtoReflect())
}

func messageHasUnknown(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() && field.MapValue().Message() != nil {
			value.Map().Range(func(_ protoreflect.MapKey, entry protoreflect.Value) bool {
				unknown = messageHasUnknown(entry.Message())
				return !unknown
			})
		} else if field.IsList() && field.Message() != nil {
			list := value.List()
			for index := 0; index < list.Len() && !unknown; index++ {
				unknown = messageHasUnknown(list.Get(index).Message())
			}
		} else if field.Message() != nil {
			unknown = messageHasUnknown(value.Message())
		}
		return !unknown
	})
	return unknown
}

func parseProposal(wire *protocol.AccessGrantProposal, node string) (identityaccess.GrantProposal, error) {
	if wire == nil || wire.NotBefore == nil || wire.NotAfter == nil || !wire.NotBefore.IsValid() || !wire.NotAfter.IsValid() {
		return identityaccess.GrantProposal{}, identityaccess.ErrInvalidArgument
	}
	scope, err := identityaccess.ParseResourceScope(wire.Scope, node)
	if err != nil {
		return identityaccess.GrantProposal{}, err
	}
	actions := make([]identityaccess.Action, len(wire.Actions))
	for index, value := range wire.Actions {
		actions[index] = identityaccess.Action(value)
	}
	return identityaccess.GrantProposal{Subject: wire.SubjectPrincipalId, Actions: actions, Scope: scope, NotBefore: wire.NotBefore.AsTime(), NotAfter: wire.NotAfter.AsTime()}, nil
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}
func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func attemptFromContext(ctx context.Context) (identityaccess.Attempt, bool) {
	attempt, ok := ctx.Value(attemptContextKey{}).(identityaccess.Attempt)
	return attempt, ok
}

func deriveAttempt(binding identityaccess.AuthenticationBinding, procedure, registeredAction, registeredResourceKind string, message any) (identityaccess.Action, identityaccess.ResourceRef, error) {
	var kind, id string
	switch procedure {
	case ardentsv1connect.IdentityServiceEnrollPrincipalProcedure:
		r, ok := message.(*protocol.EnrollPrincipalRequest)
		if !ok {
			return "", identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		challenge, _, _, _, err := parseEnrollment(
			r.Challenge,
			r.EnrollmentProof,
			r.RootPublicKey,
			nil,
		)
		if err != nil {
			return "", identityaccess.ResourceRef{}, err
		}
		kind, id = "principal", challenge.Principal
	case ardentsv1connect.IdentityServiceRevokeDeviceProcedure:
		r, ok := message.(*protocol.RevokeDeviceRequest)
		if !ok {
			return "", identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		resourceID, err := identityaccess.DeviceResourceID(r.PrincipalId, r.DeviceId)
		if err != nil {
			return "", identityaccess.ResourceRef{}, err
		}
		kind, id = "device", resourceID
	case ardentsv1connect.IdentityServiceListDeviceRevocationsProcedure:
		r, ok := message.(*protocol.ListDeviceRevocationsRequest)
		if !ok {
			return "", identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		kind, id = "device-revocation-collection", r.PrincipalId
	case ardentsv1connect.IdentityServiceIssueAccessGrantProcedure:
		r, ok := message.(*protocol.IssueAccessGrantRequest)
		if !ok || r.Proposal == nil {
			return "", identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		proposal, err := parseProposal(r.Proposal, binding.Audience.Node)
		if err != nil {
			return "", identityaccess.ResourceRef{}, err
		}
		resourceID, err := identityaccess.GrantProposalResourceID(binding.Audience.Node, binding.Audience, proposal)
		if err != nil {
			return "", identityaccess.ResourceRef{}, err
		}
		kind, id = "grant-proposal", resourceID
	case ardentsv1connect.IdentityServiceRevokeAccessGrantProcedure:
		r, ok := message.(*protocol.RevokeAccessGrantRequest)
		if !ok {
			return "", identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		kind, id = "access-grant", r.GrantId
	case ardentsv1connect.IdentityServiceListAccessGrantsProcedure:
		r, ok := message.(*protocol.ListAccessGrantsRequest)
		if !ok {
			return "", identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		kind, id = "grant-collection", r.SubjectPrincipalId
	case ardentsv1connect.IdentityServiceIssueApplicationEnrollmentTicketProcedure:
		r, ok := message.(*protocol.IssueApplicationEnrollmentTicketRequest)
		if !ok {
			return "", identityaccess.ResourceRef{}, identityaccess.ErrInvalidArgument
		}
		kind, id = "principal", r.ApplicationPrincipalId
	default:
		return "", identityaccess.ResourceRef{}, identityaccess.ErrPermissionDenied
	}
	if kind != registeredResourceKind {
		return "", identityaccess.ResourceRef{}, identityaccess.ErrPermissionDenied
	}
	parsed, err := identityaccess.ParseAction(identityprotocol.Interface_INTERFACE_OPERATOR, registeredAction)
	if err != nil {
		return "", identityaccess.ResourceRef{}, err
	}
	resource, err := identityaccess.NewResourceRef(binding.Audience.Node, identityaccess.ResourceOwner{}, kind, id)
	return parsed, resource, err
}
