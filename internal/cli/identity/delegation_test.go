package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
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

	"github.com/stretchr/testify/require"
)

type delegationSessions struct{ node string }

func (s delegationSessions) Login(context.Context) (client.SessionKey, error) {
	return client.SessionKey{}, nil
}
func (s delegationSessions) SessionStatus() client.SessionKey { return client.SessionKey{} }
func (s delegationSessions) Logout() error                    { return nil }
func (s delegationSessions) PublicIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	return nil, nil
}
func (s delegationSessions) ProtectedIdentityService() (ardentsv1connect.IdentityServiceClient, error) {
	return nil, nil
}
func (s delegationSessions) TargetNodePrincipal() string { return s.node }

func TestDelegationIssueDisplaysExactConsentAndWritesOnlyProtectedArtifact(t *testing.T) {
	now := time.Date(2034, 5, 6, 7, 8, 9, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")
	root, err := CreatePrincipal(rootPath, rand.Reader)
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, 24*time.Hour, now, rand.Reader)
	require.NoError(t, err)
	node := delegationTestPrincipal(t)
	application := delegationTestPrincipal(t)
	outputPath := filepath.Join(dir, "consent.delegation")
	var stdout, stderr bytes.Buffer
	command := NewOnline(output.NewRenderer(&stdout, &stderr, false), delegationSessions{node: node}, time.Second, strings.NewReader("yes\n"))
	command.now = func() time.Time { return now }

	code := command.Run(context.Background(), []string{
		"delegation", "issue", "--application", application,
		"--action", "application.content.put", "--action", "application.content.get",
		"--scope", "principal-owned", "--valid-for", "15m",
		"--signer-file", devicePath, "--out-file", outputPath,
	})
	require.Zero(t, code)
	require.Equal(t, "Type yes to sign this non-redelegable Delegation: ", stderr.String())
	for _, expected := range []string{root.Principal, application, node, "application.content.get,application.content.put", "principal-owned", now.Format(time.RFC3339), now.Add(15 * time.Minute).Format(time.RFC3339), "redelegation: forbidden"} {
		require.Contains(t, stdout.String(), expected)
	}

	raw, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.NotContains(t, stdout.String(), string(raw))
	require.NotContains(t, stderr.String(), string(raw))
	artifact, err := identityaccess.ParseAndVerifyDelegation(raw, now)
	require.NoError(t, err)
	payload := artifact.DelegationPayload()
	require.Equal(t, root.Principal, payload.Delegator)
	require.Equal(t, application, payload.Delegatee)
	require.Equal(t, node, payload.Audience.Node)
	require.Equal(t, identityprotocol.Interface_INTERFACE_APPLICATION, payload.Audience.Interface)
	require.EqualValues(t, identitycontract.ProtocolMajor, payload.Audience.ProtocolMajor)
	require.Equal(t, []string{"application.content.get", "application.content.put"}, payload.Actions)
	require.Equal(t, root.Principal, payload.Scope.GetPrincipalOwned().GetOwner())
}

func TestDelegationIssueRequiresExplicitJSONConfirmationAndNeverOverwrites(t *testing.T) {
	now := time.Date(2034, 5, 6, 7, 8, 9, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")
	_, err := CreatePrincipal(rootPath, rand.Reader)
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, 24*time.Hour, now, rand.Reader)
	require.NoError(t, err)
	outputPath := filepath.Join(dir, "consent.delegation")
	args := []string{"delegation", "issue", "--application", delegationTestPrincipal(t), "--action", "application.content.get", "--scope", "principal-owned", "--signer-file", devicePath, "--out-file", outputPath}
	var stdout, stderr bytes.Buffer
	command := NewOnline(output.NewRenderer(&stdout, &stderr, true), delegationSessions{node: delegationTestPrincipal(t)}, time.Second, nil)
	command.now = func() time.Time { return now }
	require.Equal(t, 1, command.Run(context.Background(), args))
	require.NoFileExists(t, outputPath)
	require.NotContains(t, stderr.String(), "private")

	require.NoError(t, os.WriteFile(outputPath, []byte("preserve"), 0o600))
	stdout.Reset()
	stderr.Reset()
	require.Equal(t, 1, command.Run(context.Background(), append(args, "--yes")))
	raw, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, []byte("preserve"), raw)
}

