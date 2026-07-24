package adapter

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkerrors "ardents/sdk/go/errors"
	sdkidentity "ardents/sdk/go/identity"
	applicationidentityv1 "ardents/sdk/go/protocol/applicationidentityv1"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	identityv1 "ardents/sdk/go/protocol/identityv1"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type authenticationStub struct {
	begin    func(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error)
	complete func(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error)
	end      func(context.Context, *connect.Request[applicationidentityv1.EndSessionRequest]) (*connect.Response[applicationidentityv1.EndSessionResponse], error)
}

func (s *authenticationStub) BeginAuthentication(ctx context.Context, request *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
	return s.begin(ctx, request)
}

func (s *authenticationStub) CompleteAuthentication(ctx context.Context, request *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
	return s.complete(ctx, request)
}
func (s *authenticationStub) EndSession(ctx context.Context, request *connect.Request[applicationidentityv1.EndSessionRequest]) (*connect.Response[applicationidentityv1.EndSessionResponse], error) {
	if s.end != nil {
		return s.end(ctx, request)
	}
	return connect.NewResponse(&applicationidentityv1.EndSessionResponse{}), nil
}

type testSessionSigner struct {
	principal  string
	credential *sdkidentity.Artifact
	device     ed25519.PrivateKey
	err        error
}

func (s *testSessionSigner) Principal(context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.principal, nil
}

func (s *testSessionSigner) Credential(context.Context) (*sdkidentity.Artifact, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.credential, nil
}

func (s *testSessionSigner) SignAuthenticationChallenge(_ context.Context, challenge sdkidentity.Challenge) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	return sdkidentity.SignAuthenticationChallenge(challenge, s.credential, s.device)
}

func TestApplicationSessionSingleFlightCachesOnlyProcessMemory(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	var begins atomic.Int32
	var ends atomic.Int32
	auth := successfulAuthentication(node, signer.principal, now, &begins)
	auth.end = func(_ context.Context, request *connect.Request[applicationidentityv1.EndSessionRequest]) (*connect.Response[applicationidentityv1.EndSessionResponse], error) {
		require.True(t, strings.HasPrefix(request.Header().Get("Authorization"), applicationSessionScheme+" "))
		ends.Add(1)
		return connect.NewResponse(&applicationidentityv1.EndSessionResponse{}), nil
	}
	manager := testSessionManager(auth, signer, node, now)

	start := make(chan struct{})
	var group sync.WaitGroup
	errs := make(chan error, 32)
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			errs <- manager.Authenticate(context.Background())
		}()
	}
	close(start)
	group.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), begins.Load())
	require.Equal(t, SessionStatus{Authenticated: true, NodePrincipal: node, SignerPrincipal: signer.principal}, manager.Status())

	manager.Logout()
	require.EqualValues(t, 1, ends.Load())
	require.Equal(t, SessionStatus{}, manager.Status())
	require.NoError(t, manager.Authenticate(context.Background()))
	require.Equal(t, int32(2), begins.Load())
}

func TestApplicationSessionValidatesEachResponseAtReceiptTime(t *testing.T) {
	issuedAt := time.Unix(1_900_000_001, 0).UTC()
	node, signer := testIdentity(t, issuedAt)
	current := issuedAt.Add(-time.Millisecond)
	auth := &authenticationStub{
		begin: func(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
			current = issuedAt
			return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{Challenge: validChallenge(node, signer.principal, issuedAt)}), nil
		},
		complete: func(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
			current = issuedAt.Add(time.Second)
			return connect.NewResponse(validComplete(issuedAt)), nil
		},
	}
	manager := &SessionManager{
		auth: auth, signer: signer, targetNode: node, now: func() time.Time { return current },
		entries: make(map[sessionKey]cachedSession), flights: make(map[sessionKey]*loginFlight),
	}
	require.NoError(t, manager.Authenticate(context.Background()))
}

