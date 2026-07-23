package identity

import (
	"context"
	"crypto/ed25519"
	"errors"
	"net/http"
	"time"

	identityaccess "ardents/internal/identity/access"
	identityprotocol "ardents/internal/identity/protocol"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	service  *identityaccess.Service
	node     string
	fallback transportPeer
}

type transportPeer struct {
	peer   [32]byte
	source identityaccess.SourceKey
}

func NewHandler(service *identityaccess.Service, node string, peerBinding [32]byte, source identityaccess.SourceKey) (string, http.Handler, error) {
	if service == nil {
		return "", nil, errors.New("identity access service is required")
	}
	h := &Handler{service: service, node: node, fallback: transportPeer{peer: peerBinding, source: source}}
	path, handler := ardentsv1connect.NewIdentityServiceHandler(h, connect.WithInterceptors(&interceptor{binding: h.binding}))
	return path, handler, nil
}

func (h *Handler) binding(ctx context.Context) (identityaccess.AuthenticationBinding, identityaccess.SourceKey) {
	return OperatorBinding(ctx, h.node, h.fallback.peer, h.fallback.source)
}

func OperatorBinding(ctx context.Context, node string, fallbackPeer [32]byte, fallbackSource identityaccess.SourceKey) (identityaccess.AuthenticationBinding, identityaccess.SourceKey) {
	peerBinding, source, ok := identityaccess.TransportPeerFromContext(ctx)
	peer := transportPeer{peer: peerBinding, source: source}
	if !ok {
		peer = transportPeer{peer: fallbackPeer, source: fallbackSource}
	}
	return identityaccess.AuthenticationBinding{Audience: identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer.peer}, peer.source
}

func (h *Handler) BeginAuthentication(ctx context.Context, request *connect.Request[protocol.BeginAuthenticationRequest]) (*connect.Response[protocol.BeginAuthenticationResponse], error) {
	binding, source := h.binding(ctx)
	challenge, err := h.service.Begin(ctx, identityaccess.BeginRequest{Principal: request.Msg.PrincipalId, Purpose: request.Msg.Purpose, Binding: binding, Source: source})
	if err != nil {
		return nil, accessError(err)
	}
	wire, err := identityaccess.ChallengeFields(challenge)
	if err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.BeginAuthenticationResponse{Challenge: wire}), nil
}

func (h *Handler) CompleteAuthentication(ctx context.Context, request *connect.Request[protocol.CompleteAuthenticationRequest]) (*connect.Response[protocol.CompleteAuthenticationResponse], error) {
	if len(request.Msg.ChallengeId) != 16 || len(request.Msg.RootPublicKey) != ed25519.PublicKeySize {
		return nil, accessError(identityaccess.ErrInvalidArgument)
	}
	var challengeID identityaccess.ChallengeID
	copy(challengeID[:], request.Msg.ChallengeId)
	var root [ed25519.PublicKeySize]byte
	copy(root[:], request.Msg.RootPublicKey)
	binding, source := h.binding(ctx)
	result, err := h.service.Complete(ctx, identityaccess.CompleteRequest{ChallengeID: challengeID, Principal: request.Msg.PrincipalId, Binding: binding, Source: source, RootPublicKey: root, Credential: request.Msg.Credential, Signature: request.Msg.Signature})
	if err != nil {
		return nil, accessError(err)
	}
	response := &protocol.CompleteAuthenticationResponse{}
	if result.Session != nil && result.SessionSecret != nil {
		response.SessionId = result.Session.ID
		response.ExpiresAt = timestamppb.New(result.Session.ExpiresAt)
		response.SessionSecret = append([]byte(nil), result.SessionSecret[:]...)
	}
	if result.EnrollmentProof != nil {
		response.EnrollmentProof = append([]byte(nil), result.EnrollmentProof[:]...)
	}
	return connect.NewResponse(response), nil
}

func (h *Handler) EndSession(ctx context.Context, _ *connect.Request[protocol.EndSessionRequest]) (*connect.Response[protocol.EndSessionResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	if err := h.service.EndSession(ctx, attempt.SessionSecret, attempt.Binding); err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.EndSessionResponse{}), nil
}

