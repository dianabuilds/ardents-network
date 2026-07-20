package record_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	discoveryfreshness "ardents/internal/discovery/freshness"
	discoveryrecord "ardents/internal/discovery/record"
	identitycontinuity "ardents/internal/identity/continuity"
	identitylifecycle "ardents/internal/identity/lifecycle"

	"github.com/stretchr/testify/require"
)

type testIdentityStore struct {
	principal string
	device    string
	publicKey string
}

func (s *testIdentityStore) LoadIdentity() (string, string, string) {
	return s.principal, s.device, s.publicKey
}

func (s *testIdentityStore) SaveIdentity(principal string, device string, publicKey string) error {
	s.principal = principal
	s.device = device
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

func TestFreshnessUsesIssuedAtBeforeExpiresAt(t *testing.T) {
	now := time.Now().UTC()
	record := discoveryrecord.Record{IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	require.Equal(t, now.UnixNano(), discoveryfreshness.Score(record))
}

func TestFreshnessFallsBackToExpiresAt(t *testing.T) {
	now := time.Now().UTC()
	record := discoveryrecord.Record{ExpiresAt: now}
	require.Equal(t, now.UnixNano(), discoveryfreshness.Score(record))
}

func signedNodeRecord(t *testing.T) (discoveryrecord.Record, ed25519.PrivateKey, error) {
	t.Helper()

	st := &testIdentityStore{}
	ids := identitylifecycle.New()
	summary, key, err := ids.EnsureNode(st, identitycontinuity.NewKeyStoreInDir(t.TempDir()))
	if err != nil {
		return discoveryrecord.Record{}, nil, err
	}
	record := discoveryrecord.Record{
		ID:        summary.Principal + ":node",
		Kind:      "node",
		Subject:   summary.Principal,
		Node:      summary.Principal,
		Device:    summary.Device,
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
