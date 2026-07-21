package transfer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartTransferRestoresMemoryWhenPersistenceFails(t *testing.T) {
	journal := NewJournal(filepath.Join(t.TempDir(), "database-directory"))
	require.NoError(t, makeDirectoryAtDatabasePath(journal.path))

	_, err := journal.Start(Record{ID: "xfer-start", Kind: "blob_fetch", State: "pending"})

	require.Error(t, err)
	_, exists := journal.Get("xfer-start")
	require.False(t, exists)
}

func TestTransferTransitionRestoresPreviousTruthWhenPersistenceFails(t *testing.T) {
	journal := NewJournal("")
	started, err := journal.Start(Record{ID: "xfer-finish", Kind: "blob_fetch", State: "pending"})
	require.NoError(t, err)
	journal.path = filepath.Join(t.TempDir(), "database-directory")
	require.NoError(t, makeDirectoryAtDatabasePath(journal.path))

	_, err = journal.Complete(started.ID, "peer-1", 42, "done")
	require.Error(t, err)
	current, exists := journal.Get(started.ID)
	require.True(t, exists)
	require.Equal(t, "pending", current.State)
	require.Zero(t, current.ProgressBytes)
	require.Nil(t, current.FinishedAt)
}

func TestStartTransferRejectsDuplicateIDWithoutRewritingHistory(t *testing.T) {
	journal := NewJournal("")
	first, err := journal.Start(Record{
		ID: "xfer-duplicate", Kind: "blob_fetch", ResourceID: "blob-original", State: "pending",
	})
	require.NoError(t, err)

	_, err = journal.Start(Record{
		ID: "xfer-duplicate", Kind: "manifest_fetch", ResourceID: "manifest-replacement", State: "failed",
	})

	require.ErrorContains(t, err, "already exists")
	current, exists := journal.Get(first.ID)
	require.True(t, exists)
	require.Equal(t, first, current)
}

func makeDirectoryAtDatabasePath(path string) error {
	return os.MkdirAll(path, 0o700)
}
