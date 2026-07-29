package deploymentjournal

import (
	"context"
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
