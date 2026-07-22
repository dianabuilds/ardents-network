package client

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	ardentsv1 "ardents/internal/localapi/protocol"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var sessionTestNow = time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

type sessionTestSigner struct {
	principal  string
	credential *identityaccess.Artifact
	signErr    error
}

func (s *sessionTestSigner) Principal(context.Context) (string, error) { return s.principal, nil }
func (s *sessionTestSigner) Credential(context.Context) (*identityaccess.Artifact, error) {
	return s.credential, nil
}
func (s *sessionTestSigner) SignAuthenticationChallenge(context.Context, identityaccess.Challenge) ([]byte, error) {
	if s.signErr != nil {
		return nil, s.signErr
	}
	return bytes.Repeat([]byte{0x5a}, ed25519.SignatureSize), nil
}

type sessionTestAuth struct {
	node          string
	principal     string
	now           time.Time
	beginCount    atomic.Int32
	completeCount atomic.Int32
	secretByte    byte
	gate          chan struct{}
	beginErrAfter int32
	beginErr      error
	beginUnknown  bool
}

func (a *sessionTestAuth) BeginAuthentication(ctx context.Context, _ *connect.Request[ardentsv1.BeginAuthenticationRequest]) (*connect.Response[ardentsv1.BeginAuthenticationResponse], error) {
	count := a.beginCount.Add(1)
	if a.beginErrAfter > 0 && count > a.beginErrAfter {
		return nil, a.beginErr
	}
	if a.gate != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-a.gate:
		}
	}
	var peer [identitycontract.PeerBindingBytes]byte
	peer[0] = 1
	challenge := identityaccess.Challenge{
		Version:   identitycontract.Version,
		Principal: a.principal,
		Binding: identityaccess.AuthenticationBinding{
			Audience:         identityaccess.Audience{Node: a.node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor},
			TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1,
			PeerBinding:      peer,
		},
		Purpose:   identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
		IssuedAt:  a.now,
		ExpiresAt: a.now.Add(identitycontract.ChallengeLifetime),
	}
	challenge.ID[0] = 2
	challenge.Nonce[0] = 3
	wire, err := identityaccess.ChallengeFields(challenge)
	if err != nil {
		return nil, err
	}
	response := &ardentsv1.BeginAuthenticationResponse{Challenge: wire}
	if a.beginUnknown {
		response.ProtoReflect().SetUnknown([]byte{0x10, 0x01})
	}
	return connect.NewResponse(response), nil
}

func (a *sessionTestAuth) CompleteAuthentication(context.Context, *connect.Request[ardentsv1.CompleteAuthenticationRequest]) (*connect.Response[ardentsv1.CompleteAuthenticationResponse], error) {
	n := a.completeCount.Add(1)
	secret := bytes.Repeat([]byte{a.secretByte + byte(n)}, identitycontract.SessionSecretBytes)
	return connect.NewResponse(&ardentsv1.CompleteAuthenticationResponse{
		SessionSecret: secret,
		SessionId:     "s1_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpiresAt:     timestamppb.New(a.now.Add(identitycontract.DefaultSessionLifetime)),
	}), nil
}

func newSessionTestSigner(t *testing.T) *sessionTestSigner {
	t.Helper()
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: identitycontract.Version, Subject: principal.String(), RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(sessionTestNow.Add(-time.Minute)), NotAfter: timestamppb.New(sessionTestNow.Add(time.Hour)),
	}, root)
	require.NoError(t, err)
	return &sessionTestSigner{principal: principal.String(), credential: credential}
}

func sessionTestPrincipal(t *testing.T, seed byte) string {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return principal.String()
}

func TestSessionManagerCachesByExactAudienceAndSingleflightsConcurrentLogin(t *testing.T) {
	signer := newSessionTestSigner(t)
	alpha := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: alpha, principal: signer.principal, now: sessionTestNow, secretByte: 0x40, gate: make(chan struct{})}
	manager := NewSessionManager(auth, signer, alpha, func() time.Time { return sessionTestNow })

	const callers = 12
	results := make(chan string, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			header, _, err := manager.authorization(context.Background())
			require.NoError(t, err)
			results <- header
		}()
	}
	require.Eventually(t, func() bool { return auth.beginCount.Load() == 1 }, time.Second, time.Millisecond)
	close(auth.gate)
	wg.Wait()
	close(results)
	var first string
	for header := range results {
		if first == "" {
			first = header
		}
		require.Equal(t, first, header)
	}
	require.EqualValues(t, 1, auth.beginCount.Load())
	require.EqualValues(t, 1, auth.completeCount.Load())
	require.Equal(t, SessionKey{NodePrincipal: alpha, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1, SignerPrincipal: signer.principal}, manager.Status())
}

