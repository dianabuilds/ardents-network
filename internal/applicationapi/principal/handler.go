// Package principal adapts Application Principal authentication to the shared
// identity/access owner without owning durable identity state.
package principal

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	identitycontract "ardents/api/ardents/identity/v1"
	applicationbinding "ardents/internal/applicationapi/binding"
	applicationv1 "ardents/internal/applicationapi/protocol/applicationv1"
	applicationv1connect "ardents/internal/applicationapi/protocol/applicationv1/applicationv1connect"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const applicationIdentityMaxUnaryMessageBytes = identitycontract.MaxArtifactBytes + 4<<10
const applicationSessionScheme = "ArdentsApplicationSession"

type principalTransport struct {
	peer   [32]byte
	source identityaccess.SourceKey
}

// PrincipalHandler adapts the Application generated authentication protocol to
// the shared transport-independent access owner. It is safe to register only on
// the protected Application Unix listener.
type PrincipalHandler struct {
	service  *identityaccess.Service
	node     string
	fallback principalTransport
}

func NewHandler(service *identityaccess.Service, node string, peer [32]byte, source identityaccess.SourceKey) (string, http.Handler, error) {
	if service == nil {
		return "", nil, errors.New("Application Principal access service is required")
	}
	if _, err := identityprincipal.Parse(node); err != nil || peer == [32]byte{} || source == (identityaccess.SourceKey{}) {
		return "", nil, errors.New("Application Principal transport binding is required")
	}
	handler := &PrincipalHandler{service: service, node: node, fallback: principalTransport{peer: peer, source: source}}
	path, httpHandler := applicationv1connect.NewIdentityServiceHandler(
		handler,
		connect.WithReadMaxBytes(applicationIdentityMaxUnaryMessageBytes),
		connect.WithSendMaxBytes(applicationIdentityMaxUnaryMessageBytes),
	)
	return path, httpHandler, nil
}

func (h *PrincipalHandler) binding(ctx context.Context) (identityaccess.AuthenticationBinding, identityaccess.SourceKey) {
	return applicationbinding.Application(ctx, h.node, h.fallback.peer, h.fallback.source)
}

func (h *PrincipalHandler) BeginAuthentication(ctx context.Context, request *connect.Request[applicationv1.BeginAuthenticationRequest]) (*connect.Response[applicationv1.BeginAuthenticationResponse], error) {
	if request == nil || request.Msg == nil || len(request.Header().Values("Authorization")) != 0 || len(request.Msg.ProtoReflect().GetUnknown()) != 0 {
		return nil, principalAccessError(identityaccess.ErrInvalidArgument)
	}
	binding, source := h.binding(ctx)
	challenge, err := h.service.Begin(ctx, identityaccess.BeginRequest{Principal: request.Msg.PrincipalId, Purpose: request.Msg.Purpose, Binding: binding, Source: source})
	if err != nil {
		return nil, principalAccessError(err)
	}
	wire, err := identityaccess.ChallengeFields(challenge)
	if err != nil {
		return nil, principalAccessError(err)
	}
	return connect.NewResponse(&applicationv1.BeginAuthenticationResponse{Challenge: wire}), nil
}

func (h *PrincipalHandler) CompleteAuthentication(ctx context.Context, request *connect.Request[applicationv1.CompleteAuthenticationRequest]) (*connect.Response[applicationv1.CompleteAuthenticationResponse], error) {
	if request == nil || request.Msg == nil || len(request.Header().Values("Authorization")) != 0 || len(request.Msg.ProtoReflect().GetUnknown()) != 0 || len(request.Msg.ChallengeId) != 16 || len(request.Msg.RootPublicKey) != ed25519.PublicKeySize {
		return nil, principalAccessError(identityaccess.ErrInvalidArgument)
	}
	var challengeID identityaccess.ChallengeID
	var root [ed25519.PublicKeySize]byte
	copy(challengeID[:], request.Msg.ChallengeId)
	copy(root[:], request.Msg.RootPublicKey)
	binding, source := h.binding(ctx)
	result, err := h.service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: challengeID, Principal: request.Msg.PrincipalId, Binding: binding, Source: source,
		RootPublicKey: root, Credential: request.Msg.Credential, Signature: request.Msg.Signature,
	})
	if err != nil {
		return nil, principalAccessError(err)
	}
	response := &applicationv1.CompleteAuthenticationResponse{}
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

