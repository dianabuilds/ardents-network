package access

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type countingViewDatabase struct {
	storage.Database
	views atomic.Int32
}

func (d *countingViewDatabase) View(ctx context.Context, callback func(storage.ReadTransaction) error) error {
	d.views.Add(1)
	return d.Database.View(ctx, callback)
}

func (f *serviceFixture) sessionSecret() SessionSecret {
	f.t.Helper()
	result, err := f.service.Complete(f.ctx, f.sessionRequest(f.begin(identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)))
	require.NoError(f.t, err)
	require.NotNil(f.t, result.SessionSecret)
	return *result.SessionSecret
}

func (f *serviceFixture) grant(actions []string, scope *identityprotocol.ResourceScope) *Artifact {
	return f.grantFor(f.principal, actions, scope)
}

func (f *serviceFixture) grantFor(subject string, actions []string, scope *identityprotocol.ResourceScope) *Artifact {
	f.t.Helper()
	grant, err := SignAccessGrant(&identityprotocol.AccessGrantPayload{
		Version:   1,
		Issuer:    f.nodeID,
		Subject:   subject,
		Audience:  protocolAudience(f.binding.Audience),
		Actions:   actions,
		Scope:     scope,
		NotBefore: timestamppb.New(f.clock.Now().Add(-time.Minute)),
		NotAfter:  timestamppb.New(f.clock.Now().Add(time.Hour)),
	}, f.node)
	require.NoError(f.t, err)
	require.NoError(f.t, f.service.grants.recordGrant(f.ctx, grant, f.node.Public().(ed25519.PublicKey), f.clock.Now()))
	return grant
}

func nodeScope() *identityprotocol.ResourceScope {
	return &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Node{Node: &identityprotocol.NodeScope{}}}
}

func ownedScope(owner string) *identityprotocol.ResourceScope {
	return &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_PrincipalOwned{PrincipalOwned: &identityprotocol.PrincipalOwnedScope{Owner: owner}}}
}

func exactScope(resource ResourceRef) *identityprotocol.ResourceScope {
	return &identityprotocol.ResourceScope{Scope: &identityprotocol.ResourceScope_Exact{Exact: &identityprotocol.ExactScope{Resource: &identityprotocol.ResourceRef{Node: resource.Node, Owner: resource.Owner.String(), Kind: string(resource.Kind), CanonicalId: resource.ID}}}}
}

func TestAdmitDirectGrantReturnsSealedActorEffectiveFacts(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	f.grant([]string{"node.status"}, nodeScope())
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)

	call, err := f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource})
	require.NoError(t, err)
	require.Equal(t, f.principal, call.Actor())
	require.Equal(t, call.Actor(), call.Effective())
	require.Equal(t, f.binding.Audience, call.Audience())
	require.Equal(t, Action("node.status"), call.Action())
	require.Equal(t, resource, call.Resource())
	require.NotEmpty(t, call.SessionID())
	require.True(t, call.IsAdmitted())
	require.False(t, (AuthorizedCall{}).IsAdmitted())
}

func TestAuditRecordsDenialAndSuccessfulMutationButNotReadAdmission(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	grant := f.grant([]string{"node.status"}, nodeScope())
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)
	var events []AuditEvent
	f.service.audit = AuditSinkFunc(func(event AuditEvent) { events = append(events, event) })

	readCall, err := f.service.Admit(f.ctx, Attempt{
		SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource,
	})
	require.NoError(t, err)
	require.Empty(t, events, "successful read admission must not append to the event stream")
	require.NotEmpty(t, readCall.CorrelationID())

	f.service.RecordSuccessfulMutation(readCall)
	require.Len(t, events, 1)
	accepted := events[0]
	require.Equal(t, "accepted", accepted.Outcome)
	require.Equal(t, "mutation_dispatched", accepted.Reason)
	require.Equal(t, readCall.CorrelationID(), accepted.CorrelationID)
	require.Equal(t, f.principal, accepted.Actor)
	require.Equal(t, accepted.Actor, accepted.Effective)
	require.Equal(t, Action("node.status"), accepted.Action)
	require.Equal(t, []string{grant.ID()}, accepted.GrantIDs)

	_, err = f.service.Admit(f.ctx, Attempt{
		SessionSecret: secret, Binding: f.binding, Action: "node.start", Resource: resource,
	})
	require.ErrorIs(t, err, ErrPermissionDenied)
	require.Len(t, events, 2)
	denied := events[1]
	require.Equal(t, "denied", denied.Outcome)
	require.NotEmpty(t, denied.CorrelationID)
	require.NotEqual(t, accepted.CorrelationID, denied.CorrelationID)
}

