package provision

import (
	"io/fs"
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
	t.Cleanup(func() { require.NoError(t, legacy.Close()) })
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	node := NodeOptions{
		DataDir: filepath.Join(root, "node"), SecretDir: filepath.Join(root, "secret"),
		Clock: func() time.Time { return now },
	}
	_, err = legacy.ProvisionNode(node, apppolicy.New(apppolicy.Config{}))
	require.NoError(t, err)
	require.NoError(t, legacy.Close())
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
	_, err = OpenOrCreate(authorityDir)
	require.Error(t, err, "legacy manager restart must honor the same fence")
}

func TestBuildLocalV2MigrationEvidenceRejectsUnfencedOrWrongKeyState(t *testing.T) {
	root := t.TempDir()
	authorityDir := filepath.Join(root, "authority")
	legacy, err := OpenOrCreate(authorityDir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, legacy.Close()) })
	node := NodeOptions{
		DataDir: filepath.Join(root, "node"), SecretDir: filepath.Join(root, "secret"),
	}
	_, err = legacy.ProvisionNode(node, apppolicy.New(apppolicy.Config{}))
	require.NoError(t, err)
	original := filepath.Join(authorityDir, "authority.json")
	backup := filepath.Join(authorityDir, "authority.backup.json")
	require.NoError(t, os.Rename(original, backup))

	_, err = BuildLocalV2MigrationEvidence(LocalV2MigrationSource{
		RequestID: "still-live", AuthorityBackupPath: backup,
		OriginalAuthorityPath: original, OldManagerStateDir: authorityDir,
		Nodes: []NodeOptions{node},
	})
	require.Error(t, err, "live legacy manager must retain the shared state lock")

	require.NoError(t, legacy.Close())
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

func TestLocalV2DowngradeRestoresCompleteStoppedBackup(t *testing.T) {
	liveRoot := filepath.Join(t.TempDir(), "live")
	backupRoot := filepath.Join(t.TempDir(), "backup")
	authorityDir := filepath.Join(liveRoot, "authority")
	node := NodeOptions{
		DataDir:   filepath.Join(liveRoot, "node"),
		SecretDir: filepath.Join(liveRoot, "secret"),
	}
	legacy, err := OpenOrCreate(authorityDir)
	require.NoError(t, err)
	before, err := legacy.ProvisionNode(node, apppolicy.New(apppolicy.Config{}))
	require.NoError(t, err)
	require.NoError(t, legacy.Close())
	require.NoError(t, copyLocalV2Tree(liveRoot, backupRoot))

	original := filepath.Join(authorityDir, "authority.json")
	authorityBackup := filepath.Join(authorityDir, "authority.migration.json")
	require.NoError(t, os.Rename(original, authorityBackup))
	evidence, err := BuildLocalV2MigrationEvidence(LocalV2MigrationSource{
		RequestID: "downgrade-drill", AuthorityBackupPath: authorityBackup,
		OriginalAuthorityPath: original, OldManagerStateDir: authorityDir,
		Nodes: []NodeOptions{node},
	})
	require.NoError(t, err)
	require.NoError(t, evidence.Close())

	require.NoError(t, os.RemoveAll(liveRoot))
	require.NoError(t, copyLocalV2Tree(backupRoot, liveRoot))
	restored, err := OpenOrCreate(filepath.Join(liveRoot, "authority"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, restored.Close()) })
	after, err := restored.ProvisionNode(NodeOptions{
		DataDir:   filepath.Join(liveRoot, "node"),
		SecretDir: filepath.Join(liveRoot, "secret"),
	}, apppolicy.New(apppolicy.Config{}))
	require.NoError(t, err)
	require.Equal(t, before.Subject, after.Subject)
	require.Equal(t, before.Issuer, after.Issuer)
	require.Equal(t, before.DiscoveryRef, after.DiscoveryRef)
	require.Equal(t, before.DataRef, after.DataRef)
}

func copyLocalV2Tree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.Name() == ".ardents-state.lock" {
			return nil
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return storage.AtomicWritePrivateFile(destination, raw)
	})
}
