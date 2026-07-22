package admission

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	applicationcall "ardents/internal/applicationapi/call"
	applicationcontent "ardents/internal/applicationapi/content"
	contentdomain "ardents/internal/content"
	contentpayload "ardents/internal/content/payload"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }

type testGrantIssuer struct{ key ed25519.PrivateKey }

func (i testGrantIssuer) PublicKey() ed25519.PublicKey {
	return i.key.Public().(ed25519.PublicKey)
}
func (i testGrantIssuer) IssueAccessGrant(payload *identityprotocol.AccessGrantPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrant(payload, i.key)
}

type realApplicationFixture struct {
	service         *identityaccess.Service
	clock           *testClock
	nodeID          string
	appPrincipal    string
	binding         identityaccess.AuthenticationBinding
	peer            [32]byte
	source          identityaccess.SourceKey
	secret          identityaccess.SessionSecret
	operatorSecret  identityaccess.SessionSecret
	operatorBinding identityaccess.AuthenticationBinding
}

func newRealApplicationFixture(t *testing.T, actions []identityaccess.Action) *realApplicationFixture {
	t.Helper()
	ctx := context.Background()
	clock := &testClock{now: time.Date(2035, 1, 2, 3, 4, 5, 0, time.UTC)}
	database, err := storage.OpenIdentityAccess(ctx, t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(ctx)) })

	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x31}, ed25519.SeedSize))
	node, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	nodeID := node.String()
	service, err := identityaccess.NewService(identityaccess.Config{
		Database: database, Clock: clock, EnableBootstrapTickets: true,
		GrantIssuer: testGrantIssuer{key: nodeKey}, EnableApplicationEnrollment: true,
	})
	require.NoError(t, err)

	var operatorPeer [32]byte
	copy(operatorPeer[:], bytes.Repeat([]byte{0x41}, 32))
	var operatorSource identityaccess.SourceKey
	copy(operatorSource[:], bytes.Repeat([]byte{0x42}, 32))
	operatorBinding := identityaccess.AuthenticationBinding{
		Audience:         identityaccess.Audience{Node: nodeID, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: operatorPeer,
	}
	operatorRoot := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize))
	operatorDevice := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x52}, ed25519.SeedSize))
	operatorPrincipal, operatorCredential := makeCredential(t, clock.Now(), operatorRoot, operatorDevice)
	bootstrap, err := service.IssueBootstrapTicket(ctx, nodeID)
	require.NoError(t, err)
	operatorProofChallenge, err := service.Begin(ctx, identityaccess.BeginRequest{Principal: operatorPrincipal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, Binding: operatorBinding, Source: operatorSource})
	require.NoError(t, err)
	operatorProofSignature, err := identityaccess.SignEnrollmentChallenge(operatorProofChallenge, operatorRoot)
	require.NoError(t, err)
	operatorProof := complete(t, service, operatorProofChallenge, operatorBinding, operatorSource, operatorRoot, nil, operatorProofSignature)
	var operatorRootPublic [ed25519.PublicKeySize]byte
	copy(operatorRootPublic[:], operatorRoot.Public().(ed25519.PublicKey))
	_, err = service.EnrollFirstPrincipal(ctx, operatorBinding, identityaccess.FirstEnrollmentRequest{
		Ticket: bootstrap, Challenge: operatorProofChallenge, Proof: *operatorProof.EnrollmentProof,
		RootPublicKey: operatorRootPublic, Credential: artifactBytes(t, operatorCredential),
	})
	require.NoError(t, err)
	operatorSecret := authenticate(t, service, operatorPrincipal, operatorRoot, operatorDevice, operatorCredential, operatorBinding, operatorSource)

	appRoot := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x61}, ed25519.SeedSize))
	appDevice := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x62}, ed25519.SeedSize))
	appPrincipal, appCredential := makeCredential(t, clock.Now(), appRoot, appDevice)
	resource, err := identityaccess.NewResourceRef(nodeID, "", "principal", appPrincipal)
	require.NoError(t, err)
	ticket, err := service.IssueApplicationEnrollmentTicket(ctx, identityaccess.IssueApplicationEnrollmentTicketRequest{
		Attempt:   identityaccess.Attempt{SessionSecret: operatorSecret, Binding: operatorBinding, Action: "identity.principal.enroll", Resource: resource},
		Principal: appPrincipal, Actions: actions,
	})
	require.NoError(t, err)

	var appPeer [32]byte
	copy(appPeer[:], bytes.Repeat([]byte{0x71}, 32))
	var appSource identityaccess.SourceKey
	copy(appSource[:], bytes.Repeat([]byte{0x72}, 32))
	appBinding := identityaccess.AuthenticationBinding{
		Audience:         identityaccess.Audience{Node: nodeID, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: identitycontract.ProtocolMajor},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: appPeer,
	}
	appProofChallenge, err := service.Begin(ctx, identityaccess.BeginRequest{Principal: appPrincipal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, Binding: appBinding, Source: appSource})
	require.NoError(t, err)
	appProofSignature, err := identityaccess.SignEnrollmentChallenge(appProofChallenge, appRoot)
	require.NoError(t, err)
	appProof := complete(t, service, appProofChallenge, appBinding, appSource, appRoot, nil, appProofSignature)
	var appRootPublic [ed25519.PublicKeySize]byte
	copy(appRootPublic[:], appRoot.Public().(ed25519.PublicKey))
	_, err = service.EnrollApplication(ctx, appBinding, identityaccess.EnrollApplicationRequest{
		Ticket: ticket.Ticket, Challenge: appProofChallenge, Proof: *appProof.EnrollmentProof,
		RootPublicKey: appRootPublic, Credential: artifactBytes(t, appCredential),
	})
	require.NoError(t, err)
	secret := authenticate(t, service, appPrincipal, appRoot, appDevice, appCredential, appBinding, appSource)
	return &realApplicationFixture{
		service: service, clock: clock, nodeID: nodeID, appPrincipal: appPrincipal,
		binding: appBinding, peer: appPeer, source: appSource, secret: secret,
		operatorSecret: operatorSecret, operatorBinding: operatorBinding,
	}
}

