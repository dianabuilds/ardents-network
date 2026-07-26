package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	"ardents/internal/cli/client"
	"ardents/internal/cli/output"
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

var administrationNow = time.Date(2032, 4, 5, 6, 7, 8, 0, time.UTC)

type fakeIdentityClient struct {
	begin             func(*protocol.BeginAuthenticationRequest) *protocol.BeginAuthenticationResponse
	complete          func(*protocol.CompleteAuthenticationRequest) *protocol.CompleteAuthenticationResponse
	enrollFirst       func(*protocol.EnrollFirstPrincipalRequest) *protocol.EnrollFirstPrincipalResponse
	enroll            func(*protocol.EnrollPrincipalRequest) *protocol.EnrollPrincipalResponse
	issue             func(*protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse
	revoke            func(*protocol.RevokeAccessGrantRequest) *protocol.RevokeAccessGrantResponse
	list              func(*protocol.ListAccessGrantsRequest) *protocol.ListAccessGrantsResponse
	revokeDevice      func(*protocol.RevokeDeviceRequest) *protocol.RevokeDeviceResponse
	applicationTicket func(*protocol.IssueApplicationEnrollmentTicketRequest) *protocol.IssueApplicationEnrollmentTicketResponse
	importDelegation  func(*protocol.ImportDelegationRevocationRequest) *protocol.ImportDelegationRevocationResponse
	mutationCalls     int
	issueErrors       []error
	issueRequests     []string
}

func (f *fakeIdentityClient) BeginAuthentication(_ context.Context, request *connect.Request[protocol.BeginAuthenticationRequest]) (*connect.Response[protocol.BeginAuthenticationResponse], error) {
	if f.begin == nil {
		return nil, errors.New("unexpected BeginAuthentication")
	}
	return connect.NewResponse(f.begin(request.Msg)), nil
}
func (f *fakeIdentityClient) CompleteAuthentication(_ context.Context, request *connect.Request[protocol.CompleteAuthenticationRequest]) (*connect.Response[protocol.CompleteAuthenticationResponse], error) {
	if f.complete == nil {
		return nil, errors.New("unexpected CompleteAuthentication")
	}
	return connect.NewResponse(f.complete(request.Msg)), nil
}
func (*fakeIdentityClient) EndSession(context.Context, *connect.Request[protocol.EndSessionRequest]) (*connect.Response[protocol.EndSessionResponse], error) {
	return connect.NewResponse(&protocol.EndSessionResponse{}), nil
}
func (f *fakeIdentityClient) EnrollFirstPrincipal(_ context.Context, request *connect.Request[protocol.EnrollFirstPrincipalRequest]) (*connect.Response[protocol.EnrollFirstPrincipalResponse], error) {
	if f.enrollFirst == nil {
		return nil, errors.New("unexpected EnrollFirstPrincipal")
	}
	f.mutationCalls++
	return connect.NewResponse(f.enrollFirst(request.Msg)), nil
}
func (f *fakeIdentityClient) EnrollPrincipal(_ context.Context, request *connect.Request[protocol.EnrollPrincipalRequest]) (*connect.Response[protocol.EnrollPrincipalResponse], error) {
	if f.enroll == nil {
		return nil, errors.New("unexpected EnrollPrincipal")
	}
	f.mutationCalls++
	return connect.NewResponse(f.enroll(request.Msg)), nil
}
func (f *fakeIdentityClient) RevokeDevice(_ context.Context, request *connect.Request[protocol.RevokeDeviceRequest]) (*connect.Response[protocol.RevokeDeviceResponse], error) {
	if f.revokeDevice == nil {
		return nil, errors.New("unexpected RevokeDevice")
	}
	f.mutationCalls++
	return connect.NewResponse(f.revokeDevice(request.Msg)), nil
}
func (*fakeIdentityClient) ListDeviceRevocations(context.Context, *connect.Request[protocol.ListDeviceRevocationsRequest]) (*connect.Response[protocol.ListDeviceRevocationsResponse], error) {
	return nil, errors.New("unexpected ListDeviceRevocations")
}
func (f *fakeIdentityClient) IssueAccessGrant(_ context.Context, request *connect.Request[protocol.IssueAccessGrantRequest]) (*connect.Response[protocol.IssueAccessGrantResponse], error) {
	if f.issue == nil {
		return nil, errors.New("unexpected IssueAccessGrant")
	}
	f.mutationCalls++
	f.issueRequests = append(f.issueRequests, request.Msg.RequestId)
	if len(f.issueErrors) != 0 {
		err := f.issueErrors[0]
		f.issueErrors = f.issueErrors[1:]
		return nil, err
	}
	return connect.NewResponse(f.issue(request.Msg)), nil
}
func (f *fakeIdentityClient) RevokeAccessGrant(_ context.Context, request *connect.Request[protocol.RevokeAccessGrantRequest]) (*connect.Response[protocol.RevokeAccessGrantResponse], error) {
	if f.revoke == nil {
		return nil, errors.New("unexpected RevokeAccessGrant")
	}
	f.mutationCalls++
	return connect.NewResponse(f.revoke(request.Msg)), nil
}
func (f *fakeIdentityClient) ListAccessGrants(_ context.Context, request *connect.Request[protocol.ListAccessGrantsRequest]) (*connect.Response[protocol.ListAccessGrantsResponse], error) {
	if f.list == nil {
		return nil, errors.New("unexpected ListAccessGrants")
	}
	return connect.NewResponse(f.list(request.Msg)), nil
}
func (f *fakeIdentityClient) IssueApplicationEnrollmentTicket(_ context.Context, request *connect.Request[protocol.IssueApplicationEnrollmentTicketRequest]) (*connect.Response[protocol.IssueApplicationEnrollmentTicketResponse], error) {
	if f.applicationTicket == nil {
		return nil, errors.New("unexpected IssueApplicationEnrollmentTicket")
	}
	f.mutationCalls++
	return connect.NewResponse(f.applicationTicket(request.Msg)), nil
}
func (f *fakeIdentityClient) ImportDelegationRevocation(_ context.Context, request *connect.Request[protocol.ImportDelegationRevocationRequest]) (*connect.Response[protocol.ImportDelegationRevocationResponse], error) {
	if f.importDelegation == nil {
		return nil, errors.New("unexpected ImportDelegationRevocation")
	}
	f.mutationCalls++
	return connect.NewResponse(f.importDelegation(request.Msg)), nil
}

type administrationSessions struct {
	public, protected ardentsv1connect.IdentityServiceClient
	node              string
}

func (*administrationSessions) Login(context.Context) (client.SessionKey, error) {
	return client.SessionKey{}, nil
}
func (*administrationSessions) SessionStatus() client.SessionKey { return client.SessionKey{} }
func (*administrationSessions) Logout() error                    { return nil }
func (s *administrationSessions) PublicIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	return s.public, nil
}
func (s *administrationSessions) ProtectedIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	return s.protected, nil
}
func (s *administrationSessions) TargetNodePrincipal() string { return s.node }

