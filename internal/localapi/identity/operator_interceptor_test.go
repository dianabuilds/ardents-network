package identity_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	operatorapi "ardents/internal/localapi"
	localidentity "ardents/internal/localapi/identity"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	"ardents/internal/localapi/rpc"
	"ardents/internal/storage"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type countingAdmitter struct {
	service *identityaccess.Service
	calls   atomic.Int32
}

func (a *countingAdmitter) AdmitTarget(ctx context.Context, attempt identityaccess.TargetAttempt) (identityaccess.AuthorizedCall, error) {
	a.calls.Add(1)
	return a.service.AdmitTarget(ctx, attempt)
}

type nodeIssuer struct{ key ed25519.PrivateKey }

func (i nodeIssuer) PublicKey() ed25519.PublicKey { return i.key.Public().(ed25519.PublicKey) }
func (i nodeIssuer) IssueAccessGrant(p *identityprotocol.AccessGrantPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrant(p, i.key)
}
func (i nodeIssuer) IssueAccessGrantRevocation(p *identityprotocol.AccessGrantRevocationPayload, grant *identityaccess.Artifact) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrantRevocation(p, i.key, p.RevokedAt.AsTime(), grant)
}
func (i nodeIssuer) IssueDeviceRevocation(p *identityprotocol.DeviceRevocationPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignDeviceRevocation(p, i.key, p.RevokedAt.AsTime())
}

type nodeStatusHandler struct {
	ardentsv1connect.UnimplementedNodeServiceHandler
	actor, effective string
	streamActor      string
}

type configurationHandler struct {
	ardentsv1connect.UnimplementedConfigurationServiceHandler
	actor string
}

type admissionGuardHandler struct {
	ardentsv1connect.UnimplementedNodeServiceHandler
	ardentsv1connect.UnimplementedConfigurationServiceHandler
	calls atomic.Int32
}