func enrollDelegator(t *testing.T, fixture *realApplicationFixture) (string, ed25519.PrivateKey, *identityprotocol.KeyCredential) {
	return enrollDelegatorWithMarkers(t, fixture, 0x81, 0x82, 0x83, 0x84)
}

func enrollDelegatorWithMarkers(t *testing.T, fixture *realApplicationFixture, rootMarker, deviceMarker, peerMarker, sourceMarker byte) (string, ed25519.PrivateKey, *identityprotocol.KeyCredential) {
	t.Helper()
	ctx := context.Background()
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{rootMarker}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{deviceMarker}, ed25519.SeedSize))
	principal, credentialArtifact := makeCredential(t, fixture.clock.Now(), root, device)
	resource, err := identityaccess.NewResourceRef(fixture.nodeID, "", "principal", principal)
	require.NoError(t, err)
	ticket, err := fixture.service.IssueApplicationEnrollmentTicket(ctx, identityaccess.IssueApplicationEnrollmentTicketRequest{
		Attempt: identityaccess.Attempt{
			SessionSecret: fixture.operatorSecret, Binding: fixture.operatorBinding,
			Action: "identity.principal.enroll", Resource: resource,
		},
		Principal: principal,
		Actions:   []identityaccess.Action{"application.content.get", "application.content.put"},
	})
	require.NoError(t, err)

	binding := fixture.binding
	copy(binding.PeerBinding[:], bytes.Repeat([]byte{peerMarker}, len(binding.PeerBinding)))
	var source identityaccess.SourceKey
	copy(source[:], bytes.Repeat([]byte{sourceMarker}, len(source)))
	challenge, err := fixture.service.Begin(ctx, identityaccess.BeginRequest{
		Principal: principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF,
		Binding: binding, Source: source,
	})
	require.NoError(t, err)
	signature, err := identityaccess.SignEnrollmentChallenge(challenge, root)
	require.NoError(t, err)
	proof := complete(t, fixture.service, challenge, binding, source, root, nil, signature)
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], root.Public().(ed25519.PublicKey))
	_, err = fixture.service.EnrollApplication(ctx, binding, identityaccess.EnrollApplicationRequest{
		Ticket: ticket.Ticket, Challenge: challenge, Proof: *proof.EnrollmentProof,
		RootPublicKey: rootPublic, Credential: artifactBytes(t, credentialArtifact),
	})
	require.NoError(t, err)

	credential := new(identityprotocol.KeyCredential)
	require.NoError(t, proto.Unmarshal(artifactBytes(t, credentialArtifact), credential))
	return principal, device, credential
}

