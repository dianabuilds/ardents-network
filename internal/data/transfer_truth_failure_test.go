package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartTransferRestoresMemoryWhenPersistenceFails(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "database-directory"))
	require.NoError(t, makeDirectoryAtDatabasePath(service.path))

	_, err := service.StartTransfer(TransferRecord{ID: "xfer-start", Kind: "blob_fetch", State: "pending"})

	require.Error(t, err)
	_, exists := service.GetTransfer("xfer-start")
	require.False(t, exists)
}

func TestTransferTransitionRestoresPreviousTruthWhenPersistenceFails(t *testing.T) {
	service := New("")
	started, err := service.StartTransfer(TransferRecord{ID: "xfer-finish", Kind: "blob_fetch", State: "pending"})
	require.NoError(t, err)
	service.path = filepath.Join(t.TempDir(), "database-directory")
	require.NoError(t, makeDirectoryAtDatabasePath(service.path))

	_, err = service.CompleteTransfer(started.ID, "peer-1", 42, "done")
	require.Error(t, err)
	current, exists := service.GetTransfer(started.ID)
	require.True(t, exists)
	require.Equal(t, "pending", current.State)
	require.Zero(t, current.ProgressBytes)
	require.Nil(t, current.FinishedAt)
}

func TestStartTransferRejectsDuplicateIDWithoutRewritingHistory(t *testing.T) {
	service := New("")
	first, err := service.StartTransfer(TransferRecord{
		ID: "xfer-duplicate", Kind: "blob_fetch", ResourceID: "blob-original", State: "pending",
	})
	require.NoError(t, err)

	_, err = service.StartTransfer(TransferRecord{
		ID: "xfer-duplicate", Kind: "manifest_fetch", ResourceID: "manifest-replacement", State: "failed",
	})

	require.ErrorContains(t, err, "already exists")
	current, exists := service.GetTransfer(first.ID)
	require.True(t, exists)
	require.Equal(t, first, current)
}

func makeDirectoryAtDatabasePath(path string) error {
	return os.MkdirAll(path, 0o700)
}