func TestDelegationIssueRejectsUnknownActionScopeAndExcessLifetime(t *testing.T) {
	command := NewOnline(output.NewRenderer(new(bytes.Buffer), new(bytes.Buffer), true), delegationSessions{node: delegationTestPrincipal(t)}, time.Second, nil)
	base := []string{"delegation", "issue", "--application", delegationTestPrincipal(t), "--out-file", filepath.Join(t.TempDir(), "delegation"), "--signer-file", filepath.Join(t.TempDir(), "missing"), "--yes"}
	for _, extra := range [][]string{
		{"--action", "application.content.delete", "--scope", "principal-owned"},
		{"--action", "application.content.get", "--scope", "node"},
		{"--action", "application.content.get", "--scope", "principal-owned", "--valid-for", "24h1s"},
	} {
		require.NotZero(t, command.Run(context.Background(), append(append([]string(nil), base...), extra...)))
	}
}

func TestDelegationRevokeSignsExactTargetAndImportsToSelectedNode(t *testing.T) {
	now := time.Date(2034, 5, 6, 7, 8, 9, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")
	delegationPath := filepath.Join(dir, "consent.delegation")
	revocationPath := filepath.Join(dir, "consent.revocation")
	root, err := CreatePrincipal(rootPath, rand.Reader)
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, 24*time.Hour, now, rand.Reader)
	require.NoError(t, err)
	node, application := delegationTestPrincipal(t), delegationTestPrincipal(t)
	signer, err := OpenDeviceFileSigner(devicePath)
	require.NoError(t, err)
	delegation, err := signer.SignDelegation(context.Background(), DelegationSpec{
		Delegatee: application,
		Audience:  identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: identitycontract.ProtocolMajor},
		Actions:   []identityaccess.Action{"application.content.get"},
		Scope:     identityaccess.ResourceScope{Kind: identityaccess.ScopePrincipalOwned, Owner: mustCLIResourceOwner(t, root.Principal)},
		NotBefore: now, NotAfter: now.Add(time.Hour),
	}, now)
	require.NoError(t, err)
	delegationRaw, err := delegation.MarshalBinary()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(delegationPath, delegationRaw, 0o600))

	var stdout, stderr bytes.Buffer
	command := New(output.NewRenderer(&stdout, &stderr, true))
	command.now = func() time.Time { return now.Add(time.Minute) }
	code := command.Run(context.Background(), []string{"delegation", "revoke", "--delegation-file", delegationPath, "--signer-file", devicePath, "--out-file", revocationPath, "--yes"})
	require.Zero(t, code)
	revocationRaw, err := os.ReadFile(revocationPath)
	require.NoError(t, err)
	require.NotContains(t, stdout.String(), string(revocationRaw))
	require.NotContains(t, stderr.String(), string(revocationRaw))
	revocation, err := identityaccess.ParseAndVerifyDelegationRevocation(revocationRaw, now.Add(time.Minute))
	require.NoError(t, err)
	payload := revocation.DelegationRevocationPayload()
	require.Equal(t, delegation.ID(), payload.TargetId)
	require.Equal(t, root.Principal, payload.Delegator)
	require.Equal(t, application, payload.Delegatee)
	require.Equal(t, node, payload.Audience.Node)

	service := &fakeIdentityClient{importDelegation: func(request *protocol.ImportDelegationRevocationRequest) *protocol.ImportDelegationRevocationResponse {
		require.Equal(t, revocationRaw, request.Revocation)
		return &protocol.ImportDelegationRevocationResponse{RevocationId: revocation.ID()}
	}}
	sessions := &administrationSessions{node: node, public: service}
	stdout.Reset()
	stderr.Reset()
	command = NewOnline(output.NewRenderer(&stdout, &stderr, true), sessions, time.Second, nil)
	command.now = func() time.Time { return now.Add(time.Minute) }
	code = command.Run(context.Background(), []string{"delegation", "import-revocation", "--revocation-file", revocationPath})
	require.Zero(t, code)
	require.Equal(t, 1, service.mutationCalls)
	require.Contains(t, stdout.String(), revocation.ID())
	require.NotContains(t, stdout.String(), string(revocationRaw))
}