func TestSessionInterceptorRetriesUnauthenticatedExactlyOnceButNeverPermissionDenied(t *testing.T) {
	signer := newSessionTestSigner(t)
	alpha := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: alpha, principal: signer.principal, now: sessionTestNow, secretByte: 0x20}
	manager := NewSessionManager(auth, signer, alpha, func() time.Time { return sessionTestNow })
	interceptor := newSessionInterceptor(manager)
	req := connect.NewRequest(&ardentsv1.GetNodeStatusRequest{})

	var calls int
	_, err := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		if calls == 1 {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("expired"))
		}
		return connect.NewResponse(&ardentsv1.NodeStatusResponse{}), nil
	})(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.EqualValues(t, 2, auth.beginCount.Load())
	require.EqualValues(t, 2, auth.completeCount.Load())

	calls = 0
	_, err = interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	})(context.Background(), connect.NewRequest(&ardentsv1.GetNodeStatusRequest{}))
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Equal(t, 1, calls)
	require.EqualValues(t, 2, auth.beginCount.Load())
}

func TestSessionManagersKeepAlphaAndBetaSecretsIsolated(t *testing.T) {
	signer := newSessionTestSigner(t)
	alphaNode := sessionTestPrincipal(t, 0x31)
	betaNode := sessionTestPrincipal(t, 0x32)
	alphaAuth := &sessionTestAuth{node: alphaNode, principal: signer.principal, now: sessionTestNow, secretByte: 0x10}
	betaAuth := &sessionTestAuth{node: betaNode, principal: signer.principal, now: sessionTestNow, secretByte: 0x70}
	alpha := NewSessionManager(alphaAuth, signer, alphaNode, func() time.Time { return sessionTestNow })
	beta := NewSessionManager(betaAuth, signer, betaNode, func() time.Time { return sessionTestNow })

	alphaHeader, _, err := alpha.authorization(context.Background())
	require.NoError(t, err)
	betaHeader, _, err := beta.authorization(context.Background())
	require.NoError(t, err)
	require.NotEqual(t, alphaHeader, betaHeader)
	require.Equal(t, alphaNode, alpha.Status().NodePrincipal)
	require.Equal(t, betaNode, beta.Status().NodePrincipal)

	alpha.Logout()
	require.Equal(t, SessionKey{}, alpha.Status())
	require.NotEqual(t, SessionKey{}, beta.Status())
}

func TestSessionManagerRejectsWrongAudienceAndSignerFailureWithoutPublishing(t *testing.T) {
	signer := newSessionTestSigner(t)
	alpha := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: sessionTestPrincipal(t, 0x32), principal: signer.principal, now: sessionTestNow, secretByte: 1}
	manager := NewSessionManager(auth, signer, alpha, func() time.Time { return sessionTestNow })
	_, _, err := manager.authorization(context.Background())
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
	require.Equal(t, SessionKey{}, manager.Status())
	require.Zero(t, auth.completeCount.Load())

	signer.signErr = errors.New("signer unavailable")
	auth.node = alpha
	_, _, err = manager.authorization(context.Background())
	require.ErrorIs(t, err, ErrSessionSignerUnavailable)
	require.NotContains(t, err.Error(), "ArdentsOperatorSession")
	require.Equal(t, SessionKey{}, manager.Status())
}

func TestSessionInterceptorStopsAfterSecondUnauthenticatedAndFailedRevocationRefresh(t *testing.T) {
	signer := newSessionTestSigner(t)
	alpha := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: alpha, principal: signer.principal, now: sessionTestNow, secretByte: 1}
	manager := NewSessionManager(auth, signer, alpha, func() time.Time { return sessionTestNow })
	interceptor := newSessionInterceptor(manager)
	calls := 0
	_, err := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("expired"))
	})(context.Background(), connect.NewRequest(&ardentsv1.GetNodeStatusRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, 2, calls)
	require.EqualValues(t, 2, auth.beginCount.Load())

	auth.beginErrAfter = auth.beginCount.Load()
	auth.beginErr = connect.NewError(connect.CodeUnauthenticated, errors.New("revoked"))
	manager.Logout()
	_, err = interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("protected operation reached after revoked device failed authentication")
		return nil, nil
	})(context.Background(), connect.NewRequest(&ardentsv1.GetNodeStatusRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, SessionKey{}, manager.Status())
}

