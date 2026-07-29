package deploymentjournal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/deployment"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestRejoinFileRoundTripRejectsUnknownAndBindingOverwrite(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	require.NoError(t, storage.EnsurePrivateDir(directory))
	path := filepath.Join(directory, "rejoin.json")
	store := RejoinFile{Path: path}
	now := time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC)
	transaction := deployment.RejoinTransaction{
		Version: deployment.RejoinTransactionVersion, Revision: 1,
		ManifestDigest:          digest('a'),
		TargetSlot:              "node-a",
		ChannelID:               "00112233445566778899aabbccddeeff",
		ExpectedPrincipalHash:   digest('b'),
		ExpectedWakuPeerIDHash:  digest('c'),
		ExpectedImageHash:       digest('d'),
		Actor:                   "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID:               "rejoin-file-journal",
		StartedAt:               now,
		Deadline:                now.Add(time.Minute),
		FenceRequestID:          "fence-file-journal",
		FenceEvidenceDigest:     digest('e'),
		RemovalOperationID:      "rao1_00112233445566778899aabbccddeeff",
		RemovalGeneration:       4,
		RemovalCheckpointDigest: digest('f'),
		Phase:                   deployment.RejoinPhaseRequested,
	}

	require.NoError(t, deployment.ValidateRejoinTransaction(transaction))
	require.NoError(t, store.Save(context.Background(), 0, transaction))
	loaded, found, err := store.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, transaction, loaded)

	next := transaction
	next.Revision = 2
	next.Phase = deployment.RejoinPhasePreflightPersisted
	next.ClockObservedAt = now
	next.ClockSkewSecond = 7
	next.IsolationConfirmed = true
	require.NoError(t, store.Save(context.Background(), transaction.Revision, next))

	other := next
	other.Revision = 3
	other.TargetSlot = "node-b"
	require.ErrorIs(
		t,
		store.Save(context.Background(), next.Revision, other),
		deployment.ErrRejoinJournalBinding,
	)

	require.NoError(
		t,
		os.WriteFile(
			path,
			[]byte(`{"version":"topology-rejoin-transaction/v1","unknown":true}`),
			0o600,
		),
	)
	_, _, err = store.Load(context.Background())
	require.ErrorIs(t, err, deployment.ErrRejoinJournalInvalid)
}

func TestRejoinFileAllowsAmbiguousTargetAcknowledgementBacktrack(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	require.NoError(t, storage.EnsurePrivateDir(directory))
	path := filepath.Join(directory, "rejoin.json")
	store := RejoinFile{Path: path}
	now := time.Date(2026, 7, 29, 14, 0, 0, 0, time.UTC)
	principals := []string{
		"p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		"p1_jjkwa23wqggjpivnxdb45wpe575akea3eyytyr2slvuhg7ujsspq",
		"p1_n55ilee3u2y3zr6s3xuph7qjcqpsunkajnlgc3dxqkgzri5oxhca",
	}
	operationID := "rao1_11223344556677889900aabbccddeeff"
	transaction := deployment.RejoinTransaction{
		Version: deployment.RejoinTransactionVersion, Revision: 13,
		ManifestDigest: digest('a'), TargetSlot: "node-a",
		ChannelID:             "00112233445566778899aabbccddeeff",
		ExpectedPrincipalHash: digest('b'), ExpectedWakuPeerIDHash: digest('c'),
		ExpectedImageHash: digest('d'), Actor: principals[0],
		RequestID: "rejoin-file-ambiguous", StartedAt: now,
		Deadline: now.Add(time.Minute), FenceRequestID: "fence-file-ambiguous",
		FenceEvidenceDigest: digest('e'),
		RemovalOperationID:  "rao1_00112233445566778899aabbccddeeff",
		RemovalGeneration:   4, RemovalCheckpointDigest: digest('f'),
		Phase:           deployment.RejoinPhaseTargetAcknowledgementPending,
		ClockObservedAt: now, ClockSkewSecond: 7, TargetObserved: true,
		Attestations: map[string]deployment.RejoinAttestation{
			principals[0]: {RecipientPrincipal: principals[0], Digest: digest('1')},
			principals[1]: {RecipientPrincipal: principals[1], Digest: digest('2')},
			principals[2]: {RecipientPrincipal: principals[2], Digest: digest('3')},
		},
		OperationID: operationID, Generation: 5,
		Deliveries: map[string]deployment.RejoinDelivery{
			principals[0]: {
				RecipientPrincipal: principals[0],
				DeliveryID:         "rad1_00000000000000000000000000000001",
				EnvelopeDigest:     digest('4'),
			},
			principals[1]: {
				RecipientPrincipal: principals[1],
				DeliveryID:         "rad1_00000000000000000000000000000002",
				EnvelopeDigest:     digest('5'),
			},
			principals[2]: {
				RecipientPrincipal: principals[2],
				DeliveryID:         "rad1_00000000000000000000000000000003",
				EnvelopeDigest:     digest('6'),
			},
		},
		DeliveryReceipts: map[string]string{
			principals[0]: digest('7'),
			principals[1]: digest('8'),
			principals[2]: digest('9'),
		},
		PrepareCheckpointDigest:    digest('a'),
		ActivationCheckpointDigest: digest('b'),
		RepositoryPersisted:        true,
		SurvivorReceipts: map[string]string{
			"node-b": digest('c'), "node-c": digest('d'),
		},
		RestorationApplied: true, ReadinessVerified: true,
	}
	require.NoError(t, deployment.ValidateRejoinTransaction(transaction))
	raw, err := json.Marshal(transaction)
	require.NoError(t, err)
	require.NoError(t, storage.AtomicWritePrivateFile(path, raw))
	persisted, found, err := store.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, transaction, persisted)

	recovery := transaction
	recovery.Revision++
	recovery.Phase = deployment.RejoinPhaseRecoveryRequired
	recovery.ResumeFrom = deployment.RejoinPhaseRestorationPending
	recovery.FailureReason = deployment.RejoinFailureTargetReceiptMismatch
	recovery.IsolationConfirmed = true
	recovery.RestorationApplied = false
	recovery.ReadinessVerified = false

	require.NoError(t, deployment.ValidateRejoinTransaction(recovery))
	require.True(t, deployment.ValidRejoinTransactionTransition(transaction, recovery))
	require.NoError(t, store.Save(context.Background(), transaction.Revision, recovery))
	loaded, found, err := store.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, recovery, loaded)
}