func TestAdmitTargetFinalizesServerResourceAfterIdentityDerivation(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	f.grant([]string{"node.status"}, nodeScope())
	finalized := false
	call, err := f.service.AdmitTarget(f.ctx, TargetAttempt{
		SessionSecret: secret,
		Binding:       f.binding,
		Action:        "node.status",
		Target:        ResourceTarget{Kind: "node"},
		Finalize: func(target ResourceTarget, audience Audience, actor, effective string) (ResourceRef, error) {
			finalized = true
			require.Equal(t, ResourceTarget{Kind: "node"}, target)
			require.Equal(t, f.binding.Audience, audience)
			require.Equal(t, f.principal, actor)
			require.Equal(t, actor, effective)
			return NewResourceRef(audience.Node, ResourceOwner{}, string(target.Kind), target.ID)
		},
	})
	require.NoError(t, err)
	require.True(t, finalized)
	require.Equal(t, ResourceRef{Node: f.nodeID, Kind: "node"}, call.Resource())

	finalized = false
	_, err = f.service.AdmitTarget(f.ctx, TargetAttempt{
		SessionSecret: SessionSecret{}, Binding: f.binding, Action: "node.status", Target: ResourceTarget{Kind: "node"},
		Finalize: func(ResourceTarget, Audience, string, string) (ResourceRef, error) {
			finalized = true
			return ResourceRef{}, nil
		},
	})
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.False(t, finalized)
}

func TestAdmitTargetDefersExpensiveResolutionUntilSessionValidation(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	f.grant([]string{"data.publish_blob"}, nodeScope())
	resolved := false
	finalized := false
	resolve := func() (ResourceTarget, error) {
		resolved = true
		return ResourceTarget{Kind: "content-blob", ID: "blob-reference"}, nil
	}
	finalize := func(target ResourceTarget, audience Audience, actor, effective string) (ResourceRef, error) {
		finalized = true
		require.True(t, resolved)
		return NewResourceRef(audience.Node, mustResourceOwner(t, effective), string(target.Kind), target.ID)
	}

	_, err := f.service.AdmitTarget(f.ctx, TargetAttempt{
		SessionSecret: SessionSecret{}, Binding: f.binding, Action: "data.publish_blob", ResolveTarget: resolve, Finalize: finalize,
	})
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.False(t, resolved)
	require.False(t, finalized)

	call, err := f.service.AdmitTarget(f.ctx, TargetAttempt{
		SessionSecret: secret, Binding: f.binding, Action: "data.publish_blob", ResolveTarget: resolve, Finalize: finalize,
	})
	require.NoError(t, err)
	require.True(t, resolved)
	require.True(t, finalized)
	require.Equal(t, f.principal, call.Resource().Owner.String())

	_, err = f.service.AdmitTarget(f.ctx, TargetAttempt{
		SessionSecret: secret, Binding: f.binding, Action: "data.publish_blob",
		ResolveTarget: func() (ResourceTarget, error) { return ResourceTarget{}, errors.New("malformed payload") },
		Finalize:      finalize,
	})
	require.ErrorIs(t, err, ErrInvalidResourceTarget)

	revocation, err := SignDeviceRevocation(&identityprotocol.DeviceRevocationPayload{
		Version: 1, TargetId: f.deviceID, TargetDeviceId: f.deviceID, Subject: f.principal, Issuer: f.nodeID,
		Audience: protocolAudience(f.binding.Audience), RevokedAt: timestamppb.New(f.clock.Now()),
	}, f.node, f.clock.Now())
	require.NoError(t, err)
	require.NoError(t, f.service.recordDeviceRevocation(f.ctx, revocation))
	resolved, finalized = false, false
	_, err = f.service.AdmitTarget(f.ctx, TargetAttempt{
		SessionSecret: secret, Binding: f.binding, Action: "data.publish_blob", ResolveTarget: resolve, Finalize: finalize,
	})
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.False(t, resolved)
	require.False(t, finalized)
}

