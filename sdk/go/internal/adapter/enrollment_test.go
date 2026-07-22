package adapter

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkerrors "ardents/sdk/go/errors"
	sdkidentity "ardents/sdk/go/identity"
	applicationidentityv1 "ardents/sdk/go/protocol/applicationidentityv1"
	identityv1 "ardents/sdk/go/protocol/identityv1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type enrollmentServiceStub struct {
	begin    func(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error)
	complete func(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error)
	enroll   func(context.Context, *connect.Request[applicationidentityv1.EnrollApplicationRequest]) (*connect.Response[applicationidentityv1.EnrollApplicationResponse], error)
}

func (s *enrollmentServiceStub) BeginAuthentication(ctx context.Context, request *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
	return s.begin(ctx, request)
}
func (s *enrollmentServiceStub) CompleteAuthentication(ctx context.Context, request *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
	return s.complete(ctx, request)
}
func (s *enrollmentServiceStub) EnrollApplication(ctx context.Context, request *connect.Request[applicationidentityv1.EnrollApplicationRequest]) (*connect.Response[applicationidentityv1.EnrollApplicationResponse], error) {
	return s.enroll(ctx, request)
}

type testEnrollmentSigner struct {
	principal  string
	credential *sdkidentity.Artifact
	root       ed25519.PrivateKey
	signed     atomic.Int32
	err        error
}

func (s *testEnrollmentSigner) Principal(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.principal, nil
}
func (s *testEnrollmentSigner) Credential(context.Context) (*sdkidentity.Artifact, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.credential, nil
}
func (s *testEnrollmentSigner) SignEnrollmentChallenge(_ context.Context, challenge sdkidentity.Challenge) ([]byte, error) {
	s.signed.Add(1)
	if s.err != nil {
		return nil, s.err
	}
	return sdkidentity.SignEnrollmentChallenge(challenge, s.root)
}

func TestApplicationEnrollmentValidatesAndClearsAllProtectedWireMaterial(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := enrollmentIdentity(t, now)
	challenge := validChallenge(node, signer.principal, now)
	challenge.Purpose = identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF
	var ticket [identitycontract.ApplicationEnrollmentTicketBytes]byte
	copy(ticket[:], bytes.Repeat([]byte{0x41}, len(ticket)))
	proof := bytes.Repeat([]byte{0x42}, 32)
	grantID := digestID(identitycontract.AccessGrantPrefix, "application-grant-test\x00", []byte("grant"))
	service := &enrollmentServiceStub{
		begin: func(_ context.Context, request *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
			require.Equal(t, signer.principal, request.Msg.PrincipalId)
			require.Equal(t, identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, request.Msg.Purpose)
			return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{Challenge: challenge}), nil
		},
		complete: func(_ context.Context, request *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
			require.Equal(t, signer.principal, request.Msg.PrincipalId)
			require.Empty(t, request.Msg.Credential)
			require.Len(t, request.Msg.Signature, ed25519.SignatureSize)
			return connect.NewResponse(&applicationidentityv1.CompleteAuthenticationResponse{EnrollmentProof: append([]byte(nil), proof...)}), nil
		},
		enroll: func(_ context.Context, request *connect.Request[applicationidentityv1.EnrollApplicationRequest]) (*connect.Response[applicationidentityv1.EnrollApplicationResponse], error) {
			require.Equal(t, ticket[:], request.Msg.ApplicationEnrollmentTicket)
			require.Equal(t, proof, request.Msg.EnrollmentProof)
			require.NotEmpty(t, request.Msg.Credential)
			return connect.NewResponse(&applicationidentityv1.EnrollApplicationResponse{PrincipalId: signer.principal, CredentialId: signer.credential.ID(), GrantId: grantID, GrantExpiresAt: timestamppb.New(now.Add(time.Hour))}), nil
		},
	}
	client := &EnrollmentClient{service: service, signer: signer, node: node, now: func() time.Time { return now }}
	result, err := client.Enroll(context.Background(), ticket)
	require.NoError(t, err)
	require.Equal(t, signer.principal, result.Principal)
	require.Equal(t, signer.credential.ID(), result.CredentialID)
	require.Equal(t, grantID, result.GrantID)
	require.Equal(t, int32(1), signer.signed.Load())
}

