package authority

import (
	"bytes"
	"context"
	"errors"
	"testing"

	identityapi "ardents/internal/identity"

	"github.com/stretchr/testify/require"
)

func TestRecoveryOnlyStartupVerifiesExactRestoreWithoutMutation(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "genesis-restore", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	beforeState := cloneLedger(fixture.store.state)
	beforeHead := fixture.repository.head
	beforeCreateCalls := fixture.repository.createCalls

	restored := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Random: bytes.NewReader(fixture.randomSeed), Clock: fixture.clock,
		Policy: allowPolicy{}, RecoveryOnly: true,
	})
	require.Equal(t, Status{
		Version: ContractVersion, SchemaVersion: SchemaVersion,
		RealmID: genesis.RealmID, RealmClass: RealmClassProduction,
		AuthorityEpoch: 1, AuthoritySequence: 1,
		CheckpointDigest: genesis.CheckpointDigest,
		Phase:            PhaseRecoveryOnly, Readiness: ReadinessDegraded,
		Reason:            ReasonRestoreVerificationRequired,
		AuditOutboxDepth:  1,
		OperationDeadline: beforeState.Operations[0].Deadline,
	}, restored.Readiness())

	_, err = restored.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "genesis-restore", RealmClass: RealmClassProduction,
	})
	require.ErrorIs(t, err, ErrRecoveryRequired)
	require.Equal(t, beforeState, fixture.store.state)
	require.Equal(t, beforeHead, fixture.repository.head)
	require.Equal(t, beforeCreateCalls, fixture.repository.createCalls)

	result, err := restored.VerifyRestoredAuthority(
		ctx,
		verifyRestoreCommand(genesis.RealmID),
		VerifyRestoreRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
			AuthoritySequence: genesis.AuthoritySequence,
			CheckpointDigest:  genesis.CheckpointDigest,
		},
	)
	require.NoError(t, err)
	require.Equal(t, VerifyRestoreResult{
		Version: ContractVersion, RealmID: genesis.RealmID,
		AuthorityEpoch: 1, AuthoritySequence: 1,
		CheckpointDigest: genesis.CheckpointDigest,
		Phase:            PhaseReady, Readiness: ReadinessReady,
	}, result)
	require.Equal(t, beforeState, fixture.store.state)
	require.Equal(t, beforeHead, fixture.repository.head)
	require.Equal(t, beforeCreateCalls, fixture.repository.createCalls)

	replayed, err := restored.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "genesis-restore", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	require.Equal(t, genesis, replayed)
}

func TestRecoveryOnlyStartupNeverRepairsMissingOrMismatchedRepositoryHead(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name       string
		arrange    func(*serviceFixture)
		wantReason string
	}{
		{
			name: "missing",
			arrange: func(f *serviceFixture) {
				f.repository.found = false
				f.repository.head = SignedCheckpoint{}
			},
			wantReason: ReasonCheckpointMissing,
		},
		{
			name: "mismatched",
			arrange: func(f *serviceFixture) {
				f.repository.head.Digest = "ac1_" + string(bytes.Repeat([]byte{'0'}, 64))
			},
			wantReason: ReasonCheckpointHistoryFork,
		},
		{
			name: "partial history",
			arrange: func(f *serviceFixture) {
				f.repository.err = ErrCheckpointHistoryPartial
			},
			wantReason: ReasonCheckpointHistoryPartial,
		},
		{
			name: "forked history",
			arrange: func(f *serviceFixture) {
				f.repository.err = ErrCheckpointHistoryFork
			},
			wantReason: ReasonCheckpointHistoryFork,
		},
		{
			name: "generation mismatch",
			arrange: func(f *serviceFixture) {
				f.repository.err = ErrAuthorityGenerationMismatch
			},
			wantReason: ReasonAuthorityGenerationMismatch,
		},
		{
			name: "restored ledger rollback",
			arrange: func(f *serviceFixture) {
				f.repository.head.AuthoritySequence++
			},
			wantReason: ReasonAuthorityRollback,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServiceFixture(t)
			genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
				Version: ContractVersion, RequestID: "restore-genesis", RealmClass: RealmClassProduction,
			})
			require.NoError(t, err)
			test.arrange(fixture)
			beforeState := cloneLedger(fixture.store.state)
			beforeHead := fixture.repository.head
			beforeFound := fixture.repository.found
			beforeCalls := fixture.repository.createCalls

			restored := New(Config{
				Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
				Policy: allowPolicy{}, RecoveryOnly: true,
			})
			status := restored.Readiness()
			require.Equal(t, PhaseRecoveryRequired, status.Phase)
			require.Equal(t, ReadinessRecoveryRequired, status.Readiness)
			require.Equal(t, test.wantReason, status.Reason)

			_, err = restored.VerifyRestoredAuthority(
				ctx, verifyRestoreCommand(genesis.RealmID),
				VerifyRestoreRequest{
					Version: ContractVersion, RealmID: genesis.RealmID,
					AuthoritySequence: genesis.AuthoritySequence,
					CheckpointDigest:  genesis.CheckpointDigest,
				},
			)
			require.ErrorIs(t, err, ErrRecoveryRequired)
			require.Equal(t, beforeState, fixture.store.state)
			require.Equal(t, beforeHead, fixture.repository.head)
			require.Equal(t, beforeFound, fixture.repository.found)
			require.Equal(t, beforeCalls, fixture.repository.createCalls)
		})
	}
}

