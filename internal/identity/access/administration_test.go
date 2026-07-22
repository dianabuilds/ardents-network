package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"sync/atomic"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type updateGateDatabase struct {
	storage.Database
	armed   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func (d *updateGateDatabase) Update(ctx context.Context, callback func(storage.WriteTransaction) error) error {
	if d.armed.CompareAndSwap(true, false) {
		close(d.entered)
		select {
		case <-d.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return d.Database.Update(ctx, callback)
}

type adminFixture struct {
	*serviceFixture
	secret       SessionSecret
	initialGrant string
}

func (f *adminFixture) enrollmentRequest(requestID string, root, device ed25519.PrivateKey) EnrollPrincipalRequest {
	f.t.Helper()
	principal, err := identityprincipal.FromEd25519PublicKey(root.Public().(ed25519.PublicKey))
	require.NoError(f.t, err)
	deviceID, err := identityprincipal.DeviceFromEd25519PublicKey(device.Public().(ed25519.PublicKey))
	require.NoError(f.t, err)
	challenge, err := f.service.Begin(f.ctx, BeginRequest{Principal: principal.String(), Purpose: identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF, Binding: f.binding, Source: f.source})
	require.NoError(f.t, err)
	signed, err := challengeSigningBytes(challenge)
	require.NoError(f.t, err)
	var rootPublic [32]byte
	copy(rootPublic[:], root.Public().(ed25519.PublicKey))
	proof, err := f.service.Complete(f.ctx, CompleteRequest{ChallengeID: challenge.ID, Principal: principal.String(), Binding: f.binding, Source: f.source, RootPublicKey: rootPublic, Signature: ed25519.Sign(root, signed)})
	require.NoError(f.t, err)
	credential, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{Version: 1, Subject: principal.String(), RootPublicKey: root.Public().(ed25519.PublicKey), DeviceId: deviceID.String(), DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(f.clock.Now().Add(-time.Minute)), NotAfter: timestamppb.New(f.clock.Now().Add(time.Hour))}, root)
	require.NoError(f.t, err)
	raw, err := credential.MarshalBinary()
	require.NoError(f.t, err)
	return EnrollPrincipalRequest{Command: f.command(requestID, "identity.principal.enroll", "principal", principal.String()), Challenge: challenge, Proof: *proof.EnrollmentProof, RootPublicKey: rootPublic, Credential: raw}
}

func newAdminFixture(t *testing.T) *adminFixture {
	f := newServiceFixture(t)
	f.service.bootstrapEnabled = true
	f.service.grantIssuer = testAccessGrantIssuer{key: f.node}
	ticket, err := f.service.IssueBootstrapTicket(f.ctx, f.nodeID)
	require.NoError(t, err)
	result, err := f.service.EnrollFirstPrincipal(f.ctx, f.binding, f.firstEnrollmentRequest(ticket))
	require.NoError(t, err)
	return &adminFixture{serviceFixture: f, secret: f.sessionSecret(), initialGrant: result.GrantID}
}

func (f *adminFixture) command(requestID, action, kind, id string) AdminCommand {
	f.t.Helper()
	resource, err := NewResourceRef(f.nodeID, "", kind, id)
	require.NoError(f.t, err)
	return AdminCommand{RequestID: requestID, Attempt: Attempt{SessionSecret: f.secret, Binding: f.binding, Action: Action(action), Resource: resource}}
}

func (f *adminFixture) issue(requestID string, actions []string) (string, error) {
	f.t.Helper()
	domainActions := make([]Action, len(actions))
	for index := range actions {
		domainActions[index] = Action(actions[index])
	}
	proposal := GrantProposal{Subject: f.principal, Actions: domainActions, Scope: ResourceScope{Kind: ScopeNode, Exact: ResourceRef{Node: f.nodeID}}, NotBefore: f.clock.Now(), NotAfter: f.clock.Now().Add(time.Hour)}
	payload, err := grantProposalPayload(f.nodeID, f.binding.Audience, proposal)
	require.NoError(f.t, err)
	proposalID, err := grantProposalID(payload)
	require.NoError(f.t, err)
	return f.service.IssueAccessGrant(f.ctx, IssueGrantRequest{Command: f.command(requestID, "identity.grant.issue", "grant-proposal", proposalID), Proposal: proposal})
}

func TestIssueAccessGrantIsAuthorizedAtomicAndIdempotent(t *testing.T) {
	f := newAdminFixture(t)
	id, err := f.issue("issue-1", []string{"node.status"})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	replayed, err := f.issue("issue-1", []string{"node.status"})
	require.NoError(t, err)
	require.Equal(t, id, replayed)
	_, err = f.issue("issue-1", []string{"node.start"})
	require.ErrorIs(t, err, ErrConflict)

	var grant *Artifact
	require.NoError(t, f.database.View(f.ctx, func(tx storage.ReadTransaction) error {
		var loadErr error
		grant, loadErr = loadGrant(tx, id, f.clock.Now())
		return loadErr
	}))
	require.Equal(t, []string{"node.status"}, grant.AccessGrantPayload().Actions)
}

func TestIssueAccessGrantRejectsProposalSubstitution(t *testing.T) {
	f := newAdminFixture(t)
	proposal := GrantProposal{Subject: f.principal, Actions: []Action{"node.status"}, Scope: ResourceScope{Kind: ScopeNode, Exact: ResourceRef{Node: f.nodeID}}, NotBefore: f.clock.Now(), NotAfter: f.clock.Now().Add(time.Hour)}
	payload, err := grantProposalPayload(f.nodeID, f.binding.Audience, proposal)
	require.NoError(t, err)
	id, err := grantProposalID(payload)
	require.NoError(t, err)
	proposal.Actions = []Action{"node.start"}
	_, err = f.service.IssueAccessGrant(f.ctx, IssueGrantRequest{Command: f.command("substitute", "identity.grant.issue", "grant-proposal", id), Proposal: proposal})
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestAdminCommandCorruptResultFailsClosed(t *testing.T) {
	f := newAdminFixture(t)
	_, err := f.issue("corrupt-result", []string{"node.status"})
	require.NoError(t, err)
	key := adminCommandKey(f.nodeID, f.principal, "identity.grant.issue", "corrupt-result")
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		record, found, readErr := tx.Get(adminCommandsBucket, key)
		if readErr != nil || !found {
			return readErr
		}
		record[len(record)-sha256.Size-1] ^= 0xff
		return tx.Put(adminCommandsBucket, key, record)
	}))
	_, err = f.issue("corrupt-result", []string{"node.status"})
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestEnrollPrincipalIsProofBoundAndIdempotent(t *testing.T) {
	f := newAdminFixture(t)
	bobRoot := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xb1}, 32))
	bobDevice := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xb2}, 32))
	request := f.enrollmentRequest("enroll-bob", bobRoot, bobDevice)
	substituted := request
	substituted.Challenge.Binding.PeerBinding[0] ^= 0xff
	_, err := f.service.EnrollPrincipal(f.ctx, substituted)
	require.ErrorIs(t, err, ErrInvalidArgument)
	principal, err := f.service.EnrollPrincipal(f.ctx, request)
	require.NoError(t, err)
	require.Equal(t, request.Challenge.Principal, principal)
	replayed, err := f.service.EnrollPrincipal(f.ctx, request)
	require.NoError(t, err)
	require.Equal(t, principal, replayed)
	loaded, err := f.service.enrollments.load(f.ctx, f.nodeID, principal)
	require.NoError(t, err)
	require.Equal(t, principal, loaded.Principal)
}

