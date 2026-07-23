package trust

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRecordAtUsesExactValidityBoundaries(t *testing.T) {
	issued := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	record := trustSignedRecord(t, issued)
	anchors := map[string]struct{}{record.PublicKeyText(): {}}

	require.Equal(t, "not_yet_valid", EvaluateRecordAt(anchors, record, issued.Add(-time.Nanosecond)).Outcome)
	require.Equal(t, "usable", EvaluateRecordAt(anchors, record, issued).Outcome)
	require.Equal(t, "usable", EvaluateRecordAt(anchors, record, record.ExpiresAt.Add(-time.Nanosecond)).Outcome)
	atExpiry := EvaluateRecordAt(anchors, record, record.ExpiresAt)
	require.Equal(t, "expired", atExpiry.Outcome)
	require.False(t, atExpiry.Usable)
}

func trustSignedRecord(t *testing.T, issued time.Time) discoveryrecord.Record {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	record := discoveryrecord.Record{Version: discoveryrecord.Version, Node: &discoveryrecord.NodeFacts{
		Principal: principal, PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)),
	}, IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)}
	payload, err := discoveryrecord.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
	return record
}
