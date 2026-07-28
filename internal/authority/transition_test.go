package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPlanAuthorityTransitionIsDualSignedAndExact(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "transition-genesis", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	beforeState := cloneLedger(fixture.store.state)
	beforeHead := fixture.repository.head
	next := newTestSigner(t, 0x72)

	transition, err := fixture.service.PlanAuthorityTransition(
		ctx,
		planTransitionCommand(genesis.RealmID),
		PlanAuthorityTransitionRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
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
	require.Equal(t, beforeState, fixture.store.state)
	require.Equal(t, beforeHead, fixture.repository.head)
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
			Version: ContractVersion, RealmID: genesis.RealmID,
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
					Version: ContractVersion, RealmID: genesis.RealmID,
					AuthoritySequence: genesis.AuthoritySequence,
					CheckpointDigest:  genesis.CheckpointDigest,
				},
				newTestSigner(t, 0x74),
			)
			require.ErrorIs(t, err, test.want)
		})
	}
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
