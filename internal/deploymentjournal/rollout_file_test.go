package deploymentjournal

import (
	"context"
	"errors"
	"os"
	"os/exec"
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
	require.NoError(t, store.Save(context.Background(), 3, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeCompensating
	require.NoError(t, store.Save(context.Background(), 4, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeFallbackRecreated
	require.NoError(t, store.Save(context.Background(), 5, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeFallbackStartPending
	require.NoError(t, store.Save(context.Background(), 6, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeFallbackStarted
	require.NoError(t, store.Save(context.Background(), 7, transaction))
	transaction.Revision++
	transaction.Nodes[0].Phase = deployment.RolloutNodeRestored
	require.NoError(t, store.Save(context.Background(), 8, transaction))
	transaction.Revision++
	transaction.Phase = deployment.RolloutPhaseCompensated
	require.NoError(t, store.Save(context.Background(), 9, transaction))
	require.NoError(t, store.Clear(context.Background(), transaction))
	_, found, err = store.Load(context.Background())
	require.NoError(t, err)
	require.False(t, found)
}

func TestRolloutFileRejectsSkippedDurableBoundaries(t *testing.T) {
	base := validRolloutTransaction()
	base.Phase = deployment.RolloutPhaseApplying
	base.Revision = 2

	t.Run("new node must begin mutation pending", func(t *testing.T) {
		after := base
		after.Revision++
		after.Nodes = []deployment.RolloutNodeTransaction{{
			Slot: "node-b", TargetImage: rolloutTargetImage(),
			FallbackImage: rolloutFallbackImage(),
			Phase:         deployment.RolloutNodeApplied,
		}}
		require.False(t, deployment.ValidRolloutTransactionTransition(base, after))
	})

	t.Run("one revision changes at most one node boundary", func(t *testing.T) {
		before := base
		before.Nodes = []deployment.RolloutNodeTransaction{
			{
				Slot: "node-b", TargetImage: rolloutTargetImage(),
				FallbackImage: rolloutFallbackImage(),
				Phase:         deployment.RolloutNodeApplied,
			},
			{
				Slot: "node-c", TargetImage: rolloutTargetImage(),
				FallbackImage: rolloutFallbackImage(),
				Phase:         deployment.RolloutNodeMutationPending,
			},
		}
		before.Revision = 8
		after := before
		after.Revision++
		after.Nodes = append([]deployment.RolloutNodeTransaction(nil), before.Nodes...)
		after.Nodes[0].Phase = deployment.RolloutNodeCompensating
		after.Nodes[1].Phase = deployment.RolloutNodeRecreated
		after.Phase = deployment.RolloutPhaseCompensating
		after.FailureReason = deployment.RolloutReasonInterrupted
		require.False(t, deployment.ValidRolloutTransactionTransition(before, after))
	})

	t.Run("activated authority truth is immutable", func(t *testing.T) {
		before := validMigrationRolloutTransaction()
		after := before
		after.Revision++
		after.AuthorityGeneration++
		require.False(t, deployment.ValidRolloutTransactionTransition(before, after))
	})
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

func TestRolloutFileRejectsCrossProcessRevisionRace(t *testing.T) {
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

	commands := []*exec.Cmd{
		exec.Command(os.Args[0], "-test.run=^TestRolloutFileProcessHelper$"),
		exec.Command(os.Args[0], "-test.run=^TestRolloutFileProcessHelper$"),
	}
	for index, command := range commands {
		command.Env = append(
			os.Environ(),
			"ARDENTS_ROLLOUT_FILE_HELPER=1",
			"ARDENTS_ROLLOUT_FILE_PATH="+path,
			"ARDENTS_ROLLOUT_FILE_CANDIDATE="+string(rune('a'+index)),
		)
		require.NoError(t, command.Start())
	}
	successes := 0
	conflicts := 0
	for _, command := range commands {
		err := command.Wait()
		if err == nil {
			successes++
			continue
		}
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr)
		if exitErr.ExitCode() == 20 {
			conflicts++
			continue
		}
		t.Fatalf("cross-process helper failed with exit code %d", exitErr.ExitCode())
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
}

func TestRolloutFileProcessHelper(t *testing.T) {
	if os.Getenv("ARDENTS_ROLLOUT_FILE_HELPER") != "1" {
		return
	}
	store := RolloutFile{Path: os.Getenv("ARDENTS_ROLLOUT_FILE_PATH")}
	transaction := validRolloutTransaction()
	transaction.Revision = 4
	transaction.Phase = deployment.RolloutPhaseApplying
	transaction.Nodes = []deployment.RolloutNodeTransaction{{
		Slot: "node-b", TargetImage: rolloutTargetImage(),
		FallbackImage: rolloutFallbackImage(),
		Phase:         deployment.RolloutNodeMutationPending,
	}}
	if os.Getenv("ARDENTS_ROLLOUT_FILE_CANDIDATE") == "a" {
		transaction.Nodes[0].Phase = deployment.RolloutNodeRecreated
	} else {
		transaction.Phase = deployment.RolloutPhaseCompensating
		transaction.FailureReason = deployment.RolloutReasonRecreateUnavailable
	}
	err := store.Save(context.Background(), 3, transaction)
	switch {
	case err == nil:
		os.Exit(0)
	case errors.Is(err, deployment.ErrRolloutJournalConflict):
		os.Exit(20)
	default:
		os.Exit(30)
	}
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

func validMigrationRolloutTransaction() deployment.RolloutTransaction {
	value := validRolloutTransaction()
	value.ChangeKind = deployment.AuthorityChangeMigration
	value.Phase = deployment.RolloutPhaseActivated
	value.Nodes = []deployment.RolloutNodeTransaction{
		{
			Slot: "node-b", TargetImage: rolloutTargetImage(),
			FallbackImage: rolloutFallbackImage(),
			Phase:         deployment.RolloutNodeApplied,
		},
		{
			Slot: "node-c", TargetImage: rolloutTargetImage(),
			FallbackImage: rolloutFallbackImage(),
			Phase:         deployment.RolloutNodeApplied,
		},
		{
			Slot: "node-a", TargetImage: rolloutTargetImage(),
			FallbackImage: rolloutFallbackImage(),
			Phase:         deployment.RolloutNodeApplied,
		},
	}
	value.AuthorityGeneration = 2
	value.CheckpointDigest = "sha256:" +
		"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	value.RepositoryPersisted = true
	value.ActiveReceiptCount = 3
	return value
}