func TestVerifyRestoredAuthorityRejectsMismatchAndPolicyDenial(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "restore-genesis", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	beforeState := cloneLedger(fixture.store.state)
	beforeHead := fixture.repository.head

	restored := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Policy: allowPolicy{}, RecoveryOnly: true,
	})
	_, err = restored.VerifyRestoredAuthority(
		ctx, verifyRestoreCommand(genesis.RealmID),
		VerifyRestoreRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
			AuthoritySequence: genesis.AuthoritySequence + 1,
			CheckpointDigest:  genesis.CheckpointDigest,
		},
	)
	require.ErrorIs(t, err, ErrRecoveryRequired)

	denied := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Policy: denyPolicy{}, RecoveryOnly: true,
	})
	_, err = denied.VerifyRestoredAuthority(
		ctx, verifyRestoreCommand(genesis.RealmID),
		VerifyRestoreRequest{
			Version: ContractVersion, RealmID: genesis.RealmID,
			AuthoritySequence: genesis.AuthoritySequence,
			CheckpointDigest:  genesis.CheckpointDigest,
		},
	)
	require.ErrorIs(t, err, ErrPermissionDenied)
	require.Equal(t, beforeState, fixture.store.state)
	require.Equal(t, beforeHead, fixture.repository.head)
}

func TestRecoveryOnlyFencesDeliveryMutationBeforePolicyAndPersistence(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	genesis, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "restore-genesis", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	beforeState := cloneLedger(fixture.store.state)
	beforeHead := fixture.repository.head

	restored := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Policy: allowPolicy{}, RecoveryOnly: true,
	})
	request := InitialGenerationRequest{
		Version: ContractVersion, RequestID: "fenced-delivery", RealmID: genesis.RealmID,
		ChannelClass: identityapi.CapabilityRealmDiscovery,
		Permissions:  identityapi.CapabilityPublish | identityapi.CapabilitySubscribe,
		ValidFor:     MaxOperationLifetime,
	}
	command := Command{
		Actor: "operator", Effective: "operator", Action: ActionIssueDelivery,
		ResourceKind: ResourceKindGenerationDelivery,
		ResourceID:   InitialGenerationDeliveryResource(genesis.RealmID, request.RequestID),
	}
	_, err = restored.IssueInitialGeneration(ctx, command, request)
	require.ErrorIs(t, err, ErrRecoveryRequired)
	require.Equal(t, beforeState, fixture.store.state)
	require.Equal(t, beforeHead, fixture.repository.head)
}

func TestRecoveryOnlyRepositoryUnavailableIsRedactedAndReadOnly(t *testing.T) {
	ctx := context.Background()
	fixture := newServiceFixture(t)
	_, err := fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "restore-genesis", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	beforeState := cloneLedger(fixture.store.state)
	fixture.repository.err = errors.New("secret backend detail")

	restored := New(Config{
		Store: fixture.store, Signer: fixture.signer, Repository: fixture.repository,
		Policy: allowPolicy{}, RecoveryOnly: true,
	})
	status := restored.Readiness()
	require.Equal(t, ReadinessUnavailable, status.Readiness)
	require.Equal(t, ReasonRepositoryUnavailable, status.Reason)
	require.NotContains(t, status.String(), "secret backend detail")
	require.Equal(t, beforeState, fixture.store.state)
}

func verifyRestoreCommand(realmID string) Command {
	return Command{
		Actor: "operator", Effective: "operator", Action: ActionVerifyRestore,
		ResourceKind: ResourceKindRealm, ResourceID: realmID,
	}
}