func principalForTest(t *testing.T, fill byte) string {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{fill}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(private.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	return principal.String()
}

func artifactID(prefix string) string {
	digest := sha256.Sum256([]byte("PIA-010C deterministic test artifact: " + prefix))
	return prefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(digest[:]))
}

func newAdministrationCommand(jsonOutput bool, input string, sessions *administrationSessions, service *fakeIdentityClient) (Command, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	sessions.protected = service
	if sessions.public == nil {
		sessions.public = service
	}
	command := NewOnline(output.NewRenderer(&stdout, &stderr, jsonOutput), sessions, time.Second, strings.NewReader(input))
	command.now = func() time.Time { return administrationNow }
	command.entropy = bytes.NewReader(bytes.Repeat([]byte{0x42}, 128))
	return command, &stdout, &stderr
}

func TestApplicationTicketIsWrittenOnlyToProtectedFile(t *testing.T) {
	node, principal := principalForTest(t, 0x51), principalForTest(t, 0x52)
	ticket := bytes.Repeat([]byte{0xa7}, identitycontract.ApplicationEnrollmentTicketBytes)
	service := &fakeIdentityClient{applicationTicket: func(request *protocol.IssueApplicationEnrollmentTicketRequest) *protocol.IssueApplicationEnrollmentTicketResponse {
		require.Equal(t, principal, request.ApplicationPrincipalId)
		require.Equal(t, []string{"application.content.get", "application.content.put"}, request.InitialActions)
		return &protocol.IssueApplicationEnrollmentTicketResponse{ApplicationEnrollmentTicket: append([]byte(nil), ticket...), ExpiresAt: timestamppb.New(administrationNow.Add(identitycontract.ApplicationEnrollmentTicketLifetime))}
	}}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	path := filepath.Join(t.TempDir(), "protected", "application-enrollment-ticket")
	code := command.Run(context.Background(), []string{"application-ticket", "issue", "--principal", principal, "--action", "application.content.put", "--action", "application.content.get", "--out-file", path})
	require.Zero(t, code)
	require.Equal(t, 1, service.mutationCalls)
	encoded, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, base64.RawURLEncoding.EncodeToString(ticket), string(encoded))
	decoded, ok := identitycontract.DecodeApplicationEnrollmentTicket(string(encoded))
	require.True(t, ok)
	require.Equal(t, ticket, decoded[:])
	require.NotContains(t, stdout.String(), base64.RawURLEncoding.EncodeToString(ticket))
	require.NotContains(t, stderr.String(), base64.RawURLEncoding.EncodeToString(ticket))
	require.Contains(t, stdout.String(), `"protected_output":"`)
}