func TestApplicationSessionLiveWaiterRetriesAfterLeaderCancellation(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var begins atomic.Int32
	success := successfulAuthentication(node, signer.principal, now, &begins)
	auth := &authenticationStub{
		begin: func(ctx context.Context, request *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
			if begins.Add(1) == 1 {
				close(firstStarted)
				<-releaseFirst
				return nil, ctx.Err()
			}
			return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{Challenge: validChallenge(node, signer.principal, now)}), nil
		},
		complete: success.complete,
	}
	manager := testSessionManager(auth, signer, node, now)
	leaderContext, cancelLeader := context.WithCancel(context.Background())
	leaderErr := make(chan error, 1)
	go func() { leaderErr <- manager.Authenticate(leaderContext) }()
	<-firstStarted
	waiterErr := make(chan error, 1)
	go func() { waiterErr <- manager.Authenticate(context.Background()) }()
	cancelLeader()
	close(releaseFirst)
	require.ErrorIs(t, <-leaderErr, context.Canceled)
	require.NoError(t, <-waiterErr)
	require.Equal(t, int32(2), begins.Load())
}

func TestApplicationSessionInterceptorRefreshesOnceOnlyOnUnauthenticated(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	var begins atomic.Int32
	manager := testSessionManager(successfulAuthentication(node, signer.principal, now, &begins), signer, node, now)
	require.NoError(t, manager.Authenticate(context.Background()))

	var calls int
	interceptor := NewSessionInterceptor(manager)
	next := interceptor.WrapUnary(func(_ context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		require.True(t, strings.HasPrefix(request.Header().Get("Authorization"), applicationSessionScheme+" "))
		if calls == 1 {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("redacted"))
		}
		return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{}), nil
	})
	_, err := next(context.Background(), connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{}))
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, int32(2), begins.Load())

	calls = 0
	next = interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		calls++
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("redacted"))
	})
	_, err = next(context.Background(), connect.NewRequest(&applicationidentityv1.BeginAuthenticationRequest{}))
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Equal(t, 1, calls)
	require.Equal(t, int32(2), begins.Load())
}

func TestApplicationSessionRejectsMismatchedAudienceBeforeSigning(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	otherNode := digestID("p1_", "ardents:principal:v1\x00", bytes.Repeat([]byte{9}, 32))
	wire := validChallenge(node, signer.principal, now)
	wire.Binding.Audience.Node = otherNode
	var signed atomic.Int32
	wrapped := &countingSigner{testSessionSigner: signer, signed: &signed}
	auth := &authenticationStub{
		begin: func(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
			return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{Challenge: wire}), nil
		},
		complete: func(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
			t.Fatal("CompleteAuthentication must not be called")
			return nil, nil
		},
	}
	err := testSessionManager(auth, wrapped, node, now).Authenticate(context.Background())
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.Internal, sdkErr.Code)
	require.Zero(t, signed.Load())
	require.NotContains(t, err.Error(), node)
	require.NotContains(t, err.Error(), signer.principal)
}

func TestApplicationSessionRejectsPaddedNodeAndSignerPrincipalBeforeAuthentication(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	paddedSigner := *signer
	paddedSigner.principal += "\t"
	for _, testCase := range []struct {
		name   string
		node   string
		signer SessionSigner
	}{
		{name: "expected node", node: " " + node, signer: signer},
		{name: "signer principal", node: node, signer: &paddedSigner},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var begins atomic.Int32
			manager := NewSessionManager(http.DefaultClient, "http://localhost", testCase.signer, testCase.node, func() time.Time { return now })
			manager.auth = successfulAuthentication(node, signer.principal, now, &begins)

			err := manager.Authenticate(context.Background())

			require.Error(t, err)
			require.Zero(t, begins.Load())
		})
	}
}

type countingSigner struct {
	*testSessionSigner
	signed *atomic.Int32
}

func (s *countingSigner) SignAuthenticationChallenge(ctx context.Context, challenge sdkidentity.Challenge) ([]byte, error) {
	s.signed.Add(1)
	return s.testSessionSigner.SignAuthenticationChallenge(ctx, challenge)
}

