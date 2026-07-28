package provision

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	apppolicy "ardents/internal/policy"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestBuildLocalV2MigrationEvidenceReadsProtectedNodeStoreAndHoldsFence(t *testing.T) {
	root := t.TempDir()
	authorityDir := filepath.Join(root, "authority")
	legacy, err := OpenOrCreate(authorityDir)
	require.NoError(t, err)
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	node := NodeOptions{
		DataDir: filepath.Join(root, "node"), SecretDir: filepath.Join(root, "secret"),
		Clock: func() time.Time { return now },
	}
	_, err = legacy.ProvisionNode(node, apppolicy.New(apppolicy.Config{}))
	require.NoError(t, err)
	original := filepath.Join(authorityDir, "authority.json")
	backup := filepath.Join(authorityDir, "authority.backup.json")
	require.NoError(t, os.Rename(original, backup))

	evidence, err := BuildLocalV2MigrationEvidence(LocalV2MigrationSource{
		RequestID: "offline-migration", AuthorityBackupPath: backup,
		OriginalAuthorityPath: original, OldManagerStateDir: authorityDir,
		Nodes: []NodeOptions{node},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, evidence.Close()) })
	require.Len(t, evidence.Request.Members, 1)
	require.Len(t, evidence.Request.Members[0].ReceiverGrants, 2)
	require.True(t, evidence.Request.OldManagerFence.OldProcessStopped)
	require.True(t, evidence.Request.OldManagerFence.SharedAuthorityRemoved)
	require.NotEmpty(t, evidence.Request.OldManagerFence.EvidenceDigest)
	_, err = storage.AcquireStateDirLock(authorityDir)
	require.Error(t, err, "migration evidence must retain exclusive old-manager ownership")
}

func TestBuildLocalV2MigrationEvidenceRejectsUnfencedOrWrongKeyState(t *testing.T) {
	root := t.TempDir()
	authorityDir := filepath.Join(root, "authority")
	legacy, err := OpenOrCreate(authorityDir)
	require.NoError(t, err)
	node := NodeOptions{
		DataDir: filepath.Join(root, "node"), SecretDir: filepath.Join(root, "secret"),
	}
	_, err = legacy.ProvisionNode(node, apppolicy.New(apppolicy.Config{}))
	require.NoError(t, err)
	original := filepath.Join(authorityDir, "authority.json")
	backup := filepath.Join(authorityDir, "authority.backup.json")

	_, err = BuildLocalV2MigrationEvidence(LocalV2MigrationSource{
		RequestID: "still-live", AuthorityBackupPath: original,
		OriginalAuthorityPath: original, OldManagerStateDir: authorityDir,
		Nodes: []NodeOptions{node},
	})
	require.Error(t, err)

	require.NoError(t, os.Rename(original, backup))
	require.NoError(t, storage.AtomicWritePrivateFile(
		filepath.Join(node.SecretDir, "channel-grant-store.key"),
		make([]byte, 32),
	))
	_, err = BuildLocalV2MigrationEvidence(LocalV2MigrationSource{
		RequestID: "wrong-key", AuthorityBackupPath: backup,
		OriginalAuthorityPath: original, OldManagerStateDir: authorityDir,
		Nodes: []NodeOptions{node},
	})
	require.Error(t, err)
}
