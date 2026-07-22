package principal

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	applicationv1 "ardents/internal/applicationapi/protocol/applicationv1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func principalID(t *testing.T, seed byte) string {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return principal.String()
}

func TestApplicationBeginDerivesApplicationUnixBinding(t *testing.T) {
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	service, err := identityaccess.NewService(identityaccess.Config{Database: database})
	require.NoError(t, err)
	peer, source := [32]byte{7}, identityaccess.SourceKey{9}
	handler := &PrincipalHandler{service: service, node: principalID(t, 1), fallback: principalTransport{peer: peer, source: source}}

	response, err := handler.BeginAuthentication(context.Background(), connect.NewRequest(&applicationv1.BeginAuthenticationRequest{PrincipalId: principalID(t, 2), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION}))
	require.NoError(t, err)
	require.Equal(t, handler.node, response.Msg.Challenge.Binding.Audience.Node)
	require.Equal(t, identityprotocol.Interface_INTERFACE_APPLICATION, response.Msg.Challenge.Binding.Audience.Interface)
	require.Equal(t, identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, response.Msg.Challenge.Binding.TransportProfile)
	require.Equal(t, peer[:], response.Msg.Challenge.Binding.PeerBinding)
	require.Equal(t, identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION, response.Msg.Challenge.Purpose)
}

func TestApplicationCompleteIssuesApplicationBoundSession(t *testing.T) {
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	service, err := identityaccess.NewService(identityaccess.Config{Database: database})
	require.NoError(t, err)
	peer, source := [32]byte{7}, identityaccess.SourceKey{9}
	handler := &PrincipalHandler{service: service, node: principalID(t, 1), fallback: principalTransport{peer: peer, source: source}}

	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: 1, Subject: principal.String(), RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(now.Add(-time.Hour)), NotAfter: timestamppb.New(now.Add(time.Hour)),
	}, root)
	require.NoError(t, err)

	begin, err := handler.BeginAuthentication(context.Background(), connect.NewRequest(&applicationv1.BeginAuthenticationRequest{
		PrincipalId: principal.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
	}))
	require.NoError(t, err)
	challenge, err := identityaccess.ParseChallengeFields(begin.Msg.Challenge)
	require.NoError(t, err)
	signature, err := identityaccess.SignAuthenticationChallenge(challenge, credential, device)
	require.NoError(t, err)
	credentialRaw, err := credential.MarshalBinary()
	require.NoError(t, err)
	complete, err := handler.CompleteAuthentication(context.Background(), connect.NewRequest(&applicationv1.CompleteAuthenticationRequest{
		ChallengeId: challenge.ID[:], PrincipalId: principal.String(), RootPublicKey: root.Public().(ed25519.PublicKey),
		Credential: credentialRaw, Signature: signature,
	}))
	require.NoError(t, err)
	require.Len(t, complete.Msg.SessionSecret, 32)
	require.Empty(t, complete.Msg.EnrollmentProof)
	require.NotEmpty(t, complete.Msg.SessionId)
	require.True(t, now.Before(complete.Msg.ExpiresAt.AsTime()))
}

func TestApplicationTransportContextOverridesFallbackAndCannotBecomeOperator(t *testing.T) {
	handler := &PrincipalHandler{node: principalID(t, 3), fallback: principalTransport{peer: [32]byte{1}, source: identityaccess.SourceKey{2}}}
	peer, source := [32]byte{3}, identityaccess.SourceKey{4}
	binding, actualSource := handler.binding(identityaccess.WithTransportPeer(context.Background(), peer, source))
	require.Equal(t, peer, binding.PeerBinding)
	require.Equal(t, source, actualSource)
	require.Equal(t, identityprotocol.Interface_INTERFACE_APPLICATION, binding.Audience.Interface)
	require.NotEqual(t, identityprotocol.Interface_INTERFACE_OPERATOR, binding.Audience.Interface)
}

func TestApplicationIdentityRejectsUnknownAndUnsupportedPurpose(t *testing.T) {
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	service, err := identityaccess.NewService(identityaccess.Config{Database: database})
	require.NoError(t, err)
	handler := &PrincipalHandler{service: service, node: principalID(t, 4), fallback: principalTransport{peer: [32]byte{1}, source: identityaccess.SourceKey{2}}}

	unknown := &applicationv1.BeginAuthenticationRequest{PrincipalId: principalID(t, 5), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION}
	unknown.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 99, protowire.VarintType), 1))
	_, err = handler.BeginAuthentication(context.Background(), connect.NewRequest(unknown))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = handler.BeginAuthentication(context.Background(), connect.NewRequest(&applicationv1.BeginAuthenticationRequest{PrincipalId: principalID(t, 5), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_UNSPECIFIED}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	withAuthorization := connect.NewRequest(&applicationv1.BeginAuthenticationRequest{PrincipalId: principalID(t, 5), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION})
	withAuthorization.Header().Add("Authorization", "ArdentsOperatorSession must-not-leak")
	_, err = handler.BeginAuthentication(context.Background(), withAuthorization)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.NotContains(t, err.Error(), "must-not-leak")
}