func signedDelegation(t *testing.T, fixture *realApplicationFixture, delegator string, device ed25519.PrivateKey, credential *identityprotocol.KeyCredential, node, delegatee string) ([]byte, string) {
	t.Helper()
	artifact, err := identityaccess.SignDelegation(&identityprotocol.DelegationPayload{
		Version: identitycontract.Version, Delegator: delegator, Delegatee: delegatee,
		Audience: &identityprotocol.Audience{
			Node: node, Interface: identityprotocol.Interface_INTERFACE_APPLICATION,
			ProtocolMajor: identitycontract.ProtocolMajor,
		},
		Actions: []string{"application.content.get", "application.content.put"},
		Scope: &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{
			PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: delegator},
		}},
		NotBefore:  timestamppb.New(fixture.clock.Now().Add(-time.Minute)),
		NotAfter:   timestamppb.New(fixture.clock.Now().Add(time.Hour)),
		Credential: credential,
	}, device, fixture.clock.Now())
	require.NoError(t, err)
	raw := artifactBytes(t, artifact)
	return raw, base64.RawURLEncoding.EncodeToString(raw)
}

func makeCredential(t *testing.T, now time.Time, root, device ed25519.PrivateKey) (string, *identityaccess.Artifact) {
	t.Helper()
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: identitycontract.Version, Subject: principal.String(), RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(now.Add(-time.Minute)), NotAfter: timestamppb.New(now.Add(time.Hour)),
	}, root)
	require.NoError(t, err)
	return principal.String(), credential
}

func artifactBytes(t *testing.T, artifact *identityaccess.Artifact) []byte {
	t.Helper()
	raw, err := artifact.MarshalBinary()
	require.NoError(t, err)
	return raw
}

func complete(t *testing.T, service *identityaccess.Service, challenge identityaccess.Challenge, binding identityaccess.AuthenticationBinding, source identityaccess.SourceKey, root ed25519.PrivateKey, credential []byte, signature []byte) identityaccess.CompleteResult {
	t.Helper()
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], root.Public().(ed25519.PublicKey))
	result, err := service.Complete(context.Background(), identityaccess.CompleteRequest{
		ChallengeID: challenge.ID, Principal: challenge.Principal, Binding: binding, Source: source,
		RootPublicKey: rootPublic, Credential: credential, Signature: signature,
	})
	require.NoError(t, err)
	return result
}

func authenticate(t *testing.T, service *identityaccess.Service, principal string, root, device ed25519.PrivateKey, credential *identityaccess.Artifact, binding identityaccess.AuthenticationBinding, source identityaccess.SourceKey) identityaccess.SessionSecret {
	t.Helper()
	challenge, err := service.Begin(context.Background(), identityaccess.BeginRequest{Principal: principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION, Binding: binding, Source: source})
	require.NoError(t, err)
	signature, err := identityaccess.SignAuthenticationChallenge(challenge, credential, device)
	require.NoError(t, err)
	result := complete(t, service, challenge, binding, source, root, artifactBytes(t, credential), signature)
	require.NotNil(t, result.SessionSecret)
	return *result.SessionSecret
}

type countingAdmitter struct {
	service   *identityaccess.Service
	calls     atomic.Int32
	presented []byte
	lastErr   error
}

func (a *countingAdmitter) AdmitTarget(ctx context.Context, attempt identityaccess.TargetAttempt) (identityaccess.AuthorizedCall, error) {
	a.calls.Add(1)
	a.presented = attempt.Delegation
	call, err := a.service.AdmitTarget(ctx, attempt)
	a.lastErr = err
	return call, err
}