func TestApplicationTicketFileCreateFailureAllowsSafeRetry(t *testing.T) {
	node, principal := principalForTest(t, 0x59), principalForTest(t, 0x5a)
	first := bytes.Repeat([]byte{0xb1}, identitycontract.ApplicationEnrollmentTicketBytes)
	second := bytes.Repeat([]byte{0xb2}, identitycontract.ApplicationEnrollmentTicketBytes)
	calls := 0
	service := &fakeIdentityClient{applicationTicket: func(*protocol.IssueApplicationEnrollmentTicketRequest) *protocol.IssueApplicationEnrollmentTicketResponse {
		calls++
		ticket := first
		if calls > 1 {
			ticket = second
		}
		return &protocol.IssueApplicationEnrollmentTicketResponse{
			ApplicationEnrollmentTicket: append([]byte(nil), ticket...),
			ExpiresAt:                   timestamppb.New(administrationNow.Add(identitycontract.ApplicationEnrollmentTicketLifetime)),
		}
	}}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	parent := filepath.Join(t.TempDir(), "blocked-parent")
	require.NoError(t, os.WriteFile(parent, []byte("not a directory"), 0o600))
	path := filepath.Join(parent, "application-enrollment-ticket")
	args := []string{"application-ticket", "issue", "--principal", principal, "--action", "application.content.get", "--out-file", path}

	require.Equal(t, 1, command.Run(context.Background(), args))
	require.Equal(t, 1, service.mutationCalls)
	require.NotContains(t, stdout.String(), base64.RawURLEncoding.EncodeToString(first))
	require.NotContains(t, stderr.String(), base64.RawURLEncoding.EncodeToString(first))

	require.NoError(t, os.Remove(parent))
	require.NoError(t, storage.EnsurePrivateDir(parent))
	require.Zero(t, command.Run(context.Background(), args), stderr.String())
	require.Equal(t, 2, service.mutationCalls)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, base64.RawURLEncoding.EncodeToString(second), string(raw))
	require.NotContains(t, stdout.String(), base64.RawURLEncoding.EncodeToString(second))
	require.NotContains(t, stderr.String(), base64.RawURLEncoding.EncodeToString(second))
}

func TestApplicationTicketRefusesExistingOutputWithoutOverwrite(t *testing.T) {
	node, principal := principalForTest(t, 0x53), principalForTest(t, 0x54)
	service := &fakeIdentityClient{applicationTicket: func(*protocol.IssueApplicationEnrollmentTicketRequest) *protocol.IssueApplicationEnrollmentTicketResponse {
		return &protocol.IssueApplicationEnrollmentTicketResponse{
			ApplicationEnrollmentTicket: bytes.Repeat([]byte{0xa8}, identitycontract.ApplicationEnrollmentTicketBytes),
			ExpiresAt:                   timestamppb.New(administrationNow.Add(identitycontract.ApplicationEnrollmentTicketLifetime)),
		}
	}}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	path := filepath.Join(t.TempDir(), "protected", "existing-application-enrollment-ticket")
	require.NoError(t, storage.AtomicCreatePrivateFile(path, []byte("existing-protected-content")))

	code := command.Run(context.Background(), []string{"application-ticket", "issue", "--principal", principal, "--action", "application.content.get", "--out-file", path})
	require.Equal(t, 1, code)
	require.Zero(t, service.mutationCalls)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "existing-protected-content", string(raw))
	require.Empty(t, stdout.String())
	require.NotContains(t, stderr.String(), base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xa8}, identitycontract.ApplicationEnrollmentTicketBytes)))
}

func TestGrantIssueIsCanonicalDeterministicAndNoninteractiveInJSON(t *testing.T) {
	node, subject := principalForTest(t, 1), principalForTest(t, 2)
	service := &fakeIdentityClient{}
	service.issue = func(request *protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		require.Equal(t, []string{"diagnostics.snapshot", "node.status"}, request.Proposal.Actions)
		require.Equal(t, subject, request.Proposal.SubjectPrincipalId)
		require.Equal(t, administrationNow, request.Proposal.NotBefore.AsTime())
		require.Equal(t, administrationNow.Add(defaultGrantTTL), request.Proposal.NotAfter.AsTime())
		require.NotEmpty(t, request.RequestId)
		return &protocol.IssueAccessGrantResponse{GrantId: artifactID(identitycontract.AccessGrantPrefix)}
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "node.status", "--action", "diagnostics.snapshot"})
	require.Zero(t, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), `"operation":"grant_issue"`)
	require.Contains(t, stdout.String(), `"target_node":"`+node+`"`)
	require.Contains(t, stdout.String(), `"actions":["diagnostics.snapshot","node.status"]`)
	require.Contains(t, stdout.String(), `"request_id":"r1_`)
	require.Empty(t, stderr.String())
}