func TestAdmitReadsAllDurableAuthorityInOneSnapshot(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	f.grant([]string{"node.status"}, nodeScope())
	counted := &countingViewDatabase{Database: f.database}
	f.service.grants.database = counted
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource})
	require.NoError(t, err)
	require.Equal(t, int32(1), counted.views.Load())
}

func TestAdmitDeniesSiblingActionCrossNodeAndMalformedDelegation(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	f.grant([]string{"node.status"}, nodeScope())
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)

	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.start", Resource: resource})
	require.ErrorIs(t, err, ErrPermissionDenied)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status.extra", Resource: resource})
	require.ErrorIs(t, err, ErrInvalidArgument)
	otherKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x66}, 32))
	otherNode, err := identityprincipal.FromEd25519PublicKey(otherKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	otherResource, err := NewResourceRef(otherNode.String(), ResourceOwner{}, "node", "")
	require.NoError(t, err)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: otherResource})
	require.ErrorIs(t, err, ErrInvalidArgument)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource, Delegation: []byte{1}})
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestAdmitKeepsAlphaBetaAndInterfaceAuthorityIndependent(t *testing.T) {
	f := newServiceFixture(t)
	alphaBinding := f.binding
	alphaNode := f.nodeID
	alphaSecret := f.sessionSecret()
	f.grant([]string{"node.status"}, nodeScope())
	alphaResource, err := NewResourceRef(alphaNode, ResourceOwner{}, "node", "")
	require.NoError(t, err)

	betaKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x68}, 32))
	betaNode, err := identityprincipal.FromEd25519PublicKey(betaKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	f.node = betaKey
	f.nodeID = betaNode.String()
	f.binding.Audience.Node = betaNode.String()
	betaSecret := f.sessionSecret()
	betaResource, err := NewResourceRef(betaNode.String(), ResourceOwner{}, "node", "")
	require.NoError(t, err)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: betaSecret, Binding: f.binding, Action: "node.status", Resource: betaResource})
	require.ErrorIs(t, err, ErrPermissionDenied)
	f.grant([]string{"node.status"}, nodeScope())
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: betaSecret, Binding: f.binding, Action: "node.status", Resource: betaResource})
	require.NoError(t, err)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: alphaSecret, Binding: alphaBinding, Action: "node.status", Resource: alphaResource})
	require.NoError(t, err)

	_, err = ParseAction(identityprotocol.Interface_INTERFACE_APPLICATION, "node.status")
	require.ErrorIs(t, err, ErrInvalidArgument)
	_, err = ParseAction(identityprotocol.Interface_INTERFACE_OPERATOR, "application.content.get")
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestAdmitRejectsUnsupportedOperatorPrincipalOwnedAndMatchesExactTuple(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	aliceBlob, err := NewResourceRef(f.nodeID, mustResourceOwner(t, f.principal), "content-blob", "b1")
	require.NoError(t, err)
	bobKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x77}, 32))
	bob, err := identityprincipal.FromEd25519PublicKey(bobKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	bobBlob, err := NewResourceRef(f.nodeID, mustResourceOwner(t, bob.String()), "content-blob", "b1")
	require.NoError(t, err)

	f.grant([]string{"data.get_blob"}, ownedScope(f.principal))
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "data.get_blob", Resource: aliceBlob})
	require.ErrorIs(t, err, ErrPermissionDenied)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "data.get_blob", Resource: bobBlob})
	require.ErrorIs(t, err, ErrPermissionDenied)

	f.grant([]string{"data.publish_blob"}, exactScope(aliceBlob))
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "data.publish_blob", Resource: aliceBlob})
	require.NoError(t, err)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "data.publish_blob", Resource: bobBlob})
	require.ErrorIs(t, err, ErrPermissionDenied)
}

