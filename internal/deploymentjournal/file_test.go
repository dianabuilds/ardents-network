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

func TestFenceFileRoundTripRejectsUnknownAndBindingOverwrite(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "private")
	require.NoError(t, storage.EnsurePrivateDir(directory))
	path := filepath.Join(directory, "fence.json")
	store := FenceFile{Path: path}
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	transaction := deployment.FenceTransaction{
		Version: deployment.FenceTransactionVersion, Revision: 1,
		ManifestDigest:         digest('a'),
		TargetSlot:             "node-a",
		ExpectedPrincipalHash:  digest('b'),
		ExpectedWakuPeerIDHash: digest('c'),
		Actor:                  "p1_euydwrsrlrtxe7misopktnf7zlk6b27waegboirnhbbu4wlen55a",
		RequestID:              "fence-file-journal",
		Reason:                 deployment.FenceReasonMembershipRemoved,
		StartedAt:              now,
		Deadline:               now.Add(time.Minute),
		Phase:                  deployment.FencePhaseRequested,
	}
	require.NoError(t, deployment.ValidateFenceTransaction(transaction))
	require.NoError(t, storage.ValidatePrivateDir(filepath.Dir(path)))
	require.NoError(t, store.Save(context.Background(), 0, transaction))
	loaded, found, err := store.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, transaction, loaded)

	next := transaction
	next.Revision = 2
	next.Phase = deployment.FencePhaseIsolationPending
	next.ClockObservedAt = now
	next.ClockSkewSecond = 7
	require.NoError(t, store.Save(context.Background(), transaction.Revision, next))
	loaded, found, err = store.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, next, loaded)

	withControls := next
	withControls.Revision = 3
	withControls.IsolationControls = []deployment.FenceControlReceipt{
		{Kind: deployment.FenceControlTargetIngressBlocked, Actor: transaction.Actor, ReceiptDigest: digest('d')},
		{Kind: deployment.FenceControlDiscoveryWithdrawn, Actor: transaction.Actor, ReceiptDigest: digest('e')},
		{Kind: deployment.FenceControlPeerIDDenied, Actor: transaction.Actor, ReceiptDigest: digest('f')},
	}
	require.NoError(t, store.Save(context.Background(), next.Revision, withControls))

	other := withControls
	other.Revision = 4
	other.TargetSlot = "node-b"
	require.ErrorIs(
		t,
		store.Save(context.Background(), withControls.Revision, other),
		deployment.ErrFenceJournalBinding,
	)

	require.NoError(
		t,
		os.WriteFile(
			path,
			[]byte(`{"version":"topology-fence-transaction/v1","unknown":true}`),
			0o600,
		),
	)
	_, _, err = store.Load(context.Background())
	require.ErrorIs(t, err, deployment.ErrFenceJournalInvalid)
}

func digest(value byte) string {
	raw := make([]byte, 64)
	for index := range raw {
		raw[index] = value
	}
	return "sha256:" + string(raw)
}