func TestDelegationRevocationRejectsWrongSignerCrossNodeAndOversizedInput(t *testing.T) {
	now := time.Date(2034, 5, 6, 7, 8, 9, 0, time.UTC)
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath, devicePath := filepath.Join(dir, "root.json"), filepath.Join(dir, "device.json")
	_, err := CreatePrincipal(rootPath, rand.Reader)
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, 24*time.Hour, now, rand.Reader)
	require.NoError(t, err)
	otherRoot, otherDevice := filepath.Join(dir, "other-root.json"), filepath.Join(dir, "other-device.json")
	_, err = CreatePrincipal(otherRoot, rand.Reader)
	require.NoError(t, err)
	_, err = CreateDevice(otherRoot, otherDevice, 24*time.Hour, now, rand.Reader)
	require.NoError(t, err)
	node, application := delegationTestPrincipal(t), delegationTestPrincipal(t)
	signer, err := OpenDeviceFileSigner(devicePath)
	require.NoError(t, err)
	delegator, err := signer.Principal(context.Background())
	require.NoError(t, err)
	delegation, err := signer.SignDelegation(context.Background(), DelegationSpec{
		Delegatee: application, Audience: identityaccess.Audience{Node: node, Interface: identityprotocol.Interface_INTERFACE_APPLICATION, ProtocolMajor: 1},
		Actions: []identityaccess.Action{"application.content.get"}, Scope: identityaccess.ResourceScope{Kind: identityaccess.ScopePrincipalOwned, Owner: mustCLIResourceOwner(t, delegator)}, NotBefore: now, NotAfter: now.Add(time.Hour),
	}, now)
	require.NoError(t, err)
	delegationRaw, err := delegation.MarshalBinary()
	require.NoError(t, err)
	delegationPath, revocationPath := filepath.Join(dir, "delegation"), filepath.Join(dir, "revocation")
	require.NoError(t, os.WriteFile(delegationPath, delegationRaw, 0o600))
	command := New(output.NewRenderer(new(bytes.Buffer), new(bytes.Buffer), true))
	command.now = func() time.Time { return now }
	require.NotZero(t, command.Run(context.Background(), []string{"delegation", "revoke", "--delegation-file", delegationPath, "--signer-file", otherDevice, "--out-file", revocationPath, "--yes"}))
	require.NoFileExists(t, revocationPath)

	revocation, err := signer.SignDelegationRevocation(context.Background(), delegation, now)
	require.NoError(t, err)
	revocationRaw, err := revocation.MarshalBinary()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(revocationPath, revocationRaw, 0o600))
	service := &fakeIdentityClient{importDelegation: func(*protocol.ImportDelegationRevocationRequest) *protocol.ImportDelegationRevocationResponse {
		t.Fatal("cross-Node revocation reached the service")
		return nil
	}}
	crossNode := &administrationSessions{node: delegationTestPrincipal(t), public: service}
	command = NewOnline(output.NewRenderer(new(bytes.Buffer), new(bytes.Buffer), true), crossNode, time.Second, nil)
	command.now = func() time.Time { return now }
	require.NotZero(t, command.Run(context.Background(), []string{"delegation", "import-revocation", "--revocation-file", revocationPath}))
	require.Zero(t, service.mutationCalls)

	oversized := filepath.Join(dir, "oversized")
	require.NoError(t, os.WriteFile(oversized, bytes.Repeat([]byte("x"), identitycontract.MaxArtifactBytes+1), 0o600))
	var stderr bytes.Buffer
	command = NewOnline(output.NewRenderer(new(bytes.Buffer), &stderr, true), &administrationSessions{node: node, public: service}, time.Second, nil)
	require.NotZero(t, command.Run(context.Background(), []string{"delegation", "import-revocation", "--revocation-file", oversized}))
	require.NotContains(t, stderr.String(), strings.Repeat("x", 128))
}

func delegationTestPrincipal(t *testing.T) string {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	return principal.String()
}

func mustCLIResourceOwner(t *testing.T, value string) identityaccess.ResourceOwner {
	t.Helper()
	owner, err := identityaccess.ParseResourceOwner(value)
	require.NoError(t, err)
	return owner
}

var _ DelegationSigner = (*DeviceFileSigner)(nil)
var _ DelegationRevocationSigner = (*DeviceFileSigner)(nil)