func TestGrantProposalRejectsPrincipalOwnedScopeOnOperatorSurface(t *testing.T) {
	f := newServiceFixture(t)
	proposal := GrantProposal{
		Subject: f.principal, Actions: []Action{"data.get_object"},
		Scope:     ResourceScope{Kind: ScopePrincipalOwned, Owner: mustResourceOwner(t, f.principal)},
		NotBefore: f.clock.Now(), NotAfter: f.clock.Now().Add(time.Hour),
	}
	_, err := GrantProposalResourceID(f.nodeID, f.binding.Audience, proposal)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestAdmitRechecksGrantRevocationOnEveryCall(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	grant := f.grant([]string{"node.status"}, nodeScope())
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)
	attempt := Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource}
	_, err = f.service.Admit(f.ctx, attempt)
	require.NoError(t, err)
	revocation, err := SignAccessGrantRevocation(&identityprotocol.AccessGrantRevocationPayload{Version: 1, TargetId: grant.ID(), Issuer: f.nodeID, Audience: protocolAudience(f.binding.Audience), RevokedAt: timestamppb.New(f.clock.Now())}, f.node, f.clock.Now(), grant)
	require.NoError(t, err)
	require.NoError(t, f.service.grants.recordRevocation(f.ctx, revocation, f.node.Public().(ed25519.PublicKey), f.clock.Now()))
	_, err = f.service.Admit(f.ctx, attempt)
	require.ErrorIs(t, err, ErrPermissionDenied)
}

func TestAdmitFailsClosedForCorruptGrantIndex(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	grant := f.grant([]string{"node.status"}, nodeScope())
	key, err := grantIndexKey(grant.AccessGrantPayload())
	require.NoError(t, err)
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		return tx.Put(grantIndexBucket, key, bytes.Repeat([]byte{0xff}, 32))
	}))
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource})
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestAdmitValidatesCorruptSiblingAfterMatchingGrant(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	grant := f.grant([]string{"node.status"}, nodeScope())
	prefix := grantIndexPrefix(f.binding.Audience, f.principal)
	validKey, err := grantIndexKey(grant.AccessGrantPayload())
	require.NoError(t, err)
	corruptKey := appendTuple(append([]byte(nil), prefix...), []byte(strings.Repeat("z", len(grant.ID()))))
	require.Greater(t, bytes.Compare(corruptKey, validKey), 0)
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		return tx.Put(grantIndexBucket, corruptKey, bytes.Repeat([]byte{0xaa}, 32))
	}))
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)
	_, err = f.service.Admit(f.ctx, Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource})
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestConcurrentAdmitAfterRevocationNeverSucceeds(t *testing.T) {
	f := newServiceFixture(t)
	secret := f.sessionSecret()
	grant := f.grant([]string{"node.status"}, nodeScope())
	resource, err := NewResourceRef(f.nodeID, ResourceOwner{}, "node", "")
	require.NoError(t, err)
	attempt := Attempt{SessionSecret: secret, Binding: f.binding, Action: "node.status", Resource: resource}
	revocation, err := SignAccessGrantRevocation(&identityprotocol.AccessGrantRevocationPayload{Version: 1, TargetId: grant.ID(), Issuer: f.nodeID, Audience: protocolAudience(f.binding.Audience), RevokedAt: timestamppb.New(f.clock.Now())}, f.node, f.clock.Now(), grant)
	require.NoError(t, err)
	require.NoError(t, f.service.grants.recordRevocation(f.ctx, revocation, f.node.Public().(ed25519.PublicKey), f.clock.Now()))
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, admitErr := f.service.Admit(context.Background(), attempt)
			require.True(t, errors.Is(admitErr, ErrPermissionDenied))
		}()
	}
	wg.Wait()
}