type recordingStore struct {
	calls    []applicationcall.Call
	blobs    map[string]contentdomain.Blob
	payloads map[string][]byte
}

type ownerBoundStore struct {
	service *contentdomain.Service
}

func newOwnerBoundStore(t *testing.T) *ownerBoundStore {
	t.Helper()
	service := contentdomain.NewInDir(t.TempDir())
	require.NoError(t, service.Load())
	return &ownerBoundStore{service: service}
}

func (s *ownerBoundStore) PublishBlob(call applicationcall.Call, command contentdomain.PublishBlobCommand) (contentdomain.Blob, error) {
	owner, err := identityprincipal.Parse(call.Effective())
	if err != nil {
		return contentdomain.Blob{}, contentdomain.ErrBlobNotFound
	}
	return s.service.StoreBlobForOwner(owner, command.Blob, command.Payload)
}

func (s *ownerBoundStore) GetBlob(call applicationcall.Call, id string) (contentdomain.Blob, bool) {
	owner, err := identityprincipal.Parse(call.Effective())
	if err != nil {
		return contentdomain.Blob{}, false
	}
	return s.service.GetBlobForOwner(owner, id)
}

func (s *ownerBoundStore) GetBlobPayload(call applicationcall.Call, id string) ([]byte, error) {
	owner, err := identityprincipal.Parse(call.Effective())
	if err != nil {
		return nil, contentdomain.ErrBlobNotFound
	}
	return s.service.GetBlobPayloadForOwner(owner, id)
}

func (*ownerBoundStore) FetchBlob(context.Context, applicationcall.Call, string) (contentdomain.Blob, error) {
	return contentdomain.Blob{}, contentdomain.ErrBlobNotFound
}

func newRecordingStore() *recordingStore {
	return &recordingStore{blobs: map[string]contentdomain.Blob{}, payloads: map[string][]byte{}}
}
func (s *recordingStore) PublishBlob(call applicationcall.Call, command contentdomain.PublishBlobCommand) (contentdomain.Blob, error) {
	s.calls = append(s.calls, call)
	hash, id, err := contentpayload.DeriveIdentity(command.Payload)
	if err != nil {
		return contentdomain.Blob{}, err
	}
	blob := command.Blob
	blob.ID, blob.CID, blob.Hash, blob.Size = id, id, hash, int64(len(command.Payload))
	s.blobs[id], s.payloads[id] = blob, append([]byte(nil), command.Payload...)
	return blob, nil
}
func (s *recordingStore) GetBlob(call applicationcall.Call, id string) (contentdomain.Blob, bool) {
	s.calls = append(s.calls, call)
	blob, ok := s.blobs[id]
	return blob, ok
}
func (s *recordingStore) GetBlobPayload(_ applicationcall.Call, id string) ([]byte, error) {
	return append([]byte(nil), s.payloads[id]...), nil
}
func (s *recordingStore) FetchBlob(context.Context, applicationcall.Call, string) (contentdomain.Blob, error) {
	return contentdomain.Blob{}, contentdomain.ErrBlobNotFound
}

func TestPrincipalContentAdmissionUsesRealAccessServiceAndPropagatesActorEffective(t *testing.T) {
	fixture := newRealApplicationFixture(t, []identityaccess.Action{"application.content.get", "application.content.put"})
	store := newRecordingStore()
	admitter := &countingAdmitter{service: fixture.service}
	client := principalContentClient(t, fixture, admitter, store)
	header := "ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(fixture.secret[:])
	put := connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("principal payload")})
	put.Header().Set("Authorization", header)
	putResponse, err := client.Put(context.Background(), put)
	require.NoError(t, err)
	get := connect.NewRequest(&applicationv1.GetContentRequest{Reference: putResponse.Msg.Reference})
	get.Header().Set("Authorization", header)
	getResponse, err := client.Get(context.Background(), get)
	require.NoError(t, err)
	require.Equal(t, []byte("principal payload"), getResponse.Msg.Payload)
	require.Equal(t, int32(2), admitter.calls.Load())
	require.NotEmpty(t, store.calls)
	for _, admitted := range store.calls {
		require.True(t, admitted.IsPrincipal())
		require.Equal(t, fixture.appPrincipal, admitted.Actor())
		require.Equal(t, admitted.Actor(), admitted.Effective())
		require.Equal(t, admitted.Effective(), admitted.ResourceOwner())
		require.Equal(t, fixture.nodeID, admitted.ResourceNode())
	}
}

