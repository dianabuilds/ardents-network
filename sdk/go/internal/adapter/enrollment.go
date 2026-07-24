package adapter

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	"ardents/internal/identity/sessionclient"
	sdkerrors "ardents/sdk/go/errors"
	sdkidentity "ardents/sdk/go/identity"
	applicationidentityv1 "ardents/sdk/go/protocol/applicationidentityv1"
	applicationidentityv1connect "ardents/sdk/go/protocol/applicationidentityv1/applicationv1connect"
	identityv1 "ardents/sdk/go/protocol/identityv1"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type EnrollmentSigner interface {
	Principal(context.Context) (string, error)
	Credential(context.Context) (*sdkidentity.Artifact, error)
	SignEnrollmentChallenge(context.Context, sdkidentity.Challenge) ([]byte, error)
}

type EnrollmentResult struct {
	Principal      string
	CredentialID   string
	GrantID        string
	GrantExpiresAt time.Time
}

type enrollmentService interface {
	authenticationService
	EnrollApplication(context.Context, *connect.Request[applicationidentityv1.EnrollApplicationRequest]) (*connect.Response[applicationidentityv1.EnrollApplicationResponse], error)
}

type EnrollmentClient struct {
	service enrollmentService
	signer  EnrollmentSigner
	node    string
	now     func() time.Time
}

func NewEnrollmentClient(httpClient connect.HTTPClient, endpoint string, signer EnrollmentSigner, node string, now func() time.Time) *EnrollmentClient {
	if now == nil {
		now = time.Now
	}
	return &EnrollmentClient{
		service: applicationidentityv1connect.NewIdentityServiceClient(httpClient, strings.TrimRight(endpoint, "/"), connect.WithReadMaxBytes(identitycontract.MaxArtifactBytes+4<<10), connect.WithSendMaxBytes(identitycontract.MaxArtifactBytes+4<<10)),
		signer:  signer, node: node, now: now,
	}
}

