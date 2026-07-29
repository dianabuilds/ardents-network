package deploymentjournal

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ardents/internal/deployment"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestRolloutFilePersistsTransitionsAndClearsTerminalTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topology-rollout.json")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(path)))
	store := RolloutFile{Path: path}
	transaction := validRolloutTransaction()

	require.NoError(t, store.Save(context.Background(), 0, transaction))
	loaded, found, err := store.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, transaction, loaded)

	transaction.Revision++
	transaction.Phase = deployment.RolloutPhaseApplying
	require.NoError(t, store.Save(context.Background(), 1, transaction))
	transaction.Revision++
	transaction.Nodes = append(transaction.Nodes, deployment.RolloutNodeTransaction{
		Slot: "node-b", TargetImage: rolloutTargetImage(),
		FallbackImage: rolloutFallbackImage(),
		Phase:         deployment.RolloutNodeMutationPending,
	})
	require.NoError(t, store.Save(context.Background(), 2, transaction))

	transaction.Revision++
	transaction.Phase = deployment.RolloutPhaseCompensating
	transaction.FailureReason = deployment.RolloutReasonReadinessUnavailable
	transaction.Nodes[0].Phase = deployment.RolloutNodeCompensating
	require.NoError(t, store.Save(context.Background(), 3, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeFallbackRecreated
	require.NoError(t, store.Save(context.Background(), 4, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeFallbackStartPending
	require.NoError(t, store.Save(context.Background(), 5, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeFallbackStarted
	require.NoError(t, store.Save(context.Background(), 6, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeRestored
	require.NoError(t, store.Save(context.Background(), 7, transaction))
	transaction.Revision++
	transaction.Phase = deployment.RolloutPhaseCompensated
	require.NoError(t, store.Save(context.Background(), 8, transaction))
	require.NoError(t, store.Clear(context.Background(), transaction))
	_, found, err = store.Load(context.Background())
	require.NoError(t, err)
	require.False(t, found)
}

func TestRolloutFileRejectsConcurrentRevisionAndRebinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topology-rollout.json")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(path)))
	store := RolloutFile{Path: path}
	transaction := validRolloutTransaction()
	require.NoError(t, store.Save(context.Background(), 0, transaction))
	transaction.Revision++
	transaction.Phase = deployment.RolloutPhaseApplying
	require.NoError(t, store.Save(context.Background(), 1, transaction))
	transaction.Revision++
	transaction.Nodes = append(transaction.Nodes, deployment.RolloutNodeTransaction{
		Slot: "node-b", TargetImage: rolloutTargetImage(),
		FallbackImage: rolloutFallbackImage(),
		Phase:         deployment.RolloutNodeMutationPending,
	})
	require.NoError(t, store.Save(context.Background(), 2, transaction))

	first := transaction
	first.Revision++
	first.Nodes = append([]deployment.RolloutNodeTransaction(nil), first.Nodes...)
	first.Nodes[0].Phase = deployment.RolloutNodeRecreated
	second := transaction
	second.Revision++
	second.Nodes = append([]deployment.RolloutNodeTransaction(nil), second.Nodes...)
	second.Phase = deployment.RolloutPhaseCompensating
	second.FailureReason = deployment.RolloutReasonRecreateUnavailable
	second.Nodes[0].Phase = deployment.RolloutNodeCompensating
	results := make(chan error, 2)
	var group sync.WaitGroup
	for _, candidate := range []deployment.RolloutTransaction{first, second} {
		group.Add(1)
		go func(value deployment.RolloutTransaction) {
			defer group.Done()
			results <- store.Save(context.Background(), 3, value)
		}(candidate)
	}
	group.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, deployment.ErrRolloutJournalConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent save error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	loaded, found, err := store.Load(context.Background())
	require.NoError(t, err)
	require.True(t, found)
	rebound := loaded
	rebound.Revision++
	rebound.RequestID = "different-request"
	require.ErrorIs(
		t,
		store.Save(context.Background(), loaded.Revision, rebound),
		deployment.ErrRolloutJournalBinding,
	)
}

func TestRolloutFileRejectsUnknownOversizedAndNonPrivateState(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(dir))
	path := filepath.Join(dir, "topology-rollout.json")
	store := RolloutFile{Path: path}
	require.NoError(t, storage.AtomicWritePrivateFile(path, []byte(
		`{"version":"topology-rollout-transaction/v1","unknown":true}`,
	)))
	_, _, err := store.Load(context.Background())
	require.ErrorIs(t, err, deployment.ErrRolloutJournalInvalid)

	require.NoError(t, storage.AtomicWritePrivateFile(
		path,
		make([]byte, deployment.MaxRolloutJournalBytes+1),
	))
	_, _, err = store.Load(context.Background())
	require.ErrorIs(t, err, deployment.ErrRolloutJournalInvalid)
}

func validRolloutTransaction() deployment.RolloutTransaction {
	started := time.Date(2026, 7, 29, 16, 0, 0, 0, time.UTC)
	return deployment.RolloutTransaction{
		Version: deployment.RolloutTransactionVersion, Revision: 1,
		ManifestDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RequestID:      "rollout-file-test",
		CompatibilityDigest: "sha256:" +
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ChangeKind: deployment.AuthorityChangeCompatible,
		StartedAt:  started, Deadline: started.Add(20 * time.Minute),
		Order: []string{"node-b", "node-c", "node-a"},
		Phase: deployment.RolloutPhasePreflighted,
		Nodes: []deployment.RolloutNodeTransaction{},
	}
}

func rolloutTargetImage() string {
	return "registry.example/ardents/node@sha256:" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
}

func rolloutFallbackImage() string {
	return "registry.example/ardents/node@sha256:" +
		"dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
}
