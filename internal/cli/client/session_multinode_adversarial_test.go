package client

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/storage"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type multiNodeClock struct{ now time.Time }

func (c multiNodeClock) Now() time.Time { return c.now }

type multiNodeGrantIssuer struct{ key ed25519.PrivateKey }

func (i multiNodeGrantIssuer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), i.key.Public().(ed25519.PublicKey)...)
}

func (i multiNodeGrantIssuer) IssueAccessGrant(payload *identityprotocol.AccessGrantPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrant(payload, i.key)
}

type multiNodeAliceSigner struct {
	principal  string
	rootPublic ed25519.PublicKey
	root       ed25519.PrivateKey
	device     ed25519.PrivateKey
	credential *identityaccess.Artifact
}

func (s *multiNodeAliceSigner) Principal(context.Context) (string, error) {
	return s.principal, nil
}

func (s *multiNodeAliceSigner) Credential(context.Context) (*identityaccess.Artifact, error) {
	return s.credential, nil
}

func (s *multiNodeAliceSigner) SignAuthenticationChallenge(_ context.Context, challenge identityaccess.Challenge) ([]byte, error) {
	return identityaccess.SignAuthenticationChallenge(challenge, s.credential, s.device)
}

type multiNodeIdentityAdapter struct {
	service *identityaccess.Service
	binding identityaccess.AuthenticationBinding
	source  identityaccess.SourceKey
}

func (a multiNodeIdentityAdapter) BeginAuthentication(ctx context.Context, request *connect.Request[ardentsv1.BeginAuthenticationRequest]) (*connect.Response[ardentsv1.BeginAuthenticationResponse], error) {
	challenge, err := a.service.Begin(ctx, identityaccess.BeginRequest{
		Principal: request.Msg.GetPrincipalId(),
		Purpose:   request.Msg.GetPurpose(),
		Binding:   a.binding,
		Source:    a.source,
	})
	if err != nil {
		return nil, err
	}
	fields, err := identityaccess.ChallengeFields(challenge)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ardentsv1.BeginAuthenticationResponse{Challenge: fields}), nil
}

func (a multiNodeIdentityAdapter) CompleteAuthentication(ctx context.Context, request *connect.Request[ardentsv1.CompleteAuthenticationRequest]) (*connect.Response[ardentsv1.CompleteAuthenticationResponse], error) {
	if len(request.Msg.GetChallengeId()) != len(identityaccess.ChallengeID{}) ||
		len(request.Msg.GetRootPublicKey()) != ed25519.PublicKeySize {
		return nil, identityaccess.ErrInvalidArgument
	}
	var challengeID identityaccess.ChallengeID
	copy(challengeID[:], request.Msg.GetChallengeId())
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], request.Msg.GetRootPublicKey())
	result, err := a.service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID:   challengeID,
		Principal:     request.Msg.GetPrincipalId(),
		Binding:       a.binding,
		Source:        a.source,
		RootPublicKey: rootPublic,
		Credential:    request.Msg.GetCredential(),
		Signature:     request.Msg.GetSignature(),
	})
	if err != nil {
		return nil, err
	}
	if result.Session == nil || result.SessionSecret == nil {
		return nil, identityaccess.ErrUnauthenticated
	}
	return connect.NewResponse(&ardentsv1.CompleteAuthenticationResponse{
		SessionSecret: append([]byte(nil), result.SessionSecret[:]...),
		SessionId:     result.Session.ID,
		ExpiresAt:     timestamppb.New(result.Session.ExpiresAt),
	}), nil
}