type delegatedAdmissionFixture struct {
	serviceFixture *serviceFixture
	secret         SessionSecret
	alice          string
	aliceDeviceID  string
	aliceDevice    ed25519.PrivateKey
	credential     *identityprotocol.KeyCredential
	delegation     *Artifact
	attempt        TargetAttempt
}

func newDelegatedAdmissionFixture(t *testing.T, actorGrant, effectiveGrant bool, delegationActions []string) *delegatedAdmissionFixture {
	t.Helper()
	f := newServiceFixture(t)
	f.binding.Audience.Interface = identityprotocol.Interface_INTERFACE_APPLICATION
	secret := f.sessionSecret()
	aliceRoot := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, 32))
	aliceDevice := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x72}, 32))
	alicePrincipal, err := identityprincipal.FromEd25519PublicKey(aliceRoot.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	aliceDeviceID, err := identityprincipal.DeviceFromEd25519PublicKey(aliceDevice.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	credentialArtifact, err := SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: 1, Subject: alicePrincipal.String(), RootPublicKey: aliceRoot.Public().(ed25519.PublicKey),
		DeviceId: aliceDeviceID.String(), DevicePublicKey: aliceDevice.Public().(ed25519.PublicKey),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(f.clock.Now().Add(-time.Hour)), NotAfter: timestamppb.New(f.clock.Now().Add(time.Hour)),
	}, aliceRoot)
	require.NoError(t, err)
	credentialRaw, err := credentialArtifact.MarshalBinary()
	require.NoError(t, err)
	credential := new(identityprotocol.KeyCredential)
	require.NoError(t, proto.Unmarshal(credentialRaw, credential))
	allActions := []string{"application.content.get", "application.content.put"}
	if actorGrant {
		f.grantFor(f.principal, allActions, nodeScope())
	}
	if effectiveGrant {
		f.grantFor(alicePrincipal.String(), allActions, ownedScope(alicePrincipal.String()))
	}
	delegation, err := SignDelegation(&identityprotocol.DelegationPayload{
		Version: 1, Delegator: alicePrincipal.String(), Delegatee: f.principal,
		Audience: protocolAudience(f.binding.Audience), Actions: delegationActions, Scope: ownedScope(alicePrincipal.String()),
		NotBefore: timestamppb.New(f.clock.Now().Add(-time.Minute)), NotAfter: timestamppb.New(f.clock.Now().Add(time.Hour)), Credential: credential,
	}, aliceDevice, f.clock.Now())
	require.NoError(t, err)
	delegationRaw, err := delegation.MarshalBinary()
	require.NoError(t, err)
	attempt := TargetAttempt{
		SessionSecret: secret, Binding: f.binding, Action: "application.content.get", Delegation: delegationRaw,
		Target: ResourceTarget{Kind: "content-blob", ID: "blob-1"},
		Finalize: func(target ResourceTarget, audience Audience, _, effective string) (ResourceRef, error) {
			return NewResourceRef(audience.Node, mustResourceOwner(t, effective), string(target.Kind), target.ID)
		},
	}
	return &delegatedAdmissionFixture{serviceFixture: f, secret: secret, alice: alicePrincipal.String(), aliceDeviceID: aliceDeviceID.String(), aliceDevice: aliceDevice, credential: credential, delegation: delegation, attempt: attempt}
}

