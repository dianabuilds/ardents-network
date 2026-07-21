package identity_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"ardents/internal/identity"
	identitykeyring "ardents/internal/identity/keyring"
	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

type keystoreTestIdentityStore struct {
	principal string
	device    string
	publicKey string
}

func (s *keystoreTestIdentityStore) LoadIdentity() (string, string, string) {
	return s.principal, s.device, s.publicKey
}

func (s *keystoreTestIdentityStore) SaveIdentity(principal string, device string, publicKey string) error {
	s.principal = principal
	s.device = device
	s.publicKey = publicKey
	return nil
}

func TestKeyStoreLoadRejectsCorruptLedger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity_key.json")
	{
		err := os.WriteFile(path, []byte("{not-json"), 0o600)
		require.NoErrorf(t, err, "write corrupt ledger: %v", err)
	}

	keys := identitykeyring.NewKeyStoreInDir(dir)
	{
		_, err := keys.Load()
		require.Error(t, err, "expected corrupt keystore load to fail")
	}
}

func TestEnsureRestoresPrivateKeyFromKeyStore(t *testing.T) {
	dir := t.TempDir()
	store := &keystoreTestIdentityStore{}
	keys := identitykeyring.NewKeyStoreInDir(dir)

	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privateText := base64.StdEncoding.EncodeToString(private)

	store.principal = identityprincipal.DeriveID("p", public)
	store.device = identityprincipal.DeriveID("d", private.Seed())
	store.publicKey = base64.StdEncoding.EncodeToString(public)
	require.NoError(t, keys.Save(privateText))

	svc := identity.NewService()
	summary, restored, err := svc.Ensure(store, keys)
	require.NoErrorf(t, err, "ensure identity: %v", err)
	require.Equal(t, store.principal, summary.Principal)
	require.Equal(t, store.device, summary.Device)
	require.Equal(t, store.publicKey, summary.PublicKey)
	{
		got := base64.StdEncoding.EncodeToString(restored)
		require.Falsef(t, got != privateText, "private key = %q, want migrated key", got)
	}

	stored, err := keys.Load()
	require.NoErrorf(t, err, "load migrated keystore: %v", err)
	require.Falsef(t, stored != privateText, "stored key = %q, want restored key", stored)
}

func TestEnsureRejectsIdentityStateWithoutMatchingKey(t *testing.T) {
	store := &keystoreTestIdentityStore{
		principal: "p_retained",
		device:    "d_retained",
		publicKey: "retained",
	}
	keys := identitykeyring.NewKeyStoreInDir(t.TempDir())

	_, _, err := identity.NewService().Ensure(store, keys)
	require.ErrorContains(t, err, "restore matching state and key backup")
	require.Equal(t, "p_retained", store.principal)
}

func TestEnsureRejectsKeyWithoutMatchingIdentityState(t *testing.T) {
	keys := identitykeyring.NewKeyStoreInDir(t.TempDir())
	require.NoError(t, keys.Save(base64.StdEncoding.EncodeToString(make([]byte, ed25519.PrivateKeySize))))

	_, _, err := identity.NewService().Ensure(&keystoreTestIdentityStore{}, keys)
	require.ErrorContains(t, err, "restore matching state and key backup")
}

func TestEnsureRejectsMismatchedIdentityKeyPair(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &keystoreTestIdentityStore{
		principal: identityprincipal.DeriveID("p", public),
		device:    identityprincipal.DeriveID("d", private.Seed()),
		publicKey: base64.StdEncoding.EncodeToString(otherPublic),
	}
	keys := identitykeyring.NewKeyStoreInDir(t.TempDir())
	require.NoError(t, keys.Save(base64.StdEncoding.EncodeToString(private)))

	_, _, err = identity.NewService().Ensure(store, keys)
	require.ErrorContains(t, err, "does not match")
}
