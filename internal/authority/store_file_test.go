package authority

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

func TestFileStoreEncryptsAndReloadsOneAtomicLedger(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "realm-authority.db")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(path)))
	key := bytes.Repeat([]byte{0x71}, AuthorityStoreKeyBytes)
	store, err := OpenFileStore(ctx, path, key)
	require.NoError(t, err)
	fixture := newServiceFixture(t)
	_, err = fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	state := fixture.store.state
	require.NoError(t, store.Create(ctx, state))
	require.NoError(t, store.Close())

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(raw), state.AuthorityPrincipal)
	require.NotContains(t, string(raw), state.RealmID)
	require.NotContains(t, string(raw), state.AuditOutbox[0].Actor)

	reopened, err := OpenFileStore(ctx, path, key)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	loaded, found, err := reopened.Load(ctx)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, state, loaded)
}

func TestFileStoreRejectsWrongKeyAndUnsupportedOrUnknownState(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "realm-authority.db")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(path)))
	key := bytes.Repeat([]byte{0x72}, AuthorityStoreKeyBytes)
	store, err := OpenFileStore(ctx, path, key)
	require.NoError(t, err)
	fixture := newServiceFixture(t)
	_, err = fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	require.NoError(t, store.Create(ctx, fixture.store.state))
	require.NoError(t, store.Close())

	wrong, err := OpenFileStore(ctx, path, bytes.Repeat([]byte{0x73}, AuthorityStoreKeyBytes))
	require.NoError(t, err)
	_, _, err = wrong.Load(ctx)
	require.ErrorIs(t, err, ErrCorruptState)
	require.NoError(t, wrong.Close())

	raw, err := json.Marshal(fixture.store.state)
	require.NoError(t, err)
	raw = bytes.TrimSuffix(raw, []byte("}"))
	raw = append(raw, []byte(`,"unknown":"rejected"}`)...)
	_, err = decodeLedger(raw)
	require.ErrorIs(t, err, ErrCorruptState)

	unsupported := fixture.store.state
	unsupported.SchemaVersion = SchemaVersion + 1
	raw, err = json.Marshal(unsupported)
	require.NoError(t, err)
	_, err = decodeLedger(raw)
	require.ErrorIs(t, err, ErrUnsupportedVersion)
}

func TestFileStoreCreateAndRevisionCompareAreFailClosed(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "realm-authority.db")
	require.NoError(t, storage.EnsurePrivateDir(filepath.Dir(path)))
	store, err := OpenFileStore(ctx, path, bytes.Repeat([]byte{0x74}, AuthorityStoreKeyBytes))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	fixture := newServiceFixture(t)
	_, err = fixture.service.CreateOrReopen(ctx, fixture.createCommand(), CreateRequest{
		Version: ContractVersion, RequestID: "request-001", RealmClass: RealmClassProduction,
	})
	require.NoError(t, err)
	state := fixture.store.state
	require.NoError(t, store.Create(ctx, state))
	require.ErrorIs(t, store.Create(ctx, state), ErrConflict)

	next := state
	next.Revision++
	require.ErrorIs(t, store.Save(ctx, state.Revision+1, next), ErrConflict)
	require.NoError(t, store.Save(ctx, state.Revision, next))
}

func TestFileStoreRejectsSymlinkDatabase(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, storage.EnsurePrivateDir(root))
	target := filepath.Join(root, "target.db")
	require.NoError(t, storage.AtomicCreatePrivateFile(target, nil))
	link := filepath.Join(root, "realm-authority.db")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	store, err := OpenFileStore(
		context.Background(), link,
		bytes.Repeat([]byte{0x75}, AuthorityStoreKeyBytes),
	)
	require.Nil(t, store)
	require.ErrorIs(t, err, ErrUnavailable)
}
