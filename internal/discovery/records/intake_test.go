package records

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	identityprincipal "ardents/internal/identity/principal"
	"github.com/stretchr/testify/require"
)

func TestImportDefaultsSourceToImported(t *testing.T) {
	record, _ := intakeNodeRecord(t, 1, time.Now().UTC())
	evidence, err := VerifyRetained(record, recordTrustGenerationForIntake, false)
	require.NoError(t, err)
	entries, result, err := ImportVerified(nil, record, "", time.Now().UTC(), evidence)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, Imported, entries[0].Source)
}

func TestImportRejectsUnknownSourceWithoutMutation(t *testing.T) {
	now := time.Now().UTC()
	record, _ := intakeNodeRecord(t, 1, now)
	evidence, err := VerifyRetained(record, recordTrustGenerationForIntake, false)
	require.NoError(t, err)
	before := []Entry{{Record: record.Clone(), Source: Local, SeenAt: now}}
	got, _, err := ImportVerified(before, record, "operator-supplied", now, evidence)
	require.Error(t, err)
	require.Equal(t, before, got)
}

const recordTrustGenerationForIntake = "0202020202020202020202020202020202020202020202020202020202020202"

func TestUpsertRejectsConflictingPublicKeyAndStaleRecord(t *testing.T) {
	now := time.Now().UTC()
	record, key := intakeNodeRecord(t, 1, now)
	conflict := record.Clone()
	conflict.Node.PublicKey = base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(bytesOfValue(2)).Public().(ed25519.PublicKey))
	_, result, err := Upsert([]Entry{{Record: record}}, Entry{Record: conflict})
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, "rejected_conflict", result.Outcome)

	stale := record.Clone()
	stale.IssuedAt = now.Add(-time.Minute)
	stale.ExpiresAt = stale.IssuedAt.Add(time.Hour)
	intakeSign(t, &stale, key)
	_, result, err = Upsert([]Entry{{Record: record}}, Entry{Record: stale})
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, "rejected_stale", result.Outcome)
}

func TestUpsertDoesNotMutateInputSlice(t *testing.T) {
	now := time.Now().UTC()
	old, _ := intakeNodeRecord(t, 1, now)
	newer, key := intakeNodeRecord(t, 1, now.Add(time.Minute))
	intakeSign(t, &newer, key)
	entries := []Entry{{Record: old}}
	updated, result, err := Upsert(entries, Entry{Record: newer})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, old.IssuedAt, entries[0].Record.IssuedAt)
	require.Equal(t, newer.IssuedAt, updated[0].Record.IssuedAt)
}

func intakeNodeRecord(t *testing.T, seed byte, issued time.Time) (Record, ed25519.PrivateKey) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytesOfValue(seed))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	record := Record{Version: Version, Node: &NodeFacts{Principal: principal, PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), Endpoints: []string{"tcp://node:9000"}}, IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)}
	intakeSign(t, &record, key)
	return record, key
}

func intakeSign(t *testing.T, record *Record, key ed25519.PrivateKey) {
	t.Helper()
	payload, err := Canonical(*record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
}

func bytesOfValue(value byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = value
	}
	return seed
}
