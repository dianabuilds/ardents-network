package adapter

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkidentity "ardents/sdk/go/identity"
	applicationidentityv1 "ardents/sdk/go/protocol/applicationidentityv1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestApplicationSessionInterceptorAttachesExactlyOneCanonicalDelegation(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, applicationSigner := testIdentity(t, now)
	delegation := testDelegation(t, now, node, applicationSigner.principal)
	manager := testSessionManager(successfulAuthentication(node, applicationSigner.principal, now, new(atomic.Int32)), applicationSigner, node, now)
	interceptor, err := NewSessionInterceptorWithDelegation(manager, delegation)
	require.NoError(t, err)

	next := interceptor.WrapUnary(func(_ context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		values := request.Header().Values(applicationDelegationHeader)
		require.Len(t, values, 1)
		require.NotContains(t, values[0], "=")
		require.LessOrEqual(t, len(values[0]), base64.RawURLEncoding.EncodedLen(identitycontract.MaxArtifactBytes))
		raw, decodeErr := base64.RawURLEncoding.DecodeString(values[0])
		require.NoError(t, decodeErr)
		expected, marshalErr := delegation.MarshalBinary()
		require.NoError(t, marshalErr)
		require.Equal(t, expected, raw)
		return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{}), nil
	})

	_, err = next(context.Background(), connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{}))
	require.NoError(t, err)
}

func TestApplicationSessionInterceptorRejectsCallerSuppliedDelegationWithoutCallingNext(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, applicationSigner := testIdentity(t, now)
	manager := testSessionManager(successfulAuthentication(node, applicationSigner.principal, now, new(atomic.Int32)), applicationSigner, node, now)
	interceptor, err := NewSessionInterceptorWithDelegation(manager, testDelegation(t, now, node, applicationSigner.principal))
	require.NoError(t, err)

	called := false
	next := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return nil, nil
	})
	request := connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{})
	request.Header().Add(applicationDelegationHeader, "first-secret-proof")
	request.Header()["ardents-delegation"] = []string{"second-secret-proof"}
	_, err = next(context.Background(), request)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.False(t, called)
	require.NotContains(t, err.Error(), "secret-proof")
}

func TestApplicationSessionInterceptorRejectsDelegationForAnotherApplication(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, applicationSigner := testIdentity(t, now)
	otherApplication := digestID("p1_", "ardents:principal:v1\x00", bytes.Repeat([]byte{91}, ed25519.PublicKeySize))
	delegation := testDelegation(t, now, node, otherApplication)
	manager := testSessionManager(successfulAuthentication(node, applicationSigner.principal, now, new(atomic.Int32)), applicationSigner, node, now)
	interceptor, err := NewSessionInterceptorWithDelegation(manager, delegation)
	require.NoError(t, err)

	called := false
	next := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return nil, nil
	})
	_, err = next(context.Background(), connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.False(t, called)
	require.NotContains(t, err.Error(), delegation.ID())
	require.NotContains(t, err.Error(), otherApplication)
}

func TestDelegationAttachmentRejectsWrongKindNodeAndExpiredArtifact(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, applicationSigner := testIdentity(t, now)
	manager := testSessionManager(successfulAuthentication(node, applicationSigner.principal, now, new(atomic.Int32)), applicationSigner, node, now)

	_, err := NewSessionInterceptorWithDelegation(manager, applicationSigner.credential)
	require.ErrorContains(t, err, "Application Delegation is invalid")
	require.NotContains(t, err.Error(), applicationSigner.credential.ID())

	otherNode := digestID("p1_", "ardents:principal:v1\x00", bytes.Repeat([]byte{92}, ed25519.PublicKeySize))
	_, err = NewSessionInterceptorWithDelegation(manager, testDelegation(t, now, otherNode, applicationSigner.principal))
	require.ErrorContains(t, err, "Application Delegation is invalid")
	require.NotContains(t, err.Error(), otherNode)

	expiredAt := now.Add(-48 * time.Hour)
	_, err = NewSessionInterceptorWithDelegation(manager, testDelegation(t, expiredAt, node, applicationSigner.principal))
	require.ErrorContains(t, err, "Application Delegation is invalid")
}

func testDelegation(t *testing.T, now time.Time, node, application string) *sdkidentity.Artifact {
	t.Helper()
	delegator, credential, device := testDelegationIdentity(t, now)
	owner, err := sdkidentity.PrincipalOwner(delegator)
	require.NoError(t, err)
	artifact, err := sdkidentity.SignDelegation(sdkidentity.DelegationSpec{
		Delegator: delegator, Delegatee: application,
		Audience:  sdkidentity.Audience{Node: node, Interface: sdkidentity.InterfaceApplication, ProtocolMajor: identitycontract.ProtocolMajor},
		Actions:   []string{"application.content.get"},
		Scope:     sdkidentity.ResourceScope{Kind: sdkidentity.ScopePrincipalOwned, Owner: owner},
		NotBefore: now, NotAfter: now.Add(15 * time.Minute), Credential: credential,
	}, device, now)
	require.NoError(t, err)
	return artifact
}

func testDiscoveryDelegation(t *testing.T, now time.Time, node, application, serviceType string) *sdkidentity.Artifact {
	t.Helper()
	delegator, credential, device := testDelegationIdentity(t, now)
	artifact, err := sdkidentity.SignDelegation(sdkidentity.DelegationSpec{
		Delegator: delegator, Delegatee: application,
		Audience: sdkidentity.Audience{Node: node, Interface: sdkidentity.InterfaceApplication, ProtocolMajor: identitycontract.ProtocolMajor},
		Actions:  []string{"application.discovery.resolve"},
		Scope: sdkidentity.ResourceScope{Kind: sdkidentity.ScopeExact, Resource: sdkidentity.ResourceRef{
			Node: node, Kind: "service-type", CanonicalID: serviceType,
		}},
		NotBefore: now, NotAfter: now.Add(15 * time.Minute), Credential: credential,
	}, device, now)
	require.NoError(t, err)
	return artifact
}

func testDelegationIdentity(t *testing.T, now time.Time) (string, *sdkidentity.Artifact, ed25519.PrivateKey) {
	t.Helper()
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{81}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{82}, ed25519.SeedSize))
	delegator := digestID("p1_", "ardents:principal:v1\x00", root.Public().(ed25519.PublicKey))
	credential, err := sdkidentity.SignKeyCredential(sdkidentity.KeyCredentialSpec{
		Subject: delegator, RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceID:        digestID("d1_", "ardents:device:v1\x00", device.Public().(ed25519.PublicKey)),
		DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []sdkidentity.CredentialPurpose{sdkidentity.PurposeAuthenticate},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}, root)
	require.NoError(t, err)
	return delegator, credential, device
}

func TestApplicationDelegationErrorNeverContainsEncodedArtifact(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, applicationSigner := testIdentity(t, now)
	delegation := testDelegation(t, now, node, applicationSigner.principal)
	manager := testSessionManager(successfulAuthentication(node, applicationSigner.principal, now, new(atomic.Int32)), applicationSigner, node, now)
	interceptor, err := NewSessionInterceptorWithDelegation(manager, delegation)
	require.NoError(t, err)
	raw, err := delegation.MarshalBinary()
	require.NoError(t, err)
	encoded := base64.RawURLEncoding.EncodeToString(raw)

	next := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	})
	_, err = next(context.Background(), connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{}))
	require.NotContains(t, err.Error(), encoded)
}
