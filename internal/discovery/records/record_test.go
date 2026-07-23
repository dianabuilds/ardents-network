package records_test

import (
	discoveryrecord "ardents/internal/discovery/records"
	identityapi "ardents/internal/identity"
	identitykeyring "ardents/internal/identity/keyring"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testIdentityStore struct {
	principal string
	publicKey string
}

func (s *testIdentityStore) LoadIdentity() (string, string) {
	return s.principal, s.publicKey
}

func (s *testIdentityStore) SaveIdentity(principal string, publicKey string) error {
	s.principal = principal
	s.publicKey = publicKey
	return nil
}

func TestValidateAcceptsSignedNodeRecord(t *testing.T) {
	record, _, err := signedNodeRecord(t)
	require.NoError(t, err)
	require.NoError(t, discoveryrecord.Validate(record))
}

func TestValidateRejectsTamperedSignature(t *testing.T) {
	record, key, err := signedNodeRecord(t)
	require.NoError(t, err)

	record.Device = "tampered"
	payload, err := discoveryrecord.Canonical(record)
	require.NoError(t, err)
	record.Signature = signRecord(key, payload[:len(payload)-1])

	require.Error(t, discoveryrecord.Validate(record))
}

func TestValidateRejectsCorrectlySignedNonCanonicalNodePrincipal(t *testing.T) {
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	invalid := "p_deadbeefdeadbeef"
	record := discoveryrecord.Record{ID: invalid + ":node", Kind: "node", Subject: invalid, Node: invalid, PublicKey: base64.StdEncoding.EncodeToString(public), IssuedAt: time.Now().UTC(), ExpiresAt: time.Now().UTC().Add(time.Hour)}
	payload, err := discoveryrecord.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
	require.Error(t, discoveryrecord.Validate(record))
}

func TestFreshnessUsesIssuedAtBeforeExpiresAt(t *testing.T) {
	now := time.Now().UTC()
	record := discoveryrecord.Record{IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	require.Equal(t, now.UnixNano(), discoveryrecord.Score(record))
}

func TestFreshnessFallsBackToExpiresAt(t *testing.T) {
	now := time.Now().UTC()
	record := discoveryrecord.Record{ExpiresAt: now}
	require.Equal(t, now.UnixNano(), discoveryrecord.Score(record))
}

func signedNodeRecord(t *testing.T) (discoveryrecord.Record, ed25519.PrivateKey, error) {
	t.Helper()

	st := &testIdentityStore{}
	ids := identityapi.NewService()
	summary, key, err := ids.EnsureNode(st, identitykeyring.NewKeyStoreInDir(t.TempDir()))
	if err != nil {
		return discoveryrecord.Record{}, nil, err
	}
	record := discoveryrecord.Record{
		ID:        summary.Principal + ":node",
		Kind:      "node",
		Subject:   summary.Principal,
		Node:      summary.Principal,
		PublicKey: summary.PublicKey,
		Endpoints: []string{"tcp://bootstrap"},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	payload, err := discoveryrecord.Canonical(record)
	if err != nil {
		return discoveryrecord.Record{}, nil, err
	}
	record.Signature = signRecord(key, payload)
	return record, key, nil
}

func signRecord(key ed25519.PrivateKey, payload []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
}
