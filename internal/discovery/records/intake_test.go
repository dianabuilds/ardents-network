package records

import (
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

func TestImportDefaultsSourceToImported(t *testing.T) {
	record, _, err := signedNodeRecord(t)
	require.NoErrorf(t, err, "signed node record: %v", err)

	entries, result, err := Import(nil, record, "", time.Now().UTC())
	require.NoErrorf(t, err, "import record: %v", err)
	require.Falsef(t, !result.Applied ||
		result.
			Outcome != "imported", "result = %#v, want applied imported", result)
	require.Falsef(t, len(entries) !=
		1 || entries[0].Source != "imported", "entries = %#v, want imported source", entries)

}

func TestUpsertRejectsConflictingPublicKey(t *testing.T) {
	record, _, err := signedNodeRecord(t)
	require.NoErrorf(t, err, "signed node record: %v", err)

	other, _, err := signedNodeRecord(t)
	require.NoErrorf(t, err, "other signed node record: %v", err)

	other.ID = record.ID
	other.Subject = record.Subject
	entries := []Entry{{Record: record, Source: "bootstrap", SeenAt: time.Now().UTC()}}
	_, result, err := Upsert(entries, Entry{Record: other, Source: "bootstrap", SeenAt: time.Now().UTC()})
	require.NoErrorf(t, err, "upsert conflict: %v", err)
	require.Falsef(t, result.Applied ||
		result.
			Outcome != "rejected_conflict", "result = %#v, want rejected_conflict", result)

}

func TestUpsertRejectsStaleRecord(t *testing.T) {
	record, key, err := signedNodeRecord(t)
	require.NoErrorf(t, err, "signed node record: %v", err)

	entries := []Entry{{Record: record, Source: "bootstrap", SeenAt: time.Now().UTC()}}
	stale := record
	stale.IssuedAt = record.IssuedAt.Add(-time.Minute)
	payload, err := Canonical(stale)
	require.NoErrorf(t, err, "canonical stale record: %v", err)

	stale.Signature = signRecord(key, payload)
	_, result, err := Upsert(entries, Entry{Record: stale, Source: "bootstrap", SeenAt: time.Now().UTC()})
	require.NoErrorf(t, err, "upsert stale: %v", err)
	require.Falsef(t, result.Applied ||
		result.
			Outcome != "rejected_stale", "result = %#v, want rejected_stale", result)

}

func signedNodeRecord(t *testing.T) (Record, ed25519.PrivateKey, error) {
	t.Helper()

	st := &testIdentityStore{}
	ids := identityapi.NewService()
	summary, key, err := ids.EnsureNode(st, identitykeyring.NewKeyStoreInDir(t.TempDir()))
	if err != nil {
		return Record{}, nil, err
	}
	record := Record{
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
	payload, err := Canonical(record)
	if err != nil {
		return Record{}, nil, err
	}
	record.Signature = signRecord(key, payload)
	return record, key, nil
}

func signRecord(key ed25519.PrivateKey, payload []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
}