func TestAdministrationListsAreSubjectFilteredAndNonSecret(t *testing.T) {
	f := newAdminFixture(t)
	grantResource, err := NewResourceRef(f.nodeID, "", "grant-collection", f.principal)
	require.NoError(t, err)
	grants, err := f.service.ListAccessGrants(f.ctx, Attempt{SessionSecret: f.secret, Binding: f.binding, Action: "identity.grant.list", Resource: grantResource}, f.principal)
	require.NoError(t, err)
	require.NotEmpty(t, grants)
	require.Equal(t, f.principal, grants[0].Subject)
	_, err = f.service.ListAccessGrants(f.ctx, Attempt{SessionSecret: f.secret, Binding: f.binding, Action: "identity.grant.list", Resource: grantResource}, f.nodeID)
	require.ErrorIs(t, err, ErrInvalidArgument)

	deviceResource, err := NewResourceRef(f.nodeID, "", "device-revocation-collection", f.principal)
	require.NoError(t, err)
	revocations, err := f.service.ListDeviceRevocations(f.ctx, Attempt{SessionSecret: f.secret, Binding: f.binding, Action: "identity.device-revocations.list", Resource: deviceResource}, f.principal)
	require.NoError(t, err)
	require.Empty(t, revocations)
}

func TestGrantRevocationPreservesLastRecoveryPath(t *testing.T) {
	f := newAdminFixture(t)
	command := f.command("revoke-initial", "identity.grant.revoke", "access-grant", f.initialGrant)
	_, err := f.service.RevokeAccessGrant(f.ctx, RevokeGrantRequest{Command: command, GrantID: f.initialGrant})
	require.ErrorIs(t, err, ErrConflict)

	replacement, err := f.issue("replacement", initialOperatorRecoveryActions)
	require.NoError(t, err)
	require.NotEqual(t, f.initialGrant, replacement)
	revocationID, err := f.service.RevokeAccessGrant(f.ctx, RevokeGrantRequest{Command: command, GrantID: f.initialGrant})
	require.NoError(t, err)
	require.NotEmpty(t, revocationID)
	f.clock.Advance(time.Second)
	replayed, err := f.service.RevokeAccessGrant(f.ctx, RevokeGrantRequest{Command: command, GrantID: f.initialGrant})
	require.NoError(t, err)
	require.Equal(t, revocationID, replayed)
}

