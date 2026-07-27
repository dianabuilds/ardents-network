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
	"strconv"
	"sync/atomic"
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
	localauthority "ardents/internal/localapi/authority"
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
		Authority:        owners.Authority,
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

func (i testNodeGrantIssuer) IssueAccessGrantRevocation(payload *identityprotocol.AccessGrantRevocationPayload, grant *identityaccess.Artifact) (*identityaccess.Artifact, error) {
	return identityaccess.SignAccessGrantRevocation(payload, i.key, payload.GetRevokedAt().AsTime(), grant)
}

func (i testNodeGrantIssuer) IssueDeviceRevocation(payload *identityprotocol.DeviceRevocationPayload) (*identityaccess.Artifact, error) {
	return identityaccess.SignDeviceRevocation(payload, i.key, payload.GetRevokedAt().AsTime())
}

func newOperatorPrincipalAccess(t *testing.T) (*identityaccess.Service, string, identityaccess.SessionSecret, [32]byte, identityaccess.SourceKey) {
	t.Helper()
	fixture := newOperatorPrincipalMaterial(t)
	return fixture.service, fixture.node, fixture.session, fixture.peer, fixture.source
}

type operatorPrincipalMaterial struct {
	service    *identityaccess.Service
	node       string
	principal  string
	session    identityaccess.SessionSecret
	binding    identityaccess.AuthenticationBinding
	peer       [32]byte
	source     identityaccess.SourceKey
	signerFile string
}

var operatorGrantSequence atomic.Uint64

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

	material := operatorPrincipalMaterial{
		service: service, node: node.String(), principal: rootInfo.Principal,
		session: *authenticated.SessionSecret, binding: binding,
		peer: peer, source: source, signerFile: devicePath,
	}
	issueOperatorGrant(t, material, grantedActions, identityaccess.ResourceScope{
		Kind: identityaccess.ScopeNode, Exact: identityaccess.ResourceRef{Node: node.String()},
	}, now)
	return material
}

func issueOperatorGrant(
	t *testing.T,
	material operatorPrincipalMaterial,
	grantedActions []identityaccess.Action,
	scope identityaccess.ResourceScope,
	now time.Time,
) {
	t.Helper()
	issueGrantFor(t, material, material.principal, grantedActions, scope, now)
}

func issueGrantFor(
	t *testing.T,
	material operatorPrincipalMaterial,
	subject string,
	grantedActions []identityaccess.Action,
	scope identityaccess.ResourceScope,
	now time.Time,
) {
	t.Helper()
	actions := append([]identityaccess.Action(nil), grantedActions...)
	sort.Slice(actions, func(left, right int) bool { return actions[left] < actions[right] })
	proposal := identityaccess.GrantProposal{
		Subject: subject, Actions: actions, Scope: scope,
		NotBefore: now, NotAfter: now.Add(time.Hour),
	}
	proposalID, err := identityaccess.GrantProposalResourceID(material.node, material.binding.Audience, proposal)
	require.NoError(t, err)
	resource, err := identityaccess.NewResourceRef(material.node, identityaccess.ResourceOwner{}, "grant-proposal", proposalID)
	require.NoError(t, err)
	requestID := "testkit-operator-grant-" + strconv.FormatUint(operatorGrantSequence.Add(1), 10)
	_, err = material.service.IssueAccessGrant(context.Background(), identityaccess.IssueGrantRequest{
		Command: identityaccess.AdminCommand{
			RequestID: requestID,
			Attempt: identityaccess.Attempt{
				SessionSecret: material.session, Binding: material.binding,
				Action: "identity.grant.issue", Resource: resource,
			},
		},
		Proposal: proposal,
	})
	require.NoError(t, err)
}

// OperatorCLIFixture exposes a Principal-only Operator endpoint over a real
// Unix socket. Root material remains in the fixture's private directory; CLI
// calls receive only the enrolled device signer bundle.
type OperatorCLIFixture struct {
	Addr          string
	SignerFile    string
	NodePrincipal string
	Principal     string
	Client        cliclient.Service
	material      operatorPrincipalMaterial
}

// GrantExact adds a test-only Operator grant for one canonical resource tuple.
// ownerPrincipal selects the fixture Principal for owner-bound resource kinds.
func (f OperatorCLIFixture) GrantExact(
	t *testing.T,
	actions []identityaccess.Action,
	kind identityaccess.ResourceKind,
	id string,
	ownerPrincipal bool,
) {
	t.Helper()
	owner := identityaccess.ResourceOwner{}
	var err error
	if ownerPrincipal {
		owner, err = identityaccess.ParseResourceOwner(f.Principal)
		require.NoError(t, err)
	}
	resource, err := identityaccess.NewResourceRef(f.NodePrincipal, owner, string(kind), id)
	require.NoError(t, err)
	issueOperatorGrant(t, f.material, actions, identityaccess.ResourceScope{
		Kind: identityaccess.ScopeExact, Exact: resource,
	}, time.Now().UTC().Truncate(time.Second))
}

func NewOperatorCLIFixture(t *testing.T, nodeRuntime *runtimeprocess.Node) OperatorCLIFixture {
	t.Helper()
	return NewOperatorCLIFixtureWithActions(t, nodeRuntime, testOperatorActions)
}