func TestApplicationEnrollmentRejectsCrossNodeBeforeRootSigning(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := enrollmentIdentity(t, now)
	challenge := validChallenge(digestID("p1_", "ardents:principal:v1\x00", bytes.Repeat([]byte{0x99}, 32)), signer.principal, now)
	challenge.Purpose = identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF
	service := &enrollmentServiceStub{
		begin: func(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
			return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{Challenge: challenge}), nil
		},
		complete: func(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
			t.Fatal("must not complete")
			return nil, nil
		},
		enroll: func(context.Context, *connect.Request[applicationidentityv1.EnrollApplicationRequest]) (*connect.Response[applicationidentityv1.EnrollApplicationResponse], error) {
			t.Fatal("must not enroll")
			return nil, nil
		},
	}
	var ticket [identitycontract.ApplicationEnrollmentTicketBytes]byte
	ticket[0] = 1
	_, err := (&EnrollmentClient{service: service, signer: signer, node: node, now: func() time.Time { return now }}).Enroll(context.Background(), ticket)
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.Internal, sdkErr.Code)
	require.Zero(t, signer.signed.Load())
	require.NotContains(t, err.Error(), node)
	require.NotContains(t, err.Error(), signer.principal)
}

func TestApplicationEnrollmentRejectsMalformedProofAndServerErrorsRemainRedacted(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := enrollmentIdentity(t, now)
	challenge := validChallenge(node, signer.principal, now)
	challenge.Purpose = identityv1.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF
	service := &enrollmentServiceStub{
		begin: func(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
			return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{Challenge: challenge}), nil
		},
		complete: func(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
			return connect.NewResponse(&applicationidentityv1.CompleteAuthenticationResponse{EnrollmentProof: []byte("ticket-and-proof-must-not-leak")}), nil
		},
		enroll: func(context.Context, *connect.Request[applicationidentityv1.EnrollApplicationRequest]) (*connect.Response[applicationidentityv1.EnrollApplicationResponse], error) {
			t.Fatal("must not enroll")
			return nil, nil
		},
	}
	var ticket [identitycontract.ApplicationEnrollmentTicketBytes]byte
	copy(ticket[:], bytes.Repeat([]byte{0x77}, len(ticket)))
	_, err := (&EnrollmentClient{service: service, signer: signer, node: node, now: func() time.Time { return now }}).Enroll(context.Background(), ticket)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "ticket-and-proof")
	require.NotContains(t, err.Error(), "777777")

	signer.err = errors.New("root-private-material-must-not-leak")
	_, err = (&EnrollmentClient{service: service, signer: signer, node: node, now: func() time.Time { return now }}).Enroll(context.Background(), ticket)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "root-private-material")
}

func enrollmentIdentity(t *testing.T, now time.Time) (string, *testEnrollmentSigner) {
	t.Helper()
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x61}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x62}, ed25519.SeedSize))
	principal := digestID("p1_", "ardents:principal:v1\x00", root.Public().(ed25519.PublicKey))
	credential, err := sdkidentity.SignKeyCredential(sdkidentity.KeyCredentialSpec{Subject: principal, RootPublicKey: root.Public().(ed25519.PublicKey), DeviceID: digestID("d1_", "ardents:device:v1\x00", device.Public().(ed25519.PublicKey)), DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []sdkidentity.CredentialPurpose{sdkidentity.PurposeAuthenticate}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}, root)
	require.NoError(t, err)
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x63}, ed25519.SeedSize))
	node := digestID("p1_", "ardents:principal:v1\x00", nodeKey.Public().(ed25519.PublicKey))
	return node, &testEnrollmentSigner{principal: principal, credential: credential, root: root}
}