func TestDelegatedContentAdmissionUsesRealAccessServiceAndClearsPresentation(t *testing.T) {
	fixture := newRealApplicationFixture(t, []identityaccess.Action{"application.content.get", "application.content.put"})
	alice, aliceDevice, aliceCredential := enrollDelegator(t, fixture)
	raw, encoded := signedDelegation(t, fixture, alice, aliceDevice, aliceCredential, fixture.nodeID, fixture.appPrincipal)
	store := newRecordingStore()
	admitter := &countingAdmitter{service: fixture.service}
	client := principalContentClient(t, fixture, admitter, store)

	request := connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("delegated payload")})
	request.Header().Set("Authorization", "ArdentsApplicationSession "+base64.RawURLEncoding.EncodeToString(fixture.secret[:]))
	request.Header().Set(applicationDelegationHeader, encoded)
	response, err := client.Put(context.Background(), request)
	require.NoError(t, err, "access error: %v", admitter.lastErr)
	require.NotNil(t, response.Msg.Reference)
	require.Equal(t, int32(1), admitter.calls.Load())
	require.Len(t, admitter.presented, len(raw))
	require.Equal(t, make([]byte, len(raw)), admitter.presented, "decoded Delegation must be cleared immediately after Admit")
	require.NotEmpty(t, store.calls)
	for _, admitted := range store.calls {
		require.Equal(t, fixture.appPrincipal, admitted.Actor())
		require.Equal(t, alice, admitted.Effective())
		require.Equal(t, alice, admitted.ResourceOwner())
	}
}

func TestDelegatedContentOwnershipUsesEffectiveAndDoesNotEnumerateSiblingBinding(t *testing.T) {
	fixture := newRealApplicationFixture(t, []identityaccess.Action{"application.content.get", "application.content.put"})
	alice, aliceDevice, aliceCredential := enrollDelegator(t, fixture)
	bob, bobDevice, bobCredential := enrollDelegatorWithMarkers(t, fixture, 0x91, 0x92, 0x93, 0x94)
	_, aliceDelegation := signedDelegation(t, fixture, alice, aliceDevice, aliceCredential, fixture.nodeID, fixture.appPrincipal)
	_, bobDelegation := signedDelegation(t, fixture, bob, bobDevice, bobCredential, fixture.nodeID, fixture.appPrincipal)
	store := newOwnerBoundStore(t)
	client := principalContentClient(t, fixture, fixture.service, store)
	authorization := "ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(fixture.secret[:])

	put := connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("same delegated bytes"), MediaType: "text/plain"})
	put.Header().Set("Authorization", authorization)
	put.Header().Set(applicationDelegationHeader, aliceDelegation)
	aliceResult, err := client.Put(context.Background(), put)
	require.NoError(t, err)
	reference := aliceResult.Msg.GetReference()

	bobBeforePut := connect.NewRequest(&applicationv1.GetContentRequest{Reference: reference})
	bobBeforePut.Header().Set("Authorization", authorization)
	bobBeforePut.Header().Set(applicationDelegationHeader, bobDelegation)
	_, err = client.Get(context.Background(), bobBeforePut)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	bobMissingBindingErr := err

	aliceUnknown := connect.NewRequest(&applicationv1.GetContentRequest{
		Reference: &applicationv1.ContentReference{Kind: "blob", Id: "unknown-content-reference"},
	})
	aliceUnknown.Header().Set("Authorization", authorization)
	aliceUnknown.Header().Set(applicationDelegationHeader, aliceDelegation)
	_, err = client.Get(context.Background(), aliceUnknown)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Equal(t, bobMissingBindingErr.Error(), err.Error(), "missing binding must not reveal payload existence")

	directAppGet := connect.NewRequest(&applicationv1.GetContentRequest{Reference: reference})
	directAppGet.Header().Set("Authorization", authorization)
	_, err = client.Get(context.Background(), directAppGet)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	bobPut := connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("same delegated bytes"), MediaType: "text/plain"})
	bobPut.Header().Set("Authorization", authorization)
	bobPut.Header().Set(applicationDelegationHeader, bobDelegation)
	bobResult, err := client.Put(context.Background(), bobPut)
	require.NoError(t, err)
	require.Equal(t, reference.GetId(), bobResult.Msg.GetReference().GetId())
	require.Len(t, store.service.ListBlobs(), 1)

	for ownerValue, delegation := range map[string]string{alice: aliceDelegation, bob: bobDelegation} {
		get := connect.NewRequest(&applicationv1.GetContentRequest{Reference: reference})
		get.Header().Set("Authorization", authorization)
		get.Header().Set(applicationDelegationHeader, delegation)
		response, getErr := client.Get(context.Background(), get)
		require.NoError(t, getErr, ownerValue)
		require.Equal(t, []byte("same delegated bytes"), response.Msg.GetPayload())
		owner, parseErr := identityprincipal.Parse(ownerValue)
		require.NoError(t, parseErr)
		require.True(t, store.service.HasBlobOwner(owner, reference.GetId()))
	}
	appOwner, err := identityprincipal.Parse(fixture.appPrincipal)
	require.NoError(t, err)
	require.False(t, store.service.HasBlobOwner(appOwner, reference.GetId()))
}