func TestGrantIssueSupportsExactApplicationDiscoveryGrant(t *testing.T) {
	node, subject := principalForTest(t, 0x71), principalForTest(t, 0x72)
	service := &fakeIdentityClient{issue: func(request *protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		require.Equal(t, subject, request.Proposal.SubjectPrincipalId)
		require.Equal(t, []string{"application.discovery.resolve"}, request.Proposal.Actions)
		exact, ok := request.Proposal.Scope.Scope.(*identityprotocol.ResourceScope_Exact)
		require.True(t, ok)
		require.Equal(t, &identityprotocol.ResourceRef{
			Node: node, Kind: "service-type", CanonicalId: "echo",
		}, exact.Exact.Resource)
		return &protocol.IssueAccessGrantResponse{GrantId: artifactID(identitycontract.AccessGrantPrefix)}
	}}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)

	code := command.Run(context.Background(), []string{
		"grant", "issue",
		"--subject", subject,
		"--action", "application.discovery.resolve",
		"--scope", "exact",
		"--resource-kind", "service-type",
		"--resource-id", "echo",
	})

	require.Zero(t, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), `"resource_kind":"service-type"`)
	require.Contains(t, stdout.String(), `"resource_id":"echo"`)
	require.Empty(t, stderr.String())
}

func TestGrantIssueHumanConsentDenialDisplaysExactMutationAndMakesNoCall(t *testing.T) {
	node, subject := principalForTest(t, 3), principalForTest(t, 4)
	service := &fakeIdentityClient{issue: func(*protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		return &protocol.IssueAccessGrantResponse{}
	}}
	command, stdout, stderr := newAdministrationCommand(false, "no\n", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "node.status", "--valid-for", "1h"})
	require.Equal(t, 1, code)
	require.Zero(t, service.mutationCalls)
	require.Contains(t, stdout.String(), subject)
	require.Contains(t, stdout.String(), node)
	require.Contains(t, stdout.String(), "node.status")
	require.Contains(t, stdout.String(), administrationNow.Add(time.Hour).Format(time.RFC3339))
	require.Contains(t, stderr.String(), "explicit confirmation is required")
}

func TestGrantListRejectsUnknownActionAndCrossNodeScope(t *testing.T) {
	node, otherNode, subject := principalForTest(t, 5), principalForTest(t, 6), principalForTest(t, 7)
	validScope := &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}}
	tests := []struct {
		name  string
		grant *protocol.AccessGrantMetadata
	}{
		{"unknown action", &protocol.AccessGrantMetadata{Id: artifactID(identitycontract.AccessGrantPrefix), SubjectPrincipalId: subject, Actions: []string{"unknown.action"}, Scope: validScope, NotBefore: timestamp(administrationNow), NotAfter: timestamp(administrationNow.Add(time.Hour))}},
		{"cross node exact", &protocol.AccessGrantMetadata{Id: artifactID(identitycontract.AccessGrantPrefix), SubjectPrincipalId: subject, Actions: []string{"node.status"}, Scope: exactScope(otherNode, "node", ""), NotBefore: timestamp(administrationNow), NotAfter: timestamp(administrationNow.Add(time.Hour))}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeIdentityClient{list: func(*protocol.ListAccessGrantsRequest) *protocol.ListAccessGrantsResponse {
				return &protocol.ListAccessGrantsResponse{Grants: []*protocol.AccessGrantMetadata{test.grant}}
			}}
			command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
			require.Equal(t, 1, command.Run(context.Background(), []string{"grant", "list", "--subject", subject}))
			require.Empty(t, stdout.String())
			require.Contains(t, stderr.String(), "identity service returned an invalid response")
		})
	}
}

func TestGrantRevokeAcceptsApplicationEnrollmentGrantFromSupportedOperatorProcedure(t *testing.T) {
	node, subject := principalForTest(t, 0x61), principalForTest(t, 0x62)
	grantID := artifactID(identitycontract.AccessGrantPrefix)
	revocationID := artifactID(identitycontract.AccessGrantRevocationPrefix)
	service := &fakeIdentityClient{
		list: func(request *protocol.ListAccessGrantsRequest) *protocol.ListAccessGrantsResponse {
			require.Equal(t, subject, request.SubjectPrincipalId)
			return &protocol.ListAccessGrantsResponse{Grants: []*protocol.AccessGrantMetadata{{
				Id: grantID, SubjectPrincipalId: subject,
				Actions:   []string{"application.content.get", "application.content.put"},
				Scope:     &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}},
				NotBefore: timestamp(administrationNow), NotAfter: timestamp(administrationNow.Add(time.Hour)),
			}}}
		},
		revoke: func(request *protocol.RevokeAccessGrantRequest) *protocol.RevokeAccessGrantResponse {
			require.Equal(t, grantID, request.GrantId)
			return &protocol.RevokeAccessGrantResponse{RevocationId: revocationID}
		},
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)

	require.Zero(t, command.Run(context.Background(), []string{
		"grant", "revoke", "--subject", subject, "--grant-id", grantID, "--yes",
	}))
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), `"actions":["application.content.get","application.content.put"]`)
	require.Contains(t, stdout.String(), revocationID)
	require.Empty(t, stderr.String())
}