func (a multiNodeIdentityAdapter) EndSession(ctx context.Context, request *connect.Request[ardentsv1.EndSessionRequest]) (*connect.Response[ardentsv1.EndSessionResponse], error) {
	value := request.Header().Get("Authorization")
	prefix := operatorSessionScheme + " "
	if !strings.HasPrefix(value, prefix) {
		return nil, identityaccess.ErrUnauthenticated
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil || len(raw) != identitycontract.SessionSecretBytes {
		return nil, identityaccess.ErrUnauthenticated
	}
	var secret identityaccess.SessionSecret
	copy(secret[:], raw)
	if err := a.service.EndSession(ctx, secret, a.binding); err != nil {
		return nil, err
	}
	return connect.NewResponse(&ardentsv1.EndSessionResponse{}), nil
}

type multiNodeAccessFixture struct {
	service *identityaccess.Service
	node    string
	binding identityaccess.AuthenticationBinding
	source  identityaccess.SourceKey
	now     time.Time
}

func TestSameAliceUsesIndependentAlphaBetaGrantsSessionsAndCaches(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	alice := newMultiNodeAlice(t, now)
	alpha := newMultiNodeAccessFixture(t, now, 0x31, 0x41)
	beta := newMultiNodeAccessFixture(t, now, 0x32, 0x42)

	alphaGrant := enrollAliceAndIssueAction(t, ctx, alpha, alice, "node.status", "alpha-status")
	betaGrant := enrollAliceAndIssueAction(t, ctx, beta, alice, "node.start", "beta-start")
	require.NotEqual(t, alpha.node, beta.node)
	require.NotEqual(t, alphaGrant, betaGrant)

	alphaManager := NewSessionManager(
		multiNodeIdentityAdapter{service: alpha.service, binding: alpha.binding, source: alpha.source},
		alice,
		alpha.node,
		func() time.Time { return now },
	)
	betaManager := NewSessionManager(
		multiNodeIdentityAdapter{service: beta.service, binding: beta.binding, source: beta.source},
		alice,
		beta.node,
		func() time.Time { return now },
	)
	t.Cleanup(alphaManager.Logout)
	t.Cleanup(betaManager.Logout)

	alphaHeader, _, err := alphaManager.authorization(ctx)
	require.NoError(t, err)
	betaHeader, _, err := betaManager.authorization(ctx)
	require.NoError(t, err)
	require.NotEqual(t, alphaHeader, betaHeader)
	require.Equal(t, alice.principal, alphaManager.Status().SignerPrincipal)
	require.Equal(t, alice.principal, betaManager.Status().SignerPrincipal)
	require.Equal(t, alpha.node, alphaManager.Status().NodePrincipal)
	require.Equal(t, beta.node, betaManager.Status().NodePrincipal)

	alphaSecret := cachedSecret(t, alphaManager)
	betaSecret := cachedSecret(t, betaManager)
	alphaResource, err := identityaccess.NewResourceRef(alpha.node, identityaccess.ResourceOwner{}, "node", "")
	require.NoError(t, err)
	betaResource, err := identityaccess.NewResourceRef(beta.node, identityaccess.ResourceOwner{}, "node", "")
	require.NoError(t, err)

	_, err = alpha.service.Admit(ctx, identityaccess.Attempt{
		SessionSecret: alphaSecret, Binding: alpha.binding, Action: "node.status", Resource: alphaResource,
	})
	require.NoError(t, err)
	_, err = alpha.service.Admit(ctx, identityaccess.Attempt{
		SessionSecret: alphaSecret, Binding: alpha.binding, Action: "node.start", Resource: alphaResource,
	})
	require.ErrorIs(t, err, identityaccess.ErrPermissionDenied)
	_, err = beta.service.Admit(ctx, identityaccess.Attempt{
		SessionSecret: betaSecret, Binding: beta.binding, Action: "node.start", Resource: betaResource,
	})
	require.NoError(t, err)
	_, err = beta.service.Admit(ctx, identityaccess.Attempt{
		SessionSecret: betaSecret, Binding: beta.binding, Action: "node.status", Resource: betaResource,
	})
	require.ErrorIs(t, err, identityaccess.ErrPermissionDenied)

	_, err = alpha.service.Admit(ctx, identityaccess.Attempt{
		SessionSecret: betaSecret, Binding: alpha.binding, Action: "node.status", Resource: alphaResource,
	})
	require.ErrorIs(t, err, identityaccess.ErrUnauthenticated)
	_, err = beta.service.Admit(ctx, identityaccess.Attempt{
		SessionSecret: alphaSecret, Binding: beta.binding, Action: "node.start", Resource: betaResource,
	})
	require.ErrorIs(t, err, identityaccess.ErrUnauthenticated)

	crossNodeManager := NewSessionManager(
		multiNodeIdentityAdapter{service: alpha.service, binding: alpha.binding, source: alpha.source},
		alice,
		beta.node,
		func() time.Time { return now },
	)
	_, _, err = crossNodeManager.authorization(ctx)
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
	require.Equal(t, SessionKey{}, crossNodeManager.Status())
}

func newMultiNodeAlice(t *testing.T, now time.Time) *multiNodeAliceSigner {
	t.Helper()
	root := ed25519.NewKeyFromSeed(repeatedSeed(0x11))
	device := ed25519.NewKeyFromSeed(repeatedSeed(0x21))
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version:         identitycontract.Version,
		Subject:         principal.String(),
		RootPublicKey:   root.Public().(ed25519.PublicKey),
		DeviceId:        deviceID.String(),
		DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes:        []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore:       timestamppb.New(now.Add(-time.Minute)),
		NotAfter:        timestamppb.New(now.Add(time.Hour)),
	}, root)
	require.NoError(t, err)
	return &multiNodeAliceSigner{
		principal: principal.String(), rootPublic: root.Public().(ed25519.PublicKey),
		root: root, device: device, credential: credential,
	}
}