func TestApplicationChallengeRejectsUnknownMalformedAndCrossSurfaceFields(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	base := validChallenge(node, signer.principal, now)
	cases := map[string]func(*identityv1.ChallengeFields){
		"unknown purpose": func(w *identityv1.ChallengeFields) { w.Purpose = identityv1.ChallengePurpose(99) },
		"unknown profile": func(w *identityv1.ChallengeFields) { w.Binding.TransportProfile = identityv1.TransportProfile(99) },
		"operator surface": func(w *identityv1.ChallengeFields) {
			w.Binding.Audience.Interface = identityv1.Interface_INTERFACE_OPERATOR
		},
		"noncanonical time": func(w *identityv1.ChallengeFields) { w.IssuedAt.Nanos = 1 },
		"zero nonce":        func(w *identityv1.ChallengeFields) { clear(w.Nonce) },
		"unknown nested": func(w *identityv1.ChallengeFields) {
			w.Binding.Audience.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			wire := proto.Clone(base).(*identityv1.ChallengeFields)
			mutate(wire)
			_, err := applicationChallenge(wire, now)
			require.Error(t, err)
		})
	}
}

func TestCompleteAuthenticationRejectsMalformedSecretIDTimeAndEnrollmentProof(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	base := validComplete(now)
	cases := map[string]func(*applicationidentityv1.CompleteAuthenticationResponse){
		"short secret": func(r *applicationidentityv1.CompleteAuthenticationResponse) { r.SessionSecret = r.SessionSecret[:31] },
		"zero session id": func(r *applicationidentityv1.CompleteAuthenticationResponse) {
			r.SessionId = "s1_" + strings.Repeat("a", 52)
		},
		"expired":             func(r *applicationidentityv1.CompleteAuthenticationResponse) { r.ExpiresAt = timestamppb.New(now) },
		"noncanonical expiry": func(r *applicationidentityv1.CompleteAuthenticationResponse) { r.ExpiresAt.Nanos = 1 },
		"enrollment proof":    func(r *applicationidentityv1.CompleteAuthenticationResponse) { r.EnrollmentProof = []byte{1} },
		"unknown field": func(r *applicationidentityv1.CompleteAuthenticationResponse) {
			r.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			response := proto.Clone(base).(*applicationidentityv1.CompleteAuthenticationResponse)
			mutate(response)
			_, err := acceptApplicationSessionCompletion(response, now)
			require.Error(t, err)
		})
	}
}

func TestApplicationSessionSignerFailureIsRedacted(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	node, signer := testIdentity(t, now)
	signer.err = errors.New("private key at C:/secret/application.key failed")
	manager := testSessionManager(successfulAuthentication(node, signer.principal, now, new(atomic.Int32)), signer, node, now)
	err := manager.Authenticate(context.Background())
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	require.Equal(t, sdkerrors.Unauthenticated, sdkErr.Code)
	require.NotContains(t, err.Error(), "C:/secret")
	require.NotContains(t, err.Error(), "private key")
}

func TestApplicationIdentityErrorsIgnoreUntrustedStructuredDetails(t *testing.T) {
	const secret = "ticket-proof-credential-must-not-leak"
	remote := connect.NewError(connect.CodeUnauthenticated, errors.New(secret))
	detail, err := connect.NewErrorDetail(&applicationv1.ApplicationError{
		Code: applicationv1.ErrorCode_ERROR_CODE_UNAUTHENTICATED, Operation: secret,
		Message: secret, Details: map[string]string{"credential": secret},
	})
	require.NoError(t, err)
	remote.AddDetail(detail)

	mapped := mapAuthenticationError(remote)
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, mapped, &sdkErr)
	require.Equal(t, sdkerrors.Unauthenticated, sdkErr.Code)
	require.Equal(t, "Application identity request failed", sdkErr.Message)
	require.Empty(t, sdkErr.Operation)
	require.Empty(t, sdkErr.Details)
	require.NotContains(t, mapped.Error(), secret)
}