func TestGrantRevocationRecapturesClockAfterWriterWait(t *testing.T) {
	f := newAdminFixture(t)
	actions := make([]Action, len(initialOperatorRecoveryActions))
	for index := range initialOperatorRecoveryActions {
		actions[index] = Action(initialOperatorRecoveryActions[index])
	}
	proposal := GrantProposal{Subject: f.principal, Actions: actions, Scope: ResourceScope{Kind: ScopeNode, Exact: ResourceRef{Node: f.nodeID}}, NotBefore: f.clock.Now(), NotAfter: f.clock.Now().Add(time.Second)}
	payload, err := grantProposalPayload(f.nodeID, f.binding.Audience, proposal)
	require.NoError(t, err)
	proposalID, err := grantProposalID(payload)
	require.NoError(t, err)
	_, err = f.service.IssueAccessGrant(f.ctx, IssueGrantRequest{Command: f.command("short-recovery", "identity.grant.issue", "grant-proposal", proposalID), Proposal: proposal})
	require.NoError(t, err)

	gate := &updateGateDatabase{Database: f.database, entered: make(chan struct{}), release: make(chan struct{})}
	gate.armed.Store(true)
	f.service.grants.database = gate
	command := f.command("stale-guard", "identity.grant.revoke", "access-grant", f.initialGrant)
	result := make(chan error, 1)
	go func() {
		_, revokeErr := f.service.RevokeAccessGrant(context.Background(), RevokeGrantRequest{Command: command, GrantID: f.initialGrant})
		result <- revokeErr
	}()
	<-gate.entered
	f.clock.Advance(3 * time.Minute)
	close(gate.release)
	require.ErrorIs(t, <-result, ErrConflict)
}

func TestAdminMutationRechecksAuthorityAfterRequestParsing(t *testing.T) {
	f := newAdminFixture(t)
	proposal := GrantProposal{Subject: f.principal, Actions: []Action{"node.status"}, Scope: ResourceScope{Kind: ScopeNode, Exact: ResourceRef{Node: f.nodeID}}, NotBefore: f.clock.Now(), NotAfter: f.clock.Now().Add(time.Hour)}
	payload, err := grantProposalPayload(f.nodeID, f.binding.Audience, proposal)
	require.NoError(t, err)
	proposalID, err := grantProposalID(payload)
	require.NoError(t, err)
	request := IssueGrantRequest{Command: f.command("authority-lost", "identity.grant.issue", "grant-proposal", proposalID), Proposal: proposal}
	gate := &updateGateDatabase{Database: f.database, entered: make(chan struct{}), release: make(chan struct{})}
	gate.armed.Store(true)
	f.service.grants.database = gate
	result := make(chan error, 1)
	go func() {
		_, issueErr := f.service.IssueAccessGrant(context.Background(), request)
		result <- issueErr
	}()
	<-gate.entered
	var initial *Artifact
	require.NoError(t, f.database.View(f.ctx, func(tx storage.ReadTransaction) error {
		var loadErr error
		initial, loadErr = loadGrant(tx, f.initialGrant, time.Time{})
		return loadErr
	}))
	revocation, err := SignAccessGrantRevocation(&identityprotocol.AccessGrantRevocationPayload{Version: 1, TargetId: initial.ID(), Issuer: f.nodeID, Audience: protocolAudience(f.binding.Audience), RevokedAt: timestamppb.New(f.clock.Now())}, f.node, f.clock.Now(), initial)
	require.NoError(t, err)
	require.NoError(t, f.service.grants.recordRevocation(f.ctx, revocation, f.node.Public().(ed25519.PublicKey), f.clock.Now()))
	close(gate.release)
	require.ErrorIs(t, <-result, ErrPermissionDenied)
}

