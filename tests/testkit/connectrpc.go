package testkit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	cliclient "ardents/internal/cli/client"
	runtimeprocess "ardents/internal/daemon"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	rpcadapter "ardents/internal/localapi"
	"ardents/internal/storage"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthorizedRequest exists to keep call sites concise. Authentication is
// supplied by NewArdentsClient's Principal-session interceptor, never by the
// request payload or a long-lived bearer credential.
func AuthorizedRequest[T any](msg *T) *connect.Request[T] {
	return connect.NewRequest(msg)
}

func ConnectDependencies(runtime *runtimeprocess.Node) rpcadapter.Dependencies {
	owners, ok := runtimeprocess.OwnersFor(runtime)
	if !ok {
		return rpcadapter.Dependencies{}
	}
	return rpcadapter.Dependencies{
		Node:             runtime,
		Discovery:        runtime,
		DiscoveryRecords: owners.DiscoveryCommands,
		Network:          runtime,
		Diagnostics:      owners.Diagnostics,
		Workload:         owners.Workloads,
		Hosting:          owners.Hosting,
		Content:          owners.Content,
		Sources:          owners.Content,
		Transfers:        owners.Transfers,
		Data:             owners.ContentCommands,
		DataFetch:        runtime,
		Configuration:    runtime,
		Audit:            owners.Events,
	}
}

func NewArdentsClient(t *testing.T, runtime *runtimeprocess.Node) cliclient.Service {
	t.Helper()

	access, node, session, peer, source := newOperatorPrincipalAccess(t)
	handler, err := rpcadapter.NewPrincipalHandler(ConnectDependencies(runtime), access, node, peer, source)
	require.NoError(t, err)

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	interceptor := principalSessionInterceptor{
		authorization: "ArdentsOperatorSession " + base64.RawURLEncoding.EncodeToString(session[:]),
	}
	return cliclient.NewService(srv.Client(), srv.URL, connect.WithGRPC(), connect.WithInterceptors(interceptor))
}

type principalSessionInterceptor struct{ authorization string }

func (i principalSessionInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if request.Header().Get("Authorization") == "" {
			request.Header().Set("Authorization", i.authorization)
		}
		return next(ctx, request)
	}
}

func (i principalSessionInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		connection := next(ctx, spec)
		if connection.RequestHeader().Get("Authorization") == "" {
			connection.RequestHeader().Set("Authorization", i.authorization)
		}
		return connection
	}
}

func (i principalSessionInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

type testNodeGrantIssuer struct{ key ed25519.PrivateKey }

func (i testNodeGrantIssuer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), i.key.Public().(ed25519.PublicKey)...)
}

func (i testNodeGrantIssuer) IssueAccessGrant(payload *identityprotocol.AccessGrantPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrant(payload, i.key)
}

