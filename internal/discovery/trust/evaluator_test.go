package trust

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"sync"
	"testing"
	"time"

	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRecordAtUsesExactValidityBoundaries(t *testing.T) {
	issued := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	record := trustSignedRecord(t, issued)
	evaluator := NewEvaluator(trustRegistryForRecord(t, record, identitytrust.PurposeDiscoveryPublish))

	evaluator.now = func() time.Time { return issued.Add(-time.Nanosecond) }
	require.Equal(t, "not_yet_valid", evaluator.Evaluate(record).Outcome)
	evaluator.now = func() time.Time { return issued }
	require.Equal(t, "usable", evaluator.Evaluate(record).Outcome)
	evaluator.now = func() time.Time { return record.ExpiresAt.Add(-time.Nanosecond) }
	require.Equal(t, "usable", evaluator.Evaluate(record).Outcome)
	evaluator.now = func() time.Time { return record.ExpiresAt }
	atExpiry := evaluator.Evaluate(record)
	require.Equal(t, "expired", atExpiry.Outcome)
	require.False(t, atExpiry.Usable)
}

func TestEvaluatorCachesSignatureVerificationButAlwaysRechecksFreshness(t *testing.T) {
	issued := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	record := trustSignedRecord(t, issued)
	registry := trustRegistryForRecord(t, record, identitytrust.PurposeDiscoveryPublish)
	evaluator := NewEvaluator(registry)
	evaluator.now = func() time.Time { return issued }

	require.Equal(t, "usable", evaluator.Evaluate(record).Outcome)
	require.Equal(t, "usable", evaluator.Evaluate(record).Outcome)
	require.Equal(t, uint64(1), evaluator.verificationCount)

	evaluator.now = func() time.Time { return record.ExpiresAt }
	expired := evaluator.Evaluate(record)
	require.Equal(t, "expired", expired.Outcome)
	require.False(t, expired.Usable)
	require.Equal(t, uint64(1), evaluator.verificationCount)
}

func TestEvaluatorTrustRotationAndPurposeIsolationInvalidateTrustOnly(t *testing.T) {
	issued := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	record := trustSignedRecord(t, issued)
	evaluator := NewEvaluator(trustRegistryForRecord(t, record, identitytrust.PurposeDiscoveryPublish))
	evaluator.now = func() time.Time { return issued }
	require.True(t, evaluator.Evaluate(record).Usable)
	require.Equal(t, uint64(1), evaluator.verificationCount)

	evaluator.ReplaceRegistry(trustRegistryForRecord(t, record, identitytrust.PurposeChannelIssue))
	rotated := evaluator.Evaluate(record)
	require.True(t, rotated.Valid)
	require.False(t, rotated.Trusted)
	require.False(t, rotated.Usable)
	require.Equal(t, uint64(1), evaluator.verificationCount)
}

func TestEvaluatorConcurrentProjectionVerifiesOnceAndRotationFinishesFailClosed(t *testing.T) {
	issued := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	record := trustSignedRecord(t, issued)
	discoveryRegistry := trustRegistryForRecord(t, record, identitytrust.PurposeDiscoveryPublish)
	channelRegistry := trustRegistryForRecord(t, record, identitytrust.PurposeChannelIssue)
	evaluator := NewEvaluator(discoveryRegistry)
	evaluator.now = func() time.Time { return issued }

	var group sync.WaitGroup
	for index := 0; index < 32; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			result := evaluator.Evaluate(record)
			evaluator.Remember(result)
		}()
	}
	for index := 0; index < 16; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			if index%2 == 0 {
				evaluator.ReplaceRegistry(channelRegistry)
				return
			}
			evaluator.ReplaceRegistry(discoveryRegistry)
		}(index)
	}
	group.Wait()
	require.Equal(t, uint64(1), evaluator.verificationCount)

	evaluator.ReplaceRegistry(channelRegistry)
	require.False(t, evaluator.Evaluate(record).Usable)
}

func TestEvaluatorBoundsSignatureVerificationCache(t *testing.T) {
	issued := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	evaluator := NewEvaluator(nil)
	evaluator.now = func() time.Time { return issued }
	var firstKey, lastKey string

	for index := 0; index <= maxVerificationCacheEntries; index++ {
		record := trustSignedRecord(t, issued)
		record.Node.Endpoints = []string{fmt.Sprintf("https://node-%d.invalid", index)}
		payload, err := discoveryrecord.Canonical(record)
		require.NoError(t, err)
		record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
		canonical, signature, _, err := discoveryrecord.Fingerprint(record)
		require.NoError(t, err)
		key := canonical + "." + signature
		if index == 0 {
			firstKey = key
		}
		lastKey = key
		require.True(t, evaluator.Evaluate(record).Valid)
	}

	require.Len(t, evaluator.verified, maxVerificationCacheEntries)
	require.NotContains(t, evaluator.verified, firstKey)
	require.Contains(t, evaluator.verified, lastKey)
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

func trustRegistryForRecord(t *testing.T, record discoveryrecord.Record, purpose identitytrust.Purpose) *identitytrust.Registry {
	t.Helper()
	public, err := base64.StdEncoding.DecodeString(record.PublicKeyText())
	require.NoError(t, err)
	registry, err := identitytrust.NewRegistry([]identitytrust.Entry{{
		Principal: record.NodeID(), PublicKey: ed25519.PublicKey(public), Purposes: []identitytrust.Purpose{purpose},
	}})
	require.NoError(t, err)
	return registry
}
