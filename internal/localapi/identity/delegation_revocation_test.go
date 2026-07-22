package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http/httptest"
	"testing"
	"time"

	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"ardents/internal/storage"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type delegationRevocationClock struct{ now time.Time }

func (c delegationRevocationClock) Now() time.Time { return c.now }

type delegationRevocationFixture struct {
	client     ardentsv1connect.IdentityServiceClient
	server     *httptest.Server
	now        time.Time
	device     ed25519.PrivateKey
	delegation *identityaccess.Artifact
	credential *identityprotocol.KeyCredential
	delegator  string
	delegatee  string
	audience   *identityprotocol.Audience
}

func newDelegationRevocationFixture(t *testing.T) *delegationRevocationFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2035, 2, 3, 4, 5, 6, 0, time.UTC)
	database, err := storage.OpenIdentityAccess(ctx, t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	service, err := identityaccess.NewService(identityaccess.Config{Database: database, Clock: delegationRevocationClock{now: now}})
	require.NoError(t, err)

	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x81}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x82}, ed25519.SeedSize))
	application := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x83}, ed25519.SeedSize))
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x84}, ed25519.SeedSize))
	delegator, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	delegatee, err := identityprincipal.FromEd25519PublicKey(application.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	node, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	credentialArtifact, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: 1, Subject: delegator.String(), RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(now.Add(-time.Hour)), NotAfter: timestamppb.New(now.Add(time.Hour)),
	}, root)
	require.NoError(t, err)
	credentialRaw, err := credentialArtifact.MarshalBinary()
	require.NoError(t, err)
	credential := new(identityprotocol.KeyCredential)
	require.NoError(t, proto.Unmarshal(credentialRaw, credential))
	audience := &identityprotocol.Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1}
	delegation, err := identityaccess.SignDelegation(&identityprotocol.DelegationPayload{
		Version: 1, Delegator: delegator.String(), Delegatee: delegatee.String(), Audience: audience,
		Actions:   []string{"application.content.get"},
		Scope:     &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: delegator.String()}}},
		NotBefore: timestamppb.New(now), NotAfter: timestamppb.New(now.Add(time.Hour)), Credential: credential,
	}, device, now)
	require.NoError(t, err)

	_, handler, err := NewHandler(service, node.String(), [32]byte{}, identityaccess.SourceKey{})
	require.NoError(t, err)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &delegationRevocationFixture{
		client: ardentsv1connect.NewIdentityServiceClient(server.Client(), server.URL), server: server,
		now: now, device: device, delegation: delegation, credential: credential,
		delegator: delegator.String(), delegatee: delegatee.String(), audience: audience,
	}
}

func (f *delegationRevocationFixture) revocation(t *testing.T, revokedAt time.Time) ([]byte, string) {
	t.Helper()
	artifact, err := identityaccess.SignDelegationRevocation(&identityprotocol.DelegationRevocationPayload{
		Version: 1, TargetId: f.delegation.ID(), Issuer: f.delegator, Audience: f.audience,
		RevokedAt: timestamppb.New(revokedAt), Delegator: f.delegator, Delegatee: f.delegatee, Credential: f.credential,
	}, f.device, f.now)
	require.NoError(t, err)
	raw, err := artifact.MarshalBinary()
	require.NoError(t, err)
	return raw, artifact.ID()
}

func TestImportDelegationRevocationIsPublicAndIdempotent(t *testing.T) {
	f := newDelegationRevocationFixture(t)
	raw, id := f.revocation(t, f.now)
	for range 2 {
		response, err := f.client.ImportDelegationRevocation(context.Background(), connect.NewRequest(&protocol.ImportDelegationRevocationRequest{Revocation: raw}))
		require.NoError(t, err)
		require.Equal(t, id, response.Msg.RevocationId)
	}
}

func TestImportDelegationRevocationRejectsMalformedOversizedAndUnknown(t *testing.T) {
	f := newDelegationRevocationFixture(t)
	valid, _ := f.revocation(t, f.now)
	unknownArtifact := append(append([]byte(nil), valid...), 0x20, 0x01)
	for name, raw := range map[string][]byte{
		"empty": nil, "malformed": {0xff, 0x01},
		"oversized": make([]byte, maxDelegationRevocationImportBytes+1), "unknown artifact field": unknownArtifact,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.client.ImportDelegationRevocation(context.Background(), connect.NewRequest(&protocol.ImportDelegationRevocationRequest{Revocation: raw}))
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
			if len(raw) != 0 {
				require.NotContains(t, err.Error(), base64.StdEncoding.EncodeToString(raw))
			}
		})
	}

	request := connect.NewRequest(&protocol.ImportDelegationRevocationRequest{Revocation: valid})
	request.Msg.ProtoReflect().SetUnknown([]byte{0x10, 0x01})
	_, err := f.client.ImportDelegationRevocation(context.Background(), request)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestImportDelegationRevocationConflictIsRedacted(t *testing.T) {
	f := newDelegationRevocationFixture(t)
	first, firstID := f.revocation(t, f.now)
	second, secondID := f.revocation(t, f.now.Add(-time.Second))
	_, err := f.client.ImportDelegationRevocation(context.Background(), connect.NewRequest(&protocol.ImportDelegationRevocationRequest{Revocation: first}))
	require.NoError(t, err)
	_, err = f.client.ImportDelegationRevocation(context.Background(), connect.NewRequest(&protocol.ImportDelegationRevocationRequest{Revocation: second}))
	require.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	require.Equal(t, "already_exists: identity state conflict", err.Error())
	require.NotContains(t, err.Error(), firstID)
	require.NotContains(t, err.Error(), secondID)
	require.NotContains(t, err.Error(), base64.StdEncoding.EncodeToString(second))
}