func (h *Handler) EnrollFirstPrincipal(ctx context.Context, request *connect.Request[protocol.EnrollFirstPrincipalRequest]) (*connect.Response[protocol.EnrollFirstPrincipalResponse], error) {
	challenge, proof, root, ticket, err := parseEnrollment(request.Msg.Challenge, request.Msg.EnrollmentProof, request.Msg.RootPublicKey, request.Msg.BootstrapTicket)
	if err != nil {
		return nil, accessError(err)
	}
	binding, _ := h.binding(ctx)
	result, err := h.service.EnrollFirstPrincipal(ctx, binding, identityaccess.FirstEnrollmentRequest{Ticket: ticket, Challenge: challenge, Proof: proof, RootPublicKey: root, Credential: request.Msg.Credential})
	if err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.EnrollFirstPrincipalResponse{PrincipalId: result.Principal}), nil
}

func (h *Handler) EnrollPrincipal(ctx context.Context, request *connect.Request[protocol.EnrollPrincipalRequest]) (*connect.Response[protocol.EnrollPrincipalResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	challenge, proof, root, _, err := parseEnrollment(request.Msg.Challenge, request.Msg.EnrollmentProof, request.Msg.RootPublicKey, nil)
	if err != nil {
		return nil, accessError(err)
	}
	result, err := h.service.EnrollPrincipal(ctx, identityaccess.EnrollPrincipalRequest{Command: identityaccess.AdminCommand{RequestID: request.Msg.RequestId, Attempt: attempt}, Challenge: challenge, Proof: proof, RootPublicKey: root, Credential: request.Msg.Credential})
	if err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.EnrollPrincipalResponse{PrincipalId: result}), nil
}

func (h *Handler) RevokeDevice(ctx context.Context, request *connect.Request[protocol.RevokeDeviceRequest]) (*connect.Response[protocol.RevokeDeviceResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	id, err := h.service.RevokeDevice(ctx, identityaccess.RevokeDeviceRequest{Command: identityaccess.AdminCommand{RequestID: request.Msg.RequestId, Attempt: attempt}, Subject: request.Msg.PrincipalId, DeviceID: request.Msg.DeviceId})
	if err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.RevokeDeviceResponse{RevocationId: id}), nil
}