func TestDeviceRevocationRequiresAnotherEnrolledRecoveryDevice(t *testing.T) {
	f := newAdminFixture(t)
	deviceResourceID, err := DeviceResourceID(f.principal, f.deviceID)
	require.NoError(t, err)
	command := f.command("revoke-device", "identity.device.revoke", "device", deviceResourceID)
	request := RevokeDeviceRequest{Command: command, Subject: f.principal, DeviceID: f.deviceID}
	_, err = f.service.RevokeDevice(f.ctx, request)
	require.ErrorIs(t, err, ErrConflict)

	secondDevice := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa2}, 32))
	secondID, err := identityprincipal.DeviceFromEd25519PublicKey(secondDevice.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	credential, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{Version: 1, Subject: f.principal, RootPublicKey: f.root.Public().(ed25519.PublicKey), DeviceId: secondID.String(), DevicePublicKey: secondDevice.Public().(ed25519.PublicKey), Purposes: []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE}, NotBefore: timestamppb.New(f.clock.Now().Add(-time.Minute)), NotAfter: timestamppb.New(f.clock.Now().Add(time.Hour))}, f.root)
	require.NoError(t, err)
	key, raw, err := prepareEnrollmentCredential(f.nodeID, f.principal, credential, f.clock.Now())
	require.NoError(t, err)
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error { return recordEnrollmentCredential(tx, key, raw) }))

	id, err := f.service.RevokeDevice(f.ctx, request)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	_, err = f.service.AuthenticateSession(f.ctx, f.secret, f.binding)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestDeviceRevocationReplayAfterClockAdvanceAndFilteredList(t *testing.T) {
	f := newAdminFixture(t)
	bobRoot := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xc1}, 32))
	bobDevice := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xc2}, 32))
	enrollment := f.enrollmentRequest("enroll-bob-device", bobRoot, bobDevice)
	bob, err := f.service.EnrollPrincipal(f.ctx, enrollment)
	require.NoError(t, err)
	bobDeviceID, err := identityprincipal.DeviceFromEd25519PublicKey(bobDevice.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	resourceID, err := DeviceResourceID(bob, bobDeviceID.String())
	require.NoError(t, err)
	command := f.command("revoke-bob-device", "identity.device.revoke", "device", resourceID)
	request := RevokeDeviceRequest{Command: command, Subject: bob, DeviceID: bobDeviceID.String()}
	id, err := f.service.RevokeDevice(f.ctx, request)
	require.NoError(t, err)
	f.clock.Advance(time.Second)
	replayed, err := f.service.RevokeDevice(f.ctx, request)
	require.NoError(t, err)
	require.Equal(t, id, replayed)

	resource, err := NewResourceRef(f.nodeID, "", "device-revocation-collection", bob)
	require.NoError(t, err)
	items, err := f.service.ListDeviceRevocations(f.ctx, Attempt{SessionSecret: f.secret, Binding: f.binding, Action: "identity.device-revocations.list", Resource: resource}, bob)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, bobDeviceID.String(), items[0].DeviceID)
}

func TestAdministrationDenialEmitsRedactedAudit(t *testing.T) {
	f := newAdminFixture(t)
	events := []AuditEvent{}
	f.service.audit = AuditSinkFunc(func(event AuditEvent) { events = append(events, event) })
	_, err := f.service.IssueAccessGrant(f.ctx, IssueGrantRequest{Command: f.command("bad", "identity.grant.issue", "grant-proposal", "not-the-proposal"), Proposal: GrantProposal{Subject: f.principal}})
	require.Error(t, err)
	require.NotEmpty(t, events)
	require.Equal(t, "denied", events[len(events)-1].Outcome)
	require.Contains(t, events[len(events)-1].Reason, "admin_")
}