func TestDeviceRevokeRequiresCanonicalInputAndConsent(t *testing.T) {
	node, subject := principalForTest(t, 8), principalForTest(t, 9)
	devicePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{10}, ed25519.SeedSize))
	device, err := identityprincipal.DeviceFromEd25519PublicKey(devicePrivate.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	service := &fakeIdentityClient{revokeDevice: func(request *protocol.RevokeDeviceRequest) *protocol.RevokeDeviceResponse {
		require.Equal(t, subject, request.PrincipalId)
		require.Equal(t, device.String(), request.DeviceId)
		return &protocol.RevokeDeviceResponse{RevocationId: artifactID(identitycontract.DeviceRevocationPrefix)}
	}}
	command, stdout, stderr := newAdministrationCommand(false, "yes\n", &administrationSessions{node: node}, service)
	require.Zero(t, command.Run(context.Background(), []string{"device", "revoke", "--principal", subject, "--device-id", device.String()}))
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), subject)
	require.Contains(t, stdout.String(), node)
	require.Contains(t, stdout.String(), device.String())
	require.Contains(t, stdout.String(), artifactID(identitycontract.DeviceRevocationPrefix))
	require.Contains(t, stderr.String(), "Type yes")
}

func TestSubsequentEnrollmentUsesTypedProofAndProtectedRPC(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(directory))
	rootPath, devicePath := filepath.Join(directory, "root.json"), filepath.Join(directory, "device.json")
	principalInfo, err := CreatePrincipal(rootPath, bytes.NewReader(bytes.Repeat([]byte{11}, ed25519.SeedSize)))
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, time.Hour, administrationNow.Add(-time.Minute), bytes.NewReader(bytes.Repeat([]byte{12}, ed25519.SeedSize)))
	require.NoError(t, err)
	node := principalForTest(t, 13)
	challenge := identityaccess.Challenge{Version: identitycontract.Version, Principal: principalInfo.Principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, IssuedAt: administrationNow.Add(-time.Second), ExpiresAt: administrationNow.Add(identitycontract.ChallengeLifetime - time.Second), Binding: identityaccess.AuthenticationBinding{Audience: identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1}}
	copy(challenge.ID[:], bytes.Repeat([]byte{1}, len(challenge.ID)))
	copy(challenge.Nonce[:], bytes.Repeat([]byte{2}, len(challenge.Nonce)))
	copy(challenge.Binding.PeerBinding[:], bytes.Repeat([]byte{3}, len(challenge.Binding.PeerBinding)))
	fields, err := identityaccess.ChallengeFields(challenge)
	require.NoError(t, err)
	service := &fakeIdentityClient{}
	service.begin = func(request *protocol.BeginAuthenticationRequest) *protocol.BeginAuthenticationResponse {
		require.Equal(t, principalInfo.Principal, request.PrincipalId)
		require.Equal(t, identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, request.Purpose)
		return &protocol.BeginAuthenticationResponse{Challenge: fields}
	}
	service.complete = func(request *protocol.CompleteAuthenticationRequest) *protocol.CompleteAuthenticationResponse {
		require.Equal(t, principalInfo.Principal, request.PrincipalId)
		require.Len(t, request.Signature, ed25519.SignatureSize)
		require.Empty(t, request.Credential)
		return &protocol.CompleteAuthenticationResponse{EnrollmentProof: bytes.Repeat([]byte{4}, len(identityaccess.EnrollmentProof{}))}
	}
	service.enroll = func(request *protocol.EnrollPrincipalRequest) *protocol.EnrollPrincipalResponse {
		require.Equal(t, "request-1", request.RequestId)
		require.NotEmpty(t, request.Credential)
		require.Empty(t, request.ProtoReflect().GetUnknown())
		return &protocol.EnrollPrincipalResponse{PrincipalId: principalInfo.Principal}
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"enroll", "--root-signer-file", rootPath, "--device-signer-file", devicePath, "--request-id", "request-1"})
	require.Zero(t, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), principalInfo.Principal)
	require.Contains(t, stdout.String(), `"mode":"administrator"`)
	if strings.Contains(stdout.String(), base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{4}, len(identityaccess.EnrollmentProof{})))) {
		t.Fatal("output leaked enrollment proof")
	}
	require.Empty(t, stderr.String())
}

