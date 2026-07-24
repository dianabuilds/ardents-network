package testkit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	cliclient "ardents/internal/cli/client"
	cliidentity "ardents/internal/cli/identity"
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
// supplied by NewArdentsClient's short-lived Principal session, never by the
// request payload.
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
	return NewOperatorCLIFixture(t, runtime).Client
}

func NewArdentsClientWithActions(t *testing.T, runtime *runtimeprocess.Node, actions []identityaccess.Action) cliclient.Service {
	t.Helper()
	return NewOperatorCLIFixtureWithActions(t, runtime, actions).Client
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
	fixture := newOperatorPrincipalMaterial(t)
	return fixture.service, fixture.node, fixture.session, fixture.peer, fixture.source
}

type operatorPrincipalMaterial struct {
	service    *identityaccess.Service
	node       string
	session    identityaccess.SessionSecret
	peer       [32]byte
	source     identityaccess.SourceKey
	signerFile string
}

func newOperatorPrincipalMaterial(t *testing.T) operatorPrincipalMaterial {
	t.Helper()
	return newOperatorPrincipalMaterialWithActions(t, testOperatorActions)
}

func newOperatorPrincipalMaterialWithActions(t *testing.T, grantedActions []identityaccess.Action) operatorPrincipalMaterial {
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
	signerDir := filepath.Join(t.TempDir(), "operator-identity")
	require.NoError(t, storage.EnsurePrivateDir(signerDir))
	rootPath := filepath.Join(signerDir, "principal-root-v1.json")
	devicePath := filepath.Join(signerDir, "device-v1.json")
	rootInfo, err := cliidentity.CreatePrincipal(rootPath, rand.Reader)
	require.NoError(t, err)
	deviceInfo, err := cliidentity.CreateDevice(rootPath, devicePath, time.Hour, now.Add(-time.Minute), rand.Reader)
	require.NoError(t, err)
	require.Equal(t, rootInfo.Principal, deviceInfo.Principal)
	rootSigner, err := cliidentity.OpenRootFileSigner(rootPath)
	require.NoError(t, err)
	deviceSigner, err := cliidentity.OpenDeviceFileSigner(devicePath)
	require.NoError(t, err)
	rootPublicRaw, err := base64.RawURLEncoding.Strict().DecodeString(rootInfo.RootPublicKey)
	require.NoError(t, err)
	require.Len(t, rootPublicRaw, ed25519.PublicKeySize)
	credential, err := deviceSigner.Credential(ctx)
	require.NoError(t, err)
	credentialWire, err := credential.MarshalBinary()
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
		Principal: rootInfo.Principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF,
		Binding: binding, Source: source,
	})
	require.NoError(t, err)
	enrollmentSignature, err := rootSigner.SignEnrollmentChallenge(ctx, enrollment)
	require.NoError(t, err)
	var rootPublic [ed25519.PublicKeySize]byte
	copy(rootPublic[:], rootPublicRaw)
	completed, err := service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: enrollment.ID, Principal: rootInfo.Principal, Binding: binding, Source: source,
		RootPublicKey: rootPublic, Signature: enrollmentSignature,
	})
	require.NoError(t, err)
	require.NotNil(t, completed.EnrollmentProof)

	_, err = service.EnrollFirstPrincipal(ctx, binding, identityaccess.FirstEnrollmentRequest{
		Ticket: ticket, Challenge: enrollment, Proof: *completed.EnrollmentProof,
		RootPublicKey: rootPublic, Credential: credentialWire,
	})
	require.NoError(t, err)

	login, err := service.Begin(ctx, identityaccess.BeginRequest{
		Principal: rootInfo.Principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION,
		Binding: binding, Source: source,
	})
	require.NoError(t, err)
	loginSignature, err := deviceSigner.SignAuthenticationChallenge(ctx, login)
	require.NoError(t, err)
	authenticated, err := service.Complete(ctx, identityaccess.CompleteRequest{
		ChallengeID: login.ID, Principal: rootInfo.Principal, Binding: binding, Source: source,
		RootPublicKey: rootPublic, Credential: credentialWire, Signature: loginSignature,
	})
	require.NoError(t, err)
	require.NotNil(t, authenticated.SessionSecret)

	actions := append([]identityaccess.Action(nil), grantedActions...)
	sort.Slice(actions, func(left, right int) bool { return actions[left] < actions[right] })
	proposal := identityaccess.GrantProposal{
		Subject: rootInfo.Principal, Actions: actions,
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
	return operatorPrincipalMaterial{
		service: service, node: node.String(),
		session: *authenticated.SessionSecret, peer: peer, source: source, signerFile: devicePath,
	}
}

// OperatorCLIFixture exposes a Principal-only Operator endpoint over a real
// Unix socket. Root material remains in the fixture's private directory; CLI
// calls receive only the enrolled device signer bundle.
type OperatorCLIFixture struct {
	Addr          string
	SignerFile    string
	NodePrincipal string
	Client        cliclient.Service
}

func NewOperatorCLIFixture(t *testing.T, nodeRuntime *runtimeprocess.Node) OperatorCLIFixture {
	t.Helper()
	return NewOperatorCLIFixtureWithActions(t, nodeRuntime, testOperatorActions)
}

func NewOperatorCLIFixtureWithActions(t *testing.T, nodeRuntime *runtimeprocess.Node, actions []identityaccess.Action) OperatorCLIFixture {
	t.Helper()
	material := newOperatorPrincipalMaterialWithActions(t, actions)
	deps := ConnectDependencies(nodeRuntime)
	deps.Node = principalBoundRuntime{Node: nodeRuntime, principal: material.node}
	_, handler, err := rpcadapter.NewProtectedHandler(deps, material.service, material.node, material.peer, material.source)
	require.NoError(t, err)

	socketDir, err := os.MkdirTemp("", "ardents-operator-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(socketDir)) })
	socketPath := filepath.Join(socketDir, "operator.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(socketPath, 0o600))
	server := &http.Server{Handler: handler}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		require.NoError(t, server.Close())
		<-done
	})

	signer, err := cliidentity.OpenDeviceFileSigner(material.signerFile)
	require.NoError(t, err)
	addr := (&url.URL{Scheme: "unix", Path: filepath.ToSlash(socketPath)}).String()
	client, err := cliclient.New(cliclient.Config{
		BaseURL: addr, ExpectedPrincipal: material.node, Signer: signer, Timeout: 10 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return OperatorCLIFixture{
		Addr: addr, SignerFile: material.signerFile, NodePrincipal: material.node, Client: client.Service(),
	}
}

type principalBoundRuntime struct {
	*runtimeprocess.Node
	principal string
}

func (r principalBoundRuntime) GetNodeRuntime() runtimeprocess.RuntimeSnapshot {
	snapshot := r.Node.GetNodeRuntime()
	snapshot.Identity.Principal = r.principal
	return snapshot
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
