package testkit

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	discoveryapi "ardents/internal/discovery"
	discoveryrecord "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"
	identitytrust "ardents/internal/identity/trust"

	"github.com/stretchr/testify/require"
)

func TrustedNetworkPublishedService(t *testing.T, serviceID, serviceType, endpoint string) (discoveryapi.Record, *identitytrust.Registry) {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x42}, ed25519.SeedSize))
	principal, err := identityprincipal.FromEd25519PublicKey(key.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	now := time.Now().UTC()
	record := discoveryapi.Record{
		Version: discoveryrecord.Version,
		Service: &discoveryrecord.ServiceFacts{
			ID: discoveryrecord.ServiceID(serviceID), Type: serviceType, NodePrincipal: principal,
			Workload: discoveryrecord.WorkloadID("work." + serviceType), Mode: "NetworkPublished",
			PublicKey: base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)),
			Endpoints: []string{endpoint},
		},
		IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
	}
	payload, err := discoveryapi.Canonical(record)
	require.NoError(t, err)
	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
	return record, DiscoveryTrustRegistry(t, record.PublicKeyText())
}

func DiscoveryTrustRegistry(t *testing.T, encodedKeys ...string) *identitytrust.Registry {
	t.Helper()
	entries := make([]identitytrust.Entry, 0, len(encodedKeys))
	for _, encoded := range encodedKeys {
		public, err := base64.StdEncoding.DecodeString(encoded)
		require.NoError(t, err)
		principalID, err := identityprincipal.FromEd25519PublicKey(ed25519.PublicKey(public))
		require.NoError(t, err)
		entries = append(entries, identitytrust.Entry{
			Principal: principalID.String(), PublicKey: ed25519.PublicKey(public),
			Purposes: []identitytrust.Purpose{identitytrust.PurposeDiscoveryPublish},
		})
	}
	registry, err := identitytrust.NewRegistry(entries)
	require.NoError(t, err)
	return registry
}