func TestBootstrapEnrollmentUsesTicketOnlyOnPublicEnrollmentRPC(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(directory))
	rootPath, devicePath, ticketPath := filepath.Join(directory, "root.json"), filepath.Join(directory, "device.json"), filepath.Join(directory, "ticket")
	principalInfo, err := CreatePrincipal(rootPath, bytes.NewReader(bytes.Repeat([]byte{21}, ed25519.SeedSize)))
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, time.Hour, administrationNow.Add(-time.Minute), bytes.NewReader(bytes.Repeat([]byte{22}, ed25519.SeedSize)))
	require.NoError(t, err)
	ticket := bytes.Repeat([]byte{0x63}, identitycontract.BootstrapTicketBytes)
	ticketText := base64.RawURLEncoding.EncodeToString(ticket)
	require.NoError(t, storage.AtomicCreatePrivateFile(ticketPath, append([]byte(ticketText), '\n')))
	node := principalForTest(t, 23)
	challenge := identityaccess.Challenge{Version: identitycontract.Version, Principal: principalInfo.Principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, IssuedAt: administrationNow.Add(-time.Second), ExpiresAt: administrationNow.Add(identitycontract.ChallengeLifetime - time.Second), Binding: identityaccess.AuthenticationBinding{Audience: identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1}}
	copy(challenge.ID[:], bytes.Repeat([]byte{1}, len(challenge.ID)))
	copy(challenge.Nonce[:], bytes.Repeat([]byte{2}, len(challenge.Nonce)))
	copy(challenge.Binding.PeerBinding[:], bytes.Repeat([]byte{3}, len(challenge.Binding.PeerBinding)))
	fields, err := identityaccess.ChallengeFields(challenge)
	require.NoError(t, err)
	service := &fakeIdentityClient{}
	service.begin = func(*protocol.BeginAuthenticationRequest) *protocol.BeginAuthenticationResponse {
		return &protocol.BeginAuthenticationResponse{Challenge: fields}
	}
	service.complete = func(*protocol.CompleteAuthenticationRequest) *protocol.CompleteAuthenticationResponse {
		return &protocol.CompleteAuthenticationResponse{EnrollmentProof: bytes.Repeat([]byte{4}, len(identityaccess.EnrollmentProof{}))}
	}
	service.enrollFirst = func(request *protocol.EnrollFirstPrincipalRequest) *protocol.EnrollFirstPrincipalResponse {
		require.Len(t, request.BootstrapTicket, identitycontract.BootstrapTicketBytes)
		require.NotEmpty(t, request.Credential)
		return &protocol.EnrollFirstPrincipalResponse{PrincipalId: principalInfo.Principal}
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"enroll", "--root-signer-file", rootPath, "--device-signer-file", devicePath, "--bootstrap-ticket-file", ticketPath})
	require.Zero(t, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), `"mode":"bootstrap"`)
	if strings.Contains(stdout.String(), ticketText) || strings.Contains(stderr.String(), ticketText) {
		t.Fatal("output leaked Bootstrap Ticket")
	}
	require.NoFileExists(t, ticketPath)
}