func (h *admissionGuardHandler) admitted(ctx context.Context) error {
	call, ok := rpc.CallFromContext(ctx)
	if !ok || call.Actor() == "" || call.Effective() == "" {
		return connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	h.calls.Add(1)
	return nil
}

func (h *admissionGuardHandler) StartNode(ctx context.Context, _ *connect.Request[protocol.StartNodeRequest]) (*connect.Response[protocol.CommandAckResponse], error) {
	return connect.NewResponse(&protocol.CommandAckResponse{}), h.admitted(ctx)
}

func (h *admissionGuardHandler) StopNode(ctx context.Context, _ *connect.Request[protocol.StopNodeRequest]) (*connect.Response[protocol.CommandAckResponse], error) {
	return connect.NewResponse(&protocol.CommandAckResponse{}), h.admitted(ctx)
}

func (h *admissionGuardHandler) GetNodeStatus(ctx context.Context, _ *connect.Request[protocol.GetNodeStatusRequest]) (*connect.Response[protocol.NodeStatusResponse], error) {
	return connect.NewResponse(&protocol.NodeStatusResponse{}), h.admitted(ctx)
}

func (h *admissionGuardHandler) GetNodeFeatures(ctx context.Context, _ *connect.Request[protocol.GetNodeFeaturesRequest]) (*connect.Response[protocol.NodeFeaturesResponse], error) {
	return connect.NewResponse(&protocol.NodeFeaturesResponse{}), h.admitted(ctx)
}

func (h *admissionGuardHandler) GetNodeRuntime(ctx context.Context, _ *connect.Request[protocol.GetNodeRuntimeRequest]) (*connect.Response[protocol.NodeRuntimeResponse], error) {
	return connect.NewResponse(&protocol.NodeRuntimeResponse{}), h.admitted(ctx)
}

func (h *admissionGuardHandler) StreamNodeEvents(ctx context.Context, _ *connect.Request[protocol.StreamNodeEventsRequest], _ *connect.ServerStream[protocol.EventEnvelope]) error {
	return h.admitted(ctx)
}

func (h *admissionGuardHandler) GetEffectiveConfiguration(ctx context.Context, _ *connect.Request[protocol.GetEffectiveConfigurationRequest]) (*connect.Response[protocol.EffectiveConfigurationResponse], error) {
	return connect.NewResponse(&protocol.EffectiveConfigurationResponse{}), h.admitted(ctx)
}

func (h *admissionGuardHandler) ReloadConfiguration(ctx context.Context, _ *connect.Request[protocol.ReloadConfigurationRequest]) (*connect.Response[protocol.ReloadConfigurationResponse], error) {
	return connect.NewResponse(&protocol.ReloadConfigurationResponse{}), h.admitted(ctx)
}

func (h *configurationHandler) GetEffectiveConfiguration(ctx context.Context, _ *connect.Request[protocol.GetEffectiveConfigurationRequest]) (*connect.Response[protocol.EffectiveConfigurationResponse], error) {
	call, ok := rpc.CallFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	h.actor = call.Actor()
	return connect.NewResponse(&protocol.EffectiveConfigurationResponse{}), nil
}

func (h *nodeStatusHandler) StreamNodeEvents(ctx context.Context, _ *connect.Request[protocol.StreamNodeEventsRequest], _ *connect.ServerStream[protocol.EventEnvelope]) error {
	call, ok := rpc.CallFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	h.streamActor = call.Actor()
	return nil
}

func (h *nodeStatusHandler) GetNodeStatus(ctx context.Context, _ *connect.Request[protocol.GetNodeStatusRequest]) (*connect.Response[protocol.NodeStatusResponse], error) {
	call, ok := rpc.CallFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	h.actor, h.effective = call.Actor(), call.Effective()
	return connect.NewResponse(&protocol.NodeStatusResponse{}), nil
}

func TestOperatorPrincipalInterceptorAdmitsExactlyOnceAndPropagatesActorEffective(t *testing.T) {
	service, node, principal, secret, peer, source, grantID := operatorAccessFixture(t)
	counter := &countingAdmitter{service: service}
	interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	handler := &nodeStatusHandler{}
	path, connectHandler := ardentsv1connect.NewNodeServiceHandler(handler, connect.WithInterceptors(interceptor))
	mux := http.NewServeMux()
	mux.Handle(path, connectHandler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	client := ardentsv1connect.NewNodeServiceClient(server.Client(), server.URL)
	request := connect.NewRequest(&protocol.GetNodeStatusRequest{})
	request.Header().Set("Authorization", "ArdentsOperatorSession "+base64.RawURLEncoding.EncodeToString(secret[:]))
	_, err = client.GetNodeStatus(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, int32(1), counter.calls.Load())
	require.Equal(t, principal, handler.actor)
	require.Equal(t, principal, handler.effective)
	configurationCounter := &countingAdmitter{service: service}
	configurationInterceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: configurationCounter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	configuration := &configurationHandler{}
	_, configurationHTTP := ardentsv1connect.NewConfigurationServiceHandler(configuration, connect.WithInterceptors(configurationInterceptor))
	configurationServer := httptest.NewServer(configurationHTTP)
	t.Cleanup(configurationServer.Close)
	configurationClient := ardentsv1connect.NewConfigurationServiceClient(configurationServer.Client(), configurationServer.URL)
	configurationRequest := connect.NewRequest(&protocol.GetEffectiveConfigurationRequest{})
	configurationRequest.Header().Set("Authorization", request.Header().Get("Authorization"))
	_, err = configurationClient.GetEffectiveConfiguration(context.Background(), configurationRequest)
	require.NoError(t, err)
	require.Equal(t, int32(1), configurationCounter.calls.Load())
	require.Equal(t, principal, configuration.actor)
	streamRequest := connect.NewRequest(&protocol.StreamNodeEventsRequest{})
	streamRequest.Header().Set("Authorization", request.Header().Get("Authorization"))
	stream, err := client.StreamNodeEvents(context.Background(), streamRequest)
	require.NoError(t, err)
	require.False(t, stream.Receive())
	require.NoError(t, stream.Err())
	require.Equal(t, int32(2), counter.calls.Load())
	require.Equal(t, principal, handler.streamActor)

	sibling := connect.NewRequest(&protocol.GetNodeFeaturesRequest{})
	sibling.Header().Set("Authorization", request.Header().Get("Authorization"))
	_, err = client.GetNodeFeatures(context.Background(), sibling)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Equal(t, int32(3), counter.calls.Load())

	legacy := connect.NewRequest(&protocol.GetNodeStatusRequest{})
	legacy.Header().Set("Authorization", "Bearer legacy-must-not-fallback")
	_, err = client.GetNodeStatus(context.Background(), legacy)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, int32(3), counter.calls.Load())

	unknown := connect.NewRequest(&protocol.GetNodeStatusRequest{})
	unknown.Msg.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	unknown.Header().Set("Authorization", request.Header().Get("Authorization"))
	_, err = client.GetNodeStatus(context.Background(), unknown)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	require.Equal(t, int32(3), counter.calls.Load())

	unknownStream := connect.NewRequest(&protocol.StreamNodeEventsRequest{})
	unknownStream.Msg.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	unknownStream.Header().Set("Authorization", request.Header().Get("Authorization"))
	stream, err = client.StreamNodeEvents(context.Background(), unknownStream)
	require.NoError(t, err)
	require.False(t, stream.Receive())
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(stream.Err()))
	require.Equal(t, int32(3), counter.calls.Load())

	binding := identityaccess.AuthenticationBinding{Audience: identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer}
	grantResource, err := identityaccess.NewResourceRef(node, identityaccess.ResourceOwner{}, "access-grant", grantID)
	require.NoError(t, err)
	_, err = service.RevokeAccessGrant(context.Background(), identityaccess.RevokeGrantRequest{Command: identityaccess.AdminCommand{RequestID: "revoke-node-status", Attempt: identityaccess.Attempt{SessionSecret: secret, Binding: binding, Action: "identity.grant.revoke", Resource: grantResource}}, GrantID: grantID})
	require.NoError(t, err)
	revokedRequest := connect.NewRequest(&protocol.GetNodeStatusRequest{})
	revokedRequest.Header().Set("Authorization", request.Header().Get("Authorization"))
	_, err = client.GetNodeStatus(context.Background(), revokedRequest)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	require.Equal(t, int32(4), counter.calls.Load())

	crossPeerCounter := &countingAdmitter{service: service}
	crossPeerInterceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: crossPeerCounter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	_, crossPeerHandler := ardentsv1connect.NewNodeServiceHandler(&nodeStatusHandler{}, connect.WithInterceptors(crossPeerInterceptor))
	crossPeerServer := httptest.NewUnstartedServer(crossPeerHandler)
	crossPeerServer.Config.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
		otherPeer := [32]byte{9}
		otherSource := identityaccess.SourceKey{8}
		return identityaccess.WithTransportPeer(ctx, otherPeer, otherSource)
	}
	crossPeerServer.Start()
	t.Cleanup(crossPeerServer.Close)
	crossPeerClient := ardentsv1connect.NewNodeServiceClient(crossPeerServer.Client(), crossPeerServer.URL)
	crossPeerRequest := connect.NewRequest(&protocol.GetNodeStatusRequest{})
	crossPeerRequest.Header().Set("Authorization", request.Header().Get("Authorization"))
	_, err = crossPeerClient.GetNodeStatus(context.Background(), crossPeerRequest)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, int32(1), crossPeerCounter.calls.Load())

	_, otherNodeKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	otherNode, err := identityprincipal.FromEd25519PublicKey(otherNodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	crossNodeCounter := &countingAdmitter{service: service}
	crossNodeInterceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: crossNodeCounter, Node: otherNode.String(), FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	_, crossNodeHandler := ardentsv1connect.NewNodeServiceHandler(&nodeStatusHandler{}, connect.WithInterceptors(crossNodeInterceptor))
	crossNodeServer := httptest.NewServer(crossNodeHandler)
	t.Cleanup(crossNodeServer.Close)
	crossNodeClient := ardentsv1connect.NewNodeServiceClient(crossNodeServer.Client(), crossNodeServer.URL)
	crossNodeRequest := connect.NewRequest(&protocol.GetNodeStatusRequest{})
	crossNodeRequest.Header().Set("Authorization", request.Header().Get("Authorization"))
	_, err = crossNodeClient.GetNodeStatus(context.Background(), crossNodeRequest)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Equal(t, int32(1), crossNodeCounter.calls.Load())
}

