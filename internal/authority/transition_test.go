package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	identitycapability "ardents/internal/identity/capability"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func TestPlanAuthorityTransitionIsDualSignedAndExact(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "transition-genesis", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	next := newTestSigner(t, 0x72)

	transition, err := fixture.service.PlanAuthorityTransition(
		ctx,
		planTransitionCommand(genesis.RealmID),
		PlanAuthorityTransitionRequest{
			Version: ContractVersion, RequestID: "transition-1", RealmID: genesis.RealmID,
			AuthoritySequence: genesis.AuthoritySequence,
			CheckpointDigest:  genesis.CheckpointDigest,
		},
		next,
	)
	require.NoError(t, err)
	require.Equal(t, genesis.RealmID, transition.RealmID)
	require.Equal(t, fixture.signer.principal, transition.FromAuthorityPrincipal)
	require.Equal(t, next.principal, transition.ToAuthorityPrincipal)
	require.Equal(t, uint64(1), transition.FromAuthorityEpoch)
	require.Equal(t, uint64(2), transition.ToAuthorityEpoch)
	require.Equal(t, genesis.AuthoritySequence, transition.AuthoritySequence)
	require.Equal(t, genesis.CheckpointDigest, transition.CheckpointDigest)
	require.NoError(t, ValidateAuthorityTransition(transition))
	require.Equal(t, uint64(2), fixture.store.state.AuthorityEpoch)
	require.Equal(t, uint64(2), fixture.store.state.AuthoritySequence)
	require.Equal(t, next.principal, fixture.store.state.AuthorityPrincipal)
	require.Equal(t, fixture.store.state.Checkpoint, fixture.repository.head)
	require.Equal(t, &transition, fixture.store.state.Checkpoint.AuthorityTransition)
	require.NotNil(t, fixture.store.state.Transition)
	require.NoError(t, validateLedger(fixture.store.state))
}

func TestAuthorityTransitionAdvancesRepositoryAndRequiresEveryChannelRotation(t *testing.T) {
	ctx := context.Background()
	legacy := newLocalV2TestInput(t)
	service, store, repository := newLocalV2MigrationService(legacy)
	migrated, err := service.MigrateLocalV2(
		ctx, migrateLocalV2Command(), legacy.request,
	)
	require.NoError(t, err)
	oldTrust, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: legacy.signer.principal,
		PublicKey: legacy.signer.private.Public().(ed25519.PublicKey),
		Purposes:  []identitytrust.Purpose{identitytrust.PurposeChannelIssue},
	}})
	require.NoError(t, err)
	member, err := identitycapability.NewService(
		t.TempDir()+"/member.db", bytes.Repeat([]byte{0x93}, 32),
		legacy.memberPrincipal, oldTrust, authorityCapabilityPolicy{}, legacy.clock,
	)
	require.NoError(t, err)
	for _, grant := range legacy.request.Members[0].ReceiverGrants {
		_, err = member.ImportGrant(grant)
		require.NoError(t, err)
	}
	attestation, err := member.AttestDeliveryPublicKey(
		legacy.memberPrivate, legacy.clock().Add(time.Hour),
	)
	require.NoError(t, err)
	service.random = bytes.NewReader(transitionTestRandom(0x20))
	channelIDs := [][16]byte{migrated.DiscoveryChannelID, migrated.DataChannelID}
	for index, channelID := range channelIDs {
		completeMigrationChannelRotation(
			t, ctx, service, member, attestation, migrated.RealmID,
			channelID, "pre-transition-"+string(rune('1'+index)),
		)
	}

	next := newTestSigner(t, 0x95)
	beforeSequence := store.state.AuthoritySequence
	beforeDigest := store.state.Checkpoint.Digest
	proof, err := service.PlanAuthorityTransition(
		ctx, planTransitionCommand(migrated.RealmID),
		PlanAuthorityTransitionRequest{
			Version: ContractVersion, RequestID: "transition-with-channels", RealmID: migrated.RealmID,
			AuthoritySequence: beforeSequence, CheckpointDigest: beforeDigest,
		},
		next,
	)
	require.NoError(t, err)
	require.Equal(t, beforeSequence+1, store.state.AuthoritySequence)
	require.Equal(t, uint64(2), store.state.AuthorityEpoch)
	require.Equal(t, store.state.Checkpoint, repository.head)
	require.Equal(t, &proof, repository.head.AuthorityTransition)
	require.True(t, service.transitionPending)
	require.Equal(t, PhaseAuthorityTransitionRotationRequired, service.Readiness().Phase)
	require.Len(t, store.state.Transition.RequiredRotationChannelIDs, 2)

	require.NoError(t, AdoptMemberAuthorityTransition(member, proof))
	service.random = bytes.NewReader(transitionTestRandom(0x60))
	for index, channelID := range channelIDs {
		completeMigrationChannelRotation(
			t, ctx, service, member, attestation, migrated.RealmID,
			channelID, "post-transition-"+string(rune('1'+index)),
		)
		if index == 0 {
			require.True(t, service.transitionPending)
			require.Equal(t, PhaseAuthorityTransitionRotationRequired, service.Readiness().Phase)
		}
	}
	require.False(t, service.transitionPending)
	require.Equal(t, PhaseReady, service.Readiness().Phase)
	require.Equal(t, ReadinessReady, service.Readiness().Readiness)
	require.Len(t, store.state.Transition.RotatedChannelIDs, 2)
	require.NoError(t, validateLedger(store.state))
}