func (c *EnrollmentClient) Enroll(ctx context.Context, ticket [identitycontract.ApplicationEnrollmentTicketBytes]byte) (EnrollmentResult, error) {
	if c == nil || c.service == nil || c.signer == nil || !validDigestID(c.node, "p1_") || ticket == [identitycontract.ApplicationEnrollmentTicketBytes]byte{} {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	principal, err := c.signer.Principal(ctx)
	if err != nil {
		return EnrollmentResult{}, enrollmentSignerUnavailable(ctx)
	}
	if !validDigestID(principal, "p1_") {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	begin, err := c.service.BeginAuthentication(ctx, connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{PrincipalId: principal, Purpose: identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF}))
	if err != nil {
		return EnrollmentResult{}, mapAuthenticationError(err)
	}
	if begin == nil || begin.Msg == nil || messageHasUnknown(begin.Msg.ProtoReflect()) {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	challenge, err := applicationChallengeForPurpose(begin.Msg.Challenge, c.now().UTC(), sdkidentity.ChallengeEnrollmentProof)
	if err != nil || challenge.Principal != principal || challenge.Binding.Audience.Node != c.node {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	credential, err := c.signer.Credential(ctx)
	if err != nil {
		return EnrollmentResult{}, enrollmentSignerUnavailable(ctx)
	}
	if credential == nil {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	payload := credential.KeyCredential()
	if payload == nil || payload.Subject != principal || len(payload.RootPublicKey) != ed25519.PublicKeySize {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	credentialRaw, err := credential.MarshalBinary()
	if err != nil {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	defer clear(credentialRaw)
	signature, err := c.signer.SignEnrollmentChallenge(ctx, challenge)
	if err != nil {
		return EnrollmentResult{}, enrollmentSignerUnavailable(ctx)
	}
	defer clear(signature)
	if len(signature) != ed25519.SignatureSize {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	completeRequest := &applicationidentityv1.CompleteAuthenticationRequest{ChallengeId: append([]byte(nil), challenge.ID[:]...), PrincipalId: principal, RootPublicKey: append([]byte(nil), payload.RootPublicKey...), Signature: append([]byte(nil), signature...)}
	complete, err := c.service.CompleteAuthentication(ctx, connect.NewRequest(completeRequest))
	clear(completeRequest.RootPublicKey)
	clear(completeRequest.Signature)
	if err != nil {
		return EnrollmentResult{}, mapAuthenticationError(err)
	}
	proof, err := enrollmentProof(complete)
	if err != nil {
		return EnrollmentResult{}, err
	}
	defer clear(proof)
	fields := challengeFields(challenge)
	enrollRequest := &applicationidentityv1.EnrollApplicationRequest{
		ApplicationEnrollmentTicket: append([]byte(nil), ticket[:]...), Challenge: fields,
		EnrollmentProof: append([]byte(nil), proof...), RootPublicKey: append([]byte(nil), payload.RootPublicKey...), Credential: append([]byte(nil), credentialRaw...),
	}
	response, err := c.service.EnrollApplication(ctx, connect.NewRequest(enrollRequest))
	clear(enrollRequest.ApplicationEnrollmentTicket)
	clear(enrollRequest.EnrollmentProof)
	clear(enrollRequest.RootPublicKey)
	clear(enrollRequest.Credential)
	if err != nil {
		return EnrollmentResult{}, mapAuthenticationError(err)
	}
	return validateEnrollmentResponse(response, c.now().UTC(), principal, credential.ID())
}

func enrollmentProof(response *connect.Response[applicationidentityv1.CompleteAuthenticationResponse]) ([]byte, error) {
	if response == nil || response.Msg == nil {
		return nil, invalidEnrollmentResponse()
	}
	defer clear(response.Msg.EnrollmentProof)
	defer clear(response.Msg.SessionSecret)
	if messageHasUnknown(response.Msg.ProtoReflect()) || len(response.Msg.EnrollmentProof) != 32 || len(response.Msg.SessionSecret) != 0 || response.Msg.SessionId != "" || response.Msg.ExpiresAt != nil {
		return nil, invalidEnrollmentResponse()
	}
	return append([]byte(nil), response.Msg.EnrollmentProof...), nil
}

func validateEnrollmentResponse(response *connect.Response[applicationidentityv1.EnrollApplicationResponse], now time.Time, principal, credentialID string) (EnrollmentResult, error) {
	if response == nil || response.Msg == nil || messageHasUnknown(response.Msg.ProtoReflect()) || response.Msg.PrincipalId != principal || response.Msg.CredentialId != credentialID || !validDigestID(response.Msg.GrantId, identitycontract.AccessGrantPrefix) || response.Msg.GrantExpiresAt == nil || !response.Msg.GrantExpiresAt.IsValid() || response.Msg.GrantExpiresAt.Nanos != 0 {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	expires := response.Msg.GrantExpiresAt.AsTime()
	if !now.Before(expires) || expires.After(now.Add(identitycontract.MaxGrantLifetime)) {
		return EnrollmentResult{}, invalidEnrollmentResponse()
	}
	return EnrollmentResult{Principal: principal, CredentialID: credentialID, GrantID: response.Msg.GrantId, GrantExpiresAt: expires}, nil
}

func applicationChallengeForPurpose(wire *identityv1.ChallengeFields, now time.Time, expected sdkidentity.ChallengePurpose) (sdkidentity.Challenge, error) {
	if wire == nil || messageHasUnknown(wire.ProtoReflect()) || wire.Binding == nil || wire.Binding.Audience == nil || len(wire.Id) != 16 || len(wire.Nonce) != 32 || len(wire.Binding.PeerBinding) != identitycontract.PeerBindingBytes || wire.Binding.Audience.Interface != identityv1.Interface_INTERFACE_APPLICATION || wire.Binding.TransportProfile != identityv1.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1 || wire.IssuedAt == nil || wire.ExpiresAt == nil || !wire.IssuedAt.IsValid() || !wire.ExpiresAt.IsValid() || wire.IssuedAt.Nanos != 0 || wire.ExpiresAt.Nanos != 0 {
		return sdkidentity.Challenge{}, errors.New("invalid challenge")
	}
	purpose := sdkidentity.ChallengePurpose("")
	switch wire.Purpose {
	case identityv1.ChallengePurpose_CHALLENGE_PURPOSE_SESSION:
		purpose = sdkidentity.ChallengeSession
	case identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF:
		purpose = sdkidentity.ChallengeEnrollmentProof
	default:
		return sdkidentity.Challenge{}, errors.New("invalid challenge")
	}
	if purpose != expected {
		return sdkidentity.Challenge{}, errors.New("invalid challenge")
	}
	var challenge sdkidentity.Challenge
	challenge.Version = wire.Version
	copy(challenge.ID[:], wire.Id)
	copy(challenge.Nonce[:], wire.Nonce)
	challenge.Principal = wire.Principal
	challenge.Binding.Audience = sdkidentity.Audience{Node: wire.Binding.Audience.Node, Interface: sdkidentity.InterfaceApplication, ProtocolMajor: wire.Binding.Audience.ProtocolMajor}
	challenge.Binding.TransportProfile = sdkidentity.TransportUnixLocalV1
	copy(challenge.Binding.PeerBinding[:], wire.Binding.PeerBinding)
	challenge.Purpose, challenge.IssuedAt, challenge.ExpiresAt = purpose, wire.IssuedAt.AsTime(), wire.ExpiresAt.AsTime()
	if sdkidentity.ValidateChallenge(challenge, challenge.IssuedAt) != nil ||
		!sessionclient.ValidAuthenticationChallengeTimes(challenge.IssuedAt, challenge.ExpiresAt, now) {
		return sdkidentity.Challenge{}, errors.New("invalid challenge")
	}
	return challenge, nil
}

func challengeFields(challenge sdkidentity.Challenge) *identityv1.ChallengeFields {
	purpose := identityv1.ChallengePurpose_CHALLENGE_PURPOSE_SESSION
	if challenge.Purpose == sdkidentity.ChallengeEnrollmentProof {
		purpose = identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF
	}
	return &identityv1.ChallengeFields{Version: challenge.Version, Id: append([]byte(nil), challenge.ID[:]...), Nonce: append([]byte(nil), challenge.Nonce[:]...), Principal: challenge.Principal, Binding: &identityv1.AuthenticationBinding{Audience: &identityv1.Audience{Node: challenge.Binding.Audience.Node, Interface: identityv1.Interface_INTERFACE_APPLICATION, ProtocolMajor: challenge.Binding.Audience.ProtocolMajor}, TransportProfile: identityv1.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: append([]byte(nil), challenge.Binding.PeerBinding[:]...)}, Purpose: purpose, IssuedAt: timestamppb.New(challenge.IssuedAt), ExpiresAt: timestamppb.New(challenge.ExpiresAt)}
}

func invalidEnrollmentResponse() error {
	return &sdkerrors.Error{Code: sdkerrors.Internal, Message: "Application enrollment response is invalid"}
}

func enrollmentSignerUnavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return &sdkerrors.Error{Code: sdkerrors.Unauthenticated, Message: "Application enrollment signer is unavailable"}
}