func TestDelegationPresentationBoundsMultiplicityAndCanonicalEncoding(t *testing.T) {
	raw := bytes.Repeat([]byte{0xa5}, 32)
	encoded := base64.RawURLEncoding.EncodeToString(raw)

	parsed, err := parseDelegation(http.Header{"ardents-delegation": []string{encoded}})
	require.NoError(t, err)
	require.Equal(t, raw, parsed)
	clear(parsed)

	for name, header := range map[string]http.Header{
		"empty":               {applicationDelegationHeader: []string{""}},
		"padded":              {applicationDelegationHeader: []string{encoded + "="}},
		"whitespace":          {applicationDelegationHeader: []string{" " + encoded}},
		"invalid alphabet":    {applicationDelegationHeader: []string{"opaque-secret-proof!"}},
		"noncanonical bits":   {applicationDelegationHeader: []string{"AB"}},
		"duplicate values":    {applicationDelegationHeader: []string{encoded, encoded}},
		"case-fold duplicate": {applicationDelegationHeader: []string{encoded}, "ardents-delegation": []string{encoded}},
		"encoded oversized":   {applicationDelegationHeader: []string{strings.Repeat("A", base64.RawURLEncoding.EncodedLen(identitycontract.MaxArtifactBytes)+1)}},
	} {
		t.Run(name, func(t *testing.T) {
			presentation, parseErr := parseDelegation(header)
			require.ErrorIs(t, parseErr, identityaccess.ErrUnauthenticated)
			require.Nil(t, presentation)
		})
	}

	tooLarge := bytes.Repeat([]byte{0x5a}, identitycontract.MaxArtifactBytes+1)
	presentation, err := parseDelegation(http.Header{applicationDelegationHeader: []string{base64.RawURLEncoding.EncodeToString(tooLarge)}})
	require.ErrorIs(t, err, identityaccess.ErrUnauthenticated)
	require.Nil(t, presentation)
}