func transitionTestRandom(offset byte) []byte {
	raw := make([]byte, 4096)
	for index := range raw {
		raw[index] = byte((index+int(offset))%251 + 1)
	}
	return raw
}

func TestAuthorityTransitionRejectsEveryExactnessAndSignatureChange(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "transition-genesis", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	transition, err := fixture.service.PlanAuthorityTransition(
		ctx,
		planTransitionCommand(genesis.RealmID),
		PlanAuthorityTransitionRequest{
			Version: ContractVersion, RequestID: "transition-tamper", RealmID: genesis.RealmID,
			AuthoritySequence: genesis.AuthoritySequence,
			CheckpointDigest:  genesis.CheckpointDigest,
		},
		newTestSigner(t, 0x73),
	)
	require.NoError(t, err)

	tests := []struct {
		name   string
		tamper func(*AuthorityTransition)
	}{
		{"realm", func(value *AuthorityTransition) { value.RealmID = "r1_" + string(bytes.Repeat([]byte{'0'}, 32)) }},
		{"from epoch", func(value *AuthorityTransition) { value.FromAuthorityEpoch++ }},
		{"to epoch", func(value *AuthorityTransition) { value.ToAuthorityEpoch++ }},
		{"sequence", func(value *AuthorityTransition) { value.AuthoritySequence++ }},
		{"checkpoint", func(value *AuthorityTransition) {
			value.CheckpointDigest = "ac1_" + string(bytes.Repeat([]byte{'0'}, 64))
		}},
		{"created at", func(value *AuthorityTransition) { value.CreatedAt = value.CreatedAt.Add(time.Second) }},
		{"from signature", func(value *AuthorityTransition) { value.FromSignature[0] ^= 1 }},
		{"to signature", func(value *AuthorityTransition) { value.ToSignature[0] ^= 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tampered := cloneAuthorityTransition(transition)
			test.tamper(&tampered)
			require.Error(t, ValidateAuthorityTransition(tampered))
		})
	}
}

func TestPlanAuthorityTransitionFailsClosedWithoutOldSignerOrExactHead(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		arrange func(*serviceFixture)
		want    error
	}{
		{
			name: "old signer unavailable",
			arrange: func(f *serviceFixture) {
				f.service.signer = failingIdentitySigner{}
			},
			want: ErrUnavailable,
		},
		{
			name: "repository head missing",
			arrange: func(f *serviceFixture) {
				f.repository.found = false
			},
			want: ErrRecoveryRequired,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServiceFixture(t)
			genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
				Version: ContractVersion, RequestID: "transition-genesis", RealmClass: RealmClassProduction,
			})
			require.NoError(t, err)
			test.arrange(fixture)
			_, err = fixture.service.PlanAuthorityTransition(
				ctx,
				planTransitionCommand(genesis.RealmID),
				PlanAuthorityTransitionRequest{
					Version: ContractVersion, RequestID: "transition-failure", RealmID: genesis.RealmID,
					AuthoritySequence: genesis.AuthoritySequence,
					CheckpointDigest:  genesis.CheckpointDigest,
				},
				newTestSigner(t, 0x74),
			)
			require.ErrorIs(t, err, test.want)
		})
	}
}