func (h *PrincipalHandler) EndSession(ctx context.Context, request *connect.Request[applicationv1.EndSessionRequest]) (*connect.Response[applicationv1.EndSessionResponse], error) {
	if request == nil || request.Msg == nil || len(request.Msg.ProtoReflect().GetUnknown()) != 0 {
		return nil, principalAccessError(identityaccess.ErrInvalidArgument)
	}
	secret, err := parseApplicationSession(request.Header())
	if err != nil {
		return nil, principalAccessError(identityaccess.ErrUnauthenticated)
	}
	binding, _ := h.binding(ctx)
	if err := h.service.EndSession(ctx, secret, binding); err != nil {
		return nil, principalAccessError(err)
	}
	return connect.NewResponse(&applicationv1.EndSessionResponse{}), nil
}

func parseApplicationSession(header http.Header) (identityaccess.SessionSecret, error) {
	var secret identityaccess.SessionSecret
	values := header.Values("Authorization")
	prefix := applicationSessionScheme + " "
	if len(values) != 1 || len(values[0]) > 128 || !strings.HasPrefix(values[0], prefix) || strings.Count(values[0], " ") != 1 {
		return secret, identityaccess.ErrUnauthenticated
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(values[0], prefix))
	if err != nil || len(raw) != len(secret) {
		return secret, identityaccess.ErrUnauthenticated
	}
	copy(secret[:], raw)
	return secret, nil
}

func (h *PrincipalHandler) EnrollApplication(ctx context.Context, request *connect.Request[applicationv1.EnrollApplicationRequest]) (*connect.Response[applicationv1.EnrollApplicationResponse], error) {
	if request == nil || request.Msg == nil || len(request.Header().Values("Authorization")) != 0 ||
		len(request.Msg.ProtoReflect().GetUnknown()) != 0 || len(request.Msg.ApplicationEnrollmentTicket) != identitycontract.ApplicationEnrollmentTicketBytes ||
		len(request.Msg.EnrollmentProof) != len(identityaccess.EnrollmentProof{}) || len(request.Msg.RootPublicKey) != ed25519.PublicKeySize {
		return nil, principalAccessError(identityaccess.ErrInvalidArgument)
	}
	challenge, err := identityaccess.ParseChallengeFields(request.Msg.Challenge)
	if err != nil {
		return nil, principalAccessError(err)
	}
	var ticket identityaccess.ApplicationEnrollmentTicket
	var proof identityaccess.EnrollmentProof
	var root [ed25519.PublicKeySize]byte
	copy(ticket[:], request.Msg.ApplicationEnrollmentTicket)
	copy(proof[:], request.Msg.EnrollmentProof)
	copy(root[:], request.Msg.RootPublicKey)
	binding, _ := h.binding(ctx)
	result, err := h.service.EnrollApplication(ctx, binding, identityaccess.EnrollApplicationRequest{
		Ticket: ticket, Challenge: challenge, Proof: proof, RootPublicKey: root, Credential: request.Msg.Credential,
	})
	if err != nil {
		return nil, principalAccessError(err)
	}
	return connect.NewResponse(&applicationv1.EnrollApplicationResponse{
		PrincipalId: result.Principal, CredentialId: result.CredentialID, GrantId: result.GrantID,
		GrantExpiresAt: timestamppb.New(result.GrantExpiresAt),
	}), nil
}

func principalAccessError(err error) error {
	switch {
	case errors.Is(err, identityaccess.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid Application identity request"))
	case errors.Is(err, identityaccess.ErrUnauthenticated):
		return connect.NewError(connect.CodeUnauthenticated, errors.New("Application authentication failed"))
	case errors.Is(err, identityaccess.ErrPermissionDenied):
		return connect.NewError(connect.CodePermissionDenied, errors.New("Application identity access denied"))
	case errors.Is(err, identityaccess.ErrResourceExhausted):
		return connect.NewError(connect.CodeResourceExhausted, errors.New("Application identity capacity exhausted"))
	case errors.Is(err, identityaccess.ErrConflict):
		return connect.NewError(connect.CodeAlreadyExists, errors.New("Application identity state conflict"))
	case errors.Is(err, identityaccess.ErrFeatureDisabled):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("Application enrollment is disabled"))
	default:
		return connect.NewError(connect.CodeUnavailable, errors.New("Application identity service unavailable"))
	}
}