func newOperatorPrincipalAccess(t *testing.T) (*identityaccess.Service, string, identityaccess.SessionSecret, [32]byte, identityaccess.SourceKey) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	database, err := storage.OpenIdentityAccess(ctx, t.TempDir(), identityaccess.StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close(context.Background())) })

	_, nodeKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	node, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	_, rootKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(rootKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	_, deviceKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	device, err := identityprincipal.DeviceFromEd25519PublicKey(deviceKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)

	service, err := identityaccess.NewService(identityaccess.Config{
		Database: database, EnableBootstrapTickets: true, EnableApplicationEnrollment: true,
		GrantIssuer: testNodeGrantIssuer{key: nodeKey},
	})
	require.NoError(t, err)
	var peer [32]byte
	peer[0] = 0x71
	var source identityaccess.SourceKey
	source[0] = 0x72
	binding := identityaccess.AuthenticationBinding{
		Audience: identityaccess.Audience{
			Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR,
			ProtocolMajor: identitycontract.ProtocolMajor,
		},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1,
		PeerBinding:      peer,
	}

	ticket, err := service.IssueBootstrapTicket(ctx, node.String())
	require.NoError(t, err)
	enrollment, err := service.Begin(ctx, identityaccess.BeginRequest{
		Principal: principal.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF,
		Binding: binding, Source: source,
	})
	require.NoError(t, err)
	enrollmentSignature, err := identityaccess.SignEnrollmentChallenge(enrollment, rootKey)
	require.NoError(t, err)
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], rootKey.Public().(ed25519.PublicKey))
	completed, err := service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: enrollment.ID, Principal: principal.String(), Binding: binding, Source: source,
		RootPublicKey: rootPublic, Signature: enrollmentSignature,
	})
	require.NoError(t, err)
	require.NotNil(t, completed.EnrollmentProof)

	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: identitycontract.Version, Subject: principal.String(),
		RootPublicKey: rootKey.Public().(ed25519.PublicKey), DeviceId: device.String(),
		DevicePublicKey: deviceKey.Public().(ed25519.PublicKey),
		Purposes:        []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore:       timestamppb.New(now.Add(-time.Minute)), NotAfter: timestamppb.New(now.Add(time.Hour)),
	}, rootKey)
	require.NoError(t, err)
	credentialWire, err := credential.MarshalBinary()
	require.NoError(t, err)
	_, err = service.EnrollFirstPrincipal(ctx, binding, identityaccess.FirstEnrollmentRequest{
		Ticket: ticket, Challenge: enrollment, Proof: *completed.EnrollmentProof,
		RootPublicKey: rootPublic, Credential: credentialWire,
	})
	require.NoError(t, err)

	login, err := service.Begin(ctx, identityaccess.BeginRequest{
		Principal: principal.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
		Binding: binding, Source: source,
	})
	require.NoError(t, err)
	loginSignature, err := identityaccess.SignAuthenticationChallenge(login, credential, deviceKey)
	require.NoError(t, err)
	authenticated, err := service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: login.ID, Principal: principal.String(), Binding: binding, Source: source,
		RootPublicKey: rootPublic, Credential: credentialWire, Signature: loginSignature,
	})
	require.NoError(t, err)
	require.NotNil(t, authenticated.SessionSecret)

	actions := append([]identityaccess.Action(nil), testOperatorActions...)
	sort.Slice(actions, func(left, right int) bool { return actions[left] < actions[right] })
	proposal := identityaccess.GrantProposal{
		Subject: principal.String(), Actions: actions,
		Scope:     identityaccess.ResourceScope{Kind: identityaccess.ScopeNode, Exact: identityaccess.ResourceRef{Node: node.String()}},
		NotBefore: now, NotAfter: now.Add(time.Hour),
	}
	proposalID, err := identityaccess.GrantProposalResourceID(node.String(), binding.Audience, proposal)
	require.NoError(t, err)
	resource, err := identityaccess.NewResourceRef(node.String(), identityaccess.ResourceOwner{}, "grant-proposal", proposalID)
	require.NoError(t, err)
	_, err = service.IssueAccessGrant(ctx, identityaccess.IssueGrantRequest{
		Command: identityaccess.AdminCommand{
			RequestID: "testkit-operator-product-grant",
			Attempt: identityaccess.Attempt{
				SessionSecret: *authenticated.SessionSecret, Binding: binding,
				Action: "identity.grant.issue", Resource: resource,
			},
		},
		Proposal: proposal,
	})
	require.NoError(t, err)
	return service, node.String(), *authenticated.SessionSecret, peer, source
}

// ApplicationPrincipalAccess is a real enrolled Application identity fixture.
// Its Session can only be admitted by the returned Service for the exact
// Application Audience and transport binding.
type ApplicationPrincipalAccess struct {
	Service   *identityaccess.Service
	Node      string
	Principal string
	Session   identityaccess.SessionSecret
	Peer      [32]byte
	Source    identityaccess.SourceKey
}