func NewOperatorCLIFixtureWithActions(t *testing.T, nodeRuntime *runtimeprocess.Node, actions []identityaccess.Action) OperatorCLIFixture {
	t.Helper()
	material := newOperatorPrincipalMaterialWithActions(t, actions)
	deps := ConnectDependencies(nodeRuntime)
	return newOperatorCLIFixture(t, nodeRuntime, material, deps)
}

// NewAuthorityOperatorCLIFixture grants only the exact configured authority
// instance create resource. Callers add the exact random RealmID inspect grant
// after genesis through GrantExact.
func NewAuthorityOperatorCLIFixture(t *testing.T, nodeRuntime *runtimeprocess.Node, service localauthority.Service) OperatorCLIFixture {
	t.Helper()
	material := newOperatorPrincipalMaterialWithActions(t, []identityaccess.Action{"node.status"})
	issueOperatorGrant(t, material, []identityaccess.Action{"realm.authority.create"}, identityaccess.ResourceScope{
		Kind: identityaccess.ScopeExact,
		Exact: identityaccess.ResourceRef{
			Node: material.node, Kind: "realm-authority-instance", ID: "primary",
		},
	}, time.Now().UTC().Truncate(time.Second))
	deps := ConnectDependencies(nodeRuntime)
	deps.Authority = service
	return newOperatorCLIFixture(t, nodeRuntime, material, deps)
}

func newOperatorCLIFixture(t *testing.T, nodeRuntime *runtimeprocess.Node, material operatorPrincipalMaterial, deps rpcadapter.Dependencies) OperatorCLIFixture {
	t.Helper()
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
		Addr: addr, SignerFile: material.signerFile, NodePrincipal: material.node,
		Principal: material.principal, Client: client.Service(), material: material,
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
	operator  operatorPrincipalMaterial
}

func NewApplicationPrincipalAccess(t *testing.T, actions []identityaccess.Action) ApplicationPrincipalAccess {
	t.Helper()
	ctx := context.Background()
	operator := newOperatorPrincipalMaterial(t)
	service, node := operator.service, operator.node
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
		TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: operator.peer,
	}
	resource, err := identityaccess.NewResourceRef(node, identityaccess.ResourceOwner{}, "principal", principal.String())
	require.NoError(t, err)
	ticket, err := service.IssueApplicationEnrollmentTicket(ctx, identityaccess.IssueApplicationEnrollmentTicketRequest{
		Attempt:   identityaccess.Attempt{SessionSecret: operator.session, Binding: operatorBinding, Action: "identity.principal.enroll", Resource: resource},
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
		Peer: peer, Source: source, operator: operator,
	}
}

// GrantExact adds a test-only finite grant for this Application Principal.
func (f ApplicationPrincipalAccess) GrantExact(
	t *testing.T,
	actions []identityaccess.Action,
	kind identityaccess.ResourceKind,
	id string,
) {
	t.Helper()
	resource, err := identityaccess.NewResourceRef(f.Node, identityaccess.ResourceOwner{}, string(kind), id)
	require.NoError(t, err)
	issueGrantFor(t, f.operator, f.Principal, actions, identityaccess.ResourceScope{
		Kind: identityaccess.ScopeExact, Exact: resource,
	}, time.Now().UTC().Truncate(time.Second))
}

// DelegateExactFromOperator creates a test-only one-hop Delegation from the
// enrolled Operator Principal and gives that Principal the matching current
// grant. The authenticated Application's own grant remains independent.
func (f ApplicationPrincipalAccess) DelegateExactFromOperator(
	t *testing.T,
	actions []identityaccess.Action,
	kind identityaccess.ResourceKind,
	id string,
) *identityaccess.Artifact {
	t.Helper()
	resource, err := identityaccess.NewResourceRef(f.Node, identityaccess.ResourceOwner{}, string(kind), id)
	require.NoError(t, err)
	return f.delegateFromOperator(t, actions, identityaccess.ResourceScope{
		Kind: identityaccess.ScopeExact, Exact: resource,
	})
}

// DelegateNodeFromOperator creates the corresponding Node-scoped fixture.
func (f ApplicationPrincipalAccess) DelegateNodeFromOperator(
	t *testing.T,
	actions []identityaccess.Action,
) *identityaccess.Artifact {
	t.Helper()
	return f.delegateFromOperator(t, actions, identityaccess.ResourceScope{
		Kind: identityaccess.ScopeNode, Exact: identityaccess.ResourceRef{Node: f.Node},
	})
}

func (f ApplicationPrincipalAccess) delegateFromOperator(
	t *testing.T,
	actions []identityaccess.Action,
	scope identityaccess.ResourceScope,
) *identityaccess.Artifact {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	issueGrantFor(t, f.operator, f.operator.principal, actions, scope, now)
	signer, err := cliidentity.OpenDeviceFileSigner(f.operator.signerFile)
	require.NoError(t, err)
	sorted := append([]identityaccess.Action(nil), actions...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	delegation, err := signer.SignDelegation(context.Background(), cliidentity.DelegationSpec{
		Delegatee: f.Principal,
		Audience: identityaccess.Audience{
			Node: f.Node, Interface: identityprotocol.Interface_INTERFACE_APPLICATION,
			ProtocolMajor: identitycontract.ProtocolMajor,
		},
		Actions: sorted, Scope: scope,
		NotBefore: now, NotAfter: now.Add(15 * time.Minute),
	}, now)
	require.NoError(t, err)
	return delegation
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