func TestInvalidCompleteAuthenticationClearsReturnedSecrets(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	response := validComplete(now)
	response.EnrollmentProof = bytes.Repeat([]byte{9}, 32)
	_, err := acceptApplicationSessionCompletion(response, now)
	require.Error(t, err)
	require.Equal(t, make([]byte, identitycontract.SessionSecretBytes), response.SessionSecret)
	require.Equal(t, make([]byte, 32), response.EnrollmentProof)
}

func testSessionManager(auth authenticationService, signer SessionSigner, node string, now time.Time) *SessionManager {
	return &SessionManager{
		auth: auth, signer: signer, targetNode: node, now: func() time.Time { return now },
		entries: make(map[sessionKey]cachedSession), flights: make(map[sessionKey]*loginFlight),
	}
}

func successfulAuthentication(node, principal string, now time.Time, begins *atomic.Int32) *authenticationStub {
	return &authenticationStub{
		begin: func(context.Context, *connect.Request[applicationidentityv1.BeginAuthenticationRequest]) (*connect.Response[applicationidentityv1.BeginAuthenticationResponse], error) {
			begins.Add(1)
			return connect.NewResponse(&applicationidentityv1.BeginAuthenticationResponse{Challenge: validChallenge(node, principal, now)}), nil
		},
		complete: func(context.Context, *connect.Request[applicationidentityv1.CompleteAuthenticationRequest]) (*connect.Response[applicationidentityv1.CompleteAuthenticationResponse], error) {
			return connect.NewResponse(validComplete(now)), nil
		},
	}
}

func validChallenge(node, principal string, now time.Time) *identityv1.ChallengeFields {
	id := bytes.Repeat([]byte{1}, 16)
	nonce := bytes.Repeat([]byte{2}, 32)
	peer := bytes.Repeat([]byte{3}, identitycontract.PeerBindingBytes)
	return &identityv1.ChallengeFields{
		Version: identitycontract.Version, Id: id, Nonce: nonce, Principal: principal,
		Binding: &identityv1.AuthenticationBinding{
			Audience:         &identityv1.Audience{Node: node, Interface: identityv1.Interface_INTERFACE_APPLICATION, ProtocolMajor: identitycontract.ProtocolMajor},
			TransportProfile: identityv1.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer,
		},
		Purpose:  identityv1.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
		IssuedAt: timestamppb.New(now), ExpiresAt: timestamppb.New(now.Add(identitycontract.ChallengeLifetime)),
	}
}

func validComplete(now time.Time) *applicationidentityv1.CompleteAuthenticationResponse {
	return &applicationidentityv1.CompleteAuthenticationResponse{
		SessionSecret: bytes.Repeat([]byte{4}, identitycontract.SessionSecretBytes),
		SessionId:     digestID("s1_", "session-test\x00", bytes.Repeat([]byte{5}, 32)),
		ExpiresAt:     timestamppb.New(now.Add(time.Hour)),
	}
}

func testIdentity(t *testing.T, now time.Time) (string, *testSessionSigner) {
	t.Helper()
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{6}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	principal := digestID("p1_", "ardents:principal:v1\x00", root.Public().(ed25519.PublicKey))
	credential, err := sdkidentity.SignKeyCredential(sdkidentity.KeyCredentialSpec{
		Subject: principal, RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceID:        digestID("d1_", "ardents:device:v1\x00", device.Public().(ed25519.PublicKey)),
		DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []sdkidentity.CredentialPurpose{sdkidentity.PurposeAuthenticate},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}, root)
	require.NoError(t, err)
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, ed25519.SeedSize))
	node := digestID("p1_", "ardents:principal:v1\x00", nodeKey.Public().(ed25519.PublicKey))
	return node, &testSessionSigner{principal: principal, credential: credential, device: device}
}

func digestID(prefix, domain string, material []byte) string {
	payload := append(append([]byte(domain), byte(1)), material...)
	sum := sha256.Sum256(payload)
	return prefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
}