func TestAdmitOneHopDelegationRequiresAllAuthorityLegsAndAttenuatesAction(t *testing.T) {
	f := newDelegatedAdmissionFixture(t, true, true, []string{"application.content.get"})
	call, err := f.serviceFixture.service.AdmitTarget(f.serviceFixture.ctx, f.attempt)
	require.NoError(t, err)
	require.Equal(t, f.serviceFixture.principal, call.Actor())
	require.Equal(t, f.alice, call.Effective())
	require.Equal(t, f.alice, call.Resource().Owner.String())
	require.Len(t, call.GrantIDs(), 2)
	require.Equal(t, f.delegation.ID(), call.DelegationID())

	missingEffective := newDelegatedAdmissionFixture(t, true, false, []string{"application.content.get"})
	_, err = missingEffective.serviceFixture.service.AdmitTarget(missingEffective.serviceFixture.ctx, missingEffective.attempt)
	require.ErrorIs(t, err, ErrPermissionDenied)
	missingActor := newDelegatedAdmissionFixture(t, false, true, []string{"application.content.get"})
	_, err = missingActor.serviceFixture.service.AdmitTarget(missingActor.serviceFixture.ctx, missingActor.attempt)
	require.ErrorIs(t, err, ErrPermissionDenied)

	f.attempt.Action = "application.content.put"
	_, err = f.serviceFixture.service.AdmitTarget(f.serviceFixture.ctx, f.attempt)
	require.ErrorIs(t, err, ErrPermissionDenied)
}

func TestDelegatedDenialAuditRetainsSafeActorEffectiveAndAuthorityProvenance(t *testing.T) {
	f := newDelegatedAdmissionFixture(t, true, true, []string{"application.content.get"})
	var events []AuditEvent
	f.serviceFixture.service.audit = AuditSinkFunc(func(event AuditEvent) { events = append(events, event) })
	f.attempt.Action = "application.content.put"

	_, err := f.serviceFixture.service.AdmitTarget(f.serviceFixture.ctx, f.attempt)
	require.ErrorIs(t, err, ErrPermissionDenied)
	require.NotEmpty(t, events)
	denied := events[len(events)-1]
	require.Equal(t, "denied", denied.Outcome)
	require.Equal(t, f.serviceFixture.principal, denied.Actor)
	require.Equal(t, f.alice, denied.Effective)
	require.Equal(t, Action("application.content.put"), denied.Action)
	require.Equal(t, f.delegation.ID(), denied.DelegationID)
	require.Len(t, denied.GrantIDs, 1, "the matched Actor grant remains safe audit provenance")
}

