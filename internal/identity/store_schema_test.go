package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	identitykeyring "ardents/internal/identity/keyring"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/storage"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func TestNodeIdentityStoreRoundTripAndRestore(t *testing.T) {
	dir := t.TempDir()
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	principal, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	encodedPublic := base64.StdEncoding.EncodeToString(public)
	store := NewStoreInDir(dir)
	require.NoError(t, store.SaveIdentity(principal.String(), encodedPublic))
	require.NoError(t, identitykeyring.NewKeyStoreInDir(dir).Save(base64.StdEncoding.EncodeToString(private)))

	reopened := NewStoreInDir(dir)
	require.NoError(t, reopened.Load())
	summary, restored, err := NewService().Ensure(reopened, identitykeyring.NewKeyStoreInDir(dir))
	require.NoError(t, err)
	require.Equal(t, principal.String(), summary.Principal)
	require.Equal(t, private, restored)
}

func TestNodeIdentityStoreRejectsLegacyUnknownAndDuplicateSchemas(t *testing.T) {
	for name, raw := range map[string][]byte{
		"legacy":      []byte(`{"identity":{"principal":"p_deadbeefdeadbeef","device":"d_deadbeefdeadbeef","public_key":"x"}}`),
		"fake_device": []byte(`{"schema_version":1,"identity":{"principal":"p1_test","device":"d1_same-seed","public_key":"x"}}`),
		"unknown":     []byte(`{"schema_version":1,"identity":{"principal":"x","public_key":"x"},"extra":true}`),
		"duplicate":   []byte(`{"schema_version":1,"schema_version":1,"identity":{"principal":"x","public_key":"x"}}`),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := storage.PathInDir(dir)
			db, err := bbolt.Open(path, 0o600, nil)
			require.NoError(t, err)
			require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
				bucket, err := tx.CreateBucket([]byte("node-runtime"))
				if err != nil {
					return err
				}
				return bucket.Put([]byte("state"), raw)
			}))
			require.NoError(t, db.Close())
			err = NewStoreInDir(dir).Load()
			require.Error(t, err)
		})
	}
}

func TestFreshNodeCreationUsesCanonicalPrincipalWithoutFakeDevice(t *testing.T) {
	dir := t.TempDir()
	store := NewStoreInDir(dir)
	keys := identitykeyring.NewKeyStoreInDir(dir)
	summary, _, err := NewService().Ensure(store, keys)
	require.NoError(t, err)
	_, err = identityprincipal.Parse(summary.Principal)
	require.NoError(t, err)
	var persisted struct {
		SchemaVersion uint32            `json:"schema_version"`
		Identity      persistedIdentity `json:"identity"`
	}
	found, err := storage.LoadJSONStrict(storage.PathInDir(dir), "node-runtime", "state", &persisted)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint32(1), persisted.SchemaVersion)
	raw, err := json.Marshal(persisted)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "device")
	require.NotContains(t, summary.Principal, "p_")
	require.Equal(t, filepath.Join(dir, "ardents.db"), storage.PathInDir(dir))
}
