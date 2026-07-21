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
	device    string
	publicKey string
}

type serviceTestKeyStore struct {
	privateKey string
}

func (s *serviceTestIdentityStore) LoadIdentity() (string, string, string) {
	return s.principal, s.device, s.publicKey
}

func (s *serviceTestIdentityStore) SaveIdentity(principal string, device string, publicKey string) error {
	s.principal = principal
	s.device = device
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

	store.principal = identityprincipal.DeriveID("p", public)
	store.device = identityprincipal.DeriveID("d", private.Seed())
	store.publicKey = base64.StdEncoding.EncodeToString(public)
	require.NoError(t, keys.Save(privateText))

	svc := NewService()
	summary, restored, err := svc.Ensure(store, keys)
	require.NoError(t, err)
	require.Equal(t, Summary{
		Principal: store.principal,
		Device:    store.device,
		PublicKey: store.publicKey,
	}, summary)
	require.Equal(t, "restored", svc.Source())
	require.Equal(t, "ready", svc.State())
	require.Equal(t, privateText, base64.StdEncoding.EncodeToString(restored))
}

func TestPrincipalFromPublicKeyRejectsInvalidInput(t *testing.T) {
	_, err := identityprincipal.FromPublicKey("not-base64")
	require.Error(t, err)
}