func TestAdmitDelegationChecksAudienceBoundsAndDelegatorDeviceRevocation(t *testing.T) {
	f := newDelegatedAdmissionFixture(t, true, true, []string{"application.content.get"})
	oversized := f.attempt
	oversized.Delegation = make([]byte, maxArtifactBytes+1)
	resolved := false
	oversized.ResolveTarget = func() (ResourceTarget, error) {
		resolved = true
		return oversized.Target, nil
	}
	_, err := f.serviceFixture.service.AdmitTarget(f.serviceFixture.ctx, oversized)
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.False(t, resolved)

	wrongBinding := f.attempt
	wrongBinding.Binding.Audience.ProtocolMajor++
	_, err = f.serviceFixture.service.AdmitTarget(f.serviceFixture.ctx, wrongBinding)
	require.ErrorIs(t, err, ErrUnauthenticated)

	revocation, err := SignDeviceRevocation(&identityprotocol.DeviceRevocationPayload{
		Version: 1, TargetId: f.aliceDeviceID, TargetDeviceId: f.aliceDeviceID, Subject: f.alice,
		Issuer: f.serviceFixture.nodeID, Audience: protocolAudience(f.serviceFixture.binding.Audience), RevokedAt: timestamppb.New(f.serviceFixture.clock.Now()),
	}, f.serviceFixture.node, f.serviceFixture.clock.Now())
	require.NoError(t, err)
	require.NoError(t, f.serviceFixture.service.recordDeviceRevocation(f.serviceFixture.ctx, revocation))
	_, err = f.serviceFixture.service.AdmitTarget(f.serviceFixture.ctx, f.attempt)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestDelegationRevocationIsIdempotentPermanentAcrossRestartAndConcurrentAdmit(t *testing.T) {
	f := newDelegatedAdmissionFixture(t, true, true, []string{"application.content.get"})
	payload := f.delegation.DelegationPayload()
	revocation, err := SignDelegationRevocation(&identityprotocol.DelegationRevocationPayload{
		Version: 1, TargetId: f.delegation.ID(), Issuer: f.alice, Audience: payload.Audience,
		RevokedAt: timestamppb.New(f.serviceFixture.clock.Now()), Delegator: f.alice, Delegatee: f.serviceFixture.principal, Credential: f.credential,
	}, f.aliceDevice, f.serviceFixture.clock.Now())
	require.NoError(t, err)
	raw, err := revocation.MarshalBinary()
	require.NoError(t, err)
	require.NoError(t, f.serviceFixture.service.ImportDelegationRevocation(f.serviceFixture.ctx, raw))
	require.NoError(t, f.serviceFixture.service.ImportDelegationRevocation(f.serviceFixture.ctx, raw))
	_, err = f.serviceFixture.service.AdmitTarget(f.serviceFixture.ctx, f.attempt)
	require.ErrorIs(t, err, ErrUnauthenticated)

	conflict, err := SignDelegationRevocation(&identityprotocol.DelegationRevocationPayload{
		Version: 1, TargetId: f.delegation.ID(), Issuer: f.alice, Audience: payload.Audience,
		RevokedAt: timestamppb.New(f.serviceFixture.clock.Now().Add(-time.Second)), Delegator: f.alice, Delegatee: f.serviceFixture.principal, Credential: f.credential,
	}, f.aliceDevice, f.serviceFixture.clock.Now())
	require.NoError(t, err)
	conflictRaw, err := conflict.MarshalBinary()
	require.NoError(t, err)
	require.ErrorIs(t, f.serviceFixture.service.ImportDelegationRevocation(f.serviceFixture.ctx, conflictRaw), ErrConflict)

	require.NoError(t, f.serviceFixture.database.Close(f.serviceFixture.ctx))
	reopened, err := storage.OpenIdentityAccess(f.serviceFixture.ctx, f.serviceFixture.dir, StorageSchema())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close(context.Background())) })
	f.serviceFixture.database = reopened
	f.serviceFixture.service, err = NewService(Config{Database: reopened, Clock: f.serviceFixture.clock, Entropy: &sequentialEntropy{next: 91}})
	require.NoError(t, err)
	f.attempt.SessionSecret = f.serviceFixture.sessionSecret()

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, admitErr := f.serviceFixture.service.AdmitTarget(context.Background(), f.attempt)
			require.ErrorIs(t, admitErr, ErrUnauthenticated)
		}()
	}
	wg.Wait()
}

func TestEnrollmentRepositoryVerifiesRootBindingOnWriteAndLoad(t *testing.T) {
	f := newServiceFixture(t)
	var root [32]byte
	copy(root[:], f.root.Public().(ed25519.PublicKey))
	record := EnrollmentRecord{Node: f.nodeID, Principal: f.principal, RootPublicKey: root, EnrolledAt: f.clock.Now()}
	require.NoError(t, f.service.enrollments.record(f.ctx, record))
	loaded, err := f.service.enrollments.load(f.ctx, f.nodeID, f.principal)
	require.NoError(t, err)
	require.Equal(t, record, loaded)

	wrong := record
	copy(wrong.RootPublicKey[:], f.node.Public().(ed25519.PublicKey))
	require.ErrorIs(t, f.service.enrollments.record(f.ctx, wrong), errInvalid)
	key, err := enrollmentKey(f.nodeID, f.principal)
	require.NoError(t, err)
	require.NoError(t, f.database.Update(f.ctx, func(tx storage.WriteTransaction) error {
		return tx.Put(enrollmentsBucket, key, append([]byte{0}, make([]byte, enrollmentRecordBytes-1)...))
	}))
	_, err = f.service.enrollments.load(f.ctx, f.nodeID, f.principal)
	require.Error(t, err)
}
