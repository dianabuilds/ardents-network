package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"

	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

type serviceTestIdentityStore struct {
	principal string
	publicKey string
}

type serviceTestKeyStore struct {
	privateKey string
}

func (s *serviceTestIdentityStore) LoadIdentity() (string, string) {
	return s.principal, s.publicKey
}

func (s *serviceTestIdentityStore) SaveIdentity(principal string, publicKey string) error {
	s.principal = principal
	s.publicKey = publicKey
	return nil
}

func (s *serviceTestKeyStore) Load() (string, error) {
	return s.privateKey, nil
}

func (s *serviceTestKeyStore) Save(privateKey string) error {
	s.privateKey = privateKey
	return nil
}

func TestEnsureReportsRestoredSourceAndReadyState(t *testing.T) {
	dir := t.TempDir()
	store := &serviceTestIdentityStore{}
	_ = dir
	keys := &serviceTestKeyStore{}

	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privateText := base64.StdEncoding.EncodeToString(private)

	store.principal = testPrincipalID(t, public)
	store.publicKey = base64.StdEncoding.EncodeToString(public)
	require.NoError(t, keys.Save(privateText))

	svc := NewService()
	summary, restored, err := svc.Ensure(store, keys)
	require.NoError(t, err)
	require.Equal(t, Summary{
		Principal: store.principal,
		PublicKey: store.publicKey,
	}, summary)
	require.Equal(t, "restored", svc.Source())
	require.Equal(t, "ready", svc.State())
	require.Equal(t, privateText, base64.StdEncoding.EncodeToString(restored))
}

func testPrincipalID(t *testing.T, public ed25519.PublicKey) string {
	t.Helper()
	id, err := identityprincipal.FromEd25519PublicKey(public)
	require.NoError(t, err)
	return id.String()
}

func TestPrincipalFromPublicKeyRejectsInvalidInput(t *testing.T) {
	_, err := identityprincipal.FromPublicKey("not-base64")
	require.Error(t, err)
}