func TestDelegationPresentationFailuresAreRedactedAndNeverReachAdmit(t *testing.T) {
	fixture := newRealApplicationFixture(t, []identityaccess.Action{"application.content.get"})
	store := newRecordingStore()
	admitter := &countingAdmitter{service: fixture.service}
	client := principalContentClient(t, fixture, admitter, store)
	authorization := "ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(fixture.secret[:])
	secretProof := "opaque-secret-proof!"

	for name, values := range map[string][]string{
		"malformed": {secretProof},
		"duplicate": {base64.RawURLEncoding.EncodeToString([]byte("one")), base64.RawURLEncoding.EncodeToString([]byte("two"))},
		"oversized": {strings.Repeat("A", base64.RawURLEncoding.EncodedLen(identitycontract.MaxArtifactBytes)+1)},
	} {
		t.Run(name, func(t *testing.T) {
			request := connect.NewRequest(&applicationv1.GetContentRequest{Reference: &applicationv1.ContentReference{Kind: "blob", Id: "missing"}})
			request.Header().Set("Authorization", authorization)
			for _, value := range values {
				request.Header().Add(applicationDelegationHeader, value)
			}
			_, err := client.Get(context.Background(), request)
			require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			require.NotContains(t, err.Error(), secretProof)
			for _, value := range values {
				require.NotContains(t, err.Error(), value)
			}
		})
	}
	require.Zero(t, admitter.calls.Load())
	require.Empty(t, store.calls)
}

func TestDelegationForAnotherNodeOrApplicationFailsClosedWithoutProofLeakage(t *testing.T) {
	fixture := newRealApplicationFixture(t, []identityaccess.Action{"application.content.get"})
	alice, aliceDevice, aliceCredential := enrollDelegator(t, fixture)
	otherKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x91}, ed25519.SeedSize))
	otherPrincipal, err := identityprincipal.FromEd25519PublicKey(otherKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	authorization := "ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(fixture.secret[:])

	for name, bounds := range map[string]struct{ node, delegatee string }{
		"other node":        {node: otherPrincipal.String(), delegatee: fixture.appPrincipal},
		"other application": {node: fixture.nodeID, delegatee: otherPrincipal.String()},
	} {
		t.Run(name, func(t *testing.T) {
			_, encoded := signedDelegation(t, fixture, alice, aliceDevice, aliceCredential, bounds.node, bounds.delegatee)
			store := newRecordingStore()
			admitter := &countingAdmitter{service: fixture.service}
			client := principalContentClient(t, fixture, admitter, store)
			request := connect.NewRequest(&applicationv1.GetContentRequest{Reference: &applicationv1.ContentReference{Kind: "blob", Id: "missing"}})
			request.Header().Set("Authorization", authorization)
			request.Header().Set(applicationDelegationHeader, encoded)
			_, err := client.Get(context.Background(), request)
			require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
			require.NotContains(t, err.Error(), encoded)
			require.Empty(t, store.calls)
		})
	}
}

func TestPrincipalAdmissionFailsClosedBeforeMutationAndNeverFallsBack(t *testing.T) {
	fixture := newRealApplicationFixture(t, []identityaccess.Action{"application.content.get"})
	store := newRecordingStore()
	admitter := &countingAdmitter{service: fixture.service}
	client := principalContentClient(t, fixture, admitter, store)
	header := "ArdentsApplicationSession " + base64.RawURLEncoding.EncodeToString(fixture.secret[:])

	put := connect.NewRequest(&applicationv1.PutContentRequest{Payload: []byte("denied")})
	put.Header().Set("Authorization", header)
	_, err := client.Put(context.Background(), put)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Empty(t, store.blobs)
	require.Equal(t, int32(1), admitter.calls.Load())

	malformed := connect.NewRequest(&applicationv1.GetContentRequest{Reference: &applicationv1.ContentReference{Kind: "blob", Id: "missing"}})
	malformed.Header().Set("Authorization", "ArdentsApplicationSession invalid second-value")
	_, err = client.Get(context.Background(), malformed)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, int32(1), admitter.calls.Load())

}

func principalContentClient(t *testing.T, fixture *realApplicationFixture, admitter Admitter, store applicationcontent.Store) applicationv1connect.ContentServiceClient {
	t.Helper()
	injector, extractor := applicationcall.NewChannel()
	interceptor, err := NewInterceptor(Config{
		Access: admitter, Node: fixture.nodeID,
		FallbackPeer: fixture.peer, FallbackSource: fixture.source, Injector: injector,
	})
	require.NoError(t, err)
	path, handler, err := applicationcontent.NewHTTPHandler(store, extractor, interceptor)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return applicationv1connect.NewContentServiceClient(server.Client(), server.URL)
}