func TestBootstrapEnrollmentReportsRedactedCleanupFailureAfterSingleCommit(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(directory))
	rootPath, devicePath, ticketPath := filepath.Join(directory, "root.json"), filepath.Join(directory, "device.json"), filepath.Join(directory, "ticket")
	principalInfo, err := CreatePrincipal(rootPath, bytes.NewReader(bytes.Repeat([]byte{46}, ed25519.SeedSize)))
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, time.Hour, administrationNow.Add(-time.Minute), bytes.NewReader(bytes.Repeat([]byte{47}, ed25519.SeedSize)))
	require.NoError(t, err)
	ticketText := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x64}, identitycontract.BootstrapTicketBytes))
	require.NoError(t, storage.AtomicCreatePrivateFile(ticketPath, append([]byte(ticketText), '\n')))
	node := principalForTest(t, 48)
	challenge := identityaccess.Challenge{Version: identitycontract.Version, Principal: principalInfo.Principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, IssuedAt: administrationNow.Add(-time.Second), ExpiresAt: administrationNow.Add(identitycontract.ChallengeLifetime - time.Second), Binding: identityaccess.AuthenticationBinding{Audience: identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1}}
	copy(challenge.ID[:], bytes.Repeat([]byte{1}, len(challenge.ID)))
	copy(challenge.Nonce[:], bytes.Repeat([]byte{2}, len(challenge.Nonce)))
	copy(challenge.Binding.PeerBinding[:], bytes.Repeat([]byte{3}, len(challenge.Binding.PeerBinding)))
	fields, err := identityaccess.ChallengeFields(challenge)
	require.NoError(t, err)
	service := &fakeIdentityClient{
		begin: func(*protocol.BeginAuthenticationRequest) *protocol.BeginAuthenticationResponse {
			return &protocol.BeginAuthenticationResponse{Challenge: fields}
		},
		complete: func(*protocol.CompleteAuthenticationRequest) *protocol.CompleteAuthenticationResponse {
			return &protocol.CompleteAuthenticationResponse{EnrollmentProof: bytes.Repeat([]byte{4}, len(identityaccess.EnrollmentProof{}))}
		},
		enrollFirst: func(*protocol.EnrollFirstPrincipalRequest) *protocol.EnrollFirstPrincipalResponse {
			return &protocol.EnrollFirstPrincipalResponse{PrincipalId: principalInfo.Principal}
		},
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	command.removeTicket = func(path string) error {
		if path != ticketPath {
			t.Fatal("cleanup targeted the wrong file")
		}
		return errors.New("Principal enrolled but Bootstrap Ticket cleanup failed")
	}
	code := command.Run(context.Background(), []string{"enroll", "--root-signer-file", rootPath, "--device-signer-file", devicePath, "--bootstrap-ticket-file", ticketPath})
	require.Equal(t, 1, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Empty(t, stdout.String())
	require.FileExists(t, ticketPath)
	require.Contains(t, stderr.String(), "cleanup failed")
	if strings.Contains(stderr.String(), ticketText) || strings.Contains(stderr.String(), ticketPath) {
		t.Fatal("cleanup failure leaked Bootstrap Ticket material or path")
	}
}

func TestBootstrapTicketParsingIsCanonicalAndRedacted(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(directory))
	valid := bytes.Repeat([]byte{0x51}, identitycontract.BootstrapTicketBytes)
	path := filepath.Join(directory, "ticket")
	require.NoError(t, storage.AtomicCreatePrivateFile(path, append([]byte(base64.RawURLEncoding.EncodeToString(valid)), '\n')))
	parsed, err := readBootstrapTicket(path)
	require.NoError(t, err)
	require.Equal(t, sha256.Sum256(valid), sha256.Sum256(parsed))

	badPath := filepath.Join(directory, "bad-ticket")
	secret := base64.RawURLEncoding.EncodeToString(valid) + "="
	require.NoError(t, storage.AtomicCreatePrivateFile(badPath, []byte(secret)))
	_, err = readBootstrapTicket(badPath)
	require.ErrorIs(t, err, errInvalidBootstrapTicket)
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), badPath)
}

func TestEnrollmentResponseUnknownFieldsAndSessionMixFailClosed(t *testing.T) {
	proof := &protocol.CompleteAuthenticationResponse{EnrollmentProof: bytes.Repeat([]byte{1}, len(identityaccess.EnrollmentProof{})), SessionId: "mixed"}
	_, err := validateEnrollmentProof(proof)
	require.ErrorIs(t, err, errInvalidIdentityResponse)
	proof.SessionId = ""
	proof.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	_, err = validateEnrollmentProof(proof)
	require.ErrorIs(t, err, errInvalidIdentityResponse)
}

func TestEnrollmentChallengeRejectsCrossSurfaceExpiryAndUnknownFields(t *testing.T) {
	principal, node := principalForTest(t, 31), principalForTest(t, 32)
	challenge := identityaccess.Challenge{Version: identitycontract.Version, Principal: principal, Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, IssuedAt: administrationNow.Add(-time.Second), ExpiresAt: administrationNow.Add(identitycontract.ChallengeLifetime - time.Second), Binding: identityaccess.AuthenticationBinding{Audience: identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor}, TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1}}
	copy(challenge.ID[:], bytes.Repeat([]byte{1}, len(challenge.ID)))
	copy(challenge.Nonce[:], bytes.Repeat([]byte{2}, len(challenge.Nonce)))
	copy(challenge.Binding.PeerBinding[:], bytes.Repeat([]byte{3}, len(challenge.Binding.PeerBinding)))
	fields, err := identityaccess.ChallengeFields(challenge)
	require.NoError(t, err)

	tests := []struct {
		name     string
		response *protocol.BeginAuthenticationResponse
		node     string
		now      time.Time
	}{
		{"cross node", &protocol.BeginAuthenticationResponse{Challenge: proto.Clone(fields).(*identityprotocol.ChallengeFields)}, principalForTest(t, 33), administrationNow},
		{"expired", &protocol.BeginAuthenticationResponse{Challenge: proto.Clone(fields).(*identityprotocol.ChallengeFields)}, node, challenge.ExpiresAt},
		{"application interface", &protocol.BeginAuthenticationResponse{Challenge: proto.Clone(fields).(*identityprotocol.ChallengeFields)}, node, administrationNow},
		{"unknown field", &protocol.BeginAuthenticationResponse{Challenge: proto.Clone(fields).(*identityprotocol.ChallengeFields)}, node, administrationNow},
	}
	tests[2].response.Challenge.Binding.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
	tests[3].response.Challenge.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateEnrollmentChallenge(test.response, principal, test.node, test.now)
			require.ErrorIs(t, err, errInvalidIdentityResponse)
		})
	}
}