func TestAuthorityTransitionRollsForwardAfterRepositoryOutageAndRestart(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(
		ctx, fixture.createCommand(),
		CreateRequest{
			Version: ContractVersion, RequestID: "transition-resume-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	next := newTestSigner(t, 0x77)
	fixture.repository.appendErr = ErrUnavailable
	proof, err := fixture.service.PlanAuthorityTransition(
		ctx, planTransitionCommand(genesis.RealmID),
		PlanAuthorityTransitionRequest{
			Version: ContractVersion, RequestID: "transition-resume", RealmID: genesis.RealmID,
			AuthoritySequence: genesis.AuthoritySequence,
			CheckpointDigest:  genesis.CheckpointDigest,
		},
		next,
	)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Equal(t, AuthorityTransition{}, proof)
	require.Equal(t, PhaseCheckpointing, fixture.store.state.Phase)
	require.Equal(t, next.principal, fixture.store.state.AuthorityPrincipal)

	fixture.repository.appendErr = nil
	restarted := New(Config{
		Store: fixture.store, Signer: fixture.signer, SuccessorSigner: next,
		Repository: fixture.repository, Policy: allowPolicy{}, Clock: fixture.clock,
	})
	require.Equal(t, PhaseReady, restarted.Readiness().Phase)
	require.Equal(t, ReadinessReady, restarted.Readiness().Readiness)
	require.Equal(t, uint64(2), fixture.repository.head.AuthorityEpoch)
	require.Equal(t, fixture.store.state.Checkpoint, fixture.repository.head)
	require.NoError(t, validateLedger(fixture.store.state))
}

func TestLostRepositoryRecoveryCreatesDifferentRealmInsteadOfRepairingOld(t *testing.T) {
	ctx := context.Background()
	old := newServiceFixture(t)
	oldGenesis, err := old.service.CreateOrReopen(ctx, old.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "old-genesis", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	old.repository.found = false
	oldStopped := New(Config{
		Store: old.store, Signer: old.signer, Repository: old.repository,
		Policy: allowPolicy{},
	})
	require.Equal(t, ReadinessRecoveryRequired, oldStopped.Readiness().Readiness)
	require.Equal(t, ReasonCheckpointMissing, oldStopped.Readiness().Reason)

	seed := bytes.Repeat([]byte{0x75}, 1024)
	replacementStore := &memoryStore{}
	replacementRepository := &memoryRepository{}
	replacement := New(Config{
		Store: replacementStore, Signer: newTestSigner(t, 0x76),
		Repository: replacementRepository, Random: bytes.NewReader(seed),
		Clock: old.clock, Policy: allowPolicy{},
	})
	replacementGenesis, err := replacement.CreateOrReopen(
		ctx,
		Command{
			Actor: "operator", Effective: "operator", Action: ActionCreate,
			ResourceKind: ResourceKindAuthorityInstance, ResourceID: PrimaryAuthorityInstance,
		},
		CreateRequest{
			Version: ContractVersion, RequestID: "replacement-genesis",
			RealmClass: RealmClassProduction,
		},
	)
	require.NoError(t, err)
	require.NotEqual(t, oldGenesis.RealmID, replacementGenesis.RealmID)
	require.Equal(t, PhaseRecoveryRequired, old.store.state.Phase)
	require.Equal(t, oldGenesis.RealmID, old.store.state.RealmID)
}

type failingIdentitySigner struct{}

func (failingIdentitySigner) Principal(context.Context) (string, error) {
	return "", ErrUnavailable
}
func (failingIdentitySigner) PublicKey(context.Context) (ed25519.PublicKey, error) {
	return nil, ErrUnavailable
}
func (failingIdentitySigner) Sign(context.Context, []byte) ([]byte, error) {
	return nil, ErrUnavailable
}

func planTransitionCommand(realmID string) Command {
	return Command{
		Actor: "operator", Effective: "operator", Action: ActionPlanTransition,
		ResourceKind: ResourceKindRealm, ResourceID: realmID,
	}
}

func cloneAuthorityTransition(value AuthorityTransition) AuthorityTransition {
	value.FromAuthorityPublicKey = append([]byte(nil), value.FromAuthorityPublicKey...)
	value.ToAuthorityPublicKey = append([]byte(nil), value.ToAuthorityPublicKey...)
	value.FromSignature = append([]byte(nil), value.FromSignature...)
	value.ToSignature = append([]byte(nil), value.ToSignature...)
	return value
}