func TestOperatorPrincipalInterceptorGuardsEveryB1ProcedureExactlyOnce(t *testing.T) {
	actions := []identityaccess.Action{
		"node.start", "node.stop", "node.status", "node.features", "node.runtime", "node.events",
		"config.effective", "config.reload",
	}
	service, node, _, secret, peer, source, _ := operatorAccessFixtureWithActions(t, actions)
	counter := &countingAdmitter{service: service}
	interceptor, err := localidentity.NewOperatorInterceptor(localidentity.OperatorInterceptorConfig{Access: counter, Node: node, FallbackPeer: peer, FallbackSource: source, Canonicalize: operatorapi.CanonicalizeOperatorResource})
	require.NoError(t, err)
	handler := &admissionGuardHandler{}
	mux := http.NewServeMux()
	nodePath, nodeHTTP := ardentsv1connect.NewNodeServiceHandler(handler, connect.WithInterceptors(interceptor))
	configurationPath, configurationHTTP := ardentsv1connect.NewConfigurationServiceHandler(handler, connect.WithInterceptors(interceptor))
	mux.Handle(nodePath, nodeHTTP)
	mux.Handle(configurationPath, configurationHTTP)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	nodeClient := ardentsv1connect.NewNodeServiceClient(server.Client(), server.URL)
	configurationClient := ardentsv1connect.NewConfigurationServiceClient(server.Client(), server.URL)
	authorization := "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(secret[:])
	call := func(request connect.AnyRequest, invoke func() error) {
		request.Header().Set("Authorization", authorization)
		beforeAdmission, beforeHandler := counter.calls.Load(), handler.calls.Load()
		require.NoError(t, invoke())
		require.Equal(t, beforeAdmission+1, counter.calls.Load())
		require.Equal(t, beforeHandler+1, handler.calls.Load())
	}

	start := connect.NewRequest(&protocol.StartNodeRequest{})
	call(start, func() error { _, err := nodeClient.StartNode(context.Background(), start); return err })
	stop := connect.NewRequest(&protocol.StopNodeRequest{})
	call(stop, func() error { _, err := nodeClient.StopNode(context.Background(), stop); return err })
	status := connect.NewRequest(&protocol.GetNodeStatusRequest{})
	call(status, func() error { _, err := nodeClient.GetNodeStatus(context.Background(), status); return err })
	features := connect.NewRequest(&protocol.GetNodeFeaturesRequest{})
	call(features, func() error { _, err := nodeClient.GetNodeFeatures(context.Background(), features); return err })
	runtime := connect.NewRequest(&protocol.GetNodeRuntimeRequest{})
	call(runtime, func() error { _, err := nodeClient.GetNodeRuntime(context.Background(), runtime); return err })
	streamRequest := connect.NewRequest(&protocol.StreamNodeEventsRequest{})
	call(streamRequest, func() error {
		stream, streamErr := nodeClient.StreamNodeEvents(context.Background(), streamRequest)
		if streamErr != nil {
			return streamErr
		}
		stream.Receive()
		return stream.Err()
	})
	effective := connect.NewRequest(&protocol.GetEffectiveConfigurationRequest{})
	call(effective, func() error {
		_, err := configurationClient.GetEffectiveConfiguration(context.Background(), effective)
		return err
	})
	reload := connect.NewRequest(&protocol.ReloadConfigurationRequest{})
	call(reload, func() error {
		_, err := configurationClient.ReloadConfiguration(context.Background(), reload)
		return err
	})
	require.Equal(t, int32(len(actions)), counter.calls.Load())
}

