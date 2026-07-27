package authority

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestFileCheckpointRepositoryCreateReadAndExactCompareAppend(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(root))
	repository, err := NewFileCheckpointRepository(root)
	require.NoError(t, err)
	signer := newTestSigner(t, 0x55)
	genesis := signedCheckpointFixture(t, signer, "r1_00112233445566778899aabbccddeeff", 1, "")

	created, err := repository.CreateIfAbsent(ctx, genesis)
	require.NoError(t, err)
	require.Equal(t, genesis, created)
	head, found, err := repository.ReadHead(ctx, genesis.RealmID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, genesis, head)

	_, err = repository.CreateIfAbsent(ctx, genesis)
	require.ErrorIs(t, err, ErrConflict)

	next := signedCheckpointFixture(t, signer, genesis.RealmID, 2, genesis.Digest)
	appended, err := repository.CompareAndAppend(ctx, genesis.RealmID, 1, next)
	require.NoError(t, err)
	require.Equal(t, next, appended)
	head, found, err = repository.ReadHead(ctx, genesis.RealmID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, next, head)
}

func TestFileCheckpointRepositoryRejectsStaleSkipForkAndBlindReplacement(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(root))
	repository, err := NewFileCheckpointRepository(root)
	require.NoError(t, err)
	signer := newTestSigner(t, 0x56)
	realmID := "r1_00112233445566778899aabbccddeeff"
	genesis := signedCheckpointFixture(t, signer, realmID, 1, "")
	_, err = repository.CreateIfAbsent(ctx, genesis)
	require.NoError(t, err)

	tests := []struct {
		name     string
		expected uint64
		next     SignedCheckpoint
	}{
		{name: "stale expected sequence", expected: 0, next: signedCheckpointFixture(t, signer, realmID, 2, genesis.Digest)},
		{name: "skip", expected: 1, next: signedCheckpointFixture(t, signer, realmID, 3, genesis.Digest)},
		{name: "fork", expected: 1, next: signedCheckpointFixture(t, signer, realmID, 2, "ac1_"+string(bytes.Repeat([]byte{'0'}, 64)))},
		{name: "blind replacement", expected: 2, next: signedCheckpointFixture(t, signer, realmID, 2, genesis.Digest)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := repository.CompareAndAppend(ctx, realmID, test.expected, test.next)
			require.ErrorIs(t, err, ErrConflict)
		})
	}
}

func TestCheckpointValidationRejectsMalformedDigestAndSignature(t *testing.T) {
	signer := newTestSigner(t, 0x57)
	checkpoint := signedCheckpointFixture(t, signer, "r1_00112233445566778899aabbccddeeff", 1, "")

	badDigest := checkpoint
	badDigest.Digest = "ac1_" + string(bytes.Repeat([]byte{'0'}, 64))
	require.ErrorIs(t, ValidateCheckpoint(badDigest), ErrCorruptState)

	badSignature := checkpoint
	badSignature.Signature = append([]byte(nil), checkpoint.Signature...)
	badSignature.Signature[0] ^= 0xff
	require.ErrorIs(t, ValidateCheckpoint(badSignature), ErrCorruptState)
}

func TestCheckpointSerializationVector(t *testing.T) {
	signer := newTestSigner(t, 0x55)
	checkpoint := signedCheckpointFixture(t, signer, "r1_00112233445566778899aabbccddeeff", 1, "")
	actual, err := json.Marshal(checkpoint)
	require.NoError(t, err)
	expected, err := os.ReadFile("testdata/checkpoint-genesis-v1.json")
	require.NoError(t, err)
	require.Equal(t, string(bytes.TrimSpace(expected)), string(actual))
}

func TestWORMCheckpointRepositoryRequiresIndependentProvisioningAssertion(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(root))

	_, err := NewWORMFileCheckpointRepository(root)
	require.ErrorIs(t, err, ErrUnavailable)

	require.NoError(t, storage.AtomicCreatePrivateFile(
		filepath.Join(root, wormMarkerFile),
		[]byte(`{"version":1,"retention":"worm","administration":"independent"}`),
	))
	repository, err := NewWORMFileCheckpointRepository(root)
	require.NoError(t, err)
	require.NotNil(t, repository)
}

func TestCheckpointRepositoryRejectsSymlinkRoot(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	require.NoError(t, storage.EnsurePrivateDir(target))
	link := filepath.Join(parent, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := NewFileCheckpointRepository(link)
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestCheckpointRepositoryRejectsSymlinkRealmDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(root))
	repository, err := NewFileCheckpointRepository(root)
	require.NoError(t, err)
	target := filepath.Join(t.TempDir(), "target")
	require.NoError(t, storage.EnsurePrivateDir(target))
	realmID := "r1_00112233445566778899aabbccddeeff"
	if err := os.Symlink(target, filepath.Join(root, realmID)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	signer := newTestSigner(t, 0x58)
	checkpoint := signedCheckpointFixture(t, signer, realmID, 1, "")
	_, err = repository.CreateIfAbsent(context.Background(), checkpoint)
	require.ErrorIs(t, err, ErrCorruptState)
}

func signedCheckpointFixture(t *testing.T, signer *testSigner, realmID string, sequence uint64, previous string) SignedCheckpoint {
	t.Helper()
	body := Checkpoint{
		Version: ContractVersion, SchemaVersion: SchemaVersion,
		RealmID: realmID, AuthorityPrincipal: signer.principal,
		AuthorityPublicKey: append([]byte(nil), signer.private.Public().(ed25519.PublicKey)...),
		AuthorityEpoch:     1, AuthoritySequence: sequence, PreviousDigest: previous,
		AuditHead: "aa1_0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CreatedAt: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC),
	}
	signed, err := SignCheckpoint(context.Background(), signer, body)
	require.NoError(t, err)
	return signed
}