func (h *Handler) ListDeviceRevocations(ctx context.Context, request *connect.Request[protocol.ListDeviceRevocationsRequest]) (*connect.Response[protocol.ListDeviceRevocationsResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	items, err := h.service.ListDeviceRevocations(ctx, attempt, request.Msg.PrincipalId)
	if err != nil {
		return nil, accessError(err)
	}
	response := &protocol.ListDeviceRevocationsResponse{Revocations: make([]*protocol.DeviceRevocationMetadata, len(items))}
	for index, item := range items {
		response.Revocations[index] = &protocol.DeviceRevocationMetadata{Id: item.ID, PrincipalId: item.Subject, DeviceId: item.DeviceID, RevokedAt: timestamppb.New(item.RevokedAt)}
	}
	return connect.NewResponse(response), nil
}

func (h *Handler) IssueAccessGrant(ctx context.Context, request *connect.Request[protocol.IssueAccessGrantRequest]) (*connect.Response[protocol.IssueAccessGrantResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	binding, _ := h.binding(ctx)
	proposal, err := parseProposal(request.Msg.Proposal, binding.Audience.Node)
	if err != nil {
		return nil, accessError(err)
	}
	id, err := h.service.IssueAccessGrant(ctx, identityaccess.IssueGrantRequest{Command: identityaccess.AdminCommand{RequestID: request.Msg.RequestId, Attempt: attempt}, Proposal: proposal})
	if err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.IssueAccessGrantResponse{GrantId: id}), nil
}

func (h *Handler) RevokeAccessGrant(ctx context.Context, request *connect.Request[protocol.RevokeAccessGrantRequest]) (*connect.Response[protocol.RevokeAccessGrantResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	id, err := h.service.RevokeAccessGrant(ctx, identityaccess.RevokeGrantRequest{Command: identityaccess.AdminCommand{RequestID: request.Msg.RequestId, Attempt: attempt}, GrantID: request.Msg.GrantId})
	if err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.RevokeAccessGrantResponse{RevocationId: id}), nil
}

func (h *Handler) ListAccessGrants(ctx context.Context, request *connect.Request[protocol.ListAccessGrantsRequest]) (*connect.Response[protocol.ListAccessGrantsResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	items, err := h.service.ListAccessGrants(ctx, attempt, request.Msg.SubjectPrincipalId)
	if err != nil {
		return nil, accessError(err)
	}
	response := &protocol.ListAccessGrantsResponse{Grants: make([]*protocol.AccessGrantMetadata, len(items))}
	for index, item := range items {
		scope, scopeErr := identityaccess.ResourceScopeFields(item.Scope, item.Audience)
		if scopeErr != nil {
			return nil, accessError(scopeErr)
		}
		actions := make([]string, len(item.Actions))
		for i := range item.Actions {
			actions[i] = string(item.Actions[i])
		}
		response.Grants[index] = &protocol.AccessGrantMetadata{Id: item.ID, SubjectPrincipalId: item.Subject, Actions: actions, Scope: scope, NotBefore: timestamppb.New(item.NotBefore), NotAfter: timestamppb.New(item.NotAfter), Revoked: item.Revoked}
	}
	return connect.NewResponse(response), nil
}

func (h *Handler) IssueApplicationEnrollmentTicket(ctx context.Context, request *connect.Request[protocol.IssueApplicationEnrollmentTicketRequest]) (*connect.Response[protocol.IssueApplicationEnrollmentTicketResponse], error) {
	attempt, ok := attemptFromContext(ctx)
	if !ok {
		return nil, accessError(identityaccess.ErrUnauthenticated)
	}
	actions := make([]identityaccess.Action, len(request.Msg.InitialActions))
	for index := range request.Msg.InitialActions {
		actions[index] = identityaccess.Action(request.Msg.InitialActions[index])
	}
	result, err := h.service.IssueApplicationEnrollmentTicket(ctx, identityaccess.IssueApplicationEnrollmentTicketRequest{
		Attempt: attempt, Principal: request.Msg.ApplicationPrincipalId, Actions: actions,
	})
	if err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.IssueApplicationEnrollmentTicketResponse{
		ApplicationEnrollmentTicket: append([]byte(nil), result.Ticket[:]...), ExpiresAt: timestamppb.New(result.ExpiresAt),
	}), nil
}

const maxDelegationRevocationImportBytes = 16 << 10

func (h *Handler) ImportDelegationRevocation(ctx context.Context, request *connect.Request[protocol.ImportDelegationRevocationRequest]) (*connect.Response[protocol.ImportDelegationRevocationResponse], error) {
	raw := request.Msg.Revocation
	if len(raw) == 0 || len(raw) > maxDelegationRevocationImportBytes {
		return nil, accessError(identityaccess.ErrInvalidArgument)
	}
	// Parse without a wall-clock bound to obtain the verified canonical ID. The
	// service repeats verification using its own clock before persisting.
	revocation, err := identityaccess.ParseAndVerifyDelegationRevocation(raw, time.Time{})
	if err != nil {
		return nil, accessError(identityaccess.ErrInvalidArgument)
	}
	if err := h.service.ImportDelegationRevocation(ctx, raw); err != nil {
		return nil, accessError(err)
	}
	return connect.NewResponse(&protocol.ImportDelegationRevocationResponse{RevocationId: revocation.ID()}), nil
}

func parseEnrollment(fields *identityprotocol.ChallengeFields, proofRaw, rootRaw, ticketRaw []byte) (identityaccess.Challenge, identityaccess.EnrollmentProof, [ed25519.PublicKeySize]byte, identityaccess.BootstrapTicket, error) {
	var proof identityaccess.EnrollmentProof
	var root [ed25519.PublicKeySize]byte
	var ticket identityaccess.BootstrapTicket
	if len(proofRaw) != len(proof) || len(rootRaw) != len(root) || ticketRaw != nil && len(ticketRaw) != len(ticket) {
		return identityaccess.Challenge{}, proof, root, ticket, identityaccess.ErrInvalidArgument
	}
	challenge, err := identityaccess.ParseChallengeFields(fields)
	if err != nil {
		return identityaccess.Challenge{}, proof, root, ticket, err
	}
	copy(proof[:], proofRaw)
	copy(root[:], rootRaw)
	copy(ticket[:], ticketRaw)
	return challenge, proof, root, ticket, nil
}

func accessError(err error) error {
	switch {
	case errors.Is(err, identityaccess.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid identity request"))
	case errors.Is(err, identityaccess.ErrUnauthenticated):
		return connect.NewError(connect.CodeUnauthenticated, errors.New("authentication failed"))
	case errors.Is(err, identityaccess.ErrPermissionDenied):
		return connect.NewError(connect.CodePermissionDenied, errors.New("identity access denied"))
	case errors.Is(err, identityaccess.ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("identity state conflict"))
	case errors.Is(err, identityaccess.ErrResourceExhausted):
		return connect.NewError(connect.CodeResourceExhausted, errors.New("identity capacity exhausted"))
	case errors.Is(err, identityaccess.ErrFeatureDisabled):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("identity feature disabled"))
	default:
		return connect.NewError(connect.CodeUnavailable, errors.New("identity service unavailable"))
	}
}