func operatorAccessFixture(t *testing.T) (*identityaccess.Service, string, string, identityaccess.SessionSecret, [32]byte, identityaccess.SourceKey, string) {
	return operatorAccessFixtureWithActions(t, []identityaccess.Action{"config.effective", "node.events", "node.status"})
}

func operatorAccessFixtureWithActions(t *testing.T, actions []identityaccess.Action) (*identityaccess.Service, string, string, identityaccess.SessionSecret, [32]byte, identityaccess.SourceKey, string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	database, err := storage.OpenIdentityAccess(ctx, t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })
	_, nodeKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	nodeID, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	_, root, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principalID, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	_, device, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	service, err := identityaccess.NewService(identityaccess.Config{Database: database, EnableBootstrapTickets: true, GrantIssuer: nodeIssuer{nodeKey}})
	require.NoError(t, err)
	var peer [32]byte
	peer[0] = 1
	var source identityaccess.SourceKey
	source[0] = 2
	binding := identityaccess.AuthenticationBinding{Audience: identityaccess.Audience{Node: nodeID.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: 1}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer}
	ticket, err := service.IssueBootstrapTicket(ctx, nodeID.String())
	require.NoError(t, err)
	enrollment, err := service.Begin(ctx, identityaccess.BeginRequest{Principal: principalID.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, Binding: binding, Source: source})
	require.NoError(t, err)
	enrollmentSignature := signChallenge(t, enrollment, root, identitycontract.EnrollmentChallengeDomain)
	var rootPublic [32]byte
	copy(rootPublic[:], root.Public().(ed25519.PublicKey))
	completed, err := service.Complete(ctx, identityaccess.CompleteRequest{ChallengeID: enrollment.ID, Principal: principalID.String(), Binding: binding, Source: source, RootPublicKey: rootPublic, Signature: enrollmentSignature})
	require.NoError(t, err)
	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{Version: 1, Subject: principalID.String(), RootPublicKey: root.Public().(ed25519.PublicKey), DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(now.Add(-time.Minute)), NotAfter: timestamppb.New(now.Add(time.Hour))}, root)
	require.NoError(t, err)
	credentialRaw, err := credential.MarshalBinary()
	require.NoError(t, err)
	_, err = service.EnrollFirstPrincipal(ctx, binding, identityaccess.FirstEnrollmentRequest{Ticket: ticket, Challenge: enrollment, Proof: *completed.EnrollmentProof, RootPublicKey: rootPublic, Credential: credentialRaw})
	require.NoError(t, err)
	login, err := service.Begin(ctx, identityaccess.BeginRequest{Principal: principalID.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION, Binding: binding, Source: source})
	require.NoError(t, err)
	loginSignature := signChallenge(t, login, device, identitycontract.AuthenticationChallengeDomain)
	session, err := service.Complete(ctx, identityaccess.CompleteRequest{ChallengeID: login.ID, Principal: principalID.String(), Binding: binding, Source: source, RootPublicKey: rootPublic, Credential: credentialRaw, Signature: loginSignature})
	require.NoError(t, err)
	canonicalActions := append([]identityaccess.Action(nil), actions...)
	sort.Slice(canonicalActions, func(i, j int) bool { return canonicalActions[i] < canonicalActions[j] })
	proposal := identityaccess.GrantProposal{Subject: principalID.String(), Actions: canonicalActions, Scope: identityaccess.ResourceScope{Kind: identityaccess.ScopeNode, Exact: identityaccess.ResourceRef{Node: nodeID.String()}}, NotBefore: now, NotAfter: now.Add(time.Hour)}
	proposalID, err := identityaccess.GrantProposalResourceID(nodeID.String(), binding.Audience, proposal)
	require.NoError(t, err)
	resource, err := identityaccess.NewResourceRef(nodeID.String(), identityaccess.ResourceOwner{}, "grant-proposal", proposalID)
	require.NoError(t, err)
	grantID, err := service.IssueAccessGrant(ctx, identityaccess.IssueGrantRequest{Command: identityaccess.AdminCommand{RequestID: "grant-node-status", Attempt: identityaccess.Attempt{SessionSecret: *session.SessionSecret, Binding: binding, Action: "identity.grant.issue", Resource: resource}}, Proposal: proposal})
	require.NoError(t, err)
	return service, nodeID.String(), principalID.String(), *session.SessionSecret, peer, source, grantID
}

func signChallenge(t *testing.T, challenge identityaccess.Challenge, key ed25519.PrivateKey, domain string) []byte {
	t.Helper()
	fields, err := identityaccess.ChallengeFields(challenge)
	require.NoError(t, err)
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(fields)
	require.NoError(t, err)
	return ed25519.Sign(key, append([]byte(domain), raw...))
}