func TestArtifactIDAndExactResourceValidationAreCanonical(t *testing.T) {
	require.True(t, validArtifactID(artifactID(identitycontract.AccessGrantPrefix), identitycontract.AccessGrantPrefix))
	require.False(t, validArtifactID(identitycontract.AccessGrantPrefix+strings.Repeat("a", 52), identitycontract.AccessGrantPrefix))
	require.False(t, validArtifactID(identitycontract.AccessGrantPrefix+strings.Repeat("a", 51)+"b", identitycontract.AccessGrantPrefix))

	node, subject := principalForTest(t, 34), principalForTest(t, 35)
	service := &fakeIdentityClient{issue: func(*protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		return &protocol.IssueAccessGrantResponse{}
	}}
	command, _, _ := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	require.Equal(t, 2, command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "workload.status", "--scope", "exact", "--resource-kind", "workload", "--resource-id", "bad\nid"}))
	require.Zero(t, service.mutationCalls)
}

func TestGrantIssueRetriesOneAmbiguousFailureWithSameRequestID(t *testing.T) {
	node, subject := principalForTest(t, 36), principalForTest(t, 37)
	service := &fakeIdentityClient{issueErrors: []error{connect.NewError(connect.CodeUnavailable, errors.New("response lost"))}}
	service.issue = func(request *protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		return &protocol.IssueAccessGrantResponse{GrantId: artifactID(identitycontract.AccessGrantPrefix)}
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "node.status", "--request-id", "stable-request"})
	require.Zero(t, code)
	require.Equal(t, 2, service.mutationCalls)
	require.Equal(t, []string{"stable-request", "stable-request"}, service.issueRequests)
	require.Contains(t, stdout.String(), `"request_id":"stable-request"`)
	require.Empty(t, stderr.String())
}

func TestGrantIssueNeverRetriesPermissionDenied(t *testing.T) {
	node, subject := principalForTest(t, 38), principalForTest(t, 39)
	service := &fakeIdentityClient{issueErrors: []error{connect.NewError(connect.CodePermissionDenied, errors.New("denied"))}}
	service.issue = func(*protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		return &protocol.IssueAccessGrantResponse{GrantId: artifactID(identitycontract.AccessGrantPrefix)}
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "node.status", "--request-id", "denied-request"})
	require.Equal(t, 1, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "permission_denied")
	require.Contains(t, stderr.String(), "denied-request")
}

func TestGrantIssueDoesNotRetryPlainLocalFailure(t *testing.T) {
	node, subject := principalForTest(t, 40), principalForTest(t, 41)
	service := &fakeIdentityClient{issueErrors: []error{errors.New("local signer unavailable")}}
	service.issue = func(*protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		return &protocol.IssueAccessGrantResponse{GrantId: artifactID(identitycontract.AccessGrantPrefix)}
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "node.status", "--request-id", "local-failure"})
	require.Equal(t, 1, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "local-failure")
}

func TestGrantIssueDoubleAmbiguousFailurePreservesRequestID(t *testing.T) {
	node, subject := principalForTest(t, 42), principalForTest(t, 43)
	failure := connect.NewError(connect.CodeUnavailable, errors.New("response lost"))
	service := &fakeIdentityClient{issueErrors: []error{failure, failure}}
	service.issue = func(*protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		return &protocol.IssueAccessGrantResponse{GrantId: artifactID(identitycontract.AccessGrantPrefix)}
	}
	command, stdout, stderr := newAdministrationCommand(true, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "node.status", "--request-id", "recoverable-request"})
	require.Equal(t, 1, code)
	require.Equal(t, 2, service.mutationCalls)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "recoverable-request")
}

func TestGrantIssueHumanYesDisplaysRequestIDBeforeCall(t *testing.T) {
	node, subject := principalForTest(t, 44), principalForTest(t, 45)
	service := &fakeIdentityClient{issue: func(*protocol.IssueAccessGrantRequest) *protocol.IssueAccessGrantResponse {
		return &protocol.IssueAccessGrantResponse{GrantId: artifactID(identitycontract.AccessGrantPrefix)}
	}}
	command, stdout, stderr := newAdministrationCommand(false, "", &administrationSessions{node: node}, service)
	code := command.Run(context.Background(), []string{"grant", "issue", "--subject", subject, "--action", "node.status", "--request-id", "yes-request", "--yes"})
	require.Zero(t, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), "request_id: yes-request")
	require.Contains(t, stdout.String(), "result_id:")
	require.Empty(t, stderr.String())
}

func timestamp(value time.Time) *timestamppb.Timestamp { return timestamppb.New(value) }

func exactScope(node, kind, owner string) *identityprotocol.ResourceScope {
	return &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Exact{Exact: &identityprotocol.ExactScope{Resource: &identityprotocol.ResourceRef{Node: node, Kind: kind, Owner: owner, CanonicalId: "id"}}}}
}