func TestValidateCompleteResponseRejectsExpiryBoundaryAndUnknownFields(t *testing.T) {
	valid := &ardentsv1.CompleteAuthenticationResponse{
		SessionSecret: bytes.Repeat([]byte{1}, identitycontract.SessionSecretBytes),
		SessionId:     "s1_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpiresAt:     timestamppb.New(sessionTestNow.Add(time.Minute)),
	}
	_, err := validateCompleteResponse(valid, sessionTestNow)
	require.NoError(t, err)

	valid.ExpiresAt = timestamppb.New(sessionTestNow)
	_, err = validateCompleteResponse(valid, sessionTestNow)
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
	valid.ExpiresAt = timestamppb.New(sessionTestNow.Add(identitycontract.MaxSessionLifetime + time.Nanosecond))
	_, err = validateCompleteResponse(valid, sessionTestNow)
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
	valid.ExpiresAt = timestamppb.New(sessionTestNow.Add(time.Minute))
	valid.ExpiresAt.Nanos = 1
	_, err = validateCompleteResponse(valid, sessionTestNow)
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
	valid.ExpiresAt = timestamppb.New(sessionTestNow.Add(time.Minute))
	valid.SessionId = "s1_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaab"
	_, err = validateCompleteResponse(valid, sessionTestNow)
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
	valid.SessionId = "s1_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	valid.ProtoReflect().SetUnknown([]byte{0x28, 0x01})
	_, err = validateCompleteResponse(valid, sessionTestNow)
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
}

func TestSessionManagerRejectsUnknownBeginEnvelope(t *testing.T) {
	signer := newSessionTestSigner(t)
	alpha := sessionTestPrincipal(t, 0x31)
	auth := &sessionTestAuth{node: alpha, principal: signer.principal, now: sessionTestNow, secretByte: 1, beginUnknown: true}
	manager := NewSessionManager(auth, signer, alpha, func() time.Time { return sessionTestNow })
	_, _, err := manager.authorization(context.Background())
	require.ErrorIs(t, err, ErrInvalidAuthenticationResponse)
	require.Zero(t, auth.completeCount.Load())
	require.Equal(t, SessionKey{}, manager.Status())
}

func TestSessionStatusEvictsExpiredSecretAtHalfOpenBoundary(t *testing.T) {
	signer := newSessionTestSigner(t)
	alpha := sessionTestPrincipal(t, 0x31)
	now := sessionTestNow
	auth := &sessionTestAuth{node: alpha, principal: signer.principal, now: now, secretByte: 1}
	manager := NewSessionManager(auth, signer, alpha, func() time.Time { return now })
	_, _, err := manager.authorization(context.Background())
	require.NoError(t, err)
	now = sessionTestNow.Add(identitycontract.DefaultSessionLifetime)
	require.Equal(t, SessionKey{}, manager.Status())
	manager.mu.Lock()
	require.Empty(t, manager.entries)
	require.Equal(t, SessionKey{}, manager.active)
	manager.mu.Unlock()
}

func TestLogoutRacingLoginPreventsLateSessionPublication(t *testing.T) {
	signer := newSessionTestSigner(t)
	alpha := sessionTestPrincipal(t, 0x31)
	gate := make(chan struct{})
	auth := &sessionTestAuth{node: alpha, principal: signer.principal, now: sessionTestNow, secretByte: 1, gate: gate}
	manager := NewSessionManager(auth, signer, alpha, func() time.Time { return sessionTestNow })
	result := make(chan error, 1)
	go func() {
		_, _, err := manager.authorization(context.Background())
		result <- err
	}()
	require.Eventually(t, func() bool { return auth.beginCount.Load() == 1 }, time.Second, time.Millisecond)
	manager.Logout()
	close(gate)
	require.ErrorIs(t, <-result, ErrSessionInvalidated)
	require.Equal(t, SessionKey{}, manager.Status())
	manager.mu.Lock()
	require.Empty(t, manager.entries)
	manager.mu.Unlock()
}

func TestPrincipalClientRefusesHTTPBeforeNetwork(t *testing.T) {
	signer := newSessionTestSigner(t)
	client := New(Config{BaseURL: "http://127.0.0.1:1", Timeout: time.Second, ExpectedPrincipal: sessionTestPrincipal(t, 0x31), Signer: signer})
	_, err := client.Login(context.Background())
	require.ErrorIs(t, err, ErrPrincipalTransportForbidden)
}