func newMultiNodeAccessFixture(t *testing.T, now time.Time, nodeSeed, sourceByte byte) multiNodeAccessFixture {
	t.Helper()
	nodeKey := ed25519.NewKeyFromSeed(repeatedSeed(nodeSeed))
	node, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	database, err := storage.OpenIdentityAccess(context.Background(), t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	service, err := identityaccess.NewService(identityaccess.Config{
		Database: database, Clock: multiNodeClock{now: now}, EnableBootstrapTickets: true,
		GrantIssuer: multiNodeGrantIssuer{key: nodeKey},
	})
	require.NoError(t, err)
	var peer [identitycontract.PeerBindingBytes]byte
	peer[0] = sourceByte
	var source identityaccess.SourceKey
	source[0] = sourceByte
	return multiNodeAccessFixture{
		service: service,
		node:    node.String(),
		binding: identityaccess.AuthenticationBinding{
			Audience: identityaccess.Audience{
				Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR,
				ProtocolMajor: identitycontract.ProtocolMajor,
			},
			TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1,
			PeerBinding:      peer,
		},
		source: source,
		now:    now,
	}
}

func enrollAliceAndIssueAction(t *testing.T, ctx context.Context, node multiNodeAccessFixture, alice *multiNodeAliceSigner, action identityaccess.Action, requestID string) string {
	t.Helper()
	ticket, err := node.service.IssueBootstrapTicket(ctx, node.node)
	require.NoError(t, err)
	enrollment, err := node.service.Begin(ctx, identityaccess.BeginRequest{
		Principal: alice.principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF,
		Binding: node.binding, Source: node.source,
	})
	require.NoError(t, err)
	signature, err := identityaccess.SignEnrollmentChallenge(enrollment, alice.root)
	require.NoError(t, err)
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], alice.rootPublic)
	proof, err := node.service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: enrollment.ID, Principal: alice.principal, Binding: node.binding, Source: node.source,
		RootPublicKey: rootPublic, Signature: signature,
	})
	require.NoError(t, err)
	require.NotNil(t, proof.EnrollmentProof)
	credential, err := alice.credential.MarshalBinary()
	require.NoError(t, err)
	_, err = node.service.EnrollFirstPrincipal(ctx, node.binding, identityaccess.FirstEnrollmentRequest{
		Ticket: ticket, Challenge: enrollment, Proof: *proof.EnrollmentProof,
		RootPublicKey: rootPublic, Credential: credential,
	})
	require.NoError(t, err)

	adminSecret := authenticateAlice(t, ctx, node, alice)
	proposal := identityaccess.GrantProposal{
		Subject: alice.principal, Actions: []identityaccess.Action{action},
		Scope: identityaccess.ResourceScope{
			Kind: identityaccess.ScopeNode, Exact: identityaccess.ResourceRef{Node: node.node},
		},
		NotBefore: node.now,
		NotAfter:  node.now.Add(time.Hour),
	}
	proposalID, err := identityaccess.GrantProposalResourceID(node.node, node.binding.Audience, proposal)
	require.NoError(t, err)
	resource, err := identityaccess.NewResourceRef(node.node, identityaccess.ResourceOwner{}, "grant-proposal", proposalID)
	require.NoError(t, err)
	grantID, err := node.service.IssueAccessGrant(ctx, identityaccess.IssueGrantRequest{
		Command: identityaccess.AdminCommand{
			RequestID: requestID,
			Attempt: identityaccess.Attempt{
				SessionSecret: adminSecret, Binding: node.binding,
				Action: "identity.grant.issue", Resource: resource,
			},
		},
		Proposal: proposal,
	})
	require.NoError(t, err)
	require.NotEmpty(t, grantID)
	return grantID
}

func authenticateAlice(t *testing.T, ctx context.Context, node multiNodeAccessFixture, alice *multiNodeAliceSigner) identityaccess.SessionSecret {
	t.Helper()
	challenge, err := node.service.Begin(ctx, identityaccess.BeginRequest{
		Principal: alice.principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
		Binding: node.binding, Source: node.source,
	})
	require.NoError(t, err)
	signature, err := alice.SignAuthenticationChallenge(ctx, challenge)
	require.NoError(t, err)
	credential, err := alice.credential.MarshalBinary()
	require.NoError(t, err)
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], alice.rootPublic)
	result, err := node.service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: challenge.ID, Principal: alice.principal, Binding: node.binding, Source: node.source,
		RootPublicKey: rootPublic, Credential: credential, Signature: signature,
	})
	require.NoError(t, err)
	require.NotNil(t, result.SessionSecret)
	return *result.SessionSecret
}

func cachedSecret(t *testing.T, manager *SessionManager) identityaccess.SessionSecret {
	t.Helper()
	key := manager.Status()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	entry, ok := manager.entries[key]
	require.True(t, ok)
	return entry.secret
}

func repeatedSeed(value byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = value
	}
	return seed
}