func NewApplicationPrincipalAccess(t *testing.T, actions []identityaccess.Action) ApplicationPrincipalAccess {
	t.Helper()
	ctx := context.Background()
	service, node, operatorSession, operatorPeer, _ := newOperatorPrincipalAccess(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, root, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	_, device, err := ed25519.GenerateKey(rand.Reader)
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
	credentialRaw, err := credential.MarshalBinary()
	require.NoError(t, err)

	operatorBinding := identityaccess.AuthenticationBinding{
		Audience:         identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: operatorPeer,
	}
	resource, err := identityaccess.NewResourceRef(node, identityaccess.ResourceOwner{}, "principal", principal.String())
	require.NoError(t, err)
	ticket, err := service.IssueApplicationEnrollmentTicket(ctx, identityaccess.IssueApplicationEnrollmentTicketRequest{
		Attempt:   identityaccess.Attempt{SessionSecret: operatorSession, Binding: operatorBinding, Action: "identity.principal.enroll", Resource: resource},
		Principal: principal.String(), Actions: append([]identityaccess.Action(nil), actions...),
	})
	require.NoError(t, err)

	var peer [32]byte
	_, err = rand.Read(peer[:])
	require.NoError(t, err)
	var source identityaccess.SourceKey
	_, err = rand.Read(source[:])
	require.NoError(t, err)
	binding := identityaccess.AuthenticationBinding{
		Audience:         identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: identitycontract.ProtocolMajor},
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer,
	}
	enrollment, err := service.Begin(ctx, identityaccess.BeginRequest{
		Principal: principal.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF,
		Binding: binding, Source: source,
	})
	require.NoError(t, err)
	enrollmentSignature, err := identityaccess.SignEnrollmentChallenge(enrollment, root)
	require.NoError(t, err)
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], root.Public().(ed25519.PublicKey))
	proof, err := service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: enrollment.ID, Principal: principal.String(), Binding: binding, Source: source,
		RootPublicKey: rootPublic, Signature: enrollmentSignature,
	})
	require.NoError(t, err)
	require.NotNil(t, proof.EnrollmentProof)
	_, err = service.EnrollApplication(ctx, binding, identityaccess.EnrollApplicationRequest{
		Ticket: ticket.Ticket, Challenge: enrollment, Proof: *proof.EnrollmentProof,
		RootPublicKey: rootPublic, Credential: credentialRaw,
	})
	require.NoError(t, err)

	login, err := service.Begin(ctx, identityaccess.BeginRequest{
		Principal: principal.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
		Binding: binding, Source: source,
	})
	require.NoError(t, err)
	loginSignature, err := identityaccess.SignAuthenticationChallenge(login, credential, device)
	require.NoError(t, err)
	authenticated, err := service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: login.ID, Principal: principal.String(), Binding: binding, Source: source,
		RootPublicKey: rootPublic, Credential: credentialRaw, Signature: loginSignature,
	})
	require.NoError(t, err)
	require.NotNil(t, authenticated.SessionSecret)
	return ApplicationPrincipalAccess{
		Service: service, Node: node, Principal: principal.String(), Session: *authenticated.SessionSecret,
		Peer: peer, Source: source,
	}
}

var testOperatorActions = []identityaccess.Action{
	"node.start", "node.stop", "node.status", "node.features", "node.runtime", "node.events",
	"config.effective", "config.reload", "transport.network_status", "transport.route_candidates",
	"discovery.status", "discovery.local_presence", "discovery.peers", "discovery.list_records",
	"discovery.resolve_record", "discovery.resolve_service", "discovery.import", "workload.register",
	"workload.start", "workload.stop", "workload.restart", "workload.status", "workload.list",
	"workload.hosted_service", "workload.service_publication", "workload.hosted_services",
	"data.publish_object", "data.get_object", "data.list_objects", "data.publish_blob", "data.get_blob",
	"data.fetch_blob", "data.retain_blob", "data.pin_blob", "data.drop_blob", "data.blob_sources",
	"data.list_blobs", "data.get_transfer", "data.list_transfers", "data.publish_manifest",
	"data.get_manifest", "data.list_manifests", "data.inventory", "diagnostics.snapshot",
	"diagnostics.health_summary", "diagnostics.pending_operations", "diagnostics.explain_failure",
	"diagnostics.recent_events",
}
